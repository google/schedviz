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

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/schedviz/tracedata/eventsetbuilder"
	"github.com/google/schedviz/tracedata/testeventsetbuilder"
	"github.com/google/schedviz/tracedata/trace"
)

const (
	intervalStartLabel = "interval_start"
	intervalEndLabel   = "interval_end"
	intervalIDLabel    = "interval_id"
)

// This test demonstrates that, and how, this system can be used to return
// intervals formed by two or more events.

type interval struct {
	CPU            CPUID
	StartTimestamp trace.Timestamp
	EndTimestamp   trace.Timestamp
	ID             int64
}

// findIntervals finds all complete intervals on the requested CPUs and the
// requested time range, given the provided Collections of intervalStart and
// intervalEnd events.  If truncate is true, intervals with one endpoint within
// the requested range whose other endpoint spans an edge of the requested time
// range are included and truncated at that edge; otherwise, only intervals
// fully contained in the requested timespan are returned.
// Matching intervalStart and intervalEnd events are identified by their
// intervalIDLabel fields, and interval IDs are assumed to be unique to, at
// most, one intervalStart and one intervalEnd event within a trace.  An error
// is returned if an interval ends before it starts, if an interval ID is
// reused, or if an interval starts and ends on different CPUs.  Returned
// intervals are grouped by CPU, in the order provided, and then ordered by
// startTimestamp increasing.
func findIntervals(cpus []CPUID, startColl, endColl *PerCPUCollection, startTimestamp, endTimestamp trace.Timestamp, truncate bool) ([]*interval, error) {
	var intervals = []*interval{}
	seenIntervalIDs := map[int64]bool{}
	// Place the event fetched from the specified collection by the specified
	// index into the provided events-by-interval-ID map.
	setEvent := func(coll *PerCPUCollection, index int, eventsByIntervalID map[int64]*trace.Event) error {
		event, err := coll.Event(index)
		if err != nil {
			return err
		}
		intervalID, ok := event.NumberProperties[intervalIDLabel]
		if !ok {
			return status.Errorf(codes.InvalidArgument, "event %d lacks interval ID", index)
		}
		if _, ok := eventsByIntervalID[intervalID]; ok {
			return status.Errorf(codes.InvalidArgument, "interval ID %d is reused", intervalID)
		}
		eventsByIntervalID[intervalID] = event
		seenIntervalIDs[intervalID] = true
		return nil
	}
	for _, cpu := range cpus {
		var cpuIvals = []*interval{}
		// Build mappings of intervals by ID to their start and end events.
		startEventsByIntervalID := map[int64]*trace.Event{}
		for _, index := range startColl.EventIndices(CPUs(cpu), StartTimestamp(startTimestamp), EndTimestamp(endTimestamp)) {
			if err := setEvent(startColl, index, startEventsByIntervalID); err != nil {
				return nil, err
			}
		}
		endEventsByIntervalID := map[int64]*trace.Event{}
		for _, index := range endColl.EventIndices(CPUs(cpu), StartTimestamp(startTimestamp), EndTimestamp(endTimestamp)) {
			if err := setEvent(endColl, index, endEventsByIntervalID); err != nil {
				return nil, err
			}
		}
		// Match starts and ends.
		for intervalID, startEvent := range startEventsByIntervalID {
			endEvent, ok := endEventsByIntervalID[intervalID]
			if !ok {
				continue
			}
			if endEvent.Timestamp < startEvent.Timestamp {
				return nil, status.Errorf(codes.InvalidArgument, "interval ID %d ends before it begins", intervalID)
			}
			cpuIvals = append(cpuIvals, &interval{
				CPU:            cpu,
				StartTimestamp: startEvent.Timestamp,
				EndTimestamp:   endEvent.Timestamp,
				ID:             intervalID,
			})
			delete(startEventsByIntervalID, intervalID)
			delete(endEventsByIntervalID, intervalID)
		}
		// If truncation was requested, run through orphan starts and ends.
		if truncate {
			for intervalID, startEvent := range startEventsByIntervalID {
				cpuIvals = append(cpuIvals, &interval{
					CPU:            cpu,
					StartTimestamp: startEvent.Timestamp,
					EndTimestamp:   endTimestamp,
					ID:             intervalID,
				})
			}
			for intervalID, endEvent := range endEventsByIntervalID {
				cpuIvals = append(cpuIvals, &interval{
					CPU:            cpu,
					StartTimestamp: startTimestamp,
					EndTimestamp:   endEvent.Timestamp,
					ID:             intervalID,
				})
			}
		}
		sort.Slice(cpuIvals, func(a, b int) bool {
			return cpuIvals[a].StartTimestamp < cpuIvals[b].StartTimestamp
		})
		intervals = append(intervals, cpuIvals...)
	}
	return intervals, nil
}

// typeFilteringCPULookupFunc returns an UnclippedReportinCPU function that
// drops all events except for the specified type.
func typeFilteringCPULookupFunc(eventType string) CPULookupFunc {
	return func(ev *trace.Event) ([]CPUID, error) {
		if ev.Name == eventType {
			return UnclippedReportingCPU(ev)
		}
		return nil, nil
	}
}

func perCPUEventsIntervalsCollection(t *testing.T, normalizeTimestamps bool) *Collection {
	t.Helper()
	evtLoaders := map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{
		intervalStartLabel: func(_ *trace.Event, _ *ThreadTransitionSetBuilder) error { return nil },
		intervalEndLabel:   func(_ *trace.Event, _ *ThreadTransitionSetBuilder) error { return nil },
	}

	// An EventSet with two CPUs and two intervals on each:
	//            0 1 2 3 4 5 6 7
	//            0 0 0 0 0 0 0 0
	//           |---------------|
	// CPU 1:
	//   ival 1:  *********
	//   ival 2:      *********
	// CPU 2:
	//   ival 3:    *************
	//   ival 4:        *****
	es := testeventsetbuilder.TestProtobuf(t,
		eventsetbuilder.NewBuilder().
			WithEventDescriptor(intervalStartLabel, eventsetbuilder.Number(intervalIDLabel)).
			WithEventDescriptor(intervalEndLabel, eventsetbuilder.Number(intervalIDLabel)).
			WithEvent(intervalStartLabel, 1, 1000, false, 1).
			WithEvent(intervalStartLabel, 2, 1010, false, 3).
			WithEvent(intervalStartLabel, 1, 1020, false, 2).
			WithEvent(intervalStartLabel, 2, 1030, false, 4).
			WithEvent(intervalEndLabel, 1, 1040, false, 1).
			WithEvent(intervalEndLabel, 2, 1050, false, 4).
			WithEvent(intervalEndLabel, 1, 1060, false, 2).
			WithEvent(intervalEndLabel, 2, 1070, false, 3))

	coll, err := NewCollection(es, evtLoaders, NormalizeTimestamps(normalizeTimestamps))
	if err != nil {
		t.Fatalf("NewCollection yielded unexpected error %s", err)
	}
	return coll
}

func TestIntervalFormation(t *testing.T) {

	tests := []struct {
		description    string
		collection     *Collection
		cpus           []CPUID
		startTimestamp trace.Timestamp
		endTimestamp   trace.Timestamp
		truncate       bool
		wantIntervals  []*interval
	}{{
		description:    "default intervals, full duration",
		collection:     perCPUEventsIntervalsCollection(t, false /*normalizeTimestamps*/),
		cpus:           []CPUID{1, 2},
		startTimestamp: 1000,
		endTimestamp:   1070,
		wantIntervals: []*interval{
			{1, 1000, 1040, 1},
			{1, 1020, 1060, 2},
			{2, 1010, 1070, 3},
			{2, 1030, 1050, 4},
		},
	}, {
		description:    "default intervals, partial duration, truncating, missing long intervals",
		collection:     perCPUEventsIntervalsCollection(t, false /*normalizeTimestamps*/),
		cpus:           []CPUID{1, 2},
		startTimestamp: 1030,
		endTimestamp:   1040,
		truncate:       true,
		wantIntervals: []*interval{
			{1, 1030, 1040, 1}, // truncated at start
			{2, 1030, 1040, 4}, // truncated at end
		},
	}, {
		description:    "default intervals, partial duration, not truncating",
		collection:     perCPUEventsIntervalsCollection(t, false /*normalizeTimestamps*/),
		cpus:           []CPUID{1, 2},
		startTimestamp: 1000,
		endTimestamp:   1050,
		wantIntervals: []*interval{
			{1, 1000, 1040, 1},
			{2, 1030, 1050, 4},
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			startColl, err := NewPerCPUCollection(test.collection, typeFilteringCPULookupFunc(intervalStartLabel))
			if err != nil {
				t.Fatalf("Failed to create start intervals: %s", err)
			}
			endColl, err := NewPerCPUCollection(test.collection, typeFilteringCPULookupFunc(intervalEndLabel))
			if err != nil {
				t.Fatalf("Failed to create end intervals: %s", err)
			}
			gotIntervals, err := findIntervals(test.cpus, startColl, endColl, test.startTimestamp, test.endTimestamp, test.truncate)
			if err != nil {
				t.Fatalf("findIntervals yielded unexpected error %s", err)
			}
			sort.Slice(gotIntervals, func(a, b int) bool {
				return gotIntervals[a].ID < gotIntervals[b].ID
			})
			if diff := cmp.Diff(test.wantIntervals, gotIntervals); diff != "" {
				t.Errorf("findIntervals() = %#v, diff (want->got) %s", gotIntervals, diff)
			}
		})
	}
}
