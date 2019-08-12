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
import {ThreadResidency} from './render_data_services';

/**
 * Collection of both running and waiting CPU Intervals
 */
export class CpuIntervalCollection {
  constructor(
      public cpu: number,
      public running: CpuInterval[] = [],
      public waiting: WaitingCpuInterval[] = [],
  ) {}
}

/**
 * Client-side CPU interval representation, for rendering.
 */
export class CpuInterval extends Interval {
  runningNs: number;
  waitingNs: number;
  idleNs: number;

  constructor(
      public parameters: CollectionParameters,
      public cpu: number,
      public startTimeNs: number,
      public endTimeNs: number,
      public running: ThreadResidency[] = [],
      public waiting: ThreadResidency[] = [],
  ) {
    super(parameters, cpu, cpu, startTimeNs, endTimeNs);
    this.renderWeight = waiting.length;

    this.running = this.running.sort((a, b) => b.duration - a.duration);
    this.waiting = this.waiting.sort((a, b) => b.duration - a.duration);

    this.waitingNs = this.waiting.reduce((acc, r) => acc + r.duration, 0);
    this.runningNs = this.running.reduce((acc, r) => acc + r.duration, 0);
    this.idleNs = this.duration - this.runningNs;
  }

  private getPercentageTimeStr(duration: number): string {
    return `${((duration / this.duration) * 100).toPrecision(3)}%`;
  }

  private getThreadName(threadResidency: ThreadResidency): string {
    return `${threadResidency.thread.pid}:${threadResidency.thread.command}`;
  }

  get tooltipProps() {
    return {
      'Running': '\n' +
          this.running
              .map(
                  r => `  (${this.getPercentageTimeStr(r.duration)}) ${
                      this.getThreadName(r)}`)
              .join('\n'),
      'CPU': `${this.cpu}`,
      'Start Time': this.formatTime(this.startTimeNs),
      'End Time': this.formatTime(this.endTimeNs),
      'Duration': this.formatTime(this.duration),
      'Idle Time': `(${this.getPercentageTimeStr(this.idleNs)}) ${
          this.formatTime(this.idleNs)}`,
      'Running Time': `(${this.getPercentageTimeStr(this.runningNs)}) ${
          this.formatTime(this.runningNs)}`,
      'Waiting Time': ` (${this.getPercentageTimeStr(this.waitingNs)}) ${
          this.formatTime(this.waitingNs)}`,
      'Waiting PID Count': `${this.waiting.length}`,
      'Waiting': '\n' +
          this.waiting
              .map(
                  w => `  (${this.getPercentageTimeStr(w.duration)}) ${
                      this.getThreadName(w)} `)
              .join('\n'),
    };
  }

  get dataType() {
    return 'CPU';
  }

  /**
   * @return relative render height, weighted by percent idle time
   */
  height(sortedFilteredCpus: number[]) {
    if (this.duration === 0) {
      return 0;
    }
    const rowHeight = this.rowHeight(sortedFilteredCpus);
    const intervalHeight = 0.5 * rowHeight;
    const percentIdle = this.idleNs / this.duration;
    return (1.0 - percentIdle) * intervalHeight;
  }
}

/**
 * Client-side CPU waiting interval representation, for rendering.
 */
export class WaitingCpuInterval extends CpuInterval {
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
    return Math.atan(this.waitingNs / this.duration) / (Math.PI / 2) *
        queueHeight;
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
