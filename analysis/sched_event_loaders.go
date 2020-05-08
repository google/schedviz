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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	elpb "github.com/google/schedviz/analysis/event_loaders_go_proto"
	"github.com/google/schedviz/tracedata/trace"
)

// LoadSchedMigrateTask loads a sched::sched_migrate_task event.
func LoadSchedMigrateTask(ev *trace.Event, ttsb *ThreadTransitionSetBuilder) error {
	md, err := LoadMigrateData(ev)
	if err != nil {
		return err
	}
	// sched:sched_migrate_task produces a single thread transition, from the PID
	// backwards on the original CPU and forwards on the destination CPU.
	ttsb.WithTransition(ev.Index, ev.Timestamp, md.PID).
		WithPrevCommand(md.Comm).
		WithNextCommand(md.Comm).
		WithPrevPriority(md.Priority).
		WithNextPriority(md.Priority).
		WithPrevCPU(md.OrigCPU).
		WithNextCPU(md.DestCPU).
		WithStatePropagatesThrough(true)
	return nil
}

// LoadSchedMigrateTaskWithDrops loads a sched::sched_migrate_task event.
// Faulty migrates with irreconcilable CPUs will be dropped.
func LoadSchedMigrateTaskWithDrops(ev *trace.Event, ttsb *ThreadTransitionSetBuilder) error {
	md, err := LoadMigrateData(ev)
	if err != nil {
		return err
	}
	// sched:sched_migrate_task produces a single thread transition, from the PID
	// backwards on the original CPU and forwards on the destination CPU.
	ttsb.WithTransition(ev.Index, ev.Timestamp, md.PID).
		WithPrevCommand(md.Comm).
		WithNextCommand(md.Comm).
		WithPrevPriority(md.Priority).
		WithNextPriority(md.Priority).
		WithPrevCPU(md.OrigCPU).
		WithNextCPU(md.DestCPU).
		OnBackwardsCPUConflict(Drop).
		OnForwardsCPUConflict(Drop).
		WithStatePropagatesThrough(true)
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
		WithCPUPropagatesThrough(true).
		WithPrevState(WaitingState | SleepingState).
		WithNextState(RunningState)
	ttsb.WithTransition(ev.Index, ev.Timestamp, sd.PrevPID).
		WithPrevCommand(sd.PrevComm).
		WithNextCommand(sd.PrevComm).
		WithPrevPriority(sd.PrevPriority).
		WithNextPriority(sd.PrevPriority).
		WithPrevCPU(CPUID(ev.CPU)).
		WithNextCPU(CPUID(ev.CPU)).
		WithCPUPropagatesThrough(true).
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
		WithCPUPropagatesThrough(true).
		WithPrevState(AnyState).
		WithNextState(WaitingState).
		OnBackwardsCPUConflict(Drop).
		OnForwardsCPUConflict(Drop).
		OnBackwardsStateConflict(Drop).
		OnForwardsStateConflict(Drop)
	return nil
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
		WithCPUPropagatesThrough(true).
		WithPrevState(SleepingState | WaitingState).
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
		WithCPUPropagatesThrough(true).
		WithPrevState(RunningState).
		WithNextState(sd.PrevState).
		OnForwardsCPUConflict(InsertSynthetic).
		OnBackwardsCPUConflict(InsertSynthetic).
		OnForwardsStateConflict(InsertSynthetic).
		OnBackwardsStateConflict(InsertSynthetic)
	return nil
}

// EventLoaders represents a grouped set of event loaders meant to be used in
// concert to load a given trace.
type EventLoaders map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error

// DefaultEventLoaders is a set of default event loaders operating on
// sched_migrate_task, sched_switch, sched_wakeup, and sched_wakeup_new.
// sched_wakeup events that cannot be reconciled are dropped.
func DefaultEventLoaders() EventLoaders {
	return map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{
		"sched_migrate_task": LoadSchedMigrateTask,
		"sched_switch":       LoadSchedSwitch,
		"sched_wakeup":       LoadSchedWakeup,
		"sched_wakeup_new":   LoadSchedWakeup,
	}
}

// SwitchOnlyLoaders is a set of event loaders operating on only sched_switch.
// CPU and thread  state transitions are inferred and may not be entirely
// accurate.  Running intervals will be entirely accurate, but waiting
// intervals and CPU wait queues may be approximate.
func SwitchOnlyLoaders() EventLoaders {
	return map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{
		"sched_switch": LoadSchedSwitchWithSynthetics,
	}
}

// FaultTolerantEventLoaders is a set of event loaders operating on
// sched_migrate_task, sched_switch, sched_wakeup, and sched_wakeup_new, and
// suitable for use on traces that may be incomplete or have out-of-order
// events.  sched_migrate events whose CPUs cannot be reconciled are dropped;
// sched_wakeup* events that cannot be reconciled are dropped, and unattested
// CPU and thread state transitions between sched_switch events are inferred.
func FaultTolerantEventLoaders() EventLoaders {
	return map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error{
		"sched_migrate_task": LoadSchedMigrateTaskWithDrops,
		"sched_switch":       LoadSchedSwitchWithSynthetics,
		"sched_wakeup":       LoadSchedWakeup,
		"sched_wakeup_new":   LoadSchedWakeup,
	}
}

// EventLoader returns the event loader specified by the provided LoaderType.
func EventLoader(elt elpb.LoadersType) (EventLoaders, error) {
	switch elt {
	case elpb.LoadersType_DEFAULT:
		return DefaultEventLoaders(), nil
	case elpb.LoadersType_SWITCH_ONLY:
		return SwitchOnlyLoaders(), nil
	case elpb.LoadersType_FAULT_TOLERANT:
		return FaultTolerantEventLoaders(), nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown event loader type %v", elt)
	}
}
