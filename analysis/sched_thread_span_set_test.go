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
)

// Test that dropped events are reported correctly.
func TestDroppedEvents(t *testing.T) {
	pid := PID(100)
	transitions := []*threadTransition{
		// Empty transition at the beginning from a synthetic initial event.
		emptyTransition(Unknown, 1000, pid).
			withCPUs(1, 1),
		emptyTransition(0, 1000, pid).
			withCPUs(1, 1).
			withStates(UnknownState, RunningState),
		// spurious wakeup-while-running
		emptyTransition(1, 1010, pid).
			withCPUs(1, 1).
			withStates(WaitingState, WaitingState).
			drop(),
		emptyTransition(2, 1020, pid).
			withCPUs(1, 1).
			withStates(RunningState, WaitingState),
		// Empty transition at the end from a synthetic final event.
		emptyTransition(Unknown, 1040, pid).
			withCPUs(1, 1).
			withStates(WaitingState, WaitingState),
	}
	tss := newThreadSpanSet(1000, &collectionOptions{})
	for _, tt := range transitions {
		if err := tss.addTransition(tt); err != nil {
			t.Fatalf("Unexpected error from addTransition: %s", err)
		}
	}
	wantDroppedEventCountsByID := map[int]int{1: 1}
	if diff := cmp.Diff(tss.droppedEventCountsByID, wantDroppedEventCountsByID); diff != "" {
		t.Fatalf("Unexpected dropped event counts by ID: want %v, got %v", tss.droppedEventCountsByID, wantDroppedEventCountsByID)
	}
}

// Test that TestTrace1's raw transitions (stripped of command and prio) are
// properly bracketed at start-and-end timestamps, inferred, and allocated
// per-PID.
func TestSpanDistributionByPID(t *testing.T) {
	transitions := []*threadTransition{
		emptyTransition(0, 1000, 300).
			withCPUs(1, 1).
			withStates(UnknownState, RunningState),
		emptyTransition(0, 1000, 200).
			withCPUs(1, 1).
			withStates(RunningState, SleepingState),
		emptyTransition(1, 1000, 100).
			withCPUs(1, 1).
			withStates(UnknownState, WaitingState).
			withCPUConflictPolicies(Drop, Drop).
			withStateConflictPolicies(Fail, Drop),
		emptyTransition(2, 1010, 100).
			withCPUs(1, 1).
			withStates(UnknownState, RunningState),
		emptyTransition(2, 1010, 300).
			withCPUs(1, 1).
			withStates(RunningState, SleepingState),
		emptyTransition(3, 1040, 200).
			withCPUs(1, 1).
			withStates(UnknownState, WaitingState).
			withCPUConflictPolicies(Drop, Drop).
			withStateConflictPolicies(Fail, Drop),
		emptyTransition(4, 1080, 200).
			withCPUs(1, 2).
			withStates(UnknownState, UnknownState),
		emptyTransition(5, 1090, 300).
			withCPUs(1, 1).
			withStates(UnknownState, WaitingState).
			withCPUConflictPolicies(Drop, Drop).
			withStateConflictPolicies(Fail, Drop),
		emptyTransition(6, 1100, 200).
			withCPUs(2, 2).
			withStates(UnknownState, RunningState),
		emptyTransition(6, 1100, 400).
			withCPUs(2, 2).
			withStates(RunningState, WaitingState),
		emptyTransition(7, 1100, 300).
			withCPUs(1, 1).
			withStates(UnknownState, RunningState),
		emptyTransition(7, 1100, 100).
			withCPUs(1, 1).
			withStates(RunningState, WaitingState),
	}
	wantSpans := map[PID][]*threadSpan{
		100: {
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
		},
		200: {
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
		},
		300: {
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
		},
		400: {
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
		},
	}
	tss := newThreadSpanSet(1000, &collectionOptions{})
	for _, tt := range transitions {
		if err := tss.addTransition(tt); err != nil {
			t.Fatalf("Unexpected error from addTransition: %s", err)
		}
	}
	gotSpans, err := tss.threadSpans(1100)
	if err != nil {
		t.Fatalf("Unexpected error from threadSpans: %s", err)
	}
	if !reflect.DeepEqual(gotSpans, wantSpans) {
		t.Fatalf("threadSpans = \n%#v; want\n%#v", gotSpans, wantSpans)
	}
}
