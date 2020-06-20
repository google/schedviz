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
	"strings"

	log "github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"

	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"
	"github.com/google/schedviz/tracedata/trace"
)

// FsStorage is a storage service that saves collections as protos on local disk
// Implements StorageService
type FsStorage struct {
	*storageBase
	StoragePath string
}

// CreateFSStorage creates a new file system storage service that stores its files at storagePath
// and has an LRU cache of size cacheSize.
func CreateFSStorage(storagePath string, cacheSize int) (*FsStorage, error) {
	sb, err := newStorageBase(cacheSize)
	if err != nil {
		return nil, err
	}
	return &FsStorage{
		storageBase: sb,
		StoragePath: storagePath,
	}, nil
}

// DeleteCollection deletes the collection with the given name.
func (fs *FsStorage) DeleteCollection(_ context.Context, _ string, collectionUniqueName string) error {
	filePath := path.Join(fs.StoragePath, collectionUniqueName+".binproto")
	fs.mu.Lock()
	fs.lruCache.Remove(filePath)
	defer fs.mu.Unlock()
	if err := os.Remove(filePath); err != nil {
		return err
	}
	return nil
}

func (fs *FsStorage) getCollectionPath(collectionName string) string {
	return path.Join(fs.StoragePath, collectionName+".binproto")
}

func (fs *FsStorage) getCollectionNameFromFileName(fileName string) string {
	return strings.TrimSuffix(fileName, ".binproto")
}

func (fs *FsStorage) getCollectionFromDisk(collectionName string) (*eventpb.Collection, error) {
	filePath := fs.getCollectionPath(collectionName)
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	collectionProto := &eventpb.Collection{}
	if err := proto.Unmarshal(bytes, collectionProto); err != nil {
		return nil, err
	}
	return collectionProto, nil
}

// GetCollection returns an already-saved collection with the given name.
// shouldCache controls whether or not the fetched collection will be saved in the cache to speed up
// future requests for the same collection.
func (fs *FsStorage) GetCollection(ctx context.Context, collectionName string) (*CachedCollection, error) {
	cachedCollection, ok, _, err := fs.getCollectionFromCache(collectionName, true /*= addCollection*/)
	if err != nil {
		return nil, err
	}
	if ok {
		if err := cachedCollection.wait(ctx); err != nil {
			return nil, err
		}
		return cachedCollection, cachedCollection.err
	}
	defer func() {
		cachedCollection.err = err
		cachedCollection.release()
	}()
	collectionProto, err := fs.getCollectionFromDisk(collectionName)
	if err != nil {
		return nil, err
	}
	collection, err := createCollection(collectionProto.EventSet)
	if err != nil {
		return nil, err
	}
	cachedCollection.Collection = collection
	cachedCollection.SystemTopology = convertTopologyProtoToStruct(collectionProto.Topology)
	cachedCollection.Payload = map[string]interface{}{}
	return cachedCollection, nil
}

// GetCollectionMetadata gets the metadata for the collection with the given name.
func (fs *FsStorage) GetCollectionMetadata(ctx context.Context, collectionUniqueName string) (models.Metadata, error) {
	collectionProto, err := fs.getCollectionFromDisk(collectionUniqueName)
	if err != nil {
		return models.Metadata{}, err
	}
	return convertMetadataProtoToStruct(collectionProto.Metadata), nil
}

// EditCollection edits the metadata for the collection with the given name.
func (fs *FsStorage) EditCollection(ctx context.Context, _ string, req *models.EditCollectionRequest) error {
	if len(req.CollectionName) == 0 {
		return missingFieldError("collection_name")
	}
	metadata, err := fs.GetCollectionMetadata(ctx, req.CollectionName)
	if err != nil {
		return err
	}

	oldTags := metadata.Tags
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
	metadata.Tags = newTags

	ownerSet := make(map[string]struct{})
	for _, owner := range metadata.Owners {
		ownerSet[owner] = struct{}{}
	}
	for _, owner := range req.AddOwners {
		ownerSet[owner] = struct{}{}
	}
	var newOwners = []string{}
	for owner := range ownerSet {
		newOwners = append(newOwners, owner)
	}
	metadata.Owners = newOwners
	metadata.Description = req.Description

	// Write updated metadata to file
	collectionProto, err := fs.getCollectionFromDisk(req.CollectionName)
	if err != nil {
		return err
	}
	metadataProto, err := convertMetadataStructToProto(&metadata)
	if err != nil {
		return err
	}
	collectionProto.Metadata = metadataProto

	outBytes, err := proto.Marshal(collectionProto)
	if err != nil {
		return err
	}
	filePath := fs.getCollectionPath(req.CollectionName)
	if err := ioutil.WriteFile(filePath, outBytes, 0644); err != nil {
		return err
	}
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
		collectionName := fs.getCollectionNameFromFileName(file.Name())
		collectionProto, err := fs.getCollectionFromDisk(collectionName)
		if err != nil {
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

// SetFailOnUnknownEventFormat configures behavior when encountering an unknown
// event format.  If the provided bool is true, parsing fails on unknown events;
// otherwise unknown events are logged and ignored.
func (fs *FsStorage) SetFailOnUnknownEventFormat(option bool) {
	fs.failOnUnknownEventFormat = option
}

// createCollection creates a collection with the default event loader, and
// will attempt to create a collection with the fault tolerant loader if the
// default loader failed.
var createCollection = func(es *eventpb.EventSet) (*sched.Collection, error) {
	coll, err := sched.NewCollection(es, sched.NormalizeTimestamps(true))
	if err == nil {
		return coll, nil
	}
	log.Warning("Failed to load collection with default loader. " +
		"Retrying with fault tolerant loader.")
	coll, err = sched.NewCollection(es,
		sched.NormalizeTimestamps(true),
		sched.UsingEventLoaders(sched.FaultTolerantEventLoaders()))
	if err != nil {
		return nil, err
	}
	return coll, nil
}

func missingFieldError(fieldName string) error {
	return fmt.Errorf("missing required field %q`", fieldName)
}
