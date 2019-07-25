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
 * Metadata contains metadata about a collection
 */
export declare interface Metadata {
  // The unique name of the collection in PCC2.
  collectionUniqueName: string;
  // The creator tag provided at this collection's creation.
  creator: string;
  // This collection's owners.
  owners: string[];
  // The collection's tags.
  tags: string[];
  // The collection's description.
  description: string;
  // The time of this collection's creation.
  creationTime: number;
  // The events collected during the collection.
  ftraceEvents: string[];
  // The target machine on which the collection was performed.
  targetMachine: string;
}
