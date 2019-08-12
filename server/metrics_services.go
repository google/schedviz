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
package models

import "github.com/google/schedviz/analysis/sched"

// ThreadSummariesRequest is a request for thread summary information across a specified timespan
// for a specified collection, filtered to the requested CPU set. If start_timestamp_ns is -1, the
// first timestamp in the collection is used  instead. If end_timestamp_ns is -1, the last timestamp
// in the collection is used instead. If the provided CPU set is empty, all CPUs are filtered in.
type ThreadSummariesRequest struct {
	CollectionName   string  `json:"collectionName"`
	StartTimestampNs int64   `json:"startTimestampNs"`
	EndTimestampNs   int64   `json:"endTimestampNs"`
	Cpus             []int64 `json:"cpus"`
}

// Metrics holds a set of aggregated metrics for some or all of the sched trace.
type Metrics struct {
	// The number of migrations observed in the aggregated trace.  If CPU
	// filtering was used generating this Metric, only migrations inbound to a
	// filtered-in CPU are aggregated.
	MigrationCount int64 `json:"migrationCount"`
	// The number of wakeups observed in the aggregated trace.
	WakeupCount int64 `json:"wakeupCount"`
	// Aggregated thread-state times over the aggregated trace.
	UnknownTimeNs int64 `json:"unknownTimeNs"`
	RunTimeNs     int64 `json:"runTimeNs"`
	WaitTimeNs    int64 `json:"waitTimeNs"`
	SleepTimeNs   int64 `json:"sleepTimeNs"`
	// Unique PIDs, COMMs, priorities, and CPUs observed in the aggregated trace.
	// Note that these fields are not correlated; if portions of trace containing
	// execution from several different PIDs are aggregated together in a metric,
	// all of their PIDs, commands, and priorities will be present here, and the
	// Metrics can reveal which PIDs were present, but it will not be possible to
	// tell from the Metrics which commands go with which PIDs, and so forth.
	// TODO(sabarabc) Create maps from PID -> ([]command, []priority),
	//  command -> ([]PID, []priority), and priority -> ([]PID, []command)
	//  so that we can tell which of these are correlated.
	Pids       []int64  `json:"pids"`
	Commands   []string `json:"commands"`
	Priorities []int64  `json:"priorities"`
	Cpus       []int64  `json:"cpus"`
	// The time range over which these metrics were aggregated.
	StartTimestampNs int64 `json:"startTimestampNs"`
	EndTimestampNs   int64 `json:"endTimestampNs"`
}

// ThreadSummariesResponse contains the response to a ThreadSummariesRequest.
type ThreadSummariesResponse struct {
	CollectionName string    `json:"collectionName"`
	Metrics        []Metrics `json:"metrics"`
}

// AntagonistsRequest is a request for antagonist information for a specified set of threads, across
// a specified timestamp for a specified collection.  If start_timestamp_ns is -1,
// the first timestamp in the collection is used instead.  If end_timestamp_ns
// is -1, the last timestamp in the collection is used instead.
type AntagonistsRequest struct {
	// The collection name.
	CollectionName   string  `json:"collectionName"`
	Pids             []int64 `json:"pids"`
	StartTimestampNs int64   `json:"startTimestampNs"`
	EndTimestampNs   int64   `json:"endTimestampNs"`
}

// AntagonistsResponse is a response for an antagonist request.
type AntagonistsResponse struct {
	CollectionName string `json:"collectionName"`
	// All matching stalls sorted in order of decreasing duration - longest first.
	Antagonists []sched.Antagonists `json:"antagonists"`
}

// EventType is an enum containing different types of events
type EventType = int64

const (
	// EventTypeUnknown is an unknown event type
	EventTypeUnknown EventType = iota

	// Regular ftrace sched events.

	// EventTypeMigrateTask is a MIGRATE_TASK event
	EventTypeMigrateTask
	// EventTypeProcessWait is a PROCESS_WAIT event
	EventTypeProcessWait
	// EventTypeWaitTask is a WAIT_TASK event
	EventTypeWaitTask
	// EventTypeSwitch is a SWITCH event
	EventTypeSwitch
	// EventTypeWakeup is a WAKEUP event
	EventTypeWakeup
	// EventTypeWakeupNew is a WAKEUP_NEW event
	EventTypeWakeupNew
	// EventTypeStatRuntime is a STAT_RUNTIME event
	EventTypeStatRuntime
)

// Event is a struct containing information from sched ftrace events
type Event struct {
	// Fields used by all Events:

	// A unique ID for this event, stable within a sched collection.  Can
	// be used to associate Events gathered by different requests to the same
	// collection they are the same if they have the same unique_id.  E.g., we
	// could associate an event on a timeline with an event from a stall if their
	// unique_ids match.
	UniqueID  int64     `json:"uniqueID "`
	EventType EventType `json:"eventType"`
	// The timestamp, in nanoseconds, at which the event occurs.  Events are
	// instantaneous (or at least modeled as such).
	TimestampNs int64 `json:"timestampNs"`
	// The primary PID of this event.  For MIGRATE_TASK, PROCESS_WAIT, WAIT_TASK,
	// WAKEUP, and WAKEUP_NEW, it is the affected PID.  For SWITCH, it is the PID
	// active after the switch, 'next_pid'.
	Pid int64 `json:"pid"`
	// The command associated with pid, if any.  Note that this field is likely
	// truncated by the target OS.
	Command string `json:"command"`
	// The primary CPU of the event.  For MIGRATE_TASK, it is the CPU on which the
	// task will be active after the migration, 'dest_cpu'.  For WAKEUP or
	// WAKEUP_NEW, it is the CPU on which the wakeup will occur, 'target_cpu'.
	// The remaining events PROCESS_WAIT, WAIT_TASK, and SWITCH, do not
	// explicitly record what CPU the event is occurring on; however, this CPU can
	// frequently be inferred from other events on the affected process:
	//  * The 'target_cpu' of WAKEUP or WAKEUP_NEW events, or the 'dest_cpu' of
	//    MIGRATE events, fixes the CPU of the affected process for subsequent
	//    events.
	//  * The 'orig_cpu' of MIGRATE_TASK events fixes the CPU of the affected
	//    process for previous events, if it is not known by other sources.
	//  * If the CPU for one process in a SWITCH is known, but the CPU for the
	//    other process is not known, the first process' CPU fixes the CPU of the
	//    other process for previous events.
	// If the CPU cannot be inferred, an empty Int64Value is returned.
	CPU int64 `json:"cpu"`
	// The priority of the primary PID of this event.  May be inferred; unknown if
	// empty.
	Priority int64 `json:"priority"`
	// The CPU that reported the event.
	ReportingCPU int64 `json:"reportingCpu"`
	// The state of the thread referenced by pid just after this event completed.
	State sched.ThreadState `json:"state"`
	// Fields only used for SWITCH events:

	// The PID active prior to the switch
	PrevPid int64 `json:"prevPid"`
	// The command associated with prev_pid, if any.  Note that this field is
	// likely truncated by the target OS.
	// BEGIn-INTERNAL
	// 'prev_comm' from ktrace.
	// END-INTERNAL
	PrevCommand string `json:"prevCommand"`
	// The priority of prev_pid.  Unused for other events.
	PrevPriority int64 `json:"prevPriority"`
	// The state of prev_pid immediately after the SWITCH.  These values are from
	// the kernel's task state bitmap, for example at
	// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/include/linux/sched.h?h=v4.3.5#n197
	// We assume that, at least, state == 0 iff the task is in the scheduler run queue.
	PrevTaskState int64 `json:"prevTaskState"`
	// The state of the prev_pid.
	PrevState sched.ThreadState
	// Fields only used for MIGRATE_TASK events:

	// The CPU from which the process is migrating
	PrevCPU int64 `json:"prevCpu"`
}

// PerThreadEventSeriesRequest is a request for all events on the specified threads across a
// specified timestamp for a specified collection.  If start_timestamp_ns is -1, the first
// timestamp in the collection is used instead.  If end_timestamp is -1, the
// last timestamp in the collection is used instead.
type PerThreadEventSeriesRequest struct {
	// The collection name.
	CollectionName   string  `json:"collectionName"`
	Pids             []int64 `json:"pids"`
	StartTimestampNs int64   `json:"startTimestampNs"`
	EndTimestampNs   int64   `json:"endTimestampNs"`
}

// PerThreadEventSeries is a tuple containing a PID and its events.
type PerThreadEventSeries struct {
	Pid    int64   `json:"pid"`
	Events []Event `json:"events"`
}

// PerThreadEventSeriesResponse is a response for a per-thread event sequence request.
// The Events are unique and are provided in increasing temporal order.
type PerThreadEventSeriesResponse struct {
	// The PCC collection name.
	CollectionName string                 `json:"collectionName"`
	EventSeries    []PerThreadEventSeries `json:"eventSeries"`
}

// UtilizationMetricsRequest is a request for the amount of time, in the specified collection over
// the specified interval and CPU set, that some of the CPUs were idle while others were overloaded.
type UtilizationMetricsRequest struct {
	CollectionName   string  `json:"collectionName"`
	Cpus             []int64 `json:"cpus"`
	StartTimestampNs int64   `json:"startTimestampNs"`
	EndTimestampNs   int64   `json:"endTimestampNs"`
}

// UtilizationMetrics contains various stats relating to the utilization of CPUs.
type UtilizationMetrics struct {
	// WallTime is the time during which at least one CPU was idle while at least one
	// other CPU was overloaded.
	WallTime int64 `json:"wallTime"`
	// PerCPUTime is the aggregated time that a single CPU was idle while another CPU was
	// overloaded.  For example, if two CPUs were idle for 1s, and two other CPUs
	// overloaded during that same 1s, that's 1s of wall time but 2s of per-CPU
	// time.
	PerCPUTime int64 `json:"perCpuTime"`
	// PerThreadTime is the aggregated time that a single CPU was idle while another thread waited
	// on some other, overloaded CPU.
	// For example, if two CPUs were overloaded for one second, one with one
	// waiting thread and the other with two waiting threads, and four other CPUs
	// were idle for that same second, the Wall Time for that interval would be
	// one second (At least one CPU was idle while another was overloaded for the
	// entire second); the Per-CPU Time would be two seconds (two CPUs were
	// overloaded while at least two others were idle); and the Per-Thread Time
	// would be three seconds (three threads were waiting while at least three
	// CPUs were idle.) If, however, only two CPUs were idle during that second,
	// Per-CPU Time would remain the same while Per-Thread Time would only be two
	// seconds, because while three threads were waiting over that second, only
	// two CPUs were idle.
	PerThreadTime int64 `json:"perThreadTime"`
	// CPUUtilizationFraction is the fraction over the requested interval and set of CPUs.
	// CPU utilization is the proportion (in the range [0,1]) of total CPU-time spent
	// not idle.  For example, a UtilizationMetricsRequest for .5s over 4 CPUs
	// would return a CPU utilization of .5 if two of those CPUs lay idle for .5s
	// each; .75 if two of those CPUs lay idle for .25s each, or one was idle for
	// .5s; and so forth.
	CPUUtilizationFraction float64 `json:"cpuUtilizationFraction"`
}

// UtilizationMetricsResponse is a response for an idle-while-overloaded request.
type UtilizationMetricsResponse struct {
	Request            UtilizationMetricsRequest `json:"request"`
	UtilizationMetrics UtilizationMetrics        `json:"utilizationMetrics"`
}

