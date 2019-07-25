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

/**
 * CreateCollectionRequest is a request to create or upload a collection
 */
export declare interface CreateCollectionRequest {
  // The role tag associated with this collection's creation.
  creator?: string;
  // The role tags to own this collection.  The creator is always an owner
  // and doesn't need to be included in this list.
  owners?: string[];
  // The tags to initially set in this collection.  Tags are string values
  // displayed in, and modifiable from, SchedViz along with collections.
  tags?: string[];
  // A collection description, displayed in and modifiable from SchedViz.
  description?: string;
  // The time of this collection's creation.  If left empty, it will be
  // autopopulated at the time of collection creation.
  creationTime?: number;
}

/**
 * A response for a collection parameters request.
 */
export declare interface CollectionParametersResponse {
  collectionName: string;
  cpus: number[];
  startTimestampNs: number;
  endTimestampNs: number;
  ftraceEvents: string[];
}

/**
 * A request to edit a given collection.
 */
export declare interface EditCollectionRequest {
  collectionName: string;
  // Any tags requested for removal are removed, then any tags requested for
  // addition are added.
  removeTags: string[];
  addTags: string[];
  description: string;
  addOwners: string[];
}


/**
 * LogicalCore contains metadata describing a logical core
 */
export declare interface LogicalCore {
  // This logical core's index in the topology.  Used as a scalar identifier
  // of this CPU in profiling tools.
  cpuId: number;
  // The 0-indexed identifier of the socket of this logical core.  'Socket'
  // represents a distinct IC package.
  socketId: number;
  // The 0-indexed NUMA node of this logical core.  NUMA nodes are groupings
  // of cores and cache hierarchy that are 'local' to their own memory;
  // accessing non-local memory is costlier than accessing local memory.
  numaNodeId: number;
  // The 0-indexed die identifier.  Some IC packages may include more than one
  // distinct dies.
  dieId: number;
  // The 0-indexed core identifier within its die.  A core is a single
  // processing unit with its own register storage and L1 caches.
  coreId: number;
  // The 0-indexed hyperthread, or hardware thread, identifier within its
  // core.  A hardware thread is a partitioning of a core that can execute a
  // single instruction stream.  Hyperthreads on a core share the core's
  // resources, such as its functional units and cache hierarchy, but maintain
  // independent registers, and help ensure that the CPU remains fully
  // utilized.
  threadId: number;
}

/**
 * System topology information.
 */
export declare interface SystemTopology {
  // The index of this platform in
  // platforminfo::PLATFORMINFO_CPU_IDENTIFIER_VALUES.
  cpuIdentifier: number;
  // CPU vendor, from platforminfo::CpuVendor.
  cpuVendor: number;
  // CPUID fields.
  cpuFamily: number;
  cpuModel: number;
  cpuStepping: number;
  // TODO(ilhamster) Look into providing a string describing the system.
  // The set of logical cores.
  logicalCores: LogicalCore[];
}

/**
 * SystemTopologyResponse is a response to a SystemTopologyRequest
 */
export declare interface SystemTopologyResponse {
  collectionName: string;
  systemTopology: SystemTopology;
}
