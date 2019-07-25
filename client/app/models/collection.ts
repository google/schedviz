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
import {CollectionParametersResponse} from './collection_data_services';
import {Metadata} from './events';

const NANOS_TO_MILLIS = 1e6;

/**
 * Describes the high-level parameters of a collection, for view sizing, etc.
 */
export class CollectionParameters {
  constructor(
      public name: string,
      public cpus: number[],
      public startTimeNs: number,
      public endTimeNs: number,
      public ftraceEventTypes: string[] = [],
  ) {}

  get size() {
    return this.cpus.length;
  }

  static fromJSON(json: CollectionParametersResponse): CollectionParameters {
    return new CollectionParameters(
        json.collectionName,
        json.cpus,
        json.startTimestampNs,
        json.endTimestampNs,
        json.ftraceEvents,
    );
  }
}

/**
 * Describes high-level metadata of a collection.
 */
export class CollectionMetadata {
  constructor(
      public name: string,
      public creator: string,
      public owners: string[],
      public tags: string[],
      public description: string,
      public creationTime: Date|undefined,
      public eventNames: string[],
      public targetMachine: string,
  ) {}

  static fromJSON(json: Metadata): CollectionMetadata {
    return new CollectionMetadata(
        json.collectionUniqueName,
        json.creator,
        json.owners,
        json.tags,
        json.description,
        new Date(json.creationTime / NANOS_TO_MILLIS),
        json.ftraceEvents,
        json.targetMachine,
    );
  }
}

/**
 * CollectionDuration contains metadata about a collection duration.
 */
export interface CollectionDuration {
  duration: number;
  name: string;
  description: string;
}


