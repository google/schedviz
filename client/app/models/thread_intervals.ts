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
import {ThreadState, threadStateToString} from './render_data_services';

/**
 * Interval subclass representing a thread state interval whose opacity is
 * determined by its state.
 */
export class ThreadInterval extends Interval {
  constructor(
      public parameters: CollectionParameters, public cpu: number,
      public startTimeNs: number, public endTimeNs: number, public pid: number,
      public command: string, public state = ThreadState.RUNNING_STATE) {
    super(parameters, pid, cpu, startTimeNs, endTimeNs);
    switch (state) {
      case ThreadState.SLEEPING_STATE:
        this.opacity = 0.1;
        break;
      case ThreadState.UNKNOWN_STATE:
        this.opacity = 0.5;
        break;
      default:
        this.opacity = 0.9;
        break;
    }
  }

  get tooltipProps() {
    return {
      'Pid': `${this.pid}`,
      'Command': this.command,
      'CPU': `${this.cpu}`,
      'Start Time': this.formatTime(this.startTimeNs),
      'End Time': this.formatTime(this.endTimeNs),
      'Duration': this.formatTime(this.endTimeNs - this.startTimeNs),
      'State': `${threadStateToString(this.state)}`,
    };
  }

  get dataType() {
    return 'Thread';
  }

  get label() {
    return `${this.dataType}:${this.id}:${this.command}`;
  }
}

/**
 * Interval subclass representing a thread in a waiting state, whose height is
 * determined by how many threads are simultaneously queued in the same CPU
 * at the same time.
 */
export class WaitingThreadInterval extends ThreadInterval {
  constructor(
      public parameters: CollectionParameters, public cpu: number,
      public startTimeNs: number, public endTimeNs: number, public pid: number,
      public command: string, public queueOffset = 0.0,
      public queueCount = 1.0) {
    super(
        parameters, cpu, startTimeNs, endTimeNs, pid, command,
        ThreadState.WAITING_STATE);
  }

  /**
   * y position in determined by the number of threads in the wait queue
   */
  y(sortedFilteredCpus: number[]) {
    const row = sortedFilteredCpus.indexOf(this.cpu);
    const rowHeight = this.rowHeight(sortedFilteredCpus);
    const intervalHeight = 0.5 * rowHeight;
    // Position wait queue row below interval row, offset by total rows in queue
    const defaultY = row * rowHeight + intervalHeight;
    return this.queueCount ? defaultY + this.queueOffset / this.queueCount :
                             defaultY;
  }

  /**
   * height position in determined by the number of threads in the wait queue
   */
  height(sortedFilteredCpus: number[]) {
    const rowHeight = this.rowHeight(sortedFilteredCpus);
    const intervalHeight = 0.5 * rowHeight;
    const queueHeight = rowHeight - intervalHeight;
    const queueCount = this.queueCount ? this.queueCount : 1;
    const queueRowHeight =
        Math.atan(this.queueCount) / (Math.PI / 2) * queueHeight / queueCount;
    return queueRowHeight;
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
