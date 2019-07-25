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
	"sort"

	// TODO(sabarabc) Write a Copybara rule to convert these to OS.
	"github.com/Workiva/go-datastructures/augmentedtree"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/google/schedviz/tracedata/trace"
)

// A threadSpan is a duration of time over which a single PID held a state
// on a single CPU.
type threadSpan struct {
	pid            PID
	startTimestamp trace.Timestamp
	endTimestamp   trace.Timestamp
	cpu            CPUID
	id             uint64 // A unique identifier for augmentedtree.Tree.
	priority       Priority
	state          ThreadState
	command        stringID
	// The IDs of any events that were dropped due to conflicts identified during
	// inference.
	droppedEventIDs []int
	syntheticStart  bool
	syntheticEnd    bool
}

func (ts *threadSpan) duration() Duration {
	return duration(ts.startTimestamp, ts.endTimestamp)
}

// less supports sorting threadSpan slices by increasing startTimestamp.
func (ts *threadSpan) less(other *threadSpan) bool {
	switch {
	case ts.startTimestamp < other.startTimestamp:
		return true
	case ts.startTimestamp > other.startTimestamp:
		return false
	}
	return ts.duration() < other.duration()
}

func (ts *threadSpan) equals(other *threadSpan) bool {
	return ts.pid == other.pid &&
		ts.startTimestamp == other.startTimestamp &&
		ts.endTimestamp == other.endTimestamp &&
		ts.cpu == other.cpu &&
		ts.id == other.id &&
		ts.priority == other.priority &&
		ts.state == other.state &&
		ts.command == other.command &&
		ts.syntheticStart == other.syntheticStart &&
		ts.syntheticEnd == other.syntheticEnd &&
		func() bool {
			if len(ts.droppedEventIDs) != len(other.droppedEventIDs) {
				return false
			}
			sort.Slice(ts.droppedEventIDs, func(a, b int) bool {
				return ts.droppedEventIDs[a] < ts.droppedEventIDs[b]
			})
			sort.Slice(other.droppedEventIDs, func(a, b int) bool {
				return other.droppedEventIDs[a] < other.droppedEventIDs[b]
			})
			for idx, tsde := range ts.droppedEventIDs {
				if tsde != other.droppedEventIDs[idx] {
					return false
				}
			}
			return true
		}()
}

func (ts *threadSpan) String() string {
	ret := fmt.Sprintf("%s (%s, %d, %s) on %s [%d - %d] (%d)", ts.pid, ts.state, ts.command, ts.priority, ts.cpu, ts.startTimestamp, ts.endTimestamp, ts.id)
	if ts.syntheticStart {
		ret = ret + " (synthetic start)"
	}
	if ts.syntheticEnd {
		ret = ret + " (synthetic end)"
	}
	return ret
}

// The ID for augmentedtree.Intervals used in queries.  It's not clear from
// augmentedtree's godoc whether query IDs matter, but if they do, best to use
// a reserved one.
const queryID uint64 = 0

// LowAtDimension returns the start timestamp of i.  Required to support
// augmentedtree.Interval.
func (ts *threadSpan) LowAtDimension(d uint64) int64 {
	return int64(ts.startTimestamp)
}

// HighAtDimension returns the end timestamp of i.  Required to support
// augmentedtree.Interval.
func (ts *threadSpan) HighAtDimension(d uint64) int64 {
	return int64(ts.endTimestamp)
}

// OverlapsAtDimension returns true if an interval overlaps this interval at
// the specified dimension.  Required to support augmentedtree.Interval.
func (ts *threadSpan) OverlapsAtDimension(j augmentedtree.Interval, d uint64) bool {
	return ts.HighAtDimension(d) >= j.LowAtDimension(d) &&
		j.HighAtDimension(d) >= ts.LowAtDimension(d)
}

// ID returns the unique identifier for this interval.  Required to support
// augmentedtree.Interval.
func (ts *threadSpan) ID() uint64 {
	return ts.id
}

// threadSpanGenerator builds running, sleeping, and waiting threadSpans for a
// single PID from that PID's threadTransitions, provided in increasing
// temporal order.  Incoming threadTransitions should already be fully
// CPU- and state-inferred, and threadSpanGenerator will raise errors wherever
// CPU or state inference seems to have failed or not been performed.
//
// threadSpanGenerator uses the following collectionOptions fields:
// * preciseCommands: if true, thread command names in intervals will be as
//   precise as possible: events lacking thread command names will be
//   populated with commands from earlier events referring to the same PID, and
//   intervals will be split on changes in thread command, even if nothing else
//   changed.
// * precisePriorities: if true, thread priorities in intervals will be as
//   precise as possible: events lacking thread priorities will be populated
//   with priorities from earlier events referring to the same PID, and
//   intervals will be split on changes in thread priority, even if nothing else
//   changed.
type threadSpanGenerator struct {
	pid          PID
	options      *collectionOptions
	current      *threadSpan
	lastCommand  stringID
	lastPriority Priority
}

func newThreadSpanGenerator(pid PID, options *collectionOptions) *threadSpanGenerator {
	return &threadSpanGenerator{
		pid:          pid,
		options:      options,
		current:      nil,
		lastCommand:  UnknownCommand,
		lastPriority: UnknownPriority,
	}
}

// checkCommon checks for inference or precondition failures in common to all
// intervals.  It compares the current threadSpan with the threadTransition tt.
// An error is returned if:
//  * tt.Timestamp is Unknown,
//  * tt.Timestamp is less than the current.startTimestamp (expect threadTransitions
//    in nondecreasing temporal order),
//  * tt.PrevState is not current.state (expect no thread state inference
//    errors)
//  * tt.PrevCPU is different from current.cpu (expect no CPU inference errors).
func (tsg *threadSpanGenerator) checkCommon(tt *threadTransition) error {
	if tt.Timestamp == UnknownTimestamp {
		return status.Errorf(codes.InvalidArgument, "missing timestamp in threadTransition %s", tt)
	}
	if tsg.current == nil {
		return nil
	}
	// Check that the threadTransition is not prior to the threadSpan.
	if tt.Timestamp < tsg.current.startTimestamp {
		return status.Errorf(codes.InvalidArgument,
			"threadSpanGenerator received out-of-time-order threadTransitions (%s > %s)", tsg.current, tt)
	}
	// Check that the threadTransition has the proper previous state.
	if tt.PrevState != tsg.current.state {
		return status.Errorf(codes.InvalidArgument,
			"threadSpanGenerator received unexpected state transition (%s -> %s)", tsg.current, tt)
	}
	// Check that the threadTransition's previous CPU is the same as the
	// threadSpan's.
	if tt.PrevCPU != tsg.current.cpu {
		return status.Errorf(codes.InvalidArgument,
			"threadSpanGenerator received unexpected CPU transition (%s -> %s)", tsg.current, tt)
	}
	return nil
}

// checkCommandAndPriority returns true if the provided threadSpan should be
// split based on changes in thread command or priority.  It also populates
// lastCommand and lastPriority, if these are unknown.
func (tsg *threadSpanGenerator) checkCommandAndPriority(tt *threadTransition) bool {
	split := false
	if tsg.current != nil {
		if tsg.options.preciseCommands {
			// We must split if the threadTransition's prev or next command is Unknown
			// and different from the threadSpan's.
			if (tt.PrevCommand != UnknownCommand && tsg.current.command != tt.PrevCommand) ||
				(tt.NextCommand != UnknownCommand && tsg.current.command != tt.NextCommand) {
				split = true
				tsg.lastCommand = UnknownCommand
			}
		}
		if tsg.options.precisePriorities {
			// We must split if the threadTransition's prev or next command is Unknown
			// and different from the threadSpan's.
			if (tt.PrevPriority != UnknownPriority && tsg.current.priority != tt.PrevPriority) ||
				(tt.NextPriority != UnknownPriority && tsg.current.priority != tt.NextPriority) {
				split = true
				tsg.lastPriority = UnknownPriority
			}
		}
	}
	// Populate the builder's last command, if it is unknown.
	if tsg.lastCommand == UnknownCommand && tt.NextCommand != UnknownCommand {
		tsg.lastCommand = tt.NextCommand
	}
	// Populate the builder's last priority, if it is unknown.
	if tsg.lastPriority == UnknownPriority && tt.NextPriority != UnknownPriority {
		tsg.lastPriority = tt.NextPriority
	}
	return split
}

// addTransition updates the current working span using the provided
// threadTransition, checks for errors, and returns any span completed by
// the transition.
func (tsg *threadSpanGenerator) addTransition(nextTT *threadTransition) (*threadSpan, error) {
	var ret *threadSpan
	// If the provided transition was dropped, note its eventID in the current
	// span, if there is one, and return early.
	if nextTT.dropped && tsg.current != nil {
		tsg.current.droppedEventIDs = append(tsg.current.droppedEventIDs, nextTT.EventID)
		return nil, nil
	}
	if err := tsg.checkCommon(nextTT); err != nil {
		return nil, err
	}
	split := tsg.checkCommandAndPriority(nextTT)
	if tsg.current != nil {
		switch tsg.current.state {
		case RunningState:
			// If the current span is running, it is checked against nextTT:
			//  * An error is returned if nextTT's nextState is RunningState and its
			//    nextCPU is different from the running's (running threads do not
			//    migrate.)
			//  * If nextTT's NextState is not RunningState, the current span is
			//    complete.
			if nextTT.NextState == RunningState && nextTT.NextCPU != tsg.current.cpu {
				return nil, status.Errorf(codes.InvalidArgument,
					"threadSpanGenerator received unexpected migration  (%s > %s)",
					tsg.current, nextTT)
			}
			if nextTT.NextState != RunningState {
				split = true
			}
		case SleepingState, WaitingState, UnknownState:
			// If the current span is sleeping, waiting, or unknown it is checked
			// against nextTT.  If nextTT's nextState is not current.state or nextTT's
			// cpu is not current.cpu, the span is complete:
			if nextTT.NextState != tsg.current.state || nextTT.NextCPU != tsg.current.cpu {
				split = true
			}
		}
		// Advance the current interval's endTimestamp to reflect this transition's
		// membership in it.
		tsg.current.endTimestamp = nextTT.Timestamp
		// If a split was requested, close out the current span and return it.
		if split {
			ret, tsg.current = tsg.current, nil
			ret.syntheticEnd = nextTT.synthetic
		}
	}
	// If the current span is nil, start a new span, with its pid, state, and cpu
	// taken from the transition's next properties, its command and priority taken
	// from lastCommand and lastPriority, and its timestamps taken from the
	// transition's timestamp.
	if tsg.current == nil {
		tsg.current = &threadSpan{
			pid:            tsg.pid,
			startTimestamp: nextTT.Timestamp,
			endTimestamp:   nextTT.Timestamp,
			state:          nextTT.NextState,
			command:        tsg.lastCommand,
			priority:       tsg.lastPriority,
			cpu:            nextTT.NextCPU,
			// id should be filled in before use in an augmentedTree.
			id:             queryID,
			syntheticStart: nextTT.synthetic,
		}
	}
	return ret, nil
}

func (tsg *threadSpanGenerator) drain() *threadSpan {
	ret := tsg.current
	tsg.current = nil
	tsg.lastCommand = UnknownCommand
	tsg.lastPriority = UnknownPriority
	return ret
}
