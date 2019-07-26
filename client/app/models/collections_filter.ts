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
import {BehaviorSubject} from 'rxjs';

/**
 * The JSON representation of CollectionsFilter
 */
export interface CollectionsFilterJSON {
  creationTime?: string;
  description?: string;
  name?: string;
  tags?: string;
  targetMachine?: string;
}

/**
 * Helper Type to get all keys of an object that have a given type.
 */
type KeysOfType<TObj, TProp, K extends keyof TObj = keyof TObj> = K extends K ?
    TObj[K] extends TProp ? K : never :
    never;

/**
 * CollectionFilterKeys is a type containing all the properties
 * (i.e. not functions) of CollectionsFilter.
 */
export type CollectionFilterKeys = KeysOfType<CollectionsFilter, string>;

/**
 * Collections filter is a model class that holds the filter state
 * of the collections_table.
 */
export class CollectionsFilter {
  changes = new BehaviorSubject<{prop: string, newVal: string}|null>(null);
  constructor(filter: {[k in CollectionFilterKeys]?: string} = {}) {
    Object.assign(this, filter);
  }

  private targetMachineVal = '';
  get targetMachine() {
    return this.targetMachineVal;
  }
  set targetMachine(newtargetMachine: string) {
    this.targetMachineVal = newtargetMachine;
    this.changes.next({prop: 'targetMachine', newVal: newtargetMachine});
  }

  private creationTimeVal = '';
  get creationTime() {
    return this.creationTimeVal;
  }
  set creationTime(newcreationTime: string) {
    this.creationTimeVal = newcreationTime;
    this.changes.next({prop: 'creationTime', newVal: newcreationTime});
  }

  private tagsVal = '';
  get tags() {
    return this.tagsVal;
  }
  set tags(newtags: string) {
    this.tagsVal = newtags;
    this.changes.next({prop: 'tags', newVal: newtags});
  }

  private descriptionVal = '';
  get description() {
    return this.descriptionVal;
  }
  set description(newdescription: string) {
    this.descriptionVal = newdescription;
    this.changes.next({prop: 'description', newVal: newdescription});
  }

  private nameVal = '';
  get name() {
    return this.nameVal;
  }
  set name(newname: string) {
    this.nameVal = newname;
    this.changes.next({prop: 'name', newVal: newname});
  }

  toJSON(): CollectionsFilterJSON {
    const ret: CollectionsFilterJSON = {};
    if (this.creationTime != null) {
      ret.creationTime = this.creationTime;
    }
    if (this.description != null) {
      ret.description = this.description;
    }
    if (this.name != null) {
      ret.name = this.name;
    }
    if (this.tags != null) {
      ret.tags = this.tags;
    }
    if (this.targetMachine != null) {
      ret.targetMachine = this.targetMachine;
    }
    return ret;
  }
}
