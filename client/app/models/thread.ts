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
import {CpuInterval} from './cpu_intervals';
import {Interval} from './interval';
import {ThreadState} from './render_data_services';
import {ThreadEvent} from './thread_event';
import {ThreadInterval} from './thread_intervals';

/**
 * Thread-specific datum with static thread metrics and a default CPU list
 * (represented as full collection-length intervals) for hover preview.
 */
export class Thread extends Interval {
  events: ThreadEvent[] = [];
  antagonists: ThreadInterval[] = [];
  constructor(
      public parameters: CollectionParameters,
      public pid: number,
      public cpus: number[],
      public command: string,
      public wakeups: number,
      public migrations: number,
      public runtime: number,
      public waittime: number,
      public sleeptime: number,
      public unknowntime: number,
      public eventsPending = false,
      public antagonistsPending = false,
  ) {
    super(parameters, pid);
    // Add a full-width interval for each CPU, for preview rendering.
    for (const cpu of cpus) {
      this.addChild(new CpuInterval(
          parameters, cpu, parameters.startTimeNs, parameters.endTimeNs, [{
            thread: {
              pid: this.pid,
              command,
              priority: 0,
            },
            duration: parameters.endTimeNs - parameters.startTimeNs,
            state: ThreadState.RUNNING_STATE,
            droppedEventIDs: [],
            includesSyntheticTransitions: false,
          }]));
    }
  }

  get tooltipProps() {
    return {
      'Pid': `${this.pid}`,
      'Command': this.command,
      'CPUs': `${this.cpus}`,
      'Start Time': this.formatTime(this.startTimeNs),
      'End Time': this.formatTime(this.endTimeNs),
      'Duration': this.formatTime(this.endTimeNs - this.startTimeNs),
    };
  }

  get dataType() {
    return 'Thread';  // TODO(sainsley): store types in a util class or enum
  }

  get label() {
    return `${this.dataType}:${this.id}:${this.command}`;
  }
}
