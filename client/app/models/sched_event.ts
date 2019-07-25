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
import {CollectionParameters} from './collection';
import {Interval} from './interval';

/**
 * Interval subclass representing an instantaneous event.
 */
export class SchedEvent extends Interval {
  constructor(
      public parameters: CollectionParameters, public uid: number,
      public name: string, public cpu: number, public timestampNs: number,
      public properties?: Map<string, string>) {
    super(parameters, uid, cpu, timestampNs);
    this.opacity = 0.5;
  }

  get tooltipProps() {
    const tooltip : {[key: string]: string} = {
      'Type': this.name,
      'CPU': `${this.cpu}`,
      'Timestamp': this.formatTime(this.startTimeNs),
    };
    if (this.properties) {
      for (const property of this.properties.keys()) {
        tooltip[property] = `${this.properties.get(property)}`;
      }
    }
    return tooltip;
  }

  get dataType() {
    return 'SchedEvent';
  }

  get label() {
    return `${this.name} : ${this.uid}`;
  }

  /**
   * @return render height, weighted by wait queue size
   */
  height(sortedFilteredCpus: number[]) {
    return this.rowHeight(sortedFilteredCpus);
  }

  /**
   * Waiting intervals have straight edges
   */
  rx(sortedFilteredCpus: number[]) {
    return 0;
  }

  /**
   * Waiting intervals have straight edges
   */
  ry(sortedFilteredCpus: number[]) {
    return 0;
  }
}
