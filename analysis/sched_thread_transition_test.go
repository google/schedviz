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
	"fmt"
	"testing"

	"github.com/google/schedviz/tracedata/trace"
)

func TestConflictResolution(t *testing.T) {
	tests := []struct {
		a, b ConflictPolicy
		want ConflictPolicy
	}{
		{Fail, Fail, Fail},
		{Fail, Drop, Drop},
		{Fail, InsertSynthetic, Fail},
		{Fail, DropOrInsertSynthetic, Drop},
		{Drop, Fail, Drop},
		{Drop, Drop, Drop},
		{Drop, InsertSynthetic, Drop},
		{Drop, DropOrInsertSynthetic, Drop},
		{InsertSynthetic, Fail, Fail},
		{InsertSynthetic, Drop, Drop},
		{InsertSynthetic, InsertSynthetic, InsertSynthetic},
		{InsertSynthetic, DropOrInsertSynthetic, InsertSynthetic},
		{DropOrInsertSynthetic, Fail, Drop},
		{DropOrInsertSynthetic, Drop, Drop},
		{DropOrInsertSynthetic, InsertSynthetic, InsertSynthetic},
		{DropOrInsertSynthetic, DropOrInsertSynthetic, Drop},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("resolveConflict(%s, %s)", test.a, test.b), func(t *testing.T) {
			got := resolveConflict(test.a, test.b)
			if got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

// emptyTransition returns a *threadTransition with the provided event ID,
// timestamp, and PID, and with all Command, Priority, CPU, and State fields
// Unknown, all boolean fields false, and all conflict policies Fail.
func emptyTransition(eventID int, timestamp trace.Timestamp, pid PID) *threadTransition {
	return &threadTransition{
		EventID:                  eventID,
		Timestamp:                timestamp,
		PID:                      pid,
		PrevCommand:              UnknownCommand,
		NextCommand:              UnknownCommand,
		PrevPriority:             UnknownPriority,
		NextPriority:             UnknownPriority,
		PrevCPU:                  UnknownCPU,
		NextCPU:                  UnknownCPU,
		PrevState:                AnyState,
		NextState:                AnyState,
		StatePropagatesThrough:   false,
		onForwardsStateConflict:  Fail,
		onBackwardsStateConflict: Fail,
		onForwardsCPUConflict:    Fail,
		onBackwardsCPUConflict:   Fail,
	}
}

func (tt *threadTransition) withCommands(prev, next stringID) *threadTransition {
	tt.PrevCommand = prev
	tt.NextCommand = next
	return tt
}

func (tt *threadTransition) withPriorities(prev, next Priority) *threadTransition {
	tt.PrevPriority = prev
	tt.NextPriority = next
	return tt
}

func (tt *threadTransition) withCPUs(prev, next CPUID) *threadTransition {
	tt.PrevCPU = prev
	tt.NextCPU = next
	return tt
}

func (tt *threadTransition) withCPUPropagatesThrough(cpuPropagatesThrough bool) *threadTransition {
	tt.CPUPropagatesThrough = cpuPropagatesThrough
	return tt
}

func (tt *threadTransition) withStates(prev, next ThreadState) *threadTransition {
	tt.PrevState = prev
	tt.NextState = next
	return tt
}

func (tt *threadTransition) withStatePropagatesThrough(statePropagatesThrough bool) *threadTransition {
	tt.StatePropagatesThrough = statePropagatesThrough
	return tt
}

func (tt *threadTransition) withStateConflictPolicies(backwards, forwards ConflictPolicy) *threadTransition {
	tt.onForwardsStateConflict = forwards
	tt.onBackwardsStateConflict = backwards
	return tt
}

func (tt *threadTransition) withCPUConflictPolicies(backwards, forwards ConflictPolicy) *threadTransition {
	tt.onForwardsCPUConflict = forwards
	tt.onBackwardsCPUConflict = backwards
	return tt
}

func (tt *threadTransition) drop() *threadTransition {
	tt.dropped = true
	return tt
}

func (tt *threadTransition) isSynthetic() *threadTransition {
	tt.synthetic = true
	return tt
}

func TestIsForwardBarrier(t *testing.T) {
	tests := []struct {
		description string
		transition  *threadTransition
		want        bool
	}{{
		description: "valid forward barrier",
		transition: emptyTransition(0, 1000, 100).
			withCPUs(UnknownCPU, 0).
			withStates(AnyState, RunningState),
		want: true,
	}, {
		description: "valid forward barrier with prior drops",
		transition: emptyTransition(0, 1000, 100).
			withCPUs(10, 0).
			withStates(SleepingState, WaitingState).
			withStateConflictPolicies(Drop, Fail).
			withCPUConflictPolicies(Drop, Fail),
		want: true,
	}, {
		description: "missing next CPU",
		transition: emptyTransition(0, 1000, 100).
			withStates(AnyState, RunningState),
		want: false,
	}, {
		description: "drop if forward CPU fails",
		transition: emptyTransition(0, 1000, 100).
			withCPUs(UnknownCPU, 10).
			withStates(AnyState, RunningState).
			withStateConflictPolicies(Fail, Drop),
		want: false,
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if got := test.transition.isForwardBarrier(); got != test.want {
				t.Errorf("transition.isForwardBarrier() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestThreadTransition(t *testing.T) {
	tests := []struct {
		description          string
		transition           *threadTransition
		forwards             bool
		cpu                  CPUID
		wantCPUPropagation   bool
		state                ThreadState
		wantStatePropagation bool
		wantTransition       *threadTransition
	}{{
		description: "infer through forwards",
		transition: emptyTransition(0, 1000, 100).
			withCPUPropagatesThrough(true).
			withStatePropagatesThrough(true),
		forwards:             true,
		cpu:                  10,
		wantCPUPropagation:   true,
		state:                RunningState,
		wantStatePropagation: true,
		wantTransition: emptyTransition(0, 1000, 100).
			withCPUs(10, 10).
			withCPUPropagatesThrough(true).
			withStates(RunningState, RunningState).
			withStatePropagatesThrough(true),
	}, {
		description: "partial inference forwards",
		transition: emptyTransition(0, 1000, 100).
			withCPUs(UnknownCPU, 5).
			withStates(AnyState, WaitingState).
			withStatePropagatesThrough(false),
		forwards:             true,
		cpu:                  10,
		wantCPUPropagation:   true,
		state:                RunningState,
		wantStatePropagation: true,
		wantTransition: emptyTransition(0, 1000, 100).
			withCPUs(10, 5).
			withStates(RunningState, WaitingState).
			withStatePropagatesThrough(false),
	}, {description: "infer through backwards",
		transition: emptyTransition(0, 1000, 100).
			withCPUPropagatesThrough(true).
			withStatePropagatesThrough(true),
		forwards:             false,
		cpu:                  10,
		wantCPUPropagation:   true,
		state:                RunningState,
		wantStatePropagation: true,
		wantTransition: emptyTransition(0, 1000, 100).
			withCPUs(10, 10).
			withCPUPropagatesThrough(true).
			withStates(RunningState, RunningState).
			withStatePropagatesThrough(true),
	}, {
		description: "partial inference backwards",
		transition: emptyTransition(0, 1000, 100).
			withCPUs(5, UnknownCPU).
			withStates(WaitingState, AnyState).
			withStatePropagatesThrough(false),
		forwards:             false,
		cpu:                  10,
		wantCPUPropagation:   true,
		state:                RunningState,
		wantStatePropagation: true,
		wantTransition: emptyTransition(0, 1000, 100).
			withCPUs(5, 10).
			withStates(WaitingState, RunningState).
			withStatePropagatesThrough(false),
	}, {
		description: "conflicting backwards CPU fails",
		transition: emptyTransition(0, 1000, 100).
			withCPUs(UnknownCPU, 5).
			withStates(AnyState, WaitingState).
			withStatePropagatesThrough(false),
		forwards:             false,
		cpu:                  10,
		wantCPUPropagation:   false,
		state:                AnyState,
		wantStatePropagation: true,
		wantTransition: emptyTransition(0, 1000, 100).
			withCPUs(UnknownCPU, 5).
			withStates(AnyState, WaitingState),
	}, {
		description: "conflicting forwards CPU fails",
		transition: emptyTransition(0, 1000, 100).
			withCPUs(5, UnknownCPU).
			withStates(AnyState, WaitingState).
			withStatePropagatesThrough(false),
		forwards:             true,
		cpu:                  10,
		wantCPUPropagation:   false,
		state:                AnyState,
		wantStatePropagation: true,
		wantTransition: emptyTransition(0, 1000, 100).
			withCPUs(5, UnknownCPU).
			withStates(AnyState, WaitingState).
			withStatePropagatesThrough(false),
	}, {
		description: "conflicting backwards state fails",
		transition: emptyTransition(0, 1000, 100).
			withCPUPropagatesThrough(true).
			withStates(AnyState, WaitingState).
			withStatePropagatesThrough(false),
		forwards:             false,
		cpu:                  UnknownCPU,
		wantCPUPropagation:   true,
		state:                RunningState,
		wantStatePropagation: false,
		wantTransition: emptyTransition(0, 1000, 100).
			withCPUPropagatesThrough(true).
			withStates(AnyState, WaitingState).
			withStatePropagatesThrough(false),
	}, {
		description: "conflicting forwards state fails",
		transition: emptyTransition(0, 1000, 100).
			withCPUPropagatesThrough(true).
			withStates(WaitingState, AnyState).
			withStatePropagatesThrough(false),
		forwards:             true,
		cpu:                  UnknownCPU,
		wantCPUPropagation:   true,
		state:                RunningState,
		wantStatePropagation: false,
		wantTransition: emptyTransition(0, 1000, 100).
			withCPUPropagatesThrough(true).
			withStates(WaitingState, AnyState).
			withStatePropagatesThrough(false),
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			var gotCPUPropagation, gotStatePropagation bool
			if test.forwards {
				gotCPUPropagation = test.transition.setCPUForwards(test.cpu)
				gotStatePropagation = test.transition.setStateForwards(test.state)
			} else {
				gotCPUPropagation = test.transition.setCPUBackwards(test.cpu)
				gotStatePropagation = test.transition.setStateBackwards(test.state)
			}
			if gotCPUPropagation != test.wantCPUPropagation {
				t.Errorf("wanted CPU propagation result %t, got %t", test.wantCPUPropagation, gotCPUPropagation)
			}
			if gotStatePropagation != test.wantStatePropagation {
				t.Errorf("wanted state propagation result %t, got %t", test.wantStatePropagation, gotStatePropagation)
			}
			if test.transition.String() != test.wantTransition.String() {
				t.Errorf("set yielded unexpected transition: got %s, want %s", test.transition, test.wantTransition)
			}
		})
	}
}
