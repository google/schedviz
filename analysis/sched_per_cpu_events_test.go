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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/schedviz/analysis/schedtestcommon"
	"github.com/google/schedviz/tracedata/eventsetbuilder"
	"github.com/google/schedviz/tracedata/testeventsetbuilder"
	"github.com/google/schedviz/tracedata/trace"
)

const (
	simpleNumericEventLabel = "simple_numeric_event"
	simpleTextualEventLabel = "simple_textual_event"
	indirectEventLabel      = "indirect_event"
)

func perCPUEventsCollection(t *testing.T, normalizeTimestamps bool) *Collection {
	evtLoaders := map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{
		simpleNumericEventLabel: func(_ *trace.Event, _ *ThreadTransitionSetBuilder) error { return nil },
		simpleTextualEventLabel: func(_ *trace.Event, _ *ThreadTransitionSetBuilder) error { return nil },
		indirectEventLabel:      func(_ *trace.Event, _ *ThreadTransitionSetBuilder) error { return nil },
	}

	es := testeventsetbuilder.TestProtobuf(t,
		eventsetbuilder.NewBuilder().
			WithEventDescriptor(
				simpleNumericEventLabel,
				eventsetbuilder.Number("value")).
			WithEventDescriptor(
				simpleTextualEventLabel,
				eventsetbuilder.Text("value")).
			WithEventDescriptor(
				indirectEventLabel,
				eventsetbuilder.Number("cpu")).
			WithEvent(indirectEventLabel, 1, 900, true, 2).
			WithEvent(simpleNumericEventLabel, 1, 1000, false, 1).
			WithEvent(simpleTextualEventLabel, 1, 1005, false, "a").
			WithEvent(simpleNumericEventLabel, 2, 1010, false, 2).
			WithEvent(simpleTextualEventLabel, 2, 1015, false, "b").
			WithEvent(indirectEventLabel, 1, 1018, false, 2).
			WithEvent(simpleNumericEventLabel, 1, 1020, false, 3).
			WithEvent(simpleTextualEventLabel, 1, 1025, false, "c").
			WithEvent(simpleNumericEventLabel, 2, 1030, false, 4).
			WithEvent(simpleTextualEventLabel, 2, 1035, false, "d").
			WithEvent(indirectEventLabel, 2, 1038, false, 1))

	coll, err := NewCollection(es, UsingEventLoaders(evtLoaders), NormalizeTimestamps(normalizeTimestamps))
	if err != nil {
		t.Fatalf("NewCollection yielded unexpected error %s", err)
	}
	return coll
}

// eventLimiter returns a CPULookupFunc that passes through
// UnclippedReportingCPU for the first n events it handles, and returns an
// empty CPU list for all subsequent events it is passed.
func eventLimiter(n int) CPULookupFunc {
	lim := 0
	return func(ev *trace.Event) ([]CPUID, error) {
		cpus, err := UnclippedReportingCPU(ev)
		if err == nil && len(cpus) > 0 {
			lim++
			if lim > n {
				return nil, nil
			}
		}
		if err != nil {
			return nil, err
		}
		return cpus, nil
	}
}

func TestPerCpuEventSetCreation(t *testing.T) {
	tests := []struct {
		description           string
		normalizeTimestamps   bool
		cpuLookup             CPULookupFunc
		wantError             bool
		wantEventIndicesByCPU map[CPUID][]int
		wantStartTimestamp    trace.Timestamp
		wantEndTimestamp      trace.Timestamp
	}{{
		description:         "normalized, default cpuLookup",
		normalizeTimestamps: true,
		cpuLookup:           nil, // use UnclippedReportingCPU.
		wantError:           false,
		wantEventIndicesByCPU: map[CPUID][]int{
			1: {1, 2, 5, 6, 7},
			2: {3, 4, 8, 9, 10},
		},
		wantStartTimestamp: 0,
		wantEndTimestamp:   38,
	}, {
		description:         "unnormalized, default cpuLookup",
		normalizeTimestamps: false,
		cpuLookup:           nil, // use UnclippedReportingCPU.
		wantError:           false,
		wantEventIndicesByCPU: map[CPUID][]int{
			1: {1, 2, 5, 6, 7},
			2: {3, 4, 8, 9, 10},
		},
		wantStartTimestamp: 1000,
		wantEndTimestamp:   1038,
	}, {
		description:         "normalized, custom cpuLookup",
		normalizeTimestamps: true,
		// Use UnclippedReportingCPU for the simple events, but the "cpu" field of
		// indirectEvents.
		cpuLookup: func(ev *trace.Event) ([]CPUID, error) {
			if ev.Clipped {
				return nil, nil
			}
			if ev.Name == indirectEventLabel {
				cpu, ok := ev.NumberProperties["cpu"]
				if !ok {
					return nil, status.Errorf(codes.InvalidArgument, "%s event lacks cpu field", indirectEventLabel)
				}
				return []CPUID{CPUID(cpu)}, nil
			}
			return []CPUID{CPUID(ev.CPU)}, nil
		},
		wantError: false,
		// The indirects are on their 'cpu' field, not their reporting CPU.
		wantEventIndicesByCPU: map[CPUID][]int{
			1: {1, 2, 6, 7, 10},
			2: {3, 4, 5, 8, 9},
		},
		wantStartTimestamp: 0,
		wantEndTimestamp:   38,
	}, {
		description:         "normalized, event-type filtering cpuLookup",
		normalizeTimestamps: true,
		// Use UnclippedReportingCPU for the simple events, omit the indirectEvents.
		cpuLookup: func(ev *trace.Event) ([]CPUID, error) {
			if ev.Name == simpleNumericEventLabel || ev.Name == simpleTextualEventLabel {
				return UnclippedReportingCPU(ev)
			}
			return nil, nil
		},
		wantError: false,
		// Everything but the indirects.
		wantEventIndicesByCPU: map[CPUID][]int{
			1: {1, 2, 6, 7},
			2: {3, 4, 8, 9},
		},
		wantStartTimestamp: 0,
		wantEndTimestamp:   38,
	}, {
		description:         "normalized, event-content filtering cpuLookup",
		normalizeTimestamps: true,
		// Select odd-valued simpleNumericEvents, vowel-valued simpleTextualEvents,
		// and no indirectEvents.
		cpuLookup: func(ev *trace.Event) ([]CPUID, error) {
			switch ev.Name {
			case simpleNumericEventLabel:
				// Only include events whose values are odd.
				value, ok := ev.NumberProperties["value"]
				if !ok {
					return nil, status.Errorf(codes.InvalidArgument, "%s event lacks value field", simpleNumericEventLabel)
				}
				if value%2 == 1 {
					return []CPUID{CPUID(ev.CPU)}, nil
				}
			case simpleTextualEventLabel:
				// Only include events whose values are vowels.
				value, ok := ev.TextProperties["value"]
				if !ok {
					return nil, status.Errorf(codes.InvalidArgument, "%s event lacks value field", simpleTextualEventLabel)
				}
				if strings.ContainsAny(value, "aeiou") {
					return []CPUID{CPUID(ev.CPU)}, nil
				}
				return nil, nil
			}
			return nil, nil
		},
		wantError: false,
		// simple*Events with values 1, 'a', and 3.
		wantEventIndicesByCPU: map[CPUID][]int{
			1: {1, 2, 6},
		},
		wantStartTimestamp: 0,
		wantEndTimestamp:   38,
	}, {
		description:         "normalized, cpuLookup filtering with local context",
		normalizeTimestamps: true,
		// Use closures to select events based on context developed from previous
		// events.
		cpuLookup: func() CPULookupFunc {
			m := map[string]CPULookupFunc{
				simpleNumericEventLabel: eventLimiter(2), // select the first 2 simpleNumericEvents
				simpleTextualEventLabel: eventLimiter(3), // select the first 3 simpleTextualEvents
				indirectEventLabel:      eventLimiter(2), // select both indirectEvents
			}
			return func(ev *trace.Event) ([]CPUID, error) {
				if lu, ok := m[ev.Name]; ok {
					return lu(ev)
				}
				return nil, nil
			}
		}(),
		wantError: false,
		wantEventIndicesByCPU: map[CPUID][]int{
			1: {1, 2, 5, 7},
			2: {3, 4, 10},
		},
		wantStartTimestamp: 0,
		wantEndTimestamp:   38,
	}, {
		description:         "normalized, validating cpuLookup",
		normalizeTimestamps: true,
		// Select odd-valued simpleNumericEvents, vowel-valued simpleTextualEvents,
		// and no indirectEvents.
		cpuLookup: func(ev *trace.Event) ([]CPUID, error) {
			switch ev.Name {
			case simpleNumericEventLabel:
				// Error on events whose values are odd.
				value, ok := ev.NumberProperties["value"]
				if !ok {
					return nil, status.Errorf(codes.InvalidArgument, "%s event lacks value field", simpleNumericEventLabel)
				}
				if value%2 == 1 {
					return nil, status.Errorf(codes.InvalidArgument, "you bloody fool, %d is odd", value)
				}
			case simpleTextualEventLabel:
				// Error on events whose values are odd.
				value, ok := ev.TextProperties["value"]
				if !ok {
					return nil, status.Errorf(codes.InvalidArgument, "%s event lacks value field", simpleTextualEventLabel)
				}
				if strings.ContainsAny(value, "aeiouy") {
					return nil, status.Errorf(codes.InvalidArgument, "you bloody fool, %s isn't a vowel (or even y!)", value)
				}
			default:
				return nil, nil
			}
			return UnclippedReportingCPU(ev)
		},
		wantError: true,
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			coll, err := NewPerCPUCollection(perCPUEventsCollection(t, test.normalizeTimestamps), test.cpuLookup)
			if err == nil && test.wantError {
				t.Errorf("NewPerCPUCollection yielded no error, but wanted one")
			}
			if err != nil && !test.wantError {
				t.Errorf("NewPerCPUCollection yielded unexpected error %s", err)
			}
			if err != nil || test.wantError {
				return
			}
			gotStartTimestamp, gotEndTimestamp := coll.UnclippedInterval()
			if gotStartTimestamp != test.wantStartTimestamp || gotEndTimestamp != test.wantEndTimestamp {
				t.Errorf("UnclippedInterval() = %v,%v; want %v,%v", gotStartTimestamp, gotEndTimestamp, test.wantStartTimestamp, test.wantEndTimestamp)
			}
			gotEventIndicesByCPU := map[CPUID][]int{}
			for _, cpu := range coll.CPUs() {
				gotEventIndicesByCPU[cpu] = coll.EventIndices(CPUs(cpu))
			}
			if diff := cmp.Diff(test.wantEventIndicesByCPU, gotEventIndicesByCPU); diff != "" {
				t.Errorf("EventIndices() = %#v; diff (want->got) %s", gotEventIndicesByCPU, diff)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		description      string
		filters          []Filter
		wantEventIndices []int
	}{{
		description:      "no filters",
		wantEventIndices: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	}, {
		description: "filter to CPU",
		filters: []Filter{
			CPUs(1),
		},
		wantEventIndices: []int{1, 2, 5, 6, 7},
	}, {
		description: "filter to time range",
		filters: []Filter{
			StartTimestamp(10),
			EndTimestamp(20),
		},
		wantEventIndices: []int{3, 4, 5, 6},
	}, {
		description: "filter to event type",
		filters: []Filter{
			EventTypes(simpleNumericEventLabel),
		},
		wantEventIndices: []int{1, 3, 6, 8},
	}, {
		description: "multifilter",
		filters: []Filter{
			CPUs(2),
			StartTimestamp(10),
			EndTimestamp(20),
			EventTypes(simpleNumericEventLabel),
		},
		wantEventIndices: []int{3},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			coll, err := NewPerCPUCollection(perCPUEventsCollection(t, true /*normalizeTimestamps*/), nil /*cpuLookup*/)
			if err != nil {
				t.Errorf("NewPerCPUCollection yielded unexpected error %s", err)
			}
			gotEventIndices := coll.EventIndices(test.filters...)
			if diff := cmp.Diff(test.wantEventIndices, gotEventIndices); diff != "" {
				t.Errorf("EventIndices() = %#v; diff (want->got) %s", gotEventIndices, diff)
			}
		})
	}
}

func TestSearchBeforeAndAfterTimestamp(t *testing.T) {
	// Unclipped events on CPU 1:
	//   ev 1 @TS 1000, simpleNumericEvent
	//   ev 2 @TS 1005, simpleTextualEvent
	//   ev 5 @TS 1018, indirectEvent
	//   ev 6 @TS 1020, simpleNumericEvent
	//   ev 7 @ts 1025, simpleTextualEvent
	coll, err := NewPerCPUCollection(perCPUEventsCollection(t, false /* normalizeTimestamps */), nil /* cpuLookup */)
	if err != nil {
		t.Fatalf("Failed to build collection: %s", err)
	}
	tests := []struct {
		description          string
		cpu                  CPUID
		timestamp            trace.Timestamp
		wantEventIndexBefore int // <0 if expect none
		wantEventIndexAfter  int // <0 if expect none
	}{{
		description:          "between events",
		cpu:                  1,
		timestamp:            1007,
		wantEventIndexBefore: 2,
		wantEventIndexAfter:  5,
	}, {
		description:          "right on an event",
		cpu:                  1,
		timestamp:            1018,
		wantEventIndexBefore: 2,
		wantEventIndexAfter:  6,
	}, {
		description:          "after end",
		cpu:                  1,
		timestamp:            1050,
		wantEventIndexBefore: 7,
		wantEventIndexAfter:  -1,
	}, {
		description:          "before start",
		cpu:                  1,
		timestamp:            950,
		wantEventIndexBefore: -1,
		wantEventIndexAfter:  1,
	}, {
		description:          "nothing found on missing CPU",
		cpu:                  5,
		timestamp:            1020,
		wantEventIndexBefore: -1,
		wantEventIndexAfter:  -1,
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if gotEventIndexBefore, ok := coll.EventIndexOnCPUBefore(test.cpu, test.timestamp); ok {
				if gotEventIndexBefore != test.wantEventIndexBefore {
					t.Errorf("EventIndexOnCPUBefore() = %d, want %d", gotEventIndexBefore, test.wantEventIndexBefore)
				}
			} else if test.wantEventIndexBefore >= 0 {
				t.Errorf("Expected EventIndexOnCPUBefore() to find %d, but it found nothing", test.wantEventIndexBefore)
			}
			if gotEventIndexAfter, ok := coll.EventIndexOnCPUAfter(test.cpu, test.timestamp); ok {
				if gotEventIndexAfter != test.wantEventIndexAfter {
					t.Errorf("EventIndexOnCPUAfter() = %d, want %d", gotEventIndexAfter, test.wantEventIndexAfter)
				}
			} else if test.wantEventIndexAfter >= 0 {
				t.Errorf("Expected EventIndexOnCPUAfter() to find %d, but it found nothing", test.wantEventIndexAfter)
			}

		})
	}
}

func TestSchedCollection(t *testing.T) {
	c, err := NewCollection(schedtestcommon.TestTrace1(t), NormalizeTimestamps(false))
	if err != nil {
		t.Fatalf("Failed to construct Collection: %s", err)
	}
	pcc, err := NewPerCPUCollection(c /*cpuLookupFunc*/, nil)
	if err != nil {
		t.Fatalf("Failed to construct PerCPUCollection: %s", err)
	}
	tests := []struct {
		description      string
		cpus             []CPUID
		wantEventIndices []int
	}{{
		description:      "All CPUs",
		cpus:             []CPUID{0, 1, 2},
		wantEventIndices: []int{1, 2, 3, 4, 5, 6, 7, 8},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			gotEventIndices := pcc.EventIndices(CPUs(test.cpus...))
			if diff := cmp.Diff(test.wantEventIndices, gotEventIndices); diff != "" {
				t.Errorf("EventIndices() = %#v; diff (want->got) %s", gotEventIndices, diff)
			}

		})
	}
}
