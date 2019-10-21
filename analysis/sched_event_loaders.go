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
	"github.com/google/schedviz/tracedata/trace"
)

// LoadSchedMigrateTask loads a sched::sched_migrate_task event.
func LoadSchedMigrateTask(ev *trace.Event, ttsb *ThreadTransitionSetBuilder) error {
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
	origCPU, ok := ev.NumberProperties["orig_cpu"]
	if !ok {
		return MissingFieldError("orig_cpu", ev)
	}
	destCPU, ok := ev.NumberProperties["dest_cpu"]
	if !ok {
		return MissingFieldError("dest_cpu", ev)
	}
	// sched:sched_migrate_task produces a single thread transition, from the PID
	// backwards on the original CPU and forwards on the destination CPU.
	ttsb.WithTransition(ev.Index, ev.Timestamp, PID(pid)).
		WithPrevCommand(comm).
		WithNextCommand(comm).
		WithPrevPriority(priority).
		WithNextPriority(priority).
		WithPrevCPU(CPUID(origCPU)).
		WithNextCPU(CPUID(destCPU))
	return nil
}

// LoadSchedSwitch loads a sched::sched_switch event.
func LoadSchedSwitch(ev *trace.Event, ttsb *ThreadTransitionSetBuilder) error {
	sd, err := LoadSwitchData(ev)
	if err != nil {
		return err
	}
	// sched:sched_switch produces two thread transitions:
	// * The next PID backwards and forwards on the reporting CPU and forwards in
	//   Running state,
	// * The previous PID backwards and forwards on the reporting CPU, backwards
	//   in Running state, and forwards in Sleeping or Waiting state, depending on
	//   its prev_state.
	ttsb.WithTransition(ev.Index, ev.Timestamp, sd.NextPID).
		WithPrevCommand(sd.NextComm).
		WithNextCommand(sd.NextComm).
		WithPrevPriority(sd.NextPriority).
		WithNextPriority(sd.NextPriority).
		WithPrevCPU(CPUID(ev.CPU)).
		WithNextCPU(CPUID(ev.CPU)).
		WithNextState(RunningState)
	ttsb.WithTransition(ev.Index, ev.Timestamp, sd.PrevPID).
		WithPrevCommand(sd.PrevComm).
		WithNextCommand(sd.PrevComm).
		WithPrevPriority(sd.PrevPriority).
		WithNextPriority(sd.PrevPriority).
		WithPrevCPU(CPUID(ev.CPU)).
		WithNextCPU(CPUID(ev.CPU)).
		WithPrevState(RunningState).
		WithNextState(sd.PrevState)
	return nil
}

// LoadSchedWakeup loads a sched::sched_wakeup or sched::sched_wakeup_new event.
func LoadSchedWakeup(ev *trace.Event, ttsb *ThreadTransitionSetBuilder) error {
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
	// sched:sched_wakeup and sched:sched_wakeup_new produce a single thread
	// transition, which expects the targetCPU both before and after the
	// transition, and expects the state after the transition to be Waiting.
	//
	// sched_wakeups are quite prone to misbehavior.  They are frequently produced
	// as part of an interrupt, so they may appear misordered relative to other
	// events, and they can be reported by a different CPU than their target CPU.
	// Moreover, wakeups can occur on threads that are already running.
	// Therefore, all assertions sched_wakeup transitions make -- CPU backwards
	// and forwards, and state forwards -- are relaxed, such that sched_wakeups
	// that disagree with other events on these assertions are dropped.
	ttsb.WithTransition(ev.Index, ev.Timestamp, PID(pid)).
		WithPrevCommand(comm).
		WithNextCommand(comm).
		WithPrevPriority(priority).
		WithNextPriority(priority).
		WithPrevCPU(CPUID(targetCPU)).
		WithNextCPU(CPUID(targetCPU)).
		WithNextState(WaitingState).
		OnBackwardsCPUConflict(Drop).
		OnForwardsCPUConflict(Drop).
		OnForwardsStateConflict(Drop)
	return nil
}

// DefaultEventLoaders is a set of event loader functions for standard
// scheduling tracepoints.
func DefaultEventLoaders() map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error {
	return map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{
		"sched_migrate_task": LoadSchedMigrateTask,
		"sched_switch":       LoadSchedSwitch,
		"sched_wakeup":       LoadSchedWakeup,
		"sched_wakeup_new":   LoadSchedWakeup,
	}
}

// LoadSchedSwitchWithSynthetics loads a sched::sched_switch event from a trace
// that lacks other events that could signal thread state or CPU changes.
// Wherever a state or CPU transition is missing, a synthetic transition will
// be inserted midway between the two adjacent known transitions.
func LoadSchedSwitchWithSynthetics(ev *trace.Event, ttsb *ThreadTransitionSetBuilder) error {
	sd, err := LoadSwitchData(ev)
	if err != nil {
		return err
	}
	ttsb.WithTransition(ev.Index, ev.Timestamp, sd.NextPID).
		WithPrevCommand(sd.NextComm).
		WithNextCommand(sd.NextComm).
		WithPrevPriority(sd.NextPriority).
		WithNextPriority(sd.NextPriority).
		WithPrevCPU(CPUID(ev.CPU)).
		WithNextCPU(CPUID(ev.CPU)).
		WithPrevState(WaitingState).
		WithNextState(RunningState).
		OnForwardsCPUConflict(InsertSynthetic).
		OnBackwardsCPUConflict(InsertSynthetic).
		OnForwardsStateConflict(InsertSynthetic).
		OnBackwardsStateConflict(InsertSynthetic)
	ttsb.WithTransition(ev.Index, ev.Timestamp, sd.PrevPID).
		WithPrevCommand(sd.PrevComm).
		WithNextCommand(sd.PrevComm).
		WithPrevPriority(sd.PrevPriority).
		WithNextPriority(sd.PrevPriority).
		WithPrevCPU(CPUID(ev.CPU)).
		WithNextCPU(CPUID(ev.CPU)).
		WithPrevState(RunningState).
		WithNextState(sd.PrevState).
		OnForwardsCPUConflict(InsertSynthetic).
		OnBackwardsCPUConflict(InsertSynthetic).
		OnForwardsStateConflict(InsertSynthetic).
		OnBackwardsStateConflict(InsertSynthetic)
	return nil
}

// SwitchOnlyLoaders is a set of loaders suitable for use on traces in
// which scheduling behavior is only attested by sched_switch events.
func SwitchOnlyLoaders() map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error {
	return map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{
		"sched_switch": LoadSchedSwitchWithSynthetics,
	}
}
