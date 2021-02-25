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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/schedviz/analysis/schedtestcommon"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/google/schedviz/tracedata/testeventsetbuilder"
	"github.com/google/schedviz/tracedata/trace"
)

func elementaryCPUIntervalListStr(ecis []*ElementaryCPUInterval) string {
	sortElementaryCPUIntervalList(ecis)
	var ret []string
	for _, eci := range ecis {
		ret = append(ret, eci.String())
	}
	return "[\n  " + strings.Join(ret, "\n  ") + "]"
}

func sortElementaryCPUIntervalList(ecis []*ElementaryCPUInterval) {
	sort.Slice(ecis, func(a, b int) bool {
		return ecis[a].StartTimestamp < ecis[b].StartTimestamp
	})
	for _, eci := range ecis {
		sort.Slice(eci.CPUStates, func(a, b int) bool {
			return eci.CPUStates[a].CPU < eci.CPUStates[b].CPU
		})
		for _, cs := range eci.CPUStates {
			sort.Slice(cs.Waiting, func(a, b int) bool {
				return cs.Waiting[a].PID < cs.Waiting[b].PID
			})
		}
	}
}

func TestElementaryCPUIntervals(t *testing.T) {
	elementaryCPUInterval := func(startTimestamp, endTimestamp trace.Timestamp, cpuStates ...*CPUState) *ElementaryCPUInterval {
		return &ElementaryCPUInterval{
			StartTimestamp: startTimestamp,
			EndTimestamp:   endTimestamp,
			CPUStates:      cpuStates,
		}
	}
	cpuState := func(cpu CPUID, mergeType CPUStateMergeType, running *Thread, waiting ...*Thread) *CPUState {
		return &CPUState{
			CPU:       cpu,
			MergeType: mergeType,
			Running:   running,
			Waiting:   waiting,
		}
	}
	thread1 := &Thread{PID(100), "Process1", Priority(50)}
	thread2 := &Thread{PID(200), "Process2", Priority(50)}
	thread3 := &Thread{PID(300), "Process3", Priority(50)}
	thread4 := &Thread{PID(400), "Process4", Priority(50)}
	tests := []struct {
		description                string
		filters                    []Filter
		diffOutput                 bool
		eventSet                   *eventpb.EventSet
		wantElementaryCPUIntervals []*ElementaryCPUInterval
	}{{
		description: "CPU 1, test trace 1, full range",
		filters:     []Filter{CPUs(1)},
		diffOutput:  false,
		eventSet:    schedtestcommon.TestTrace1(t),
		wantElementaryCPUIntervals: []*ElementaryCPUInterval{
			elementaryCPUInterval(
				trace.Timestamp(1000), trace.Timestamp(1010),
				cpuState(CPUID(1), Full, thread3, thread1),
			),
			elementaryCPUInterval(
				trace.Timestamp(1010), trace.Timestamp(1040),
				cpuState(CPUID(1), Full, thread1),
			),
			elementaryCPUInterval(
				trace.Timestamp(1040), trace.Timestamp(1080),
				cpuState(CPUID(1), Full, thread1, thread2),
			),
			elementaryCPUInterval(
				trace.Timestamp(1080), trace.Timestamp(1090),
				cpuState(CPUID(1), Full, thread1),
			),
			elementaryCPUInterval(
				trace.Timestamp(1090), trace.Timestamp(1100),
				cpuState(CPUID(1), Full, thread1, thread3),
			),
			elementaryCPUInterval(
				trace.Timestamp(1100), trace.Timestamp(1100),
				cpuState(CPUID(1), Full, thread3, thread1),
			),
		},
	}, {
		description: "CPU 1, test trace 1, full range, diff",
		filters:     []Filter{CPUs(1)},
		diffOutput:  true,
		eventSet:    schedtestcommon.TestTrace1(t),
		wantElementaryCPUIntervals: []*ElementaryCPUInterval{
			elementaryCPUInterval(
				trace.Timestamp(1000), trace.Timestamp(1010),
				cpuState(CPUID(1), Add, thread3, thread1),
			),
			elementaryCPUInterval(
				trace.Timestamp(1010), trace.Timestamp(1040),
				cpuState(CPUID(1), Remove, thread3, thread1),
				cpuState(CPUID(1), Add, thread1),
			),
			elementaryCPUInterval(
				trace.Timestamp(1040), trace.Timestamp(1080),
				cpuState(CPUID(1), Add, nil, thread2),
			),
			elementaryCPUInterval(
				trace.Timestamp(1080), trace.Timestamp(1090),
				cpuState(CPUID(1), Remove, nil, thread2),
			),
			elementaryCPUInterval(
				trace.Timestamp(1090), trace.Timestamp(1100),
				cpuState(CPUID(1), Add, nil, thread3),
			),
			elementaryCPUInterval(
				trace.Timestamp(1100), trace.Timestamp(1100),
				cpuState(CPUID(1), Remove, thread1, thread3),
				cpuState(CPUID(1), Add, thread3, thread1),
			),
		},
	}, {
		description: "CPU 1, test trace 1, partial range, truncated",
		filters:     []Filter{CPUs(1), TimeRange(1050, 1095), TruncateToTimeRange(true)},
		diffOutput:  false,
		eventSet:    schedtestcommon.TestTrace1(t),
		wantElementaryCPUIntervals: []*ElementaryCPUInterval{
			elementaryCPUInterval(
				trace.Timestamp(1050), trace.Timestamp(1080),
				cpuState(CPUID(1), Full, thread1, thread2),
			),
			elementaryCPUInterval(
				trace.Timestamp(1080), trace.Timestamp(1090),
				cpuState(CPUID(1), Full, thread1),
			),
			elementaryCPUInterval(
				trace.Timestamp(1090), trace.Timestamp(1095),
				cpuState(CPUID(1), Full, thread1, thread3),
			),
		},
	}, {
		description: "CPUs 1 and 2, test trace 1, full range, truncated",
		filters:     []Filter{CPUs(1, 2), TruncateToTimeRange(true)},
		diffOutput:  false,
		eventSet:    schedtestcommon.TestTrace1(t),
		wantElementaryCPUIntervals: []*ElementaryCPUInterval{
			elementaryCPUInterval(
				trace.Timestamp(1000), trace.Timestamp(1010),
				cpuState(CPUID(1), Full, thread3, thread1),
				cpuState(CPUID(2), Full, thread4),
			),
			elementaryCPUInterval(
				trace.Timestamp(1010), trace.Timestamp(1040),
				cpuState(CPUID(1), Full, thread1),
				cpuState(CPUID(2), Full, thread4),
			),
			elementaryCPUInterval(
				trace.Timestamp(1040), trace.Timestamp(1080),
				cpuState(CPUID(1), Full, thread1, thread2),
				cpuState(CPUID(2), Full, thread4),
			),
			elementaryCPUInterval(
				trace.Timestamp(1080), trace.Timestamp(1090),
				cpuState(CPUID(1), Full, thread1),
				cpuState(CPUID(2), Full, thread4, thread2),
			),
			elementaryCPUInterval(
				trace.Timestamp(1090), trace.Timestamp(1100),
				cpuState(CPUID(1), Full, thread1, thread3),
				cpuState(CPUID(2), Full, thread4, thread2),
			),
			elementaryCPUInterval(
				trace.Timestamp(1100), trace.Timestamp(1100),
				cpuState(CPUID(1), Full, thread3, thread1),
				cpuState(CPUID(2), Full, thread2, thread4),
			),
		},
	}, {
		description: "CPU 1, test trace 1, full range, only running",
		filters:     []Filter{CPUs(1), ThreadStates(RunningState)},
		diffOutput:  false,
		eventSet:    schedtestcommon.TestTrace1(t),
		wantElementaryCPUIntervals: []*ElementaryCPUInterval{
			elementaryCPUInterval(
				trace.Timestamp(1000), trace.Timestamp(1010),
				cpuState(CPUID(1), Full, thread3),
			),
			elementaryCPUInterval(
				trace.Timestamp(1010), trace.Timestamp(1100),
				cpuState(CPUID(1), Full, thread1),
			),
			elementaryCPUInterval(
				trace.Timestamp(1100), trace.Timestamp(1100),
				cpuState(CPUID(1), Full, thread3),
			),
		},
	}, {
		description: "CPU 1, test trace 1, full range, only waiting",
		filters:     []Filter{CPUs(1), ThreadStates(WaitingState)},
		diffOutput:  false,
		eventSet:    schedtestcommon.TestTrace1(t),
		wantElementaryCPUIntervals: []*ElementaryCPUInterval{
			elementaryCPUInterval(
				trace.Timestamp(1000), trace.Timestamp(1010),
				cpuState(CPUID(1), Full, nil, thread1),
			),
			elementaryCPUInterval(
				trace.Timestamp(1010), trace.Timestamp(1040),
				cpuState(CPUID(1), Full, nil),
			),
			elementaryCPUInterval(
				trace.Timestamp(1040), trace.Timestamp(1080),
				cpuState(CPUID(1), Full, nil, thread2),
			),
			elementaryCPUInterval(
				trace.Timestamp(1080), trace.Timestamp(1090),
				cpuState(CPUID(1), Full, nil),
			),
			elementaryCPUInterval(
				trace.Timestamp(1090), trace.Timestamp(1100),
				cpuState(CPUID(1), Full, nil, thread3),
			),
			elementaryCPUInterval(
				trace.Timestamp(1100), trace.Timestamp(1100),
				cpuState(CPUID(1), Full, nil, thread1),
			),
		},
	}, {
		description: "overlapping waits",
		filters:     nil,
		diffOutput:  false,
		eventSet: testeventsetbuilder.TestProtobuf(t,
			schedtestcommon.UnpopulatedBuilder().
				// PID 100 switches in on CPU 1 at time 1000, while PID 200 switches out WAITING
				WithEvent("sched_switch", 1, 1000, false,
					200, "Process2", 50, schedtestcommon.Runnable,
					100, "Process1", 50).
				WithEvent("sched_wakeup", 1, 1025, false,
					300, "Process3", 50, 1).
				WithEvent("sched_switch", 1, 1050, false,
					100, "Process1", 50, schedtestcommon.Runnable,
					300, "Process3", 50)),
		wantElementaryCPUIntervals: []*ElementaryCPUInterval{
			elementaryCPUInterval(
				trace.Timestamp(1000), trace.Timestamp(1025),
				cpuState(CPUID(1), Full, thread1, thread2),
			),
			elementaryCPUInterval(
				trace.Timestamp(1025), trace.Timestamp(1050),
				cpuState(CPUID(1), Full, thread1, thread2, thread3),
			),
			elementaryCPUInterval(
				trace.Timestamp(1050), trace.Timestamp(1050),
				cpuState(CPUID(1), Full, thread3, thread1, thread2),
			),
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			coll, err := NewCollection(test.eventSet)
			if err != nil {
				t.Fatalf("NewCollection yielded unexpected error %s", err)
			}
			provider, err := coll.NewElementaryCPUIntervalProvider(test.diffOutput, test.filters...)
			if err != nil {
				t.Fatalf("NewElementaryCPUIntervalProvider yielded unexpected error %s", err)
			}
			var gotElementaryCPUIntervals []*ElementaryCPUInterval
			for {
				nextInterval, err := provider.NextInterval()
				if err != nil {
					t.Fatalf("NextInterval yielded unexpected error %s", err)
				}
				if nextInterval == nil {
					break
				}
				gotElementaryCPUIntervals = append(gotElementaryCPUIntervals, nextInterval)
			}
			got := elementaryCPUIntervalListStr(gotElementaryCPUIntervals)
			want := elementaryCPUIntervalListStr(test.wantElementaryCPUIntervals)
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("ElementaryCPUIntervals() = %s, -got +want %s", got, diff)
			}
		})
	}
}
