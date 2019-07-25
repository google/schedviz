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
	"github.com/google/schedviz/analysis/schedtestcommon"
	"github.com/google/schedviz/tracedata/eventsetbuilder"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/google/schedviz/tracedata/trace"
)

func TestEventLoaderCreation(t *testing.T) {
	tests := []struct {
		description string
		loaders     map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error
		wantErrs    bool
	}{{
		description: "empty loader",
		loaders:     map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{},
		wantErrs:    true,
	}, {
		description: "predefined loaders",
		loaders:     DefaultEventLoaders(),
		wantErrs:    false,
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			_, err := newEventLoader(test.loaders, newStringBank())
			if err == nil && test.wantErrs {
				t.Errorf("newEventLoader yielded no errors, wanted one")
			}
			if err != nil && !test.wantErrs {
				t.Errorf("wanted no EventLoader errors but got %s", err)
			}
		})
	}
}

func TestEventsAreLoaded(t *testing.T) {
	loaders := map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{
		"migrate-n-sleep": func(ev *trace.Event, ttsb *ThreadTransitionSetBuilder) error {
			pid, ok := ev.NumberProperties["pid"]
			if !ok {
				return MissingFieldError("pid", ev)
			}
			comm := ev.TextProperties["comm"]
			prio, ok := ev.NumberProperties["prio"]
			priority := Priority(prio)
			if !ok {
				priority = UnknownPriority
			}
			targetCPU, ok := ev.NumberProperties["target_cpu"]
			if !ok {
				return MissingFieldError("target_cpu", ev)
			}
			ttsb.WithTransition(ev.Index, ev.Timestamp, PID(pid)).
				WithPrevCommand(comm).
				WithNextCommand(comm).
				WithPrevPriority(priority).
				WithNextPriority(priority).
				WithPrevCPU(CPUID(ev.CPU)).
				WithNextCPU(CPUID(targetCPU)).
				WithNextState(SleepingState)
			return nil
		},
		"wake-n-switch-in": func(ev *trace.Event, ttsb *ThreadTransitionSetBuilder) error {
			pid, ok := ev.NumberProperties["pid"]
			if !ok {
				return MissingFieldError("pid", ev)
			}
			comm := ev.TextProperties["comm"]
			prio, ok := ev.NumberProperties["prio"]
			priority := Priority(prio)
			if !ok {
				priority = UnknownPriority
			}
			ttsb.WithTransition(ev.Index, ev.Timestamp, PID(pid)).
				WithPrevCommand(comm).
				WithNextCommand(comm).
				WithPrevPriority(priority).
				WithNextPriority(priority).
				WithPrevCPU(CPUID(ev.CPU)).
				WithNextCPU(CPUID(ev.CPU)).
				WithPrevState(SleepingState).
				WithNextState(RunningState)
			return nil
		}}
	eventSetBase := eventsetbuilder.NewBuilder().
		WithEventDescriptor(
			"migrate-n-sleep",
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio"),
			eventsetbuilder.Number("target_cpu")).
		WithEventDescriptor(
			"wake-n-switch-in",
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio"))
	tests := []struct {
		description     string
		eventSet        *eventpb.EventSet
		wantErr         bool // True if a loader will return an error.
		wantTransitions func(sb *stringBank) []*threadTransition
	}{{
		description: "events load",
		eventSet: eventSetBase.TestClone(t).
			WithEvent("migrate-n-sleep", 1, 1000, false,
				100, "thread 1", 50, 2).
			WithEvent("wake-n-switch-in", 2, 1010, false,
				100, "thread 1", 50).
			TestProtobuf(t),
		wantErr: false,
		wantTransitions: func(sb *stringBank) []*threadTransition {
			return []*threadTransition{
				emptyTransition(0, 1000, 100).
					withCommands(sb.stringIDByString("thread 1"), sb.stringIDByString("thread 1")).
					withPriorities(50, 50).
					withCPUs(1, 2).
					withStates(UnknownState, SleepingState),
				emptyTransition(1, 1010, 100).
					withCommands(sb.stringIDByString("thread 1"), sb.stringIDByString("thread 1")).
					withPriorities(50, 50).
					withCPUs(2, 2).
					withStates(SleepingState, RunningState),
			}
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			sb := newStringBank()
			el, err := newEventLoader(loaders, sb)
			if err != nil {
				t.Fatalf("wanted no EventLoader errors but got %s", err)
			}
			var tts []*threadTransition
			coll, err := trace.NewCollection(test.eventSet)
			if err != nil {
				t.Fatalf("wanted no trace.NewCollection errors but got %s", err)
			}
			for i := 0; i < coll.EventCount(); i++ {
				ev, err := coll.EventByIndex(i)
				if err != nil {
					t.Fatalf("wanted no trace.EventByIndex errors but got %s", err)
				}
				res, err := el.threadTransitions(ev)
				if test.wantErr && err == nil {
					t.Fatalf("threadTransitions yielded no errors, wanted one")
				}
				if err != nil && !test.wantErr {
					t.Fatalf("threadTransitions yielded unepected error %s", err)
				}
				tts = append(tts, res...)
			}
			wantTts := test.wantTransitions(sb)
			if len(tts) != len(wantTts) {
				t.Fatalf("Expected %d transitions, got %d", len(wantTts), len(tts))
			}
			for i := 0; i < len(tts); i++ {
				got, want := tts[i], wantTts[i]
				if !cmp.Equal(want, got, cmp.AllowUnexported(threadTransition{})) {
					t.Errorf("transition mismatch: want %s, got %s", want, got)
				}
			}
		})
	}
}

func TestTestTrace1Transitions(t *testing.T) {
	// Ensure that schedtestcommon.TestTrace1 translates into transitions as
	// expected.
	es := schedtestcommon.TestTrace1(t)
	sb := newStringBank()
	el, err := newEventLoader(DefaultEventLoaders(), sb)
	if err != nil {
		t.Fatalf("wanted no EventLoader errors but got %s", err)
	}
	var tts []*threadTransition
	coll, err := trace.NewCollection(es)
	if err != nil {
		t.Fatalf("wanted no trace.NewCollection errors but got %s", err)
	}
	for i := 0; i < coll.EventCount(); i++ {
		ev, err := coll.EventByIndex(i)
		if err != nil {
			t.Fatalf("wanted no trace.EventByIndex errors but got %s", err)
		}
		res, err := el.threadTransitions(ev)
		if err != nil {
			t.Fatalf("threadTransitions yielded unepected error %s", err)
		}
		tts = append(tts, res...)
	}
	process1ID := sb.stringIDByString("Process1")
	process2ID := sb.stringIDByString("Process2")
	process3ID := sb.stringIDByString("Process3")
	process4ID := sb.stringIDByString("Process4")
	wantTts := []*threadTransition{
		emptyTransition(0, 1000, 300).
			withCommands(process3ID, process3ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(UnknownState, RunningState),
		emptyTransition(0, 1000, 200).
			withCommands(process2ID, process2ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(RunningState, SleepingState),
		emptyTransition(1, 1000, 100).
			withCommands(process1ID, process1ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(UnknownState, WaitingState).
			withCPUConflictPolicies(Drop, Drop).
			withStateConflictPolicies(Fail, Drop),
		emptyTransition(2, 1010, 100).
			withCommands(process1ID, process1ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(UnknownState, RunningState),
		emptyTransition(2, 1010, 300).
			withCommands(process3ID, process3ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(RunningState, SleepingState),
		emptyTransition(3, 1040, 200).
			withCommands(process2ID, process2ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(UnknownState, WaitingState).
			withCPUConflictPolicies(Drop, Drop).
			withStateConflictPolicies(Fail, Drop),
		emptyTransition(4, 1080, 200).
			withCommands(process2ID, process2ID).
			withPriorities(50, 50).
			withCPUs(1, 2).
			withStates(UnknownState, UnknownState),
		emptyTransition(5, 1090, 300).
			withCommands(process3ID, process3ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(UnknownState, WaitingState).
			withCPUConflictPolicies(Drop, Drop).
			withStateConflictPolicies(Fail, Drop),
		emptyTransition(6, 1100, 200).
			withCommands(process2ID, process2ID).
			withPriorities(50, 50).
			withCPUs(2, 2).
			withStates(UnknownState, RunningState),
		emptyTransition(6, 1100, 400).
			withCommands(process4ID, process4ID).
			withPriorities(50, 50).
			withCPUs(2, 2).
			withStates(RunningState, WaitingState),
		emptyTransition(7, 1100, 300).
			withCommands(process3ID, process3ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(UnknownState, RunningState),
		emptyTransition(7, 1100, 100).
			withCommands(process1ID, process1ID).
			withPriorities(50, 50).
			withCPUs(1, 1).
			withStates(RunningState, WaitingState),
	}
	if len(tts) != len(wantTts) {
		for _, tt := range tts {
			t.Log(tt)
		}
		t.Fatalf("Expected %d transitions, got %d", len(wantTts), len(tts))
	}
	for i := 0; i < len(tts); i++ {
		got, want := tts[i], wantTts[i]
		if !cmp.Equal(want, got, cmp.AllowUnexported(threadTransition{})) {
			t.Errorf("transition mismatch: want %#v, got %#v", want, got)
		}
	}
}
