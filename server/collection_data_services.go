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
// Package models contains struct representing the JSON requests/responses.
package models


// CreateCollectionRequest is a request to create or upload a collection
type CreateCollectionRequest struct {
	// The role tag associated with this collection's creation.
	Creator string `json:"creator"`
	// The role tags to own this collection.  The creator is always an owner
	// and doesn't need to be included in this list.
	Owners []string `json:"owners"`
	// The tags to initially set in this collection.  Tags are string values
	// displayed in, and modifiable from, SchedViz along with collections.
	Tags []string `json:"tags"`
	// A collection description, displayed in and modifiable from SchedViz.
	Description string `json:"description"`
	// The time of this collection's creation.  If left empty, it will be
	// autopopulated at the time of collection creation.
	CreationTime int64 `json:"creationTime"`
}

// CollectionParametersResponse is a response for a collection parameters request.
type CollectionParametersResponse struct {
	CollectionName   string   `json:"collectionName"`
	CPUs             []int64  `json:"cpus"`
	StartTimestampNs int64    `json:"startTimestampNs"`
	EndTimestampNs   int64    `json:"endTimestampNs"`
	FtraceEvents     []string `json:"ftraceEvents"`
}

// EditCollectionRequest is a request to edit a given collection.
type EditCollectionRequest struct {
	CollectionName string `json:"collectionName"`
	// Any tags requested for removal are removed, then any tags requested for
	// addition are added.
	RemoveTags  []string `json:"removeTags"`
	AddTags     []string `json:"addTags"`
	Description string   `json:"description"`
	AddOwners   []string `json:"addOwners"`
}


// UnknownLogicalID is a value used to represent a core, NUMA node, die, thread or socket ID that
// has not been set
const UnknownLogicalID = -1

// LogicalCore contains metadata describing a logical core
type LogicalCore struct {
	// This logical core's index in the topology.  Used as a scalar identifier
	// of this CPU in profiling tools.
	CPUID uint64 `json:"cpuId"`
	// The 0-indexed identifier of the socket of this logical core.  'Socket'
	// represents a distinct IC package.
	SocketID int32 `json:"socketId"`
	// The 0-indexed NUMA node of this logical core.  NUMA nodes are groupings
	// of cores and cache hierarchy that are 'local' to their own memory;
	// accessing non-local memory is costlier than accessing local memory.
	NumaNodeID int32 `json:"numaNodeId"`
	// The 0-indexed die identifier.  Some IC packages may include more than one
	// distinct dies.
	DieID int32 `json:"dieId"`
	// The 0-indexed core identifier within its die.  A core is a single
	// processing unit with its own register storage and L1 caches.
	CoreID int32 `json:"coreId"`
	// The 0-indexed hyperthread, or hardware thread, identifier within its
	// core.  A hardware thread is a partitioning of a core that can execute a
	// single instruction stream.  Hyperthreads on a core share the core's
	// resources, such as its functional units and cache hierarchy, but maintain
	// independent registers, and help ensure that the CPU remains fully
	// utilized.
	ThreadID int32 `json:"threadId"`
}

// SystemTopology information.
type SystemTopology struct {
	// The index of this platform in
	// platforminfo::PLATFORMINFO_CPU_IDENTIFIER_VALUES.
	CPUIdentifier int32 `json:"cpuIdentifier"`
	// CPU vendor, from platforminfo::CpuVendor.
	CPUVendor int32 `json:"cpuVendor"`
	// CPUID fields.
	CPUFamily   int32 `json:"cpuFamily"`
	CPUModel    int32 `json:"cpuModel"`
	CPUStepping int32 `json:"cpuStepping"`
	// TODO(ilhamster) Look into providing a string describing the system.
	// The set of logical cores.
	LogicalCores []*LogicalCore `json:"logicalCores"`
}

// SystemTopologyResponse is a response to a SystemTopologyRequest
type SystemTopologyResponse struct {
	CollectionName string          `json:"collectionName"`
	SystemTopology *SystemTopology `json:"systemTopology"`
}
