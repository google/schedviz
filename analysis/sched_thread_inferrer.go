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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/google/schedviz/tracedata/trace"
)

// threadInferrer consumes a stream of threadTransitions, with nondecreasing
// Timestamps, and produces a corresponding stream of threadTransitions with
// formerly-Unknown states and CPUs inferred from neighboring
// threadTransitions.
// threadInferrer uses the following collectionOptions field:
// * dropOnConflict: if true, threadTransitions that conflict with their
//   neighbors, and which allow for dropping on that conflict, are dropped.
type threadInferrer struct {
	pid                PID
	options            *collectionOptions
	lastTimestamp      trace.Timestamp
	pendingTransitions []*threadTransition
}

func newThreadInferrer(pid PID, options *collectionOptions) *threadInferrer {
	return &threadInferrer{
		pid:           pid,
		options:       options,
		lastTimestamp: UnknownTimestamp,
	}
}

// findConflict iterates through the pendingTransitions, looking for
// disagreements between adjacent transitions on CPU or state.  If a
// disagreement is found, the indices of the disagreeing transitions are
// returned, as well as true.  If no disagreement is found, false is
// returned.
func (inferrer *threadInferrer) findConflict() (idx1, idx2 int, conflicted bool) {
	currentCPU := UnknownCPU
	currentCPUIdx := 0
	currentState := AnyState
	currentStateIdx := 0
	for idx, tt := range inferrer.pendingTransitions {
		if tt.dropped {
			continue
		}
		// Find CPU conflicts.
		if currentCPU == UnknownCPU {
			currentCPU = tt.PrevCPU
			currentCPUIdx = idx
		} else if tt.PrevCPU != UnknownCPU && tt.PrevCPU != currentCPU {
			return currentCPUIdx, idx, true
		}
		if tt.CPUPropagatesThrough {
			if tt.NextCPU != UnknownCPU && tt.NextCPU != currentCPU {
				return currentCPUIdx, idx, true
			}
		} else {
			currentCPU = tt.NextCPU
			currentCPUIdx = idx
		}
		// Find state conflicts.  Update currentStateIdx whenever the currentState
		// is further restricted.
		var merged bool
		if currentState != tt.PrevState && tt.PrevState != AnyState {
			if currentState == AnyState {
				currentState = tt.PrevState
				currentStateIdx = idx
			} else {
				currentState, merged = mergeState(currentState, tt.PrevState)
				if !merged {
					return currentStateIdx, idx, true
				}
				currentStateIdx = idx
			}
		}
		if tt.StatePropagatesThrough {
			if currentState != tt.NextState && tt.NextState != AnyState {
				if currentState == AnyState {
					currentState = tt.NextState
					currentStateIdx = idx
				} else {
					currentState, merged = mergeState(currentState, tt.NextState)
					if !merged {
						return currentStateIdx, idx, true
					}
					currentStateIdx = idx
				}
			}
		} else {
			currentState = tt.NextState
			currentStateIdx = idx
		}
	}
	return 0, 0, false
}

// handleConflicts searches for state or CPU conflicts among
// pendingTransitions.  Upon finding such a conflict, it resolves it according
// to the conflicting transitions' policies, returning whether a retry should
// be attempted, and any terminal error encountered.  If retry is not requested
// and no error is returned, no conflicts were detected.
func (inferrer *threadInferrer) handleConflicts() (retry bool, err error) {
	idx1, idx2, conflict := inferrer.findConflict()
	if !conflict {
		// If no conflict, there's nothing to handle.
		return false, nil
	}
	// If the conflict outcome is to insert a synthetic transition, it may
	// reflect a CPU conflict, a state conflict, or both.  We check for both
	// CPU and state conflicts, and after both, if needed, insert the
	// appropriate synthetic transition.
	insertSynthetic := false
	syntheticPrevState, syntheticNextState := AnyState, AnyState
	syntheticStatePropagatesThrough := true
	syntheticPrevCPU, syntheticNextCPU := UnknownCPU, UnknownCPU
	syntheticCPUPropagatesThrough := true
	tt1, tt2 := inferrer.pendingTransitions[idx1], inferrer.pendingTransitions[idx2]
	// Check for a disagreement on CPU, and handle failing and dropping if
	// necessary.
	if tt1.NextCPU != Unknown && tt2.PrevCPU != Unknown && tt1.NextCPU != tt2.PrevCPU {
		// We have a CPU conflict.
		resolution := resolveConflict(tt1.onForwardsCPUConflict, tt2.onBackwardsCPUConflict)
		switch resolution {
		case Fail:
			return false, status.Errorf(codes.Internal,
				"inference error (CPU) between '%s' and '%s'",
				tt1, tt2)
		case Drop:
			// We dropped something, and must retry the check pass.
			if (tt1.onForwardsCPUConflict & Drop) == Drop {
				tt1.dropped = true
			}
			if (tt2.onBackwardsCPUConflict & Drop) == Drop {
				tt2.dropped = true
			}
			return true, nil
		case InsertSynthetic:
			insertSynthetic = true
			syntheticPrevCPU = tt1.NextCPU
			syntheticNextCPU = tt2.PrevCPU
			syntheticCPUPropagatesThrough = false
		}
	}
	// Check for a disagreement on state and handle failing and dropping if
	// necessary.
	if _, merged := mergeState(tt1.NextState, tt2.PrevState); !merged {
		// We have a state conflict.
		resolution := resolveConflict(tt1.onForwardsStateConflict, tt2.onBackwardsStateConflict)
		switch resolution {
		case Fail:
			return false, status.Errorf(codes.Internal,
				"inference error (thread state) between '%s' and '%s'",
				tt1, tt2)
		case Drop:
			// We dropped something, and must retry the check pass.
			if (tt1.onForwardsStateConflict & Drop) == Drop {
				tt1.dropped = true
			}
			if (tt2.onBackwardsStateConflict & Drop) == Drop {
				tt2.dropped = true
			}
			return true, nil
		case InsertSynthetic:
			insertSynthetic = true
			syntheticPrevState = tt1.NextState
			syntheticNextState = tt2.PrevState
			syntheticStatePropagatesThrough = false
		}
	}
	if insertSynthetic {
		syntheticTransition := &threadTransition{
			EventID:                Unknown,
			Timestamp:              tt1.Timestamp + (tt2.Timestamp-tt1.Timestamp)/2,
			PID:                    tt1.PID,
			PrevCommand:            UnknownCommand,
			NextCommand:            UnknownCommand,
			PrevPriority:           UnknownPriority,
			NextPriority:           UnknownPriority,
			PrevCPU:                syntheticPrevCPU,
			NextCPU:                syntheticNextCPU,
			CPUPropagatesThrough:   syntheticCPUPropagatesThrough,
			PrevState:              syntheticPrevState,
			NextState:              syntheticNextState,
			StatePropagatesThrough: syntheticStatePropagatesThrough,
			synthetic:              true,
		}
		inferrer.pendingTransitions = append(inferrer.pendingTransitions, syntheticTransition)
		sort.Slice(inferrer.pendingTransitions, func(a, b int) bool {
			return inferrer.pendingTransitions[a].Timestamp < inferrer.pendingTransitions[b].Timestamp
		})
		// We inserted something, and must retry the check pass.
		return true, nil
	}
	return false, nil
}

// inferForwards scans forwards through pendingTransitions, tracking last-known
// CPU and state, and updating unknown CPUs and states with the last-known
// data.
func (inferrer *threadInferrer) inferForwards() error {
	var lastKnownCPU, lastKnownState *threadTransition
	for _, tt := range inferrer.pendingTransitions {
		if tt.dropped {
			continue
		}
		if lastKnownCPU != nil {
			if !tt.setCPUForwards(lastKnownCPU.NextCPU) {
				return status.Errorf(codes.Internal, "inference error (CPU): at time %d, failed to propagate CPU %d forwards from '%s' to '%s'", tt.Timestamp, lastKnownCPU.NextCPU, lastKnownCPU, tt)
			}
		}
		if lastKnownState != nil {
			if !tt.setStateForwards(lastKnownState.NextState) {
				return status.Errorf(codes.Internal, "inference error (state): at time %d, failed to propagate state %d forwards from '%s' to '%s'", tt.Timestamp, lastKnownState.NextState, lastKnownState, tt)
			}
		}
		if tt.NextCPU == UnknownCPU {
			lastKnownCPU = nil
		} else {
			lastKnownCPU = tt
		}
		lastKnownState = tt
	}
	return nil
}

// inferBackwards scans backwards through pendingTransitions, tracking
// last-known CPU and state, and updating unknown CPUs and states with the
// last-known data.
func (inferrer *threadInferrer) inferBackwards() error {
	var lastKnownCPU, lastKnownState *threadTransition
	for idx := len(inferrer.pendingTransitions) - 1; idx >= 0; idx-- {
		tt := inferrer.pendingTransitions[idx]
		if tt.dropped {
			continue
		}
		if lastKnownCPU != nil {
			if !tt.setCPUBackwards(lastKnownCPU.PrevCPU) {
				return status.Errorf(codes.Internal, "inference error (CPU): at time %d, failed to propagate CPU %d backwards from '%s' to '%s'", tt.Timestamp, lastKnownCPU.PrevCPU, lastKnownCPU, tt)
			}
		}
		if lastKnownState != nil {
			if !tt.setStateBackwards(lastKnownState.PrevState) {
				return status.Errorf(codes.Internal, "inference error (state): at time %d, failed to propagate state %d backwards from '%s' to '%s'", tt.Timestamp, lastKnownState.PrevState, lastKnownState, tt)
			}
		}
		if tt.PrevCPU == UnknownCPU {
			lastKnownCPU = nil
		} else {
			lastKnownCPU = tt
		}
		lastKnownState = tt
	}
	return nil
}

// inferPending performs an inference pass on the receiver's pending
// transitions.  It can only guarantee sensical inferences (or errors) if
// pendingTransitions contains all transitions between two adjacent forward
// barriers (where the start-of-trace is considered a forward barrier.)
// If this were not the case, inferences could be made and committed, that
// would be invalidated on subsequent passes, when it's too late to do anything
// about.
// inferPending should only be called when the pendingTransitions ends with a
// forward barrier or when no further transitions will be forthcoming from the
// trace.  lastBatch specifies which of these cases applies: if true,
// inferences are made on the entire pending transition set, and the entire
// set is returned and drained.  If false, the last transition is expected to
// be a forward barrier; all transitions before the last are inferred and
// returned, and the last is retained as the sole remaining entry in
// pendingTransitions.
func (inferrer *threadInferrer) inferPending(lastBatch bool) ([]*threadTransition, error) {
	// Check for inference errors.  On finding an error, if the transition can be
	// dropped, do so and retry, otherwise return an error.
	for {
		if retry, err := inferrer.handleConflicts(); err != nil {
			return nil, err
		} else if retry {
			continue
		}
		break
	}
	// If this is not the last batch, we must ensure that the last pending
	// transition is still a forward barrier -- that barrier may have been
	// dropped in the check phase.  If it isn't, we return early and continue
	// to build our pending transitions; if it is, we proceed to inference.
	if !lastBatch {
		if !inferrer.pendingTransitions[len(inferrer.pendingTransitions)-1].isForwardBarrier() {
			return nil, nil
		}
	}
	// Now, infer forwards and backwards, populating Unknown values in
	// pending transitions.
	if err := inferrer.inferForwards(); err != nil {
		return nil, err
	}
	if err := inferrer.inferBackwards(); err != nil {
		return nil, err
	}
	// If this is the last batch, return all pending transitions.  Otherwise,
	// return all but the last.
	cutPoint := len(inferrer.pendingTransitions)
	if !lastBatch {
		cutPoint--
	}
	if cutPoint < 0 {
		return nil, nil
	}
	ret := make([]*threadTransition, 0, cutPoint)
	var newPt = []*threadTransition{}
	for idx, pt := range inferrer.pendingTransitions {
		if idx < cutPoint {
			ret = append(ret, pt)
		} else {
			newPt = append(newPt, pt)
		}
	}
	inferrer.pendingTransitions = newPt
	return ret, nil
}

// addTransition adds the provided threadTransition to the receiver, and
// returns zero or more fully-inferred threadTransitions.
func (inferrer *threadInferrer) addTransition(nextTT *threadTransition) ([]*threadTransition, error) {
	if nextTT.PID != inferrer.pid {
		return nil, status.Errorf(codes.InvalidArgument, "incorrect PID for %s", nextTT)
	}
	if nextTT.Timestamp == UnknownTimestamp {
		return nil, status.Errorf(codes.InvalidArgument, "missing timestamp in threadTransition")
	}
	if inferrer.lastTimestamp != UnknownTimestamp && inferrer.lastTimestamp > nextTT.Timestamp {
		return nil, status.Errorf(codes.InvalidArgument, "out-of-order threadTransitions at %d", nextTT.Timestamp)
	}
	if nextTT.PrevState&UnknownState != 0 || nextTT.NextState&UnknownState != 0 {
		return nil, status.Errorf(codes.InvalidArgument, "threadTransitions may not specify UnknownState as prev or next state")
	}
	inferrer.lastTimestamp = nextTT.Timestamp
	inferrer.pendingTransitions = append(inferrer.pendingTransitions, nextTT)
	// Only infer on the pending transitions if we just placed a forward barrier
	// at the end.
	if nextTT.isForwardBarrier() {
		return inferrer.inferPending( /*lastBatch*/ false)
	}
	return nil, nil
}

// drain infers (as much as is possible) on the pendingTransitions, then
// returns them.
func (inferrer *threadInferrer) drain() ([]*threadTransition, error) {
	ret, err := inferrer.inferPending( /*lastBatch*/ true)
	inferrer.lastTimestamp = UnknownTimestamp
	inferrer.pendingTransitions = nil
	return ret, err
}
