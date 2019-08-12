//
// Copyright 2019 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS-IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
package storageservice

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/golang/groupcache/lru"

	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"
	"github.com/google/schedviz/tracedata/trace"
)

// FsStorage is a storage service that saves collections as protos on local disk
// Implements StorageService
type FsStorage struct {
	StoragePath string
	lruCache    cache
}

// CreateFSStorage creates a new file system storage service that stores its files at storagePath
// and has an LRU cache of size cacheSize.
func CreateFSStorage(storagePath string, cacheSize int) StorageService {
	return &FsStorage{
		StoragePath: storagePath,
		lruCache:    lru.New(cacheSize),
	}
}


// DeleteCollection deletes the collection with the given name.
func (fs *FsStorage) DeleteCollection(_ context.Context, _ string, collectionUniqueName string) error {
	filePath := path.Join(fs.StoragePath, collectionUniqueName+".binproto")
	fs.lruCache.Remove(filePath)
	if err := os.Remove(filePath); err != nil {
		return err
	}
	return nil
}

func (fs *FsStorage) getCollectionFromCache(filePath string) (*CachedCollection, bool, error) {
	cached, ok := fs.lruCache.Get(filePath)
	if ok {
		cachedCollection, ok := cached.(*CachedCollection)
		if ok {
			return cachedCollection, true, nil
		}
		return nil, false, status.Error(codes.Internal, "unknown type stored in collection cache")
	}
	return nil, false, nil
}

// GetCollection returns the collection with the given name.
// shouldCache controls whether or not the fetched collection will be saved in the cache to speed up
// future requests for the same collection.
func (fs *FsStorage) GetCollection(_ context.Context, collectionName string) (*CachedCollection, error) {
	filePath := path.Join(fs.StoragePath, collectionName+".binproto")
	cachedCollection, ok, err := fs.getCollectionFromCache(filePath)
	if err != nil {
		return nil, err
	}
	if ok {
		return cachedCollection, nil
	}

	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	collectionProto := &eventpb.Collection{}
	if err := proto.Unmarshal(bytes, collectionProto); err != nil {
		return nil, err
	}
	collection, err := sched.NewCollection(collectionProto.EventSet,
		sched.DefaultEventLoaders(),
		sched.NormalizeTimestamps(true))
	if err != nil {
		return nil, err
	}
	cacheValue := &CachedCollection{
		Collection:     collection,
		Metadata:       convertMetadataProtoToStruct(collectionProto.Metadata),
		SystemTopology: convertTopologyProtoToStruct(collectionProto.Topology),
		Payload:        map[string]interface{}{},
	}
	fs.lruCache.Add(filePath, cacheValue)
	return cacheValue, nil
}

// GetCollectionMetadata gets the metadata for the collection with the given name.
func (fs *FsStorage) GetCollectionMetadata(ctx context.Context, collectionUniqueName string) (models.Metadata, error) {
	collection, err := fs.GetCollection(ctx, collectionUniqueName)
	if err != nil {
		return models.Metadata{}, err
	}
	return collection.Metadata, nil
}

// EditCollection edits the metadata for the collection with the given name.
func (fs *FsStorage) EditCollection(ctx context.Context, _ string, req *models.EditCollectionRequest) error {
	if len(req.CollectionName) == 0 {
		return missingFieldError("collection_name")
	}
	collection, err := fs.GetCollection(ctx, req.CollectionName)
	if err != nil {
		return err
	}

	oldTags := collection.Metadata.Tags
	tagSet := make(map[string]struct{})
	for _, tag := range oldTags {
		tagSet[tag] = struct{}{}
	}
	for _, tag := range req.RemoveTags {
		delete(tagSet, tag)
	}
	for _, tag := range req.AddTags {
		tagSet[tag] = struct{}{}
	}
	var newTags = []string{}
	for tag := range tagSet {
		newTags = append(newTags, tag)
	}
	collection.Metadata.Tags = newTags

	ownerSet := make(map[string]struct{})
	for _, owner := range collection.Metadata.Owners {
		ownerSet[owner] = struct{}{}
	}
	for _, owner := range req.AddOwners {
		ownerSet[owner] = struct{}{}
	}
	var newOwners = []string{}
	for owner := range ownerSet {
		newOwners = append(newOwners, owner)
	}
	collection.Metadata.Owners = newOwners
	collection.Metadata.Description = req.Description

	// Write updated metadata to file
	filePath := path.Join(fs.StoragePath, req.CollectionName+".binproto")
	inBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	collectionProto := &eventpb.Collection{}
	if err := proto.Unmarshal(inBytes, collectionProto); err != nil {
		return err
	}
	metadataProto, err := convertMetadataStructToProto(&collection.Metadata)
	if err != nil {
		return err
	}
	collectionProto.Metadata = metadataProto

	outBytes, err := proto.Marshal(collectionProto)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filePath, outBytes, 0644); err != nil {
		return err
	}

	// Update cache
	fs.lruCache.Add(filePath, collection)
	return nil
}

// ListCollectionMetadata gets the metadata for all collections.
func (fs *FsStorage) ListCollectionMetadata(ctx context.Context, _ string, _ string) ([]models.Metadata, error) {
	files, err := ioutil.ReadDir(fs.StoragePath)
	if err != nil {
		return nil, err
	}
	// Force initialize as an empty, not nil, slice.
	// Without the curly braces, this will appear as null when empty and serialized to JSON, instead
	// of an empty array.
	var ret = []models.Metadata{}
	for _, file := range files {
		// Check if cache contains metadata already
		filePath := path.Join(fs.StoragePath, file.Name())
		cachedCollection, ok, err := fs.getCollectionFromCache(filePath)
		if err == nil && ok {
			ret = append(ret, cachedCollection.Metadata)
			continue
		}
		// If not, read just the metadata (i.e. don't create a new collection)
		// This is faster than performing the full analysis on every collection.
		bytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		collectionProto := &eventpb.Collection{}
		if err := proto.Unmarshal(bytes, collectionProto); err != nil {
			return nil, err
		}
		metadata := convertMetadataProtoToStruct(collectionProto.Metadata)
		ret = append(ret, metadata)
	}
	return ret, nil
}

// GetCollectionParameters gets the parameters for the collection with the given name.
func (fs *FsStorage) GetCollectionParameters(ctx context.Context, collectionName string) (models.CollectionParametersResponse, error) {
	if len(collectionName) == 0 {
		return models.CollectionParametersResponse{}, missingFieldError("collection_name")
	}
	collection, err := fs.GetCollection(ctx, collectionName)
	if err != nil {
		return models.CollectionParametersResponse{}, err
	}

	startTimestamp, endTimestamp := collection.Collection.Interval()

	ftraceEvents := collection.Collection.TraceCollection.EventNames()

	ret := models.CollectionParametersResponse{
		CollectionName:   collectionName,
		CPUs:             collection.Collection.ExpandCPUs(nil),
		StartTimestampNs: int64(startTimestamp),
		EndTimestampNs:   int64(endTimestamp),
		FtraceEvents:     ftraceEvents,
	}

	return ret, nil
}

// GetFtraceEvents returns events for the specified collection
func (fs *FsStorage) GetFtraceEvents(ctx context.Context, req *models.FtraceEventsRequest) (models.FtraceEventsResponse, error) {
	if len(req.CollectionName) == 0 {
		return models.FtraceEventsResponse{}, missingFieldError("collection_name")
	}
	collection, err := fs.GetCollection(ctx, req.CollectionName)
	if err != nil {
		return models.FtraceEventsResponse{}, err
	}

	var cpus = []sched.CPUID{}
	for _, cpu := range req.Cpus {
		cpus = append(cpus, sched.CPUID(cpu))
	}

	events, err := collection.Collection.GetRawEvents(
		sched.CPUs(cpus...),
		sched.TimeRange(trace.Timestamp(req.StartTimestamp), trace.Timestamp(req.EndTimestamp)),
		sched.EventTypes(req.EventTypes...))
	if err != nil {
		return models.FtraceEventsResponse{}, err
	}

	var fTraceEventsByCPU = make(map[sched.CPUID][]*trace.Event)
	for _, ev := range events {
		cpuID := sched.CPUID(ev.CPU)
		fTraceEventsByCPU[cpuID] = append(fTraceEventsByCPU[cpuID], ev)
	}

	return models.FtraceEventsResponse{
		CollectionName: req.CollectionName,
		EventsByCPU:    fTraceEventsByCPU,
	}, nil
}

func missingFieldError(fieldName string) error {
	return fmt.Errorf("missing required field %q`", fieldName)
}
