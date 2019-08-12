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
import {ThreadState} from './render_data_services';

/**
 * Canonical trace event types.
 */
export enum ThreadEventType {
  UNKNOWN = 'Unknown',
  MIGRATE_TASK = 'Migration',
  PROCESS_WAIT = 'Process Wait',
  WAIT_TASK = 'Wait Task',
  SWITCH = 'Switch',
  WAKEUP = 'Wakeup',
  WAKEUP_NEW = 'New Wakeup',
  STAT_RUNTIME = 'Stat Runtime',
}

/**
 * Interval subclass representing an instantaneous event, which may occur
 * between multiple CPUs.
 */
export class ThreadEvent extends Interval {
  constructor(
      public parameters: CollectionParameters, public uid: number,
      public eventType: string, public timestampNs: number, public pid: number,
      public command: string, public priority: number, public cpu: number,
      public threadState: ThreadState, public prevPid?: number,
      public prevCommand?: string, public prevPriority?: number,
      public prevCpu?: number, public prevState?: ThreadState) {
    super(parameters, uid, cpu, timestampNs);
    this.minWidth = true;
    // TODO(sainsley): Add back when we support rendering individual migrations
    /*if (cpus.length > 1) {
      let opacity = 0.5 / cpus.length;
      // Add child intervals for event that spans multiple CPUs
      let prevChild;
      for (const cpu of cpus) {
        const child = new ThreadEvent(
            parameters, uid, eventType, pid, command, priorities, [cpu],
            timestampNs);
        child.opacity = opacity;
        this.addChild(child);
        if (prevChild) {
          this.addEdge(prevChild, child);
        }
        opacity += 0.5 / cpus.length;
        prevChild = child;
        // TODO(sainsley): Scale width as well as opacity
      }
    }*/
  }

  get tooltipProps() {
    const tooltip = {
      'Type': this.eventType,
      'Pid': `${this.pid}`,
      'Command': this.command,
      'Thread Priority': `${this.priority}`,
      'CPU(s)': this.prevCpu !== undefined ? `${this.prevCpu} to ${this.cpu}` :
                                             `${this.cpu}`,
      'Timestamp': this.formatTime(this.startTimeNs),
    };
    return tooltip;
  }

  get dataType() {
    return 'ThreadEvent';
  }

  get label() {
    return `${this.pid} : ${this.eventType} at ${this.timestampNs}`;
  }

  get description() {
    const threadString =
        ThreadEvent.getThreadString(this.pid, this.command, this.priority);
    switch (this.eventType) {
      case ThreadEventType.MIGRATE_TASK:
        return `MIGRATE    ${threadString} from CPU ` +
            `${this.prevCpu} to CPU ${this.cpu}`;
      case ThreadEventType.PROCESS_WAIT:
        return `WAIT       ${threadString} on CPU ${this.cpu}`;
      case ThreadEventType.WAIT_TASK:
        return `WAIT       ${threadString} on CPU ${this.cpu}`;
      case ThreadEventType.SWITCH:
        const prevThreadString = ThreadEvent.getThreadString(
            this.prevPid, this.prevCommand, this.prevPriority);
        return `SWITCH     ${prevThreadString} to ${threadString} on CPU ${
            this.cpu}`;
      case ThreadEventType.WAKEUP:
        return `WAKEUP     ${threadString} on CPU ${this.cpu}`;
      case ThreadEventType.WAKEUP_NEW:
        return `WAKEUP_NEW ${threadString} on CPU ${this.cpu}`;
      default:
        return `(UNKNOWN)`;
    }
  }

  static getThreadString(pid?: number, command?: string, priority?: number) {
    const unknown = '<Unknown>';
    return `PID ${pid !== undefined ? pid : unknown} ('${
        command ?
            command :
            unknown}', prio ${priority !== undefined ? priority : unknown})`;
  }
}
