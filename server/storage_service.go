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
// Package storageservice contains services for storing collection info
package storageservice

import (
	"context"
	"io"
	"sort"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/hashicorp/golang-lru/simplelru"
	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"

	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

// CachedCollection is a collection and its metadata that is stored in the LRU cache.
type CachedCollection struct {
	Collection     *sched.Collection
	SystemTopology models.SystemTopology
	// Payload stores arbitrary data by a string key.
	Payload map[string]interface{}
	// ready blocks until the collection is ready to be read.
	ready chan struct{}
	// Any error encountered while creating the collection.
	err error
}

func newCachedCollection() *CachedCollection {
	return &CachedCollection{
		ready: make(chan struct{}),
	}
}

// wait blocks until release() has been called on the receiver.  At that point,
// the receiver should no longer be modified.  Returns the CachedCollection's
// error, if returning because release was called, or the context's error, if
// the context was cancelled.
func (cc *CachedCollection) wait(ctx context.Context) error {
	// Block on the cached collection's Ready channel, or the context ending,
	// whichever comes first.
	select {
	case <-cc.ready:
		return cc.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// release unblocks any outstanding or future wait calls on the receiver.  It
// should only be called when the receiver is fully populated and will no
// longer be modified.
func (cc *CachedCollection) release() {
	close(cc.ready)
}

type storageBase struct {
	lruCache                 *simplelru.LRU
	mu                       sync.Mutex
	failOnUnknownEventFormat bool
}

func newStorageBase(cacheSize int) (*storageBase, error) {
	lru, err := simplelru.NewLRU(cacheSize, nil)
	if err != nil {
		return nil, err
	}
	return &storageBase{
		lruCache: lru,
	}, nil
}

// addToCache must only be called when sb.mu is held.
var addToCache = func(sb *storageBase, collectionName string, collection *CachedCollection) {
	sb.lruCache.Add(collectionName, collection)
}

func (sb *storageBase) dropCollectionFromCache(collectionName string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.lruCache.Remove(collectionName)
}

// extractEventNames gets a list of all event descriptor names in the event set
func (sb *storageBase) extractEventNames(es *eventpb.EventSet) ([]string, error) {
	var events []string
	for idx, ed := range es.EventDescriptor {
		if ed.Name < 0 || int(ed.Name) >= len(es.StringTable) {
			return nil, status.Errorf(codes.Internal, "invalid event name %d in event descriptor %d", ed.Name, idx)
		}
		events = append(events, es.StringTable[ed.Name])
	}
	sort.Strings(events)
	return events, nil
}

// getCollectionFromCache returns the named collection, if it is stored in the
// cache.  It also returns a bool signifying whether the collection was in the
// cache at the start of the call.
// If addCollection is true, a new, empty, CachedCollection will be placed in
// the cache under the provided name.  Note that if this occurs, the returned
// bool will still be false, though the returned CachedCollection will be in
// the cache.  release() should be called on the returned CachedCollection when
// it will no longer be modified.
func (sb *storageBase) getCollectionFromCache(collectionName string, addCollection bool) (*CachedCollection, bool, error) {
	sb.mu.Lock()
	cachedValue, ok := sb.lruCache.Get(collectionName)
	if !ok && addCollection {
		defer sb.mu.Unlock()
		cachedCollection := newCachedCollection()
		addToCache(sb, collectionName, cachedCollection)
		return cachedCollection, false, nil
	}
	sb.mu.Unlock()
	var cachedCollection *CachedCollection
	if ok {
		cachedCollection, ok = cachedValue.(*CachedCollection)
		if !ok {
			return nil, false, status.Error(codes.Internal, "unknown type stored in collection cache")
		}
	}
	return cachedCollection, ok, nil
}

// StorageService is an interface containing the APIs that storage services expose
type StorageService interface {
	UploadFile(ctx context.Context, req *models.CreateCollectionRequest, fileHeader io.Reader) (string, error)
	DeleteCollection(ctx context.Context, editor string, collectionUniqueName string) error
	// GetCollection returns the specified collection, or any error encountered
	// procuring it.  If the collection exists in the cache, the cached version
	// will be returned, otherwise, it will be created and added to the cache
	// before being returned.
	// Implementations should use CachedCollection's synchronization properties:
	//  * When adding a new collection to the cache, implementors should call
	//    release() on the CachedCollection after populating it and before
	//    returning.  If any error is encountered while populating the
	//    CachedCollection, its err field should be set accordingly.
	//  * When finding a CachedCollection already in the cache, implementors
	//    should call wait() on that CachedCollection before returning it.
	//    After wait() returns, the CachedCollection's err field should be
	//    checked, and GetCollection should return any non-nil error there.
	GetCollection(ctx context.Context, collectionName string) (*CachedCollection, error)
	GetCollectionMetadata(ctx context.Context, collectionUniqueName string) (models.Metadata, error)
	EditCollection(ctx context.Context, editor string, req *models.EditCollectionRequest) error
	ListCollectionMetadata(ctx context.Context, user string, collectionName string) ([]models.Metadata, error)
	GetCollectionParameters(ctx context.Context, collectionName string) (models.CollectionParametersResponse, error)
	GetFtraceEvents(ctx context.Context, req *models.FtraceEventsRequest) (models.FtraceEventsResponse, error)
	// Helper
	SetFailOnUnknownEventFormat(option bool)
}
