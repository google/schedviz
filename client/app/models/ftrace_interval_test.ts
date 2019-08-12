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
import {FtraceEventType, FtraceInterval} from './ftrace_interval';
import {FtraceEvent} from './render_data_services';

describe('Ftrace Interval', () => {
  function getRawEvent(name: string): FtraceEvent {
    return {
      index: 1, name, cpu: 0, timestamp: 1000, clipped: false,
          numberProperties: {
            'cpu': 1,
            'orig_cpu': 2,
            'dest_cpu': 3,
            'target_cpu': 4,
            'pid': 5,
            'next_pid': 6,
            'prev_pid': 7,
            'next_prio': 8,
            'prev_prio': 9,
            'prio': 10,
            'prev_state': 11,
          },
          textProperties: {
            'comm': 'comm',
            'next_comm': 'comm_next',
            'prev_comm': 'comm_prev',
          },
    };
  }

  function mockParameters(): CollectionParameters {
    const eventTypes = [
      'sched_switch', 'sched_migrate_task', 'sched_wakeup', 'sched_wakeup_new',
      'sched_process_wait', 'sched_wait_task'
    ];
    return new CollectionParameters(
        'foo', [0, 1, 2, 3, 4], 0, 10000, eventTypes);
  }

  const testCases: Array<{
    name: string,
    processor: keyof FtraceInterval,
    expected: {[k in keyof FtraceInterval]?: FtraceInterval[k];}
  }> =
      [
        {
          name: 'sched_switch',
          processor: 'processRawSchedSwitch',
          expected: {
            eventType: FtraceEventType.SWITCH,
            description:
                'SWITCH     PID 7 (\'comm_prev\', prio 9) (state 11) to PID 6 (\'comm_next\', prio 8) on CPU 1',
            command: 'comm_next',
            prevCommand: 'comm_prev',
            cpu: 1,
            pid: 6,
            prevPid: 7,
            priority: 8,
            prevPriority: 9,
            prevState: 11,
          }
        },
        {
          name: 'sched_migrate_task',
          processor: 'processRawSchedMigrateTask',
          expected: {
            eventType: FtraceEventType.MIGRATE_TASK,
            description:
                'MIGRATE    PID 5 (\'comm\', prio 10) from CPU 2 to CPU 3',
            command: 'comm',
            cpu: 3,
            prevCpu: 2,
            pid: 5,
            priority: 10,
          }
        },
        {
          name: 'sched_wakeup',
          processor: 'processRawSchedWakeup',
          expected: {
            eventType: FtraceEventType.WAKEUP,
            description: 'WAKEUP     PID 5 (\'comm\', prio 10) on CPU 4',
            command: 'comm',
            cpu: 4,
            pid: 5,
            priority: 10,
          }
        },
        {
          name: 'sched_wakeup_new',
          processor: 'processRawSchedWakeup',
          expected: {
            eventType: FtraceEventType.WAKEUP_NEW,
            description: 'WAKEUP_NEW PID 5 (\'comm\', prio 10) on CPU 4',
            command: 'comm',
            cpu: 4,
            pid: 5,
            priority: 10,
          }
        },
        {
          name: 'sched_process_wait',
          processor: 'processRawSchedWait',
          expected: {
            eventType: FtraceEventType.PROCESS_WAIT,
            description: 'WAIT       PID 5 (\'comm\', prio 10) on CPU 0',
            command: 'comm',
            cpu: 0,
            pid: 5,
            priority: 10,
          }
        },
        {
          name: 'sched_wait_task',
          processor: 'processRawSchedWait',
          expected: {
            eventType: FtraceEventType.WAIT_TASK,
            description: 'WAIT       PID 5 (\'comm\', prio 10) on CPU 0',
            command: 'comm',
            cpu: 0,
            pid: 5,
            priority: 10,
          }
        },
      ];

  for (const test of testCases) {
    it(`extracts the correct fields from a ${test.name} event`, () => {
      const processSpy =
          spyOn(FtraceInterval.prototype, test.processor).and.callThrough();
      const rawEvent = getRawEvent(test.name);
      const ftraceInterval = new FtraceInterval(mockParameters(), rawEvent);

      expect(processSpy).toHaveBeenCalledTimes(1);
      expect(processSpy).toHaveBeenCalledWith(rawEvent);

      const expectedKeys =
          Object.keys(test.expected) as Array<keyof FtraceInterval>;
      for (const key of expectedKeys) {
        expect(ftraceInterval[key]).toBe(test.expected[key]);
      }
    });
  }
});
