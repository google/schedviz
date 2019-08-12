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

import (
	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/tracedata/trace"
)

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

// CPUIntervals is a tuple holding a CPU ID and its running and waiting intervals
type CPUIntervals struct {
	CPU     int64             `json:"cpu"`
	Running []*sched.Interval `json:"running"`
	Waiting []*sched.Interval `json:"waiting"`
}

// CPUIntervalsResponse is a response for a CPU intervals request.  If no matching collection
// was found, cpu_intervals is empty.
type CPUIntervalsResponse struct {
	CollectionName string         `json:"collectionName"`
	Intervals      []CPUIntervals `json:"intervals"`
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
	EventsByCPU map[sched.CPUID][]*trace.Event `json:"eventsByCpu"`
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

// PIDIntervals is a tuple holding a PID and its intervals
type PIDIntervals struct {
	// The PID that these intervals correspond to
	PID int64 `json:"pid"`
	// A list of PID intervals
	Intervals []*sched.Interval `json:"intervals"`
}

// PIDntervalsResponse is a response for a PID intervals request. If no matching collection was
// found, pid_intervals is empty.
type PIDntervalsResponse struct {
	// The name of the collection
	CollectionName string `json:"collectionName"`
	// A list of PID intervals
	PIDIntervals []PIDIntervals `json:"pidIntervals"`
}
