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

// CPUIntervalsRequest is a request for CPU intervals for the specified collection.
type CPUIntervalsRequest struct {
	CollectionName string `json:"collectionName"`
	// The CPUs to request intervals for.  If empty, all CPUs are selected.
	CPUs []int64 `json:"cpus"`
	// Designates a minimum interval duration.  Adjacent intervals smaller than
	// this duration may be merged together, retaining waiting PID count data but
	// possibly losing running thread data; merged intervals are truncated as soon
	// as they meet or exceed the specified duration.  Intervals smaller than this
	// may still appear in the output, if they could not be merged with neighbors.
	// If 0, no merging is performed.
	MinIntervalDurationNs int64 `json:"minIntervalDurationNs"`
	// The time span over which to request CPU intervals, specified in
	// nanoseconds.  If start_timestamp_ns is -1, the time span will
	// begin at the first valid collection timestamp.  If end_timestamp_ns is -1,
	// the time span will end at the last valid collection timestamp.
	StartTimestampNs int64 `json:"startTimestampNs"`
	EndTimestampNs   int64 `json:"endTimestampNs"`
}

// CPUInterval contains information about a what was running and waiting on a CPU during an interval
type CPUInterval struct {
	CPU int64 `json:"cpu"`
	// If running_pid is not populated, the running thread is unknown or not
	// provided, and the running_command and running_priority are not meaningful.
	RunningPid      int64  `json:"runningPid"`
	RunningCommand  string `json:"runningCommand"`
	RunningPriority int64  `json:"runningPriority"`
	// The set of PIDs that waited during this interval.  If intervals were split
	// on a change in waiting PIDs, this is the set of PIDs that waited throughout
	// the entire interval; otherwise it's the set of PIDs that waited for any
	// time during this interval.
	WaitingPids []int64 `json:"waitingPids"`
	// The average number of known waiting PIDs.
	WaitingPidCount  float32 `json:"waitingPidCount"`
	StartTimestampNs int64   `json:"startTimestampNs"`
	EndTimestampNs   int64   `json:"endTimestampNs"`
	// How many CPUIntervals were merged to form this one.
	MergedIntervalCount int32 `json:"mergedIntervalCount"`
	// The amount of time, over this interval, that the CPU was idle.  A CPU is
	// considered idle when it is running PID 0, the swapper.  If this interval is
	// not merged, idle_ns will either be its entire duration (if the running PID
	// was 0) or 0 (if the running PID was not 0).
	IdleNs int64 `json:"idleNs"`
	// TODO(ilhamster) Consider whether we should also include switch-out state
	// (sleeping or round-robin) of the running PID here.  Note that this can be
	// gathered from PID intervals, though.
}

// CPUIntervals is a tuple holding a CPU ID and its intervals
type CPUIntervals struct {
	CPU       int64         `json:"cpu"`
	Intervals []CPUInterval `json:"intervals"`
}

// CPUIntervalsResponse is a response for a CPU intervals request.  If no matching collection
// was found, cpu_intervals is empty.
type CPUIntervalsResponse struct {
	CollectionName string         `json:"collectionName"`
	CPUIntervals   []CPUIntervals `json:"cpuIntervals"`
}

// FtraceEventsRequest is a request for a view of ftrace events in the collection.
type FtraceEventsRequest struct {
	// The name of the collection.
	CollectionName string `json:"collectionName"`
	// The CPUs to request intervals for.  If empty, all CPUs are selected.
	Cpus []int64 `json:"cpus"`
	// The time span (in nanoseconds) over which to request ftrace events.
	StartTimestamp int64 `json:"startTimestamp"`
	EndTimestamp   int64 `json:"endTimestamp"`
	// The event type names to fetch.  If empty, no events are returned.
	EventTypes []string `json:"eventTypes"`
}

// FtraceEventsResponse is a response for a ftrace events request.
type FtraceEventsResponse struct {
	// The name of the collection.
	CollectionName string `json:"collectionName"`
	// A map from CPU to lists of events that occurred on that CPU.
	EventsByCPU map[int64][]FtraceEvent `json:"eventsByCpu"`
}

// FtraceEvent describes a single trace event.
// A Collection stores its constituent events in a much more compact, but less
// usable, format than this, so it is recommended to generate Events on demand
// (via Collection::EventByIndex) rather than persisting more than a few Events.
type FtraceEvent struct {
	// An index uniquely identifying this Event within its Collection.
	Index int64 `json:"index"`
	// The name of the event's type.
	Name string `json:"name"`
	// The CPU that logged the event.  Note that the CPU that logs an event may be
	// otherwise unrelated to the event.
	CPU int64 `json:"cpu"`
	// The event timestamp.
	Timestamp int64 `json:"timestamp"`
	// True if this Event fell outside of the known-valid range of a trace which
	// experienced buffer overruns.  Some kinds of traces are only valid for
	// unclipped events.
	Clipped bool `json:"clipped"`
	// A map of text properties, indexed by name.
	TextProperties map[string]string `json:"textProperties"`
	// A map of numeric properties, indexed by name.
	NumberProperties map[string]int64 `json:"numberProperties"`
}

// PidIntervalsRequest is a request for PID intervals for the specified collection and PIDs.
type PidIntervalsRequest struct {
	// The name of the collection to look up intervals in
	CollectionName string `json:"collectionName"`
	// The PIDs to request intervals for
	Pids []int64 `json:"pids"`
	// The time span over which to request PID intervals, specified in
	// nanoseconds.  If start_timestamp_ns is -1, the time span will
	// begin at the first valid collection timestamp.  If end_timestamp_ns is -1,
	// the time span will end at the last valid collection timestamp.
	StartTimestampNs int64 `json:"startTimestampNs"`
	EndTimestampNs   int64 `json:"endTimestampNs"`
	// Designates a minimum interval duration.  Adjacent intervals on the same CPU
	// smaller than this duration may be merged together, losing state and
	// post-wakeup status; merged intervals are truncated as soon as they meet or
	// exceed the specified duration.  Intervals smaller than this may still
	// appear in the output, if they could not be merged with neighbors.  If 0, no
	// merging is performed.
	MinIntervalDurationNs int64 `json:"minIntervalDurationNs"`
}

// ThreadState is an enum describing the state of a thread
type ThreadState = int64

const (
	// ThreadStateUnknownState means unknown thread state
	ThreadStateUnknownState ThreadState = iota
	// ThreadStateRunningState means Scheduled and switched-in on a CPU.
	ThreadStateRunningState
	// ThreadStateWaitingState means Not running but runnable.
	ThreadStateWaitingState
	// ThreadStateSleepingState means neither running nor on the run queue.
	ThreadStateSleepingState
)

// PIDInterval describes a maximal interval over a PID's lifetime during which
// its command, priority, state, and CPU remain unchanged.
type PIDInterval struct {
	Pid      int64  `json:"pid"`
	Command  string `json:"command"`
	Priority int64  `json:"priority"`
	// If this PIDInterval is the result of merging several intervals, state will
	// be set to UNKNOWN.  This can be distinguished from actually unknown state
	// by checking merged_interval_count; if it is == 1, the thread's state is
	// actually unknown over the interval; if it is > 1, the thread had several
	// states over the merged interval.
	State ThreadState `json:"state"`
	// If state is WAITING, post_wakeup determines if the thread started waiting
	// as the result of a wakeup (true) or as a result of round-robin descheduling
	// (false).  post_wakeup is always false for merged intervals.
	PostWakeup       bool  `json:"postWakeup"`
	CPU              int64 `json:"cpu"`
	StartTimestampNs int64 `json:"startTimestampNs"`
	EndTimestampNs   int64 `json:"endTimestampNs"`
	// How many PIDIntervals were merged to form this one.
	MergedIntervalCount int64 `json:"mergedIntervalCount"`
}

// PIDIntervals is a tuple holding a PID and its intervals
type PIDIntervals struct {
	// The PID that these intervals correspond to
	PID int64 `json:"pid"`
	// A list of PID intervals
	Intervals []PIDInterval `json:"intervals"`
}

// PIDntervalsResponse is a response for a PID intervals request. If no matching collection was
// found, pid_intervals is empty.
type PIDntervalsResponse struct {
	// The name of the collection
	CollectionName string `json:"collectionName"`
	// A list of PID intervals
	PIDIntervals []PIDIntervals `json:"pidIntervals"`
}
