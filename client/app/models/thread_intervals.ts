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
import {ThreadResidency, ThreadState, threadStateToString} from './render_data_services';

/**
 * Interval subclass representing a thread state interval whose opacity is
 * determined by its state.
 *
 * If this represents a thread in a waiting state, then its height is
 * determined by how many threads are simultaneously queued in the same CPU
 * at the same time.
 */
export class ThreadInterval extends Interval {
  state: ThreadState;
  duration: number;

  constructor(
      public parameters: CollectionParameters,
      public cpu: number,
      public startTimeNs: number,
      public endTimeNs: number,
      public pid: number,
      public command: string,
      public threadResidencies: ThreadResidency[] = [],
      public queueOffset = 0.0,
      public queueCount = 1.0,
  ) {
    super(parameters, pid, cpu, startTimeNs, endTimeNs);
    this.duration = endTimeNs - startTimeNs;

    if (threadResidencies.length === 1) {
      this.state = threadResidencies[0].state;
    } else {
      this.state = ThreadState.UNKNOWN_STATE;
      const allStates = new Set(threadResidencies.map(tr => tr.state));
      // Don't render intervals with no states or entirely unknown states.
      if (!threadResidencies.length ||
          allStates.has(ThreadState.UNKNOWN_STATE) && allStates.size === 1) {
        this.shouldRender = false;
      }
    }

    switch (this.state) {
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
      'State': `${this.threadStatesToString()}`,
    };
  }

  get dataType() {
    return 'Thread';
  }

  get label() {
    return `${this.dataType}:${this.id}:${this.command}`;
  }

  /**
   * Get a string representation of the current thread state (if there's only
   * one) or a distribution of thread states (if there's more than one).
   */
  private threadStatesToString(): string {
    if (this.threadResidencies.length === 1) {
      return threadStateToString(this.state);
    }

    const stateTimes = this.threadResidencies.reduce((acc, tr) => {
      const base = acc[tr.state] != null ? acc[tr.state] : 0;
      acc[tr.state] = base + tr.duration;
      return acc;
    }, {} as {[k in ThreadState]: number});


    return '\n' +
        Object.keys(stateTimes)
            .map(
                (key):
                    [ThreadState,
                     number] => [+key, stateTimes[(+key) as ThreadState]])
            .sort((a, b) => b[1] - a[1])
            .map(([key, time]) => {
              const percentageTime =
                  ((time / this.duration) * 100).toPrecision(3);
              return `\t(${percentageTime}%) ${threadStateToString(key)}`;
            })
            .join('\n');
  }

  y(sortedFilteredCpus: number[]) {
    if (this.state !== ThreadState.WAITING_STATE) {
      return super.y(sortedFilteredCpus);
    }
    // y position in determined by the number of threads in the wait queue
    const row = sortedFilteredCpus.indexOf(this.cpu);
    const rowHeight = this.rowHeight(sortedFilteredCpus);
    const intervalHeight = 0.5 * rowHeight;
    // Position wait queue row below interval row, offset by total rows in queue
    const defaultY = row * rowHeight + intervalHeight;
    return this.queueCount ? defaultY + this.queueOffset / this.queueCount :
                             defaultY;
  }

  height(sortedFilteredCpus: number[]) {
    if (this.state !== ThreadState.WAITING_STATE) {
      return super.height(sortedFilteredCpus);
    }
    // Height position in determined by the number of threads in the wait queue
    const rowHeight = this.rowHeight(sortedFilteredCpus);
    const intervalHeight = 0.5 * rowHeight;
    const queueHeight = rowHeight - intervalHeight;
    const queueCount = this.queueCount ? this.queueCount : 1;
    const queueRowHeight =
        Math.atan(this.queueCount) / (Math.PI / 2) * queueHeight / queueCount;
    return queueRowHeight;
  }

  rx(sortedFilteredCpus: number[]) {
    if (this.state !== ThreadState.WAITING_STATE) {
      return super.rx(sortedFilteredCpus);
    }
    // Waiting intervals have straight edges
    return 0;
  }

  ry(sortedFilteredCpus: number[]) {
    if (this.state !== ThreadState.WAITING_STATE) {
      return super.ry(sortedFilteredCpus);
    }
    // Waiting intervals have straight edges
    return 0;
  }
}
