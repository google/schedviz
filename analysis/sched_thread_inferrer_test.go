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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInference(t *testing.T) {
	var pid PID = 100
	tests := []struct {
		description     string
		transitions     []*threadTransition
		wantErr         bool
		wantTransitions []*threadTransition
	}{{
		description: "infer forwards",
		transitions: []*threadTransition{
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(WaitingState|SleepingState, RunningState),
			emptyTransition(1, 1010, pid).withStatePropagatesThrough(true),
		},
		wantErr: false,
		wantTransitions: []*threadTransition{
			emptyTransition(0, 1000, 100).
				withCPUs(1, 1).
				withStates(WaitingState|SleepingState, RunningState),
			emptyTransition(1, 1010, 100).
				withCPUs(1, 1).
				withStates(RunningState, RunningState).
				withStatePropagatesThrough(true),
		},
	}, {
		description: "infer backwards",
		transitions: []*threadTransition{
			emptyTransition(0, 1000, pid).
				withStatePropagatesThrough(true),
			emptyTransition(1, 1010, pid).
				withCPUs(1, 1).
				withStates(RunningState, SleepingState),
		},
		wantErr: false,
		wantTransitions: []*threadTransition{
			emptyTransition(0, 1000, 100).
				withCPUs(1, 1).
				withStates(RunningState, RunningState).
				withStatePropagatesThrough(true),
			emptyTransition(1, 1010, 100).
				withCPUs(1, 1).
				withStates(RunningState, SleepingState),
		},
	}, {
		description: "inference error",
		transitions: []*threadTransition{
			emptyTransition(0, 1000, pid).
				withStates(AnyState, WaitingState),
			emptyTransition(1, 1010, pid).
				withStates(RunningState, AnyState),
		},
		wantErr: true,
	}, {
		description: "drop on conflict",
		transitions: []*threadTransition{
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState),
			emptyTransition(1, 1010, pid).
				withCPUs(2, UnknownCPU).
				withCPUConflictPolicies(Drop, Fail),
			emptyTransition(2, 1020, pid).
				withStatePropagatesThrough(true),
		},
		wantErr: false,
		wantTransitions: []*threadTransition{
			emptyTransition(0, 1000, 100).
				withCPUs(1, 1).
				withStates(AnyState, RunningState),
			emptyTransition(1, 1010, 100).
				withCPUs(2, UnknownCPU).
				withCPUConflictPolicies(Drop, Fail).
				drop(),
			emptyTransition(2, 1020, 100).
				withCPUs(1, 1).
				withStates(RunningState, RunningState).
				withStatePropagatesThrough(true),
		},
	}, {
		description: "insert synthetic on CPU conflict",
		transitions: []*threadTransition{
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withCPUConflictPolicies(Fail, InsertSynthetic),
			emptyTransition(1, 1010, pid).
				withCPUs(2, UnknownCPU).
				withCPUConflictPolicies(InsertSynthetic, Fail).
				withStatePropagatesThrough(true),
		},
		wantErr: false,
		wantTransitions: []*threadTransition{
			emptyTransition(0, 1000, 100).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withCPUConflictPolicies(Fail, InsertSynthetic),
			emptyTransition(Unknown, 1005, 100).
				withCPUs(1, 2).
				withStates(RunningState, RunningState).
				withStatePropagatesThrough(true).
				isSynthetic(),
			emptyTransition(1, 1010, 100).
				withCPUs(2, UnknownCPU).
				withStates(RunningState, RunningState).
				withCPUConflictPolicies(InsertSynthetic, Fail).
				withStatePropagatesThrough(true),
		},
	}, {
		description: "insert synthetic on state conflict",
		transitions: []*threadTransition{
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withStateConflictPolicies(Fail, InsertSynthetic),
			emptyTransition(1, 1010, pid).
				withStates(WaitingState, RunningState).
				withStateConflictPolicies(InsertSynthetic, Fail),
		},
		wantErr: false,
		wantTransitions: []*threadTransition{
			emptyTransition(0, 1000, 100).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withStateConflictPolicies(Fail, InsertSynthetic),
			emptyTransition(Unknown, 1005, 100).
				withCPUs(1, 1).
				withStates(RunningState, WaitingState).
				isSynthetic(),
			emptyTransition(1, 1010, 100).
				withCPUs(1, 1).
				withStates(WaitingState, RunningState).
				withStateConflictPolicies(InsertSynthetic, Fail),
		},
	}, {
		description: "insert multiple synthetic",
		transitions: []*threadTransition{
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withCPUConflictPolicies(Fail, InsertSynthetic).
				withStateConflictPolicies(Fail, InsertSynthetic),
			emptyTransition(1, 1010, pid).
				withCPUs(2, 2).
				withStates(WaitingState, RunningState).
				withCPUConflictPolicies(InsertSynthetic, Fail).
				withStateConflictPolicies(InsertSynthetic, Fail),
		},
		wantErr: false,
		wantTransitions: []*threadTransition{
			emptyTransition(0, 1000, 100).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withCPUConflictPolicies(Fail, InsertSynthetic).
				withStateConflictPolicies(Fail, InsertSynthetic),
			emptyTransition(Unknown, 1005, 100).
				withCPUs(1, 2).
				withStates(RunningState, WaitingState).
				isSynthetic(),
			emptyTransition(1, 1010, 100).
				withCPUs(2, 2).
				withStates(WaitingState, RunningState).
				withCPUConflictPolicies(InsertSynthetic, Fail).
				withStateConflictPolicies(InsertSynthetic, Fail),
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			var sawErr error
			var inferredTts = []*threadTransition{}
			ti := newThreadInferrer(pid, &collectionOptions{})
			for _, uninferredTt := range test.transitions {
				res, err := ti.addTransition(uninferredTt)
				if err != nil {
					sawErr = err
				}
				inferredTts = append(inferredTts, res...)
			}
			res, err := ti.drain()
			if err != nil {
				sawErr = err
			}
			if sawErr == nil && test.wantErr {
				t.Fatalf("wanted an addTransition error but got none")
			}
			if sawErr != nil && !test.wantErr {
				t.Fatalf("wanted no addTransition error but got %s", sawErr)
			}
			inferredTts = append(inferredTts, res...)
			if len(inferredTts) != len(test.wantTransitions) {
				t.Fatalf("Expected %d transitions, got %d", len(test.wantTransitions), len(inferredTts))
			}
			for i := 0; i < len(inferredTts); i++ {
				got, want := inferredTts[i], test.wantTransitions[i]
				if !cmp.Equal(want, got, cmp.AllowUnexported(threadTransition{})) {
					t.Errorf("inferred transition mismatch: want %#v, got %#v", want, got)
				}
			}
		})
	}
}
