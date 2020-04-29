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

	"github.com/google/schedviz/tracedata/trace"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ConflictPolicy sets a policy to use when two threadTransitions are found
// to be in conflict with one another regarding a thread's state or CPU during
// inference.  Each transition has separate policies for forward-state,
// backward-state, forward-CPU, and backward-CPU conflicts.  If two adjacent
// threadTransitions (A and B in temporal order) conflict -- say, if A's
// NextState conflicts with B's PrevState -- then the threadTransition's
// appropriate ConflictPolicies are compared, and the resulting policy
// implemented.
// During inference, transitions are accumulated in increasing temporal order
// until a 'forward barrier' transition is reached -- one which has a fixed
// forward state (with known NextCPU and NextState fields) and which cannot
// be dropped on any forward conflict.  When a forward-barrier transition is
// discovered, all conflicts prior to that transition can be resolved,
// conversely, if no forward-barrier transition is discovered before the end
// of the transition stream, then all the thread's transitions must be kept in
// memory and must be reexamined upon any conflict.
// Accordingly, the choice of conflict policy can significantly affect
// collection inference time: if few events are forward barriers, then
// inference will be more complex.
type ConflictPolicy int

const (
	// Fail specifies that conflicting thread state or CPU should
	// yield a collection failure.
	Fail ConflictPolicy = 0
	// Drop specifies that a threadTransition in conflict with its neighbor on
	// thread state or CPU may be dropped.
	Drop = 1
	// InsertSynthetic specifies that two threadTransitions that are in conflict
	// on thread state or CPU may be resolved by inserting a synthetic
	// threadTransition between them.  Synthetic transitions are only inserted if
	// both conflictees agree on this action.
	InsertSynthetic = 2
	// DropOrInsertSynthetic specifies that threadTransition conflicts may be
	// resolved either by dropping conflicting events or inserting synthetic
	// threadTransitions.
	DropOrInsertSynthetic = Drop | InsertSynthetic
)

func (policy ConflictPolicy) String() string {
	switch policy {
	case Fail:
		return "Fail"
	case Drop:
		return "Drop"
	case InsertSynthetic:
		return "Insert Synthetic"
	case DropOrInsertSynthetic:
		return "Drop or Insert Synthetic"
	default:
		return "UNKNOWN"
	}
}

// resolveConflict compares the provided ConflictPolicies and returns the
// resolution, with the provided permitted policy applied.  The resolution
// must be a single policy: Fail, Drop, or InsertSynthetic.
// ConflictPolicies are ranked by strictness, with Drop being strictest,
// InsertSynthetic being next strict, and Fail being least strict.  The
// strictest policy satisfying both conflictants is returned.  Note that
// if one policy is Drop and the other is InsertSynthetic, the result devolves
// to Fail.
func resolveConflict(a, b ConflictPolicy) ConflictPolicy {
	if a > b {
		a, b = b, a
	}
	var result ConflictPolicy = -1
	switch {
	case a == b:
		// If the two policies are the same, we have concord.
		result = a
	case a == Fail && (b&Drop == Drop):
		// If one policy is Fail and the other includes Drop, we can drop.
		result = Drop
	case a == Drop && (b&InsertSynthetic == InsertSynthetic):
		// If one policy is Drop and the other includes InsertSynthetic,
		// we can drop.
		result = Drop
	default:
		result = a & b
	}
	if result == DropOrInsertSynthetic {
		result = Drop
	}
	return result
}

// threadTransition represents a transition of a thread's state or CPU at a
// moment of time.  Previous or next state or CPU may be inferred from other
// threadTransitions.
//
// Forwards inferences propagate known values in the direction of increasing
// timestamp, replacing all Unknown fields until a known value (which may
// disagree with the propagating value) is encountered.
//
// Backwards inferences propagate known values in the direction of decreasing
// timestamp. replacing all Unknown fields until a known value (which may
// disagree with the propagating value) is encountered.
//
// Ideally, such inference disagreements would not occur.  However, as a
// noncritical monitoring mechanism, tracepoints are occasionally prone to such
// hijinks, particularly as tracepoint events are not emitted atomically with
// the phenomenon they describe.  In the case of a disagreement, there are
// three recourses:
//
//  1. We may raise an inference error, signalling that the trace did not
//     behave as expected.
//  2. We may add synthetic threadTransitions between the disagreeing
//     transitions to rectify the disagreement.  We can estimate the time of
//     such synthetic threadTransitions as lying exactly between the
//     disagreers.
//  3. We may permit threadTransitions to be dropped if they disagree with
//     their neighbors.
type threadTransition struct {
	// The index of the trace.Event that produced this threadTransition.  If
	// Unknown, there is no such Event: this is then a synthetic threadTransition
	// representing, for instance, inferred trace-initial thread state or inferred
	// migration.
	EventID   int
	Timestamp trace.Timestamp
	// The PID described in this threadTransition.
	PID PID
	// The command name for PID prior to this threadTransition.
	PrevCommand stringID
	// The command name for PID after this threadTransition.
	NextCommand stringID
	// The priority for PID prior to this threadTransition.
	PrevPriority Priority
	// The priority for PID after this threadTransition.
	NextPriority Priority
	// The CPU on which PID was located prior to this threadTransition.  If
	// Unknown, may be inferred from other threadTransitions.
	PrevCPU CPUID
	// The CPU on which PID was located after this threadTransition.  If Unknown,
	// may be inferred from other threadTransitions.
	NextCPU CPUID
	// The state PID may have held prior to this threadTransition.
	PrevState ThreadState
	// The state PID held after this threadTransition.  If Unknown, may be
	// inferred from other threadTransitions.
	NextState ThreadState
	// Whether states can propagate through this transition during inference.
	// This should be true for events that do not affect a thread's state, and
	// false for events that do.
	StatePropagatesThrough bool
	// Conflict resolution policies.  Some events are unreliable; for example,
	// sched_wakeup can occur on a running or waiting thread.  Events that can be
	// emitted as part of an interrupt are perhaps more prone to require these
	// directives.
	onForwardsStateConflict  ConflictPolicy
	onBackwardsStateConflict ConflictPolicy
	onForwardsCPUConflict    ConflictPolicy
	onBackwardsCPUConflict   ConflictPolicy
	// True if this threadTransition was dropped due to a conflict detected during
	// inference.
	dropped bool
	// True if this is a synthetic threadTransition inserted to resolve a
	// conflict detected during inference.
	synthetic bool
}

// A threadTransition is a 'forward barrier' if its next CPU and state are
// known, and the threadTransition may not be dropped on forward inference
// errors.  Forward barriers mark the point at which no subsequent transition
// can conflict with a prior transition, and therefore we perform bulk
// inference passes on groups of adjacent transitions which start with, and run
// up to just prior to, forward barriers.
func (tt *threadTransition) isForwardBarrier() bool {
	return tt.NextCPU != UnknownCPU && tt.NextState.isKnown() &&
		(tt.onForwardsStateConflict&Drop) != Drop && (tt.onForwardsCPUConflict&Drop) != Drop
}

// setCPUForwards propagates a CPU, known to hold for the receiver's PID just
// prior to its timestamp, forward into and possibly through the receiver.
func (tt *threadTransition) setCPUForwards(cpu CPUID) error {
	if cpu == UnknownCPU || tt.PrevCPU == cpu {
		return nil
	}
	if tt.PrevCPU != UnknownCPU {
		return status.Errorf(codes.Internal, "setCPUForwards called on threadTransition with different PrevCPU")
	}
	tt.PrevCPU = cpu
	// If both previous and next CPUs were Unknown, propagate through to Next.
	if tt.NextCPU == UnknownCPU {
		tt.NextCPU = cpu
	}
	return nil
}

// setCPUBackwards propagates a CPU, known to hold for the receiver's PID just
// after its Timestamp, backward into and possibly through the receiver.
func (tt *threadTransition) setCPUBackwards(cpu CPUID) error {
	if cpu == UnknownCPU || tt.NextCPU == cpu {
		return nil
	}
	if tt.NextCPU != UnknownCPU {
		return status.Errorf(codes.Internal, "setCPUBackwards called on threadTransition with different NextCPU")
	}
	tt.NextCPU = cpu
	if tt.PrevCPU == UnknownCPU {
		tt.PrevCPU = cpu
	}
	return nil
}

// setStateForwards propagates a thread state, known to hold for the receiver's
// PID just prior to its timestamp, forward into and, if requested, through
// the receiver.
func (tt *threadTransition) setStateForwards(state ThreadState) error {
	prevState, err := mergeState(state, tt.PrevState)
	if err != nil {
		return status.Errorf(codes.Internal, "setStateForwards attempted to propagate state %s into incompatible state %s", state, tt.PrevState)
	}
	tt.PrevState = prevState
	if tt.StatePropagatesThrough {
		nextState, err := mergeState(tt.PrevState, tt.NextState)
		if err != nil {
			return status.Errorf(codes.Internal, "setStateForwards attempted to through-propagate state %s into incompatible state %s", tt.PrevState, tt.NextState)
		}
		tt.NextState = nextState
	}
	return nil
}

// setStateBackwards propagates a thread state, known to hold for the
// receiver's PID just after its timestamp, backwards into and, if requested,
// through the receiver.
func (tt *threadTransition) setStateBackwards(state ThreadState) error {
	nextState, err := mergeState(state, tt.NextState)
	if err != nil {
		return status.Errorf(codes.Internal, "setStateBackwards attempted to propagate state %s into incompatible state %s", state, tt.NextState)
	}
	tt.NextState = nextState
	if tt.StatePropagatesThrough {
		prevState, err := mergeState(tt.NextState, tt.PrevState)
		if err != nil {
			return status.Errorf(codes.Internal, "setStateBackwards attempted to through-propagate state %s into incompatible state %s", tt.NextState, tt.PrevState)
		}
		tt.PrevState = prevState
	}
	return nil
}

func (tt *threadTransition) String() string {
	if tt == nil {
		return "<nil>"
	}
	ret := "<unknown>"
	if tt.EventID != Unknown {
		ret = fmt.Sprintf("[Event %d] ", tt.EventID)
	}
	if tt.dropped {
		ret = ret + "(dropped) "
	}
	if tt.synthetic {
		ret = ret + "(synthetic) "
	}
	ret = ret + fmt.Sprintf("CPU policies: [%s, %s] ", tt.onBackwardsCPUConflict, tt.onForwardsCPUConflict)
	ret = ret + fmt.Sprintf("State policies: [%s, %s] ", tt.onBackwardsStateConflict, tt.onForwardsStateConflict)
	if tt.StatePropagatesThrough {
		ret = ret + "(state propagates through) "
	}
	return ret + fmt.Sprintf("@%-18d %s Command: [%d->%d] Priority: [%d->%d] CPU: [%s->%s] State: [%s->%s]", tt.Timestamp, tt.PID, tt.PrevCommand, tt.NextCommand, tt.PrevPriority, tt.NextPriority, tt.PrevCPU, tt.NextCPU, tt.PrevState, tt.NextState)
}
