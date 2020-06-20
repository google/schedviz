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

	"github.com/google/schedviz/tracedata/trace"
)

func emptySpan(pid PID) *threadSpan {
	return &threadSpan{
		pid:            pid,
		startTimestamp: UnknownTimestamp,
		endTimestamp:   UnknownTimestamp,
		cpu:            UnknownCPU,
		id:             queryID,
		priority:       UnknownPriority,
		state:          UnknownState,
		command:        UnknownCommand,
	}
}

func (ts *threadSpan) withTimeRange(startTimestamp, endTimestamp trace.Timestamp) *threadSpan {
	ts.startTimestamp, ts.endTimestamp = startTimestamp, endTimestamp
	return ts
}

func (ts *threadSpan) withCPU(cpu CPUID) *threadSpan {
	ts.cpu = cpu
	return ts
}

func (ts *threadSpan) withTreeID(id uint64) *threadSpan {
	ts.id = id
	return ts
}

func (ts *threadSpan) withPriority(priority Priority) *threadSpan {
	ts.priority = priority
	return ts
}

func (ts *threadSpan) withState(state ThreadState) *threadSpan {
	ts.state = state
	return ts
}

func (ts *threadSpan) withCommand(command stringID) *threadSpan {
	ts.command = command
	return ts
}

func (ts *threadSpan) withDroppedEventIDs(evIDs ...int) *threadSpan {
	ts.droppedEventIDs = append(ts.droppedEventIDs, evIDs...)
	return ts
}

func (ts *threadSpan) withSynthetic(start, end bool) *threadSpan {
	ts.syntheticStart, ts.syntheticEnd = start, end
	return ts
}

func TestSpanGeneration(t *testing.T) {
	var pid PID = 100
	tests := []struct {
		description string
		transitions []*threadTransition
		options     *collectionOptions
		wantErr     bool
		wantSpans   []*threadSpan
	}{{
		description: "span sequence",
		transitions: []*threadTransition{
			emptyTransition(Unknown, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState),
			emptyTransition(0, 1010, pid).
				withCPUs(1, 1).
				withStates(RunningState, WaitingState),
			emptyTransition(1, 1020, pid).
				withCPUs(1, 2).
				withStates(WaitingState, WaitingState),
			// Empty transition at the end from a synthetic final event.
			emptyTransition(Unknown, 1040, pid).
				withCPUs(2, 2).
				withStates(WaitingState, WaitingState),
		},
		options: &collectionOptions{},
		wantErr: false,
		wantSpans: []*threadSpan{
			emptySpan(pid).
				withTimeRange(1000, 1010).
				withCPU(1).
				withState(RunningState),
			emptySpan(pid).
				withTimeRange(1010, 1020).
				withCPU(1).
				withState(WaitingState),
			emptySpan(pid).
				withTimeRange(1020, 1040).
				withCPU(2).
				withState(WaitingState),
		},
	}, {
		description: "zero-length interval",
		transitions: []*threadTransition{
			// Empty transition at the beginning from a synthetic initial event.
			emptyTransition(Unknown, 1000, pid).
				withCPUs(1, 1),
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState),
			emptyTransition(1, 1010, pid).
				withCPUs(1, 1).
				withStates(RunningState, WaitingState),
			emptyTransition(2, 1020, pid).
				withCPUs(1, 2).
				withStates(WaitingState, WaitingState),
			// Empty transition at the end from a synthetic final event.
			emptyTransition(Unknown, 1040, pid).
				withCPUs(2, 2).
				withStates(WaitingState, WaitingState),
		},
		options: &collectionOptions{},
		wantErr: false,
		wantSpans: []*threadSpan{
			emptySpan(pid).
				withTimeRange(1000, 1000).
				withCPU(1),
			emptySpan(pid).
				withTimeRange(1000, 1010).
				withCPU(1).
				withState(RunningState),
			emptySpan(pid).
				withTimeRange(1010, 1020).
				withCPU(1).
				withState(WaitingState),
			emptySpan(pid).
				withTimeRange(1020, 1040).
				withCPU(2).
				withState(WaitingState),
		},
	}, {
		description: "where'd that migrate come from",
		transitions: []*threadTransition{
			// Empty transition at the beginning from a synthetic initial event.
			emptyTransition(Unknown, 1000, pid).
				withCPUs(1, 1),
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState),
			emptyTransition(1, 1010, pid).
				withCPUs(2, 2).
				withStates(RunningState, WaitingState),
			emptyTransition(2, 1020, pid).
				withCPUs(1, 2).
				withStates(WaitingState, WaitingState),
			// Empty transition at the end from a synthetic final event.
			emptyTransition(Unknown, 1040, pid).
				withCPUs(2, 2).
				withStates(WaitingState, WaitingState),
		},
		options: &collectionOptions{},
		wantErr: true,
	}, {
		description: "dropped events get rolled in",
		transitions: []*threadTransition{
			// Empty transition at the beginning from a synthetic initial event.
			emptyTransition(Unknown, 1000, pid).
				withCPUs(1, 1),
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState),
			// spurious wakeup-while-running
			emptyTransition(1, 1010, pid).
				withCPUs(1, 1).
				withStates(WaitingState, WaitingState),
			emptyTransition(2, 1020, pid).
				withCPUs(1, 1).
				withStates(RunningState, WaitingState),
			// Empty transition at the end from a synthetic final event.
			emptyTransition(Unknown, 1040, pid).
				withCPUs(1, 1).
				withStates(WaitingState, WaitingState),
		},
		options: &collectionOptions{},
		wantErr: true,
		wantSpans: []*threadSpan{
			emptySpan(pid).
				withTimeRange(1000, 1000).
				withCPU(1),
			// The dropped transition's event is noted in the span containing it.
			emptySpan(pid).
				withTimeRange(1000, 1020).
				withCPU(1).
				withState(RunningState).
				withDroppedEventIDs(1),
			emptySpan(pid).
				withTimeRange(1020, 1040).
				withCPU(1).
				withState(WaitingState),
		},
	}, {
		description: "merged commands and priorities",
		transitions: []*threadTransition{
			// Empty transition at the beginning from a synthetic initial event.
			emptyTransition(Unknown, 1000, pid).
				withCPUs(1, 1),
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withCommands(1, 1).
				withPriorities(120, 120),
			// thread changes command & prio somehow
			emptyTransition(1, 1010, pid).
				withCPUs(1, 1).
				withStates(RunningState, RunningState).
				withCommands(2, 2).
				withPriorities(150, 150),
			emptyTransition(2, 1020, pid).
				withCPUs(1, 1).
				withStates(RunningState, WaitingState).
				withCommands(2, 2).
				withPriorities(150, 150),
			// Empty transition at the end from a synthetic final event.
			emptyTransition(Unknown, 1040, pid).
				withCPUs(1, 1).
				withStates(WaitingState, WaitingState),
		},
		options: &collectionOptions{},
		wantSpans: []*threadSpan{
			emptySpan(pid).
				withTimeRange(1000, 1000).
				withCPU(1),
			emptySpan(pid).
				withTimeRange(1000, 1020).
				withCPU(1).
				withState(RunningState).
				withCommand(1).
				withPriority(120),
			emptySpan(pid).
				withTimeRange(1020, 1040).
				withCPU(1).
				withState(WaitingState).
				withCommand(1).
				withPriority(120),
		},
	}, {
		description: "split commands and priorities",
		transitions: []*threadTransition{
			// Empty transition at the beginning from a synthetic initial event.
			emptyTransition(Unknown, 1000, pid).
				withCPUs(1, 1),
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withCommands(1, 1).
				withPriorities(120, 120),
			// thread changes command & prio somehow
			emptyTransition(1, 1010, pid).
				withCPUs(1, 1).
				withStates(RunningState, RunningState).
				withCommands(2, 2).
				withPriorities(150, 150),
			emptyTransition(2, 1020, pid).
				withCPUs(1, 1).
				withStates(RunningState, WaitingState).
				withCommands(2, 2).
				withPriorities(150, 150),
			// Empty transition at the end from a synthetic final event.
			emptyTransition(Unknown, 1040, pid).
				withCPUs(1, 1).
				withStates(WaitingState, WaitingState),
		},
		options: &collectionOptions{
			preciseCommands:   true,
			precisePriorities: true,
		},
		wantSpans: []*threadSpan{
			emptySpan(pid).
				withTimeRange(1000, 1000).
				withCPU(1),
			emptySpan(pid).
				withTimeRange(1000, 1010).
				withCPU(1).
				withState(RunningState).
				withCommand(1).
				withPriority(120),
			// New span, same as before except with new command and prio.
			emptySpan(pid).
				withTimeRange(1010, 1020).
				withCPU(1).
				withState(RunningState).
				withCommand(2).
				withPriority(150),
			emptySpan(pid).
				withTimeRange(1020, 1040).
				withCPU(1).
				withState(WaitingState).
				withCommand(2).
				withPriority(150),
		},
	}, {
		description: "spans from synthetic transitions",
		transitions: []*threadTransition{
			emptyTransition(0, 1000, pid).
				withCPUs(1, 1).
				withStates(AnyState, RunningState).
				withCommands(2, 2).
				withPriorities(50, 50),
			emptyTransition(Unknown, 1005, pid).
				withCPUs(1, 2).
				withStates(RunningState, WaitingState).
				isSynthetic(),
			emptyTransition(1, 1010, pid).
				withCPUs(2, 2).
				withStates(WaitingState, RunningState).
				withCPUConflictPolicies(InsertSynthetic, Fail).
				withStateConflictPolicies(InsertSynthetic, Fail),
		},
		options: &collectionOptions{},
		wantSpans: []*threadSpan{
			emptySpan(pid).
				withTimeRange(1000, 1005).
				withCPU(1).
				withState(RunningState).
				withCommand(2).
				withPriority(50).
				withSynthetic(false, true),
			emptySpan(pid).
				withTimeRange(1005, 1010).
				withCPU(2).
				withState(WaitingState).
				withCommand(2).
				withPriority(50).
				withSynthetic(true, false),
			emptySpan(pid).
				withTimeRange(1010, 1010).
				withCPU(2).
				withState(RunningState).
				withCommand(2).
				withPriority(50),
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			var sawErr error
			var spans = []*threadSpan{}
			tsg := newThreadSpanGenerator(pid, test.options)
			for _, tt := range test.transitions {
				if ts, err := tsg.addTransition(tt); err != nil {
					sawErr = err
				} else if ts != nil {
					spans = append(spans, ts)
				}
			}
			if sawErr == nil && test.wantErr {
				t.Fatalf("wanted an addTransition error but got none")
			}
			if sawErr != nil && !test.wantErr {
				t.Fatalf("wanted no addTransition error but got %s", sawErr)
			}
			if sawErr != nil || test.wantErr {
				return
			}
			if ts := tsg.drain(); ts != nil {
				spans = append(spans, ts)
			}
			if len(spans) != len(test.wantSpans) {
				t.Fatalf("Expected %d spans, got %d", len(test.wantSpans), len(spans))
			}
			for i := 0; i < len(spans); i++ {
				got, want := spans[i], test.wantSpans[i]
				if !got.equals(want) {
					t.Errorf("span %d mismatch: want %s, got %s", i, want, got)
				}
			}
		})
	}
}
