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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/google/schedviz/tracedata/testeventsetbuilder"

	"github.com/google/schedviz/analysis/schedtestcommon"
	"github.com/google/schedviz/tracedata/trace"
)

func TestPIDsAndComms(t *testing.T) {
	coll, err := NewCollection(
		testeventsetbuilder.TestProtobuf(t,
			schedtestcommon.UnpopulatedBuilder().
				WithEvent("sched_switch", 1, 1000, false,
					100, "Process A", 50, schedtestcommon.Interruptible,
					200, "Constant", 50).
				WithEvent("sched_switch", 1, 1010, false,
					200, "Constant", 50, schedtestcommon.Runnable,
					100, "Process B", 70).
				WithEvent("sched_switch", 1, 1020, false,
					100, "Process B", 70, schedtestcommon.Interruptible,
					200, "Constant", 50).
				WithEvent("sched_switch", 1, 1030, false,
					200, "Constant", 50, schedtestcommon.Interruptible,
					100, "Process C", 70).
				WithEvent("sched_wakeup", 0, 1040, false,
					200, "Constant", 50, 1)), DefaultEventLoaders(), PreciseCommands(true), PrecisePriorities(true))
	if err != nil {
		t.Fatalf("Unexpected collection creation error %s", err)
	}
	tests := []struct {
		description      string
		filters          []Filter
		wantPIDsAndComms map[PID][]string
	}{{
		description: "whole range",
		filters:     []Filter{},
		wantPIDsAndComms: map[PID][]string{
			100: {"Process A", "Process B", "Process C"},
			200: {"Constant"},
		},
	}, {
		description: "partial intervals 1, PID 100",
		filters:     []Filter{TimeRange(1005, 1030), PIDs(100)},
		wantPIDsAndComms: map[PID][]string{
			100: {"Process A", "Process B", "Process C"},
		},
	}, {
		description: "partial intervals 1, PID 100",
		filters:     []Filter{TimeRange(1015, 1025), PIDs(100)},
		wantPIDsAndComms: map[PID][]string{
			100: {"Process B"},
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			gotPIDsAndComms, err := coll.PIDsAndComms(test.filters...)
			if err != nil {
				t.Fatalf("gotPIDsAndComms yielded error %s but wanted none", err)
			}
			if !reflect.DeepEqual(gotPIDsAndComms, test.wantPIDsAndComms) {
				t.Fatalf("PIDsAndComms = %#v, want %#v", gotPIDsAndComms, test.wantPIDsAndComms)
			}
		})
	}
}

func interval(mergedIntervalCount int, startTimestamp trace.Timestamp, duration Duration, cpu CPUID, threadResidencies ...*ThreadResidency) *Interval {
	return &Interval{
		StartTimestamp:      startTimestamp,
		Duration:            duration,
		CPU:                 cpu,
		ThreadResidencies:   threadResidencies,
		MergedIntervalCount: mergedIntervalCount,
	}
}

func threadResidency(thread *Thread, duration Duration, threadState ThreadState, droppedEventIDs ...int) *ThreadResidency {
	return &ThreadResidency{
		Thread:          thread,
		Duration:        duration,
		State:           threadState,
		DroppedEventIDs: droppedEventIDs,
	}
}

func syntheticThreadResidency(thread *Thread, duration Duration, threadState ThreadState, droppedEventIDs ...int) *ThreadResidency {
	return &ThreadResidency{
		Thread:                       thread,
		Duration:                     duration,
		State:                        threadState,
		DroppedEventIDs:              droppedEventIDs,
		IncludesSyntheticTransitions: true,
	}
}

func TestThreadInterval(t *testing.T) {
	coll, err := NewCollection(schedtestcommon.TestTrace1(t), DefaultEventLoaders(), PreciseCommands(true), PrecisePriorities(true))
	if err != nil {
		t.Fatalf("Unexpected collection creation error %s", err)
	}
	thread1 := &Thread{
		PID:      100,
		Command:  "Process1",
		Priority: 50,
	}
	thread2 := &Thread{
		PID:      200,
		Command:  "Process2",
		Priority: 50,
	}
	tests := []struct {
		description   string
		filters       []Filter
		wantIntervals []*Interval
	}{{
		description: "pid 200, post wakeup, whole time range, unmerged, truncated",
		filters:     []Filter{PIDs(200), TruncateToTimeRange(true)},
		wantIntervals: []*Interval{
			interval(
				1, trace.Timestamp(1000), Duration(0), CPUID(1),
				threadResidency(thread2, Duration(0), RunningState),
			),
			interval(
				1, trace.Timestamp(1000), Duration(40), CPUID(1),
				threadResidency(thread2, Duration(40), SleepingState),
			),
			interval(
				1, trace.Timestamp(1040), Duration(40), CPUID(1),
				threadResidency(thread2, Duration(40), WaitingState),
			),
			interval(
				1, trace.Timestamp(1080), Duration(20), CPUID(2),
				threadResidency(thread2, Duration(20), WaitingState),
			),
			interval(
				1, trace.Timestamp(1100), Duration(0), CPUID(2),
				threadResidency(thread2, Duration(0), RunningState),
			),
		},
	}, {
		description: "pid 200, post wakeup, partial time range, unmerged, untruncated",
		filters:     []Filter{TimeRange(1020, 1099), PIDs(200)},
		wantIntervals: []*Interval{
			interval(
				1, trace.Timestamp(1000), Duration(40), CPUID(1),
				threadResidency(thread2, Duration(40), SleepingState),
			),
			interval(
				1, trace.Timestamp(1040), Duration(40), CPUID(1),
				threadResidency(thread2, Duration(40), WaitingState),
			),
			interval(
				1, trace.Timestamp(1080), Duration(20), CPUID(2),
				threadResidency(thread2, Duration(20), WaitingState),
			),
		},
	}, {
		description: "pid 100, wakeup and round-robin, whole range, unmerged, truncated",
		filters:     []Filter{PIDs(100), TruncateToTimeRange(true)},
		wantIntervals: []*Interval{
			interval(
				1, trace.Timestamp(1000), Duration(0), CPUID(1),
				threadResidency(thread1, Duration(0), UnknownState),
			),
			interval(
				1, trace.Timestamp(1000), Duration(10), CPUID(1),
				threadResidency(thread1, Duration(10), WaitingState),
			),
			interval(
				1, trace.Timestamp(1010), Duration(90), CPUID(1),
				threadResidency(thread1, Duration(90), RunningState),
			),
			interval(
				1, trace.Timestamp(1100), Duration(0), CPUID(1),
				threadResidency(thread1, Duration(0), WaitingState),
			),
		},
	}, {
		description: "pid 200, post-wakeup, whole range, merged at 50, truncated",
		filters:     []Filter{PIDs(200), TruncateToTimeRange(true), MinIntervalDuration(50)},
		wantIntervals: []*Interval{
			interval(
				2, trace.Timestamp(1000), Duration(40), CPUID(1),
				threadResidency(thread2, Duration(0), RunningState),
				threadResidency(thread2, Duration(40), SleepingState),
			),
			interval(
				1, trace.Timestamp(1040), Duration(40), CPUID(1),
				threadResidency(thread2, Duration(40), WaitingState),
			),
			interval(
				2, trace.Timestamp(1080), Duration(20), CPUID(2),
				threadResidency(thread2, Duration(0), RunningState),
				threadResidency(thread2, Duration(20), WaitingState),
			),
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			gotIntervals, err := coll.ThreadIntervals(test.filters...)
			if err != nil {
				t.Fatalf("ThreadIntervals yielded unexpected error %s", err)
			}
			if !reflect.DeepEqual(gotIntervals, test.wantIntervals) {
				t.Logf("Got:")
				for _, ti := range gotIntervals {
					t.Logf("   %s", ti)
				}
				t.Logf("Want:")
				for _, ti := range test.wantIntervals {
					t.Logf("   %s", ti)
				}
				t.Errorf("Unexpected ThreadIntervals output: got %#v, want %#v", gotIntervals, test.wantIntervals)
			}
		})
	}
}

func TestCPUIntervals(t *testing.T) {
	coll, err := NewCollection(schedtestcommon.TestTrace1(t), DefaultEventLoaders(), PreciseCommands(true), PrecisePriorities(true))
	if err != nil {
		t.Fatalf("Unexpected collection creation error %s", err)
	}
	thread1 := &Thread{
		PID:      100,
		Command:  "Process1",
		Priority: 50,
	}
	thread2 := &Thread{
		PID:      200,
		Command:  "Process2",
		Priority: 50,
	}
	thread3 := &Thread{
		PID:      300,
		Command:  "Process3",
		Priority: 50,
	}
	thread4 := &Thread{
		PID:      400,
		Command:  "Process4",
		Priority: 50,
	}
	tests := []struct {
		description             string
		splitOnWaitingPIDChange bool
		filters                 []Filter
		wantIntervals           []*Interval
	}{{
		description:             "cpu 1, whole time range, unmerged, split on waits",
		splitOnWaitingPIDChange: true,
		filters:                 []Filter{CPUs(1)},
		wantIntervals: []*Interval{
			interval(
				1, trace.Timestamp(1000), Duration(10), CPUID(1),
				threadResidency(thread3, Duration(10), RunningState),
				threadResidency(thread1, Duration(10), WaitingState),
			),
			interval(
				1, trace.Timestamp(1010), Duration(30), CPUID(1),
				threadResidency(thread1, Duration(30), RunningState),
			),
			interval(
				1, trace.Timestamp(1040), Duration(40), CPUID(1),
				threadResidency(thread1, Duration(40), RunningState),
				threadResidency(thread2, Duration(40), WaitingState),
			),
			interval(
				1, trace.Timestamp(1080), Duration(10), CPUID(1),
				threadResidency(thread1, Duration(10), RunningState),
			),
			interval(
				1, trace.Timestamp(1090), Duration(10), CPUID(1),
				threadResidency(thread1, Duration(10), RunningState),
				threadResidency(thread3, Duration(10), WaitingState),
			),
			interval(
				1, trace.Timestamp(1100), Duration(0), CPUID(1),
				threadResidency(thread3, Duration(0), RunningState),
				threadResidency(thread1, Duration(0), WaitingState),
			),
		},
	}, {
		description:             "cpu 1, whole time range, unmerged, not split on waits",
		splitOnWaitingPIDChange: false,
		filters:                 []Filter{CPUs(1)},
		wantIntervals: []*Interval{
			interval(
				1, trace.Timestamp(1000), Duration(10), CPUID(1),
				threadResidency(thread3, Duration(10), RunningState),
				threadResidency(thread1, Duration(10), WaitingState),
			),
			interval(
				4, trace.Timestamp(1010), Duration(90), CPUID(1),
				threadResidency(thread1, Duration(90), RunningState),
				threadResidency(thread2, Duration(40), WaitingState),
				threadResidency(thread3, Duration(10), WaitingState),
			),
			interval(
				1, trace.Timestamp(1100), Duration(0), CPUID(1),
				threadResidency(thread3, Duration(0), RunningState),
				threadResidency(thread1, Duration(0), WaitingState),
			),
		},
	}, {
		description:             "cpu 2, whole time range, unmerged, split on waits",
		splitOnWaitingPIDChange: true,
		filters:                 []Filter{CPUs(2)},
		wantIntervals: []*Interval{
			interval(
				1, trace.Timestamp(1000), Duration(80), CPUID(2),
				threadResidency(thread4, Duration(80), RunningState),
			),
			interval(
				1, trace.Timestamp(1080), Duration(20), CPUID(2),
				threadResidency(thread4, Duration(20), RunningState),
				threadResidency(thread2, Duration(20), WaitingState),
			),
			interval(
				1, trace.Timestamp(1100), Duration(0), CPUID(2),
				threadResidency(thread2, Duration(0), RunningState),
				threadResidency(thread4, Duration(0), WaitingState),
			),
		},
	}, {
		description:             "cpu 1, whole time range, merged at 40, split on waits",
		splitOnWaitingPIDChange: true,
		filters:                 []Filter{CPUs(1), MinIntervalDuration(40)},
		wantIntervals: []*Interval{
			interval(
				2, trace.Timestamp(1000), Duration(40), CPUID(1),
				threadResidency(thread1, Duration(30), RunningState),
				threadResidency(thread3, Duration(10), RunningState),
				threadResidency(thread1, Duration(10), WaitingState),
			),
			interval(
				1, trace.Timestamp(1040), Duration(40), CPUID(1),
				threadResidency(thread1, Duration(40), RunningState),
				threadResidency(thread2, Duration(40), WaitingState),
			),
			interval(
				3, trace.Timestamp(1080), Duration(20), CPUID(1),
				threadResidency(thread1, Duration(20), RunningState),
				threadResidency(thread3, Duration(0), RunningState),
				threadResidency(thread1, Duration(0), WaitingState),
				threadResidency(thread3, Duration(10), WaitingState),
			),
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			gotIntervals, err := coll.CPUIntervals(test.splitOnWaitingPIDChange, test.filters...)
			if err != nil {
				t.Fatalf("CPUIntervals yielded unexpected error %s", err)
			}
			if diff := cmp.Diff(gotIntervals, test.wantIntervals); diff != "" {
				t.Errorf("Unexpected ThreadIntervals output; Diff -want +got %v", diff)
			}
		})
	}
}

func TestSwitchOnly(t *testing.T) {
	thread1 := &Thread{
		PID:      100,
		Command:  "Process1",
		Priority: 50,
	}
	thread2 := &Thread{
		PID:      200,
		Command:  "Process2",
		Priority: 50,
	}
	thread3 := &Thread{
		PID:      300,
		Command:  "Process3",
		Priority: 50,
	}
	thread4 := &Thread{
		PID:      400,
		Command:  "Process4",
		Priority: 50,
	}
	type wantPIDIntervals struct {
		filters   []Filter
		intervals []*Interval
	}
	tests := []struct {
		description string
		eventSet    *eventpb.EventSet
		want        []wantPIDIntervals
	}{{
		description: "simple switch-only",
		eventSet: testeventsetbuilder.TestProtobuf(t,
			schedtestcommon.UnpopulatedBuilder().
				WithEvent("sched_switch", 0, 1000, false,
					100, "Process1", 50, schedtestcommon.Interruptible,
					200, "Process2", 50).
				WithEvent("sched_switch", 1, 1010, false,
					300, "Process3", 50, schedtestcommon.Runnable,
					400, "Process4", 50).
				// We should infer that Process3 switched from CPU 1 to CPU 0 at time 1015.
				WithEvent("sched_switch", 0, 1020, false,
					200, "Process2", 50, schedtestcommon.Interruptible,
					300, "Process3", 50).
				// We should infer that Process1 switched from CPU 0 to CPU 1, and from
				// Sleeping to Waiting state, at time 1015.
				WithEvent("sched_switch", 1, 1030, false,
					400, "Process4", 50, schedtestcommon.Runnable,
					100, "Process1", 50)),
		want: []wantPIDIntervals{
			{
				[]Filter{PIDs(100)},
				[]*Interval{
					interval(
						1, trace.Timestamp(1000), Duration(0), CPUID(0),
						threadResidency(thread1, Duration(0), RunningState),
					),
					// The two intervals around the synthetic migration/wakeup include
					// synthetic transitions.
					interval(
						1, trace.Timestamp(1000), Duration(15), CPUID(0),
						syntheticThreadResidency(thread1, Duration(15), SleepingState),
					),
					interval(
						1, trace.Timestamp(1015), Duration(15), CPUID(1),
						syntheticThreadResidency(thread1, Duration(15), WaitingState),
					),
					interval(
						1, trace.Timestamp(1030), Duration(1), CPUID(1),
						threadResidency(thread1, Duration(1), RunningState),
					),
				},
			}, {
				[]Filter{PIDs(200)},
				[]*Interval{
					interval(
						1, trace.Timestamp(1000), Duration(0), CPUID(0),
						threadResidency(thread2, Duration(0), WaitingState),
					),
					interval(
						1, trace.Timestamp(1000), Duration(20), CPUID(0),
						threadResidency(thread2, Duration(20), RunningState),
					),
					interval(
						1, trace.Timestamp(1020), Duration(11), CPUID(0),
						threadResidency(thread2, Duration(11), SleepingState),
					),
				},
			}, {
				[]Filter{PIDs(300)},
				[]*Interval{
					interval(
						1, trace.Timestamp(1000), Duration(10), CPUID(1),
						threadResidency(thread3, Duration(10), RunningState),
					),
					// The two intervals around the synthetic migration include
					// synthetic transitions.
					interval(
						1, trace.Timestamp(1010), Duration(5), CPUID(1),
						syntheticThreadResidency(thread3, Duration(5), WaitingState),
					),
					interval(
						1, trace.Timestamp(1015), Duration(5), CPUID(0),
						syntheticThreadResidency(thread3, Duration(5), WaitingState),
					),
					interval(
						1, trace.Timestamp(1020), Duration(11), CPUID(0),
						threadResidency(thread3, Duration(11), RunningState),
					),
				},
			}, {
				[]Filter{PIDs(400)},
				[]*Interval{
					interval(
						1, trace.Timestamp(1000), Duration(10), CPUID(1),
						threadResidency(thread4, Duration(10), WaitingState),
					),
					interval(
						1, trace.Timestamp(1010), Duration(20), CPUID(1),
						threadResidency(thread4, Duration(20), RunningState),
					),
					interval(
						1, trace.Timestamp(1030), Duration(1), CPUID(1),
						threadResidency(thread4, Duration(1), WaitingState),
					),
				},
			}},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			coll, err := NewCollection(
				test.eventSet,
				SwitchOnlyLoaders(),
				PreciseCommands(true),
				PrecisePriorities(true))
			if err != nil {
				t.Fatalf("Unexpected collection creation error %s", err)
			}
			for _, want := range test.want {
				gotIntervals, err := coll.ThreadIntervals(want.filters...)
				if err != nil {
					t.Fatalf("ThreadIntervals yielded unexpected error %s", err)
				}
				if !reflect.DeepEqual(gotIntervals, want.intervals) {
					t.Logf("Got:")
					for _, ti := range gotIntervals {
						t.Logf("   %s", ti)
					}
					t.Logf("Want:")
					for _, ti := range want.intervals {
						t.Logf("   %s", ti)
					}
					t.Errorf("Unexpected ThreadIntervals output: got %#v, want %#v", gotIntervals, want.intervals)
				}
			}
		})
	}
}
