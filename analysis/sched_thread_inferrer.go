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

// checkCPUs scans forward through pendingTransitions, tracking the last
// transition with known CPU, and comparing this with subsequent transitions.
// If a CPU conflict is found, the applicable conflict policy is determined and
// applied: an error is returned, one or both of the conflictants is dropped,
// or a synthetic event is placed between the conflictants resolving the
// conflict.  In the case of a conflict, the checking pass must be retried;
// whether this retry is required is returned.
func (inferrer *threadInferrer) checkCPUs() (retry bool, err error) {
	var lastKnownCPU *threadTransition
	for _, tt := range inferrer.pendingTransitions {
		if tt.dropped {
			continue
		}
		if lastKnownCPU != nil && tt.PrevCPU != UnknownCPU && tt.PrevCPU != lastKnownCPU.NextCPU {
			// We have a conflict; resolve.
			resolution := resolveConflict(tt.onBackwardsCPUConflict, lastKnownCPU.onForwardsCPUConflict)
			switch resolution {
			case Fail:
				return false, status.Errorf(codes.InvalidArgument,
					"inference error (CPU) between '%s' and '%s'",
					lastKnownCPU, tt)
			case Drop:
				if (tt.onBackwardsCPUConflict & Drop) == Drop {
					tt.dropped = true
				}
				if (lastKnownCPU.onForwardsCPUConflict & Drop) == Drop {
					lastKnownCPU.dropped = true
				}
				// We dropped something, and must retry the check pass.
				return true, nil
			case InsertSynthetic:
				syntheticTransition := &threadTransition{
					EventID:      Unknown,
					Timestamp:    lastKnownCPU.Timestamp + (tt.Timestamp-lastKnownCPU.Timestamp)/2,
					PID:          tt.PID,
					PrevCommand:  UnknownCommand,
					NextCommand:  UnknownCommand,
					PrevPriority: UnknownPriority,
					NextPriority: UnknownPriority,
					PrevCPU:      lastKnownCPU.NextCPU,
					NextCPU:      tt.PrevCPU,
					PrevState:    UnknownState,
					NextState:    UnknownState,
					synthetic:    true,
				}
				inferrer.pendingTransitions = append(inferrer.pendingTransitions, syntheticTransition)
				sort.Slice(inferrer.pendingTransitions, func(a, b int) bool {
					return inferrer.pendingTransitions[a].Timestamp < inferrer.pendingTransitions[b].Timestamp
				})
				// We inserted something, and must retry the check pass.
				return true, nil
			}
		}
		if tt.NextCPU == UnknownCPU {
			lastKnownCPU = nil
		} else {
			lastKnownCPU = tt
		}
	}
	return false, nil
}

// checkStates scans forward through pendingTransitions, tracking the last
// transition with known state, and comparing this with subsequent transitions.
// If a state conflict is found, the applicable conflict policy is determined
// and applied: an error is returned, one or both of the conflictants is
// dropped, or a synthetic event is placed between the conflictants resolving
// the conflict.  In the case of a conflict, the checking pass must be retried;
// whether this retry is required is returned.
func (inferrer *threadInferrer) checkStates() (retry bool, err error) {
	var lastKnownState *threadTransition
	for _, tt := range inferrer.pendingTransitions {
		if tt.dropped {
			continue
		}
		if lastKnownState != nil && tt.PrevState != UnknownState && tt.PrevState != lastKnownState.NextState {
			// We have a conflict; resolve.
			resolution := resolveConflict(tt.onBackwardsStateConflict, lastKnownState.onForwardsStateConflict)
			switch resolution {
			case Fail:
				return false, status.Errorf(codes.InvalidArgument,
					"inference error (thread state) between '%s' and '%s'",
					lastKnownState, tt)
			case Drop:
				if (tt.onBackwardsStateConflict & Drop) == Drop {
					tt.dropped = true
				}
				if (lastKnownState.onForwardsStateConflict & Drop) == Drop {
					lastKnownState.dropped = true
				}
				// We dropped something, and must retry the check pass.
				return true, nil
			case InsertSynthetic:
				syntheticTransition := &threadTransition{
					EventID:      Unknown,
					Timestamp:    lastKnownState.Timestamp + (tt.Timestamp-lastKnownState.Timestamp)/2,
					PID:          tt.PID,
					PrevCommand:  UnknownCommand,
					NextCommand:  UnknownCommand,
					PrevPriority: UnknownPriority,
					NextPriority: UnknownPriority,
					PrevCPU:      UnknownCPU,
					NextCPU:      UnknownCPU,
					PrevState:    lastKnownState.NextState,
					NextState:    tt.PrevState,
					synthetic:    true,
				}
				inferrer.pendingTransitions = append(inferrer.pendingTransitions, syntheticTransition)
				sort.Slice(inferrer.pendingTransitions, func(a, b int) bool {
					return inferrer.pendingTransitions[a].Timestamp < inferrer.pendingTransitions[b].Timestamp
				})
				// We inserted something, and must retry the check pass.
				return true, nil
			}
		}
		if tt.NextState != UnknownState {
			lastKnownState = tt
		}
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
			if err := tt.setCPUForwards(lastKnownCPU.NextCPU); err != nil {
				return err
			}
		}
		if lastKnownState != nil {
			if err := tt.setStateForwards(lastKnownState.NextState); err != nil {
				return err
			}
		}
		if tt.NextCPU == UnknownCPU {
			lastKnownCPU = nil
		} else {
			lastKnownCPU = tt
		}
		if tt.NextState != UnknownState {
			lastKnownState = tt
		}
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
			if err := tt.setCPUBackwards(lastKnownCPU.PrevCPU); err != nil {
				return err
			}
		}
		if lastKnownState != nil {
			if err := tt.setStateBackwards(lastKnownState.PrevState); err != nil {
				return err
			}
		}
		if tt.PrevCPU == UnknownCPU {
			lastKnownCPU = nil
		} else {
			lastKnownCPU = tt
		}
		if tt.PrevState == UnknownState {
			lastKnownState = nil
		} else {
			lastKnownState = tt
		}
	}
	return nil
}

// mergePendingSynthetics merges synthetic transitions with the same timestamp
// in the receiver's pendingTransitions.  Merging synthetic transitions avoids
// creation of spurious zero-length threadSpans in the event that both thread
// state and CPU are synthetically transitioned at the same time.
func (inferrer *threadInferrer) mergePendingSynthetics() error {
	var syntheticRun []*threadTransition
	var newPending []*threadTransition

	var handleSyntheticRun = func() error {
		if len(syntheticRun) == 0 {
			return nil
		}
		var err error
		newSynthetic := syntheticRun[0]
		for _, tt := range syntheticRun[1:] {
			if newSynthetic, err = mergeSynthetic(newSynthetic, tt); err != nil {
				return err
			}
		}
		newPending, syntheticRun = append(newPending, newSynthetic), nil
		return nil
	}
	lastTimestamp := UnknownTimestamp
	for _, tt := range inferrer.pendingTransitions {
		if tt.Timestamp != lastTimestamp {
			if err := handleSyntheticRun(); err != nil {
				return err
			}
			lastTimestamp = tt.Timestamp
		}
		if tt.synthetic {
			syntheticRun = append(syntheticRun, tt)
		} else {
			newPending = append(newPending, tt)
		}
	}
	if err := handleSyntheticRun(); err != nil {
		return err
	}
	inferrer.pendingTransitions = newPending
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
		if retry, err := inferrer.checkCPUs(); err != nil {
			return nil, err
		} else if retry {
			continue
		}
		if retry, err := inferrer.checkStates(); err != nil {
			return nil, err
		} else if retry {
			continue
		}
		break
	}
	// If synthetic transitions are allowed, both check passes may have inserted
	// synthetic events.  Merge all synthetic events happening at the same
	// timestamp.
	if err := inferrer.mergePendingSynthetics(); err != nil {
		return nil, err
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
