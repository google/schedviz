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
// Package schedevent provides utilities for working with scheduling
// trace.Events.
package schedevent

import (
	"fmt"
	"strconv"

	"github.com/google/schedviz/tracedata/trace"
)

// String returns a human-readable formatted sched event if the provided
// trace.Event is a supported scheduling event, and the raw printed event
// otherwise.
func String(ev *trace.Event) string {
	prefix := fmt.Sprintf("[%3d] %-22s %-10s ", ev.CPU, strconv.Itoa(int(ev.Timestamp)), ev.Name)
	switch ev.Name {
	case "sched_switch":
		return fmt.Sprintf("%s PID %d ('%s', prio %d, task state %d) to PID %d ('%s', prio %d) on CPU %3d",
			prefix,
			ev.NumberProperties["prev_pid"], ev.TextProperties["prev_comm"], ev.NumberProperties["prev_prio"], ev.NumberProperties["prev_state"],
			ev.NumberProperties["next_pid"], ev.TextProperties["next_comm"], ev.NumberProperties["next_prio"],
			ev.CPU)
	case "sched_wakeup", "sched_wakeup_new":
		return fmt.Sprintf("%s PID %d ('%s', prio %d) on CPU %3d",
			prefix,
			ev.NumberProperties["pid"], ev.TextProperties["comm"], ev.NumberProperties["prio"],
			ev.NumberProperties["target_cpu"])
	case "sched_migrate_task":
		return fmt.Sprintf("%s PID %d ('%s', prio %d) from CPU %3d to CPU %3d",
			prefix,
			ev.NumberProperties["pid"], ev.TextProperties["comm"], ev.NumberProperties["prio"],
			ev.NumberProperties["orig_cpu"], ev.NumberProperties["dest_cpu"])
	case "sched_wait_task", "sched_process_wait":
		return fmt.Sprintf("%s PID %d ('%s', prio %d) on CPU %3d",
			prefix,
			ev.NumberProperties["pid"], ev.TextProperties["comm"], ev.NumberProperties["prio"],
			ev.CPU)
	default:
		return fmt.Sprintf("NON-SCHED %s", ev.String())
	}
}
