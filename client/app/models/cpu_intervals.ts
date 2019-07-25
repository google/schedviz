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
 * Client-side CPU interval representation, for rendering.
 */
export class CpuInterval extends Interval {
  constructor(
      public parameters: CollectionParameters, public cpu: number,
      public startTimeNs: number, public endTimeNs: number,
      public command: string, public runningPid = -1, public idleNs = 0,
      public waitingPidCount = 0, public waitingPidList:number[] = []) {
    super(parameters, cpu, cpu, startTimeNs, endTimeNs);
    this.renderWeight = waitingPidCount;
  }

  get tooltipProps() {
    return {
      'Command': this.command,
      'CPU': `${this.cpu}`,
      'Start Time': this.formatTime(this.startTimeNs),
      'End Time': this.formatTime(this.endTimeNs),
      'Duration': this.formatTime(this.endTimeNs - this.startTimeNs),
      'Running PID': `${this.runningPid > -1 ? this.runningPid : 'Unknown'}`,
      'Waiting PID Count': `${this.waitingPidCount}`,
      'Waiting PID List': `${this.waitingPidList}`,
    };
  }

  get dataType() {
    return 'CPU';
  }

  /**
   * @return relative render height, weighted by percent idle time
   */
  height(sortedFilteredCpus: number[]) {
    const rowHeight = this.rowHeight(sortedFilteredCpus);
    const intervalHeight = 0.5 * rowHeight;
    const duration = this.endTimeNs - this.startTimeNs;
    const percentIdle = this.idleNs / duration;
    if (duration === 0) {
      return 0;
    }
    return (1.0 - percentIdle) * intervalHeight;
  }
}

/**
 * Client-side CPU waiting interval representation, for rendering.
 */
export class WaitingCpuInterval extends CpuInterval {
  constructor(
      public parameters: CollectionParameters, public cpu: number,
      public startTimeNs: number, public endTimeNs: number,
      public command: string, public waitingPidCount: number,
      public runningPid = -1, public waitingPidList:number[] = []) {
    super(
        parameters, cpu, startTimeNs, endTimeNs, command, runningPid, 0,
        waitingPidCount, waitingPidList);
  }

  /**
   * @return y relative position, below CPU running intervals
   */
  y(sortedFilteredCpus: number[]) {
    const row = sortedFilteredCpus.indexOf(this.cpu);
    const rowHeight = this.rowHeight(sortedFilteredCpus);
    const intervalHeight = 0.5 * rowHeight;
    // Position wait queue row below interval row, offset by total rows in queue
    return row * rowHeight + intervalHeight;
  }

  /**
   * @return render height, weighted by wait queue size
   */
  height(sortedFilteredCpus: number[]) {
    const rowHeight = this.rowHeight(sortedFilteredCpus);
    const intervalHeight = 0.5 * rowHeight;
    const queueHeight = rowHeight - intervalHeight;
    return Math.atan(this.waitingPidCount) / (Math.PI / 2) * queueHeight;
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
