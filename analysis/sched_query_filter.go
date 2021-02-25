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
	// The thread states to be included.  Defaults to AnyState.
	threadStates ThreadState
}

// Filter specifies a filter to a sched collection query.  Filters can limit
// the data processed to respond to a query, for instance by limiting the
// time range, CPUs, or threads over which the query applies.  They can also
// specify how the query analysis is performed, for instance by requesting
// intervals be aggregated up to a specific duration.  Not all queries observe
// all filters; each query function should document what filters it honors
// and how it uses them.
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

// EventTypes filters to the specified event types, overriding any previous
// event type filtering.
func EventTypes(eventTypes ...string) func(*filter) {
	return func(f *filter) {
		f.eventTypes = map[string]struct{}{}
		for _, eventType := range eventTypes {
			f.eventTypes[eventType] = struct{}{}
		}
	}
}

// CPUs filters to the specified CPUs, overriding any previous CPU filtering.
func CPUs(cpus ...CPUID) func(*filter) {
	return func(f *filter) {
		f.cpus = map[CPUID]struct{}{}
		for _, cpu := range cpus {
			f.cpus[cpu] = struct{}{}
		}
	}
}

// PIDs filters to the specified PIDs, overriding any previous PID filtering.
func PIDs(pids ...PID) func(*filter) {
	return func(f *filter) {
		f.pids = map[PID]struct{}{}
		for _, pid := range pids {
			f.pids[pid] = struct{}{}
		}
	}
}

// ThreadStates filters to the specified ThreadStates, overriding any previous
// thread state filtering.  Multiple ThreadStates may be specified by joining
// with bitwise OR.
func ThreadStates(threadStates ThreadState) func(*filter) {
	return func(f *filter) {
		f.threadStates = threadStates
	}
}

// duplicateFilter duplicates the provided filter.
func duplicateFilter(inF *filter) func(*filter) {
	return func(outF *filter) {
		outF.truncateToTimeRange = inF.truncateToTimeRange
		outF.minIntervalDuration = inF.minIntervalDuration
		outF.startTimestamp = inF.startTimestamp
		outF.endTimestamp = inF.endTimestamp
		outF.eventTypes = map[string]struct{}{}
		for et := range inF.eventTypes {
			outF.eventTypes[et] = struct{}{}
		}
		outF.cpus = map[CPUID]struct{}{}
		for cpuid := range inF.cpus {
			outF.cpus[cpuid] = struct{}{}
		}
		outF.pids = map[PID]struct{}{}
		for pid := range inF.pids {
			outF.pids[pid] = struct{}{}
		}
		outF.threadStates = inF.threadStates
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
		threadStates:        RunningState | WaitingState | SleepingState | UnknownState,
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
//  * s's ThreadState must be among f's filtered-in states.
func (f *filter) spanFilteredIn(span *threadSpan) bool {
	_, inCPUs := f.cpus[span.cpu]
	_, inPIDs := f.pids[span.pid]
	return span.endTimestamp >= f.startTimestamp &&
		span.startTimestamp <= f.endTimestamp &&
		inCPUs && inPIDs && ((span.state & f.threadStates) == span.state)
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

func (f *filter) threadStateFilteredIn(threadState ThreadState) bool {
	return (threadState & f.threadStates) == threadState
}
