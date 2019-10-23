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
package models

import elpb "github.com/google/schedviz/analysis/event_loaders_go_proto"

// Metadata contains metadata about a collection
type Metadata struct {
	// The unique name of the collection.
	CollectionUniqueName string `json:"collectionUniqueName"`
	// The creator tag provided at this collection's creation.
	Creator string `json:"creator"`
	// This collection's owners.
	Owners []string `json:"owners"`
	// The collection's tags.
	Tags []string `json:"tags"`
	// The collection's description.
	Description string `json:"description"`
	// The time of this collection's creation.
	CreationTime int64 `json:"creationTime"`
	// The events collected during the collection.
	FtraceEvents []string `json:"ftraceEvents"`
	// The target machine on which the collection was performed.
	TargetMachine string `json:"targetMachine"`
	// The elpb.LoadersType used by default for this collection.
	DefaultEventLoader elpb.LoadersType `json:"defaultEventLoader"`
}
