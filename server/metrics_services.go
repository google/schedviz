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

// ThreadSummariesRequest is a request for thread summary information across a specified timespan
// for a specified collection, filtered to the requested CPU set. If start_timestamp_ns is -1, the
// first timestamp in the collection is used  instead. If end_timestamp_ns is -1, the last timestamp
// in the collection is used instead. If the provided CPU set is empty, all CPUs are filtered in.
type ThreadSummariesRequest struct {
	CollectionName   string          `json:"collectionName"`
	StartTimestampNs trace.Timestamp `json:"startTimestampNs"`
	EndTimestampNs   trace.Timestamp `json:"endTimestampNs"`
	Cpus             []sched.CPUID   `json:"cpus"`
}

// ThreadSummariesResponse contains the response to a ThreadSummariesRequest.
type ThreadSummariesResponse struct {
	CollectionName string           `json:"collectionName"`
	Metrics        []*sched.Metrics `json:"metrics"`
}

// AntagonistsRequest is a request for antagonist information for a specified set of threads, across
// a specified timestamp for a specified collection.  If start_timestamp_ns is -1,
// the first timestamp in the collection is used instead.  If end_timestamp_ns
// is -1, the last timestamp in the collection is used instead.
type AntagonistsRequest struct {
	// The collection name.
	CollectionName   string          `json:"collectionName"`
	Pids             []sched.PID     `json:"pids"`
	StartTimestampNs trace.Timestamp `json:"startTimestampNs"`
	EndTimestampNs   trace.Timestamp `json:"endTimestampNs"`
}

// AntagonistsResponse is a response for an antagonist request.
type AntagonistsResponse struct {
	CollectionName string `json:"collectionName"`
	// All matching stalls sorted in order of decreasing duration - longest first.
	Antagonists []*sched.Antagonists `json:"antagonists"`
}

// PerThreadEventSeriesRequest is a request for all events on the specified threads across a
// specified timestamp for a specified collection.  If start_timestamp_ns is -1, the first
// timestamp in the collection is used instead.  If end_timestamp is -1, the
// last timestamp in the collection is used instead.
type PerThreadEventSeriesRequest struct {
	// The collection name.
	CollectionName   string          `json:"collectionName"`
	Pids             []sched.PID     `json:"pids"`
	StartTimestampNs trace.Timestamp `json:"startTimestampNs"`
	EndTimestampNs   trace.Timestamp `json:"endTimestampNs"`
}

// PerThreadEventSeries is a tuple containing a PID and its events.
type PerThreadEventSeries struct {
	Pid    sched.PID      `json:"pid"`
	Events []*trace.Event `json:"events"`
}

// PerThreadEventSeriesResponse is a response for a per-thread event sequence request.
// The Events are unique and are provided in increasing temporal order.
type PerThreadEventSeriesResponse struct {
	// The PCC collection name.
	CollectionName string                  `json:"collectionName"`
	EventSeries    []*PerThreadEventSeries `json:"eventSeries"`
}

// UtilizationMetricsRequest is a request for the amount of time, in the specified collection over
// the specified interval and CPU set, that some of the CPUs were idle while others were overloaded.
type UtilizationMetricsRequest struct {
	CollectionName   string          `json:"collectionName"`
	Cpus             []sched.CPUID   `json:"cpus"`
	StartTimestampNs trace.Timestamp `json:"startTimestampNs"`
	EndTimestampNs   trace.Timestamp `json:"endTimestampNs"`
}

// UtilizationMetricsResponse is a response for an idle-while-overloaded request.
type UtilizationMetricsResponse struct {
	Request            *UtilizationMetricsRequest `json:"request"`
	UtilizationMetrics *sched.Utilization         `json:"utilizationMetrics"`
}

