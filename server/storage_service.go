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

	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"
	"github.com/golang/groupcache/lru"
)

// CachedCollection is a collection and its metadata that is stored in the LRU cache.
type CachedCollection struct {
	Collection     *sched.Collection
	Metadata       models.Metadata
	SystemTopology models.SystemTopology
	// Payload stores arbitrary data by a string key.
	Payload map[string]interface{}
}

// cache is an interface wrapping the lru cache
type cache interface {
	Add(key lru.Key, value interface{})
	Get(key lru.Key) (value interface{}, ok bool)
	Remove(key lru.Key)
	RemoveOldest()
	Len() int
}

// StorageService is an interface containing the APIs that storage services expose
type StorageService interface {
	UploadFile(ctx context.Context, creator string, req *models.CreateCollectionRequest, fileHeader io.Reader) (string, error)
	DeleteCollection(ctx context.Context, editor string, collectionUniqueName string) error
	GetCollection(ctx context.Context, collectionName string) (*CachedCollection, error)
	GetCollectionMetadata(ctx context.Context, collectionUniqueName string) (models.Metadata, error)
	EditCollection(ctx context.Context, editor string, req *models.EditCollectionRequest) error
	ListCollectionMetadata(ctx context.Context, user string, collectionName string) ([]models.Metadata, error)
	GetCollectionParameters(ctx context.Context, collectionName string) (models.CollectionParametersResponse, error)
	GetFtraceEvents(ctx context.Context, req *models.FtraceEventsRequest) (models.FtraceEventsResponse, error)
}
