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
// Package schedtestcommon provides common types and pre-populated
// eventsetbuilder.Builders for testing.
package schedtestcommon

import (
	"testing"

	"github.com/google/schedviz/tracedata/eventsetbuilder"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/google/schedviz/tracedata/testeventsetbuilder"
)

// UnpopulatedBuilder returns a new Builder with sched event descriptors but
// no events.
func UnpopulatedBuilder() *eventsetbuilder.Builder {
	return eventsetbuilder.NewBuilder().
		WithEventDescriptor(
			"sched_wakeup",
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio"),
			eventsetbuilder.Number("target_cpu")).
		WithEventDescriptor(
			"sched_wakeup_new",
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio"),
			eventsetbuilder.Number("target_cpu")).
		WithEventDescriptor(
			"sched_switch",
			eventsetbuilder.Number("prev_pid"),
			eventsetbuilder.Text("prev_comm"),
			eventsetbuilder.Number("prev_prio"),
			eventsetbuilder.Number("prev_state"),
			eventsetbuilder.Number("next_pid"),
			eventsetbuilder.Text("next_comm"),
			eventsetbuilder.Number("next_prio")).
		WithEventDescriptor(
			"sched_migrate_task",
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio"),
			eventsetbuilder.Number("orig_cpu"),
			eventsetbuilder.Number("dest_cpu")).
		WithEventDescriptor(
			"sched_wait_task",
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio")).
		WithEventDescriptor(
			"sched_process_wait",
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio")).
		WithEventDescriptor(
			"sched_stat_runtime",
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("runtime"))
}

// Used to set switching-out threads' states.
const (
	Runnable      = 0
	Interruptible = 1
)

// TestTrace1 returns a small but interesting trace.
//
// Per-PID events:
//
// For PID 100:
// 	1µs                WAKEUP  PID 100 ('Process1', Unknown, prio 50) on CPU   1
// 	1.01µs             SWITCH  PID 300 ('Process3', Sleeping, prio 50, task state 1) to PID 100 ('Process1', Running, prio 50) on CPU   1
// 	1.1µs              SWITCH  PID 100 ('Process1', Waiting, prio 50, task state 0) to PID 300 ('Process3', Running, prio 50) on CPU   1
// For PID 300:
// 	1µs                SWITCH  PID 200 ('Process2', Sleeping, prio 50) to PID 300 ('Process3', Running, prio 50) on CPU   1
// 	1.01µs             SWITCH  PID 300 ('Process3', Sleeping, prio 50, task state 1) to PID 100 ('Process1', Running, prio 50) on CPU   1
// 	1.09µs             WAKEUP  PID 300 ('Process3', Waiting, prio 50) on CPU   1
// 	1.1µs              SWITCH  PID 100 ('Process1', Waiting, prio 50, task state 0) to PID 300 ('Process3', Running, prio 50) on CPU   1
// For PID 200:
// 	1µs                SWITCH  PID 200 ('Process2', Sleeping, prio 50) to PID 300 ('Process3', Running, prio 50) on CPU   1
// 	1.04µs             WAKEUP  PID 200 ('Process2', Unknown, prio 50) on CPU   1
// 	1.08µs             MIGRATE PID 200 ('Process2', Waiting, prio 50) from CPU   1 to CPU   2
// 	1.1µs              SWITCH  PID 400 ('Process4', Waiting, prio 50, task state 0) to PID 200 ('Process2', Running, prio 50) on CPU   2
// For PID 400:
// 	1µs                INITIAL PID 400 ('Process4', Running, prio 50) on CPU   2
// 	1.1µs              SWITCH  PID 400 ('Process4', Waiting, prio 50, task state 0) to PID 200 ('Process2', Running, prio 50) on CPU   2
//
// Per-CPU events:
//
// For CPU   1:
// 	1µs                SWITCH  PID 200 ('Process2', Sleeping, prio 50) to PID 300 ('Process3', Running, prio 50) on CPU   1
// 	1µs                WAKEUP  PID 100 ('Process1', Unknown, prio 50) on CPU   1
// 	1.01µs             SWITCH  PID 300 ('Process3', Sleeping, prio 50, task state 1) to PID 100 ('Process1', Running, prio 50) on CPU   1
// 	1.04µs             WAKEUP  PID 200 ('Process2', Unknown, prio 50) on CPU   1
// 	1.08µs             MIGRATE PID 200 ('Process2', Waiting, prio 50) from CPU   1 to CPU   2
// 	1.09µs             WAKEUP  PID 300 ('Process3', Waiting, prio 50) on CPU   1
// 	1.1µs              SWITCH  PID 100 ('Process1', Waiting, prio 50, task state 0) to PID 300 ('Process3', Running, prio 50) on CPU   1
// For CPU   2:
// 	1µs                INITIAL PID 400 ('Process4', Running, prio 50) on CPU   2
// 	1.08µs             MIGRATE PID 200 ('Process2', Waiting, prio 50) from CPU   1 to CPU   2
// 	1.1µs              SWITCH  PID 400 ('Process4', Waiting, prio 50, task state 0) to PID 200 ('Process2', Running, prio 50) on CPU   2
//
// Per-PID spans:
//
// PID 100:
//   PID     100 (Waiting, Process1, prio   50) on CPU   1 [0 - 10] (0)
//   PID     100 (Running, Process1, prio   50) on CPU   1 [10 - 101] (0)
// PID 200:
//   PID     200 (Running, Process2, prio   50) on CPU   1 [0 - 0] (0)
//   PID     200 (Sleeping, Process2, prio   50) on CPU   1 [0 - 40] (0)
//   PID     200 (Waiting, Process2, prio   50) on CPU   1 [40 - 80] (0)
//   PID     200 (Waiting, Process2, prio   50) on CPU   2 [80 - 101] (0)
// PID 300:
//   PID     300 (Running, Process3, prio   50) on CPU   1 [0 - 10] (0)
//   PID     300 (Sleeping, Process3, prio   50) on CPU   1 [10 - 90] (0)
//   PID     300 (Waiting, Process3, prio   50) on CPU   1 [90 - 101] (0)
// PID 400:
//   PID     400 (Running, Process4, prio   50) on CPU   2 [0 - 101] (0)
//
// Per-CPU spans:
//
// CPU 1:
//   PID     200 (Running, Process2, prio   50) on CPU   1 [0 - 0] (0)
//   PID     100 (Waiting, Process1, prio   50) on CPU   1 [0 - 10] (0)
//   PID     300 (Running, Process3, prio   50) on CPU   1 [0 - 10] (0)
//   PID     200 (Sleeping, Process2, prio   50) on CPU   1 [0 - 40] (0)
//   PID     300 (Sleeping, Process3, prio   50) on CPU   1 [10 - 90] (0)
//   PID     100 (Running, Process1, prio   50) on CPU   1 [10 - 101] (0)
//   PID     200 (Waiting, Process2, prio   50) on CPU   1 [40 - 80] (0)
//   PID     300 (Waiting, Process3, prio   50) on CPU   1 [90 - 101] (0)
// CPU 2:
//   PID     200 (Waiting, Process2, prio   50) on CPU   2 [80 - 101] (0)
//   PID     400 (Running, Process4, prio   50) on CPU   2 [0 - 101] (0)
func TestTrace1(t *testing.T) *eventpb.EventSet {
	return testeventsetbuilder.TestProtobuf(t,
		UnpopulatedBuilder().
			// PID 600 wakes up on CPU 3 at time 500, but this is clipped.
			WithEvent("sched_wakeup", 3, 500, true,
				600, "Process6", 50, 1).
			// PID 300 switches in on CPU 1 at time 1000, PID 200 switches out SLEEPING
			WithEvent("sched_switch", 1, 1000, false,
				200, "Process2", 50, Interruptible,
				300, "Process3", 50).
			// PID 100 wakes up on CPU 1 at time 1000
			WithEvent("sched_wakeup", 0, 1000, false,
				100, "Process1", 50, 1).
			// PID 100 switches in on CPU 1 at time 1010; PID 300 switches out SLEEPING
			WithEvent("sched_switch", 1, 1010, false,
				300, "Process3", 50, Interruptible,
				100, "Process1", 50).
			// PID 200 wakes up on CPU 1 at time 1040
			WithEvent("sched_wakeup", 0, 1040, false,
				200, "Process2", 50, 1).
			// PID 200 migrates from CPU 1 to CPU 2 at time 1080
			WithEvent("sched_migrate_task", 0, 1080, false,
				200, "Process2", 50,
				1, 2).
			// PID 300 wakes up on CPU 1 at time 1090
			WithEvent("sched_wakeup", 0, 1090, false,
				300, "Process3", 50, 1).
			// PID 200 switches in on CPU 2 at time 1100; PID 400 switches out Runnable
			WithEvent("sched_switch", 2, 1100, false,
				400, "Process4", 50, Runnable,
				200, "Process2", 50).
			// PID 300 switches in on CPU 1 at time 1100; PID 100 switches out Runnable
			WithEvent("sched_switch", 1, 1100, false,
				100, "Process1", 50, Runnable,
				300, "Process3", 50).
			// PID 500 yields to PID 501 at time 99000, but this is clipped.
			WithEvent("sched_switch", 0, 99000, true,
				500, "InvalidProcess", 50, 130,
				501, "InvalidProcess", 50))
}
