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
)

// Test that TestTrace1's spans (stripped of command and prio) are properly
// allocated per-CPU.
func TestSpanDistributionByCPU(t *testing.T) {
	spans := []*threadSpan{
		// PID 100's spans
		emptySpan(100).
			withTimeRange(1000, 1000).
			withCPU(1).
			withState(UnknownState).
			withTreeID(1),
		emptySpan(100).
			withTimeRange(1000, 1010).
			withCPU(1).
			withState(WaitingState).
			withTreeID(2),
		emptySpan(100).
			withTimeRange(1010, 1100).
			withCPU(1).
			withState(RunningState).
			withTreeID(3),
		emptySpan(100).
			withTimeRange(1100, 1100).
			withCPU(1).
			withState(WaitingState).
			withTreeID(4),
		// PID 200's spans
		emptySpan(200).
			withTimeRange(1000, 1000).
			withCPU(1).
			withState(RunningState).
			withTreeID(5),
		emptySpan(200).
			withTimeRange(1000, 1040).
			withCPU(1).
			withState(SleepingState).
			withTreeID(6),
		emptySpan(200).
			withTimeRange(1040, 1080).
			withCPU(1).
			withState(WaitingState).
			withTreeID(7),
		emptySpan(200).
			withTimeRange(1080, 1100).
			withCPU(2).
			withState(WaitingState).
			withTreeID(8),
		emptySpan(200).
			withTimeRange(1100, 1100).
			withCPU(2).
			withState(RunningState).
			withTreeID(9),
		// PID 300's spans
		emptySpan(300).
			withTimeRange(1000, 1000).
			withCPU(1).
			withState(UnknownState).
			withTreeID(10),
		emptySpan(300).
			withTimeRange(1000, 1010).
			withCPU(1).
			withState(RunningState).
			withTreeID(11),
		emptySpan(300).
			withTimeRange(1010, 1090).
			withCPU(1).
			withState(SleepingState).
			withTreeID(12),
		emptySpan(300).
			withTimeRange(1090, 1100).
			withCPU(1).
			withState(WaitingState).
			withTreeID(13),
		emptySpan(300).
			withTimeRange(1100, 1100).
			withCPU(1).
			withState(RunningState).
			withTreeID(14),
		// PID 400's spans
		emptySpan(400).
			withTimeRange(1000, 1100).
			withCPU(2).
			withState(RunningState).
			withTreeID(15),
		emptySpan(400).
			withTimeRange(1100, 1100).
			withCPU(2).
			withState(WaitingState).
			withTreeID(16),
	}
	css := newCPUSpanSet()
	for _, span := range spans {
		css.addSpan(span)
	}
	running, sleeping, waiting, err := css.cpuTrees()
	if err != nil {
		t.Fatalf("Unexpected error from cpuTrees: %s", err)
	}
	// augmentedtree.Tree can only be read via its Query method, and its internals
	// are opaque.  So, we must query everything from the sleeping and waiting
	// trees, and assemble them into a structure we can compare:
	// CPU->ThreadState->span slice.
	wantSpans := map[CPUID]map[ThreadState][]*threadSpan{
		1: {
			RunningState: {
				emptySpan(200).
					withTimeRange(1000, 1000).
					withCPU(1).
					withState(RunningState).
					withTreeID(5),
				emptySpan(300).
					withTimeRange(1000, 1010).
					withCPU(1).
					withState(RunningState).
					withTreeID(11),
				emptySpan(100).
					withTimeRange(1010, 1100).
					withCPU(1).
					withState(RunningState).
					withTreeID(3),
				emptySpan(300).
					withTimeRange(1100, 1100).
					withCPU(1).
					withState(RunningState).
					withTreeID(14),
			},
			SleepingState: {
				emptySpan(200).
					withTimeRange(1000, 1040).
					withCPU(1).
					withState(SleepingState).
					withTreeID(6),
				emptySpan(300).
					withTimeRange(1010, 1090).
					withCPU(1).
					withState(SleepingState).
					withTreeID(12),
			},
			WaitingState: {
				emptySpan(100).
					withTimeRange(1000, 1010).
					withCPU(1).
					withState(WaitingState).
					withTreeID(2),
				emptySpan(200).
					withTimeRange(1040, 1080).
					withCPU(1).
					withState(WaitingState).
					withTreeID(7),
				emptySpan(300).
					withTimeRange(1090, 1100).
					withCPU(1).
					withState(WaitingState).
					withTreeID(13),
				emptySpan(100).
					withTimeRange(1100, 1100).
					withCPU(1).
					withState(WaitingState).
					withTreeID(4),
			},
		},
		2: {
			RunningState: {
				emptySpan(400).
					withTimeRange(1000, 1100).
					withCPU(2).
					withState(RunningState).
					withTreeID(15),
				emptySpan(200).
					withTimeRange(1100, 1100).
					withCPU(2).
					withState(RunningState).
					withTreeID(9),
			},
			WaitingState: {
				emptySpan(200).
					withTimeRange(1080, 1100).
					withCPU(2).
					withState(WaitingState).
					withTreeID(8),
				emptySpan(400).
					withTimeRange(1100, 1100).
					withCPU(2).
					withState(WaitingState).
					withTreeID(16),
			},
		},
	}
	gotSpans := map[CPUID]map[ThreadState][]*threadSpan{}
	for _, cpu := range []CPUID{1, 2} {
		gotSpans[cpu] = map[ThreadState][]*threadSpan{}
		for _, span := range running[cpu] {
			gotSpans[cpu][RunningState] = append(gotSpans[cpu][RunningState], span)
		}
		query := &threadSpan{
			startTimestamp: 1000,
			endTimestamp:   1100,
			id:             queryID,
		}
		sleepingSpans := sleeping[cpu].Query(query)
		for _, span := range sleepingSpans {
			gotSpans[cpu][SleepingState] = append(gotSpans[cpu][SleepingState], span.(*threadSpan))
		}
		waitingSpans := waiting[cpu].Query(query)
		for _, span := range waitingSpans {
			gotSpans[cpu][WaitingState] = append(gotSpans[cpu][WaitingState], span.(*threadSpan))
		}
	}
	if !reflect.DeepEqual(gotSpans, wantSpans) {
		t.Fatalf("cpuSpans = \n%#v; want\n%#v", gotSpans, wantSpans)
	}
}
