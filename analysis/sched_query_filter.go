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
package sched

import "github.com/google/schedviz/tracedata/trace"

type filter struct {
	// If true, intervals that overlap the start or end timestamps will be
	// truncated so that they do not overlap the requested range.
	truncateToTimeRange bool
	// The target minimum interval duration.  If >0, wherever possible, adjacent
	// intervals will be merged
	minIntervalDuration Duration
	// If Unknown, the start of the trace.
	startTimestamp trace.Timestamp
	// If Unknown, the end of the trace.
	endTimestamp trace.Timestamp
	// If empty, all event types
	eventTypes map[string]struct{}
	// If empty, all CPUs.
	cpus map[CPUID]struct{}
	// if empty, all PIDs.
	pids map[PID]struct{}
}

// Filter specifies a filter to a sched collection query.
type Filter func(*filter)

// TruncateToTimeRange sets whether intervals will be allowed to overlap
// the start or end timestamp of the filter.
func TruncateToTimeRange(truncateToTimeRange bool) func(*filter) {
	return func(f *filter) {
		f.truncateToTimeRange = truncateToTimeRange
	}
}

// MinIntervalDuration sets the minimum duration that intervals will, wherever
// possible, be merged up to.
func MinIntervalDuration(minIntervalDuration Duration) func(*filter) {
	return func(f *filter) {
		f.minIntervalDuration = minIntervalDuration
	}
}

// StartTimestamp sets the inclusive start of the filtered-in time-range.
func StartTimestamp(startTimestamp trace.Timestamp) func(*filter) {
	return func(f *filter) {
		f.startTimestamp = startTimestamp
	}
}

// EndTimestamp sets the inclusive start of the filtered-in time-range.
func EndTimestamp(endTimestamp trace.Timestamp) func(*filter) {
	return func(f *filter) {
		f.endTimestamp = endTimestamp
	}
}

// TimeRange filters to the specified time-range, inclusive.
func TimeRange(startTimestamp, endTimestamp trace.Timestamp) func(*filter) {
	return func(f *filter) {
		f.startTimestamp, f.endTimestamp = startTimestamp, endTimestamp
	}
}

// EventTypes filters to the specified event types.
func EventTypes(eventTypes ...string) func(*filter) {
	return func(f *filter) {
		for _, eventType := range eventTypes {
			f.eventTypes[eventType] = struct{}{}
		}
	}
}

// CPUs filters to the specified CPUs.
func CPUs(cpus ...CPUID) func(*filter) {
	return func(f *filter) {
		for _, cpu := range cpus {
			f.cpus[cpu] = struct{}{}
		}
	}
}

// PIDs filters to the specified PIDs.
func PIDs(pids ...PID) func(*filter) {
	return func(f *filter) {
		for _, pid := range pids {
			f.pids[pid] = struct{}{}
		}
	}
}

func buildFilter(c *Collection, filtFuncs []Filter) *filter {
	f := &filter{
		truncateToTimeRange: false,
		minIntervalDuration: 0,
		startTimestamp:      UnknownTimestamp,
		endTimestamp:        UnknownTimestamp,
		eventTypes:          map[string]struct{}{},
		cpus:                map[CPUID]struct{}{},
		pids:                map[PID]struct{}{},
	}
	for _, ff := range filtFuncs {
		ff(f)
	}
	// Populate unspecified values from the collection's.
	if f.startTimestamp == UnknownTimestamp {
		f.startTimestamp = c.startTimestamp
	}
	if f.endTimestamp == UnknownTimestamp {
		f.endTimestamp = c.endTimestamp
	}
	if len(f.cpus) == 0 {
		f.cpus = c.cpus
	} else {
		for cpu := range f.cpus {
			if _, ok := c.cpus[cpu]; !ok {
				delete(f.cpus, cpu)
			}
		}
	}
	if len(f.pids) == 0 {
		f.pids = c.pids
	} else {
		for pid := range f.pids {
			if _, ok := c.pids[pid]; !ok {
				delete(f.pids, pid)
			}
		}
	}
	return f
}

// spanFilteredIn returns true if the receiver filters in the provided span.
// To filter in a span s with filter f,
//  * s's time range must overlap f's time range,
//  * s's PID must be present in f's pid set,
//  * s's CPU must be present in f's cpu set.
func (f *filter) spanFilteredIn(span *threadSpan) bool {
	_, inCPUs := f.cpus[span.cpu]
	_, inPIDs := f.pids[span.pid]
	return span.endTimestamp >= f.startTimestamp &&
		span.startTimestamp <= f.endTimestamp &&
		inCPUs && inPIDs
}

func (f *filter) maxCPUID() CPUID {
	var max CPUID
	for cpu := range f.cpus {
		if cpu > max {
			max = cpu
		}
	}
	return max
}
