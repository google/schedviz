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
import {FtraceEvent} from './render_data_services';

/**
 * Canonical trace event types.
 */
export enum FtraceEventType {
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
export class FtraceInterval extends Interval {
  eventType: string;

  command?: string;
  prevCommand?: string;
  prevCpu?: number;
  pid?: number;
  prevState?: number;
  prevPid?: number;
  priority?: number;
  prevPriority?: number;

  constructor(
      public parameters: CollectionParameters,
      event: FtraceEvent,
  ) {
    super(parameters, event.index, event.cpu, event.timestamp);
    this.minWidth = true;
    switch (event.name) {
      case 'sched_switch':
        this.processRawSchedSwitch(event);
        this.eventType = FtraceEventType.SWITCH;
        break;
      case 'sched_migrate_task':
        this.processRawSchedMigrateTask(event);
        this.eventType = FtraceEventType.MIGRATE_TASK;
        break;
      case 'sched_wakeup':
        this.processRawSchedWakeup(event);
        this.eventType = FtraceEventType.WAKEUP;
        break;
      case 'sched_wakeup_new':
        this.processRawSchedWakeup(event);
        this.eventType = FtraceEventType.WAKEUP_NEW;
        break;
      case 'sched_process_wait':
        this.processRawSchedWait(event);
        this.eventType = FtraceEventType.PROCESS_WAIT;
        break;
      case 'sched_wait_task':
        this.processRawSchedWait(event);
        this.eventType = FtraceEventType.WAIT_TASK;
        break;
      default:
        this.eventType = FtraceEventType.UNKNOWN;
    }
    // TODO(sainsley): Add back when we support rendering individual migrations
    /*if (cpus.length > 1) {
      let opacity = 0.5 / cpus.length;
      // Add child intervals for event that spans multiple CPUs
      let prevChild;
      for (const cpu of cpus) {
        const child = new FtraceInterval(
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
      'Command': `${this.command}`,
      'Thread Priority': `${this.priority}`,
      'CPU(s)': this.prevCpu !== undefined ? `${this.prevCpu} to ${this.cpu}` :
                                             `${this.cpu}`,
      'Timestamp': this.formatTime(this.startTimeNs),
    };
    return tooltip;
  }

  get dataType() {
    return 'FtraceInterval';
  }

  get label() {
    return `${this.pid} : ${this.eventType} at ${this.startTimeNs}`;
  }

  get description() {
    const threadString =
        FtraceInterval.getThreadString(this.pid, this.command, this.priority);
    switch (this.eventType) {
      case FtraceEventType.MIGRATE_TASK:
        return `MIGRATE    ${threadString} from CPU ` +
            `${this.prevCpu} to CPU ${this.cpu}`;
      case FtraceEventType.PROCESS_WAIT:
        return `WAIT       ${threadString} on CPU ${this.cpu}`;
      case FtraceEventType.WAIT_TASK:
        return `WAIT       ${threadString} on CPU ${this.cpu}`;
      case FtraceEventType.SWITCH:
        const prevThreadString = FtraceInterval.getThreadString(
            this.prevPid, this.prevCommand, this.prevPriority);
        return `SWITCH     ${prevThreadString} (state ${this.prevState}) to ${
            threadString} on CPU ${this.cpu}`;
      case FtraceEventType.WAKEUP:
        return `WAKEUP     ${threadString} on CPU ${this.cpu}`;
      case FtraceEventType.WAKEUP_NEW:
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

  processRawSchedSwitch(event: FtraceEvent) {
    this.command = event.textProperties['next_comm'];
    this.prevCommand = event.textProperties['prev_comm'];

    const eventCpu = event.numberProperties['cpu'];
    this.cpu = eventCpu != null ? eventCpu : this.cpu;

    this.pid = event.numberProperties['next_pid'];
    this.prevPid = event.numberProperties['prev_pid'];

    this.priority = event.numberProperties['next_prio'];
    this.prevPriority = event.numberProperties['prev_prio'];

    this.prevState = event.numberProperties['prev_state'];
  }

  processRawSchedMigrateTask(event: FtraceEvent) {
    this.command = event.textProperties['comm'];

    const eventCpu = event.numberProperties['dest_cpu'];
    this.cpu = eventCpu != null ? eventCpu : this.cpu;
    this.prevCpu = event.numberProperties['orig_cpu'];

    this.pid = event.numberProperties['pid'];

    this.priority = event.numberProperties['prio'];
  }

  processRawSchedWakeup(event: FtraceEvent) {
    this.command = event.textProperties['comm'];

    const eventCpu = event.numberProperties['target_cpu'];
    this.cpu = eventCpu != null ? eventCpu : this.cpu;

    this.pid = event.numberProperties['pid'];

    this.priority = event.numberProperties['prio'];
  }

  processRawSchedWait(event: FtraceEvent) {
    this.command = event.textProperties['comm'];

    this.cpu = event.cpu;

    this.pid = event.numberProperties['pid'];

    this.priority = event.numberProperties['prio'];
  }
}
