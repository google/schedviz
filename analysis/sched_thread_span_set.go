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

	"github.com/google/schedviz/tracedata/trace"
)

// threadSpanSet accepts a sequence of uninferred threadTransitions in
// increasing timestamp order, and produces as output, for each observed PID, a
// slice of threadSpans describing that PID's scheduling behavior, as far as
// can be inferred, over the trace.
type threadSpanSet struct {
	startTimestamp         trace.Timestamp
	options                *collectionOptions
	inferrerByPID          map[PID]*threadInferrer
	spanGeneratorByPID     map[PID]*threadSpanGenerator
	spansByPID             map[PID][]*threadSpan
	droppedEventCountsByID map[int]int
}

// newThreadSpanSet returns a new, empty threadSpanSet.  The provided
// startTimestamp should be set to the start time of the valid collection,
// which will usually be the timestamp of the first unclipped event (if
// timestamps are normalized, 0.)
func newThreadSpanSet(startTimestamp trace.Timestamp, options *collectionOptions) *threadSpanSet {
	return &threadSpanSet{
		startTimestamp:         startTimestamp,
		options:                options,
		inferrerByPID:          map[PID]*threadInferrer{},
		spanGeneratorByPID:     map[PID]*threadSpanGenerator{},
		spansByPID:             map[PID][]*threadSpan{},
		droppedEventCountsByID: map[int]int{},
	}
}

// inferrer returns the threadInferrer for the thread in the provided
// thread.  If one doesn't exist yet, it is created, and an initial interval
// with unknown prev and next CPUs and states is inserted at the
// threadSpanSet's startTimestamp, to ensure that the first span for this
// thread will begin at that timestamp.
func (tss *threadSpanSet) inferrer(pid PID, command stringID, priority Priority) *threadInferrer {
	inferrer, ok := tss.inferrerByPID[pid]
	if !ok {
		inferrer = newThreadInferrer(pid, tss.options)
		tss.inferrerByPID[pid] = inferrer
		// Add an initial transition, starting at the trace start timestamp,
		inferrer.addTransition(&threadTransition{
			EventID:      Unknown,
			Timestamp:    tss.startTimestamp,
			PID:          pid,
			PrevCommand:  command,
			NextCommand:  command,
			PrevPriority: priority,
			NextPriority: priority,
			PrevCPU:      UnknownCPU,
			NextCPU:      UnknownCPU,
			PrevState:    UnknownState,
			NextState:    UnknownState,
		})
	}
	return inferrer
}

// spanGenerator returns the threadSpanGenerator for the specified PID.  If one
// doesn't exist yet, it is created.
func (tss *threadSpanSet) spanGenerator(pid PID) *threadSpanGenerator {
	sb, ok := tss.spanGeneratorByPID[pid]
	if !ok {
		sb = newThreadSpanGenerator(pid, tss.options)
		tss.spanGeneratorByPID[pid] = sb
	}
	return sb
}

// createSpans accepts a slice of already-inferred threadTransitions and
// converts them into threadSpanSet.
func (tss *threadSpanSet) createSpans(pid PID, infTTs []*threadTransition) error {
	// Add any transitions the inferrer has produced to its thread's spanbuilder.
	sb := tss.spanGenerator(pid)
	for _, infTT := range infTTs {
		if infTT.dropped && infTT.EventID != Unknown {
			tss.droppedEventCountsByID[infTT.EventID]++
		}

		ts, err := sb.addTransition(infTT)
		if err != nil {
			return err
		}
		if ts != nil {
			tss.spansByPID[pid] = append(tss.spansByPID[pid], ts)
		}
	}
	return nil
}

// addTransition adds the provided transition to the threadSpanSet, which
// passes it through its PID's threadInferrer, and passes any resulting
// inferred threadTransitions on to its PID's threadSpanGenerator.
func (tss *threadSpanSet) addTransition(tt *threadTransition) error {
	pid := tt.PID
	if pid == 0 {
		// Do not attempt to build thread spans for PID 0, 'swapper'.  This is the
		// idle process that nominally runs on unloaded CPUs, and it can run
		// concurrently on many CPUs, yielding CPU and state inference errors if it
		// is treated as a normal thread.
		return nil
	}
	// Add this transition to its thread's inferrer.
	inf := tss.inferrer(tt.PID, tt.PrevCommand, tt.PrevPriority)
	infTTs, err := inf.addTransition(tt)
	if err != nil {
		return err
	}
	return tss.createSpans(pid, infTTs)
}

// threadSpans drains all inferrers, then returns the assembled per-PID
// threadSpans, in sorted order of increasing startTimestamp.  It also clears
// the receiver.
func (tss *threadSpanSet) threadSpans(endTimestamp trace.Timestamp) (map[PID][]*threadSpan, error) {
	// For each pid, add a final all-Unknown transition at the endTimestamp, then
	// drain the inferrer.
	for pid, inferrer := range tss.inferrerByPID {
		var infTTs = []*threadTransition{}
		infTTs, err := inferrer.addTransition(&threadTransition{
			EventID:      Unknown,
			Timestamp:    endTimestamp,
			PID:          pid,
			PrevCommand:  UnknownCommand,
			NextCommand:  UnknownCommand,
			PrevPriority: UnknownPriority,
			NextPriority: UnknownPriority,
			PrevCPU:      UnknownCPU,
			NextCPU:      UnknownCPU,
			PrevState:    UnknownState,
			NextState:    UnknownState,
		})
		if err != nil {
			return nil, err
		}
		finalInfTTs, err := inferrer.drain()
		if err != nil {
			return nil, err
		}
		infTTs = append(infTTs, finalInfTTs...)
		if err := tss.createSpans(pid, infTTs); err != nil {
			return nil, err
		}
	}
	// Drain the per-PID thread span builders.
	for pid, tsg := range tss.spanGeneratorByPID {
		ts := tsg.drain()
		if ts != nil {
			tss.spansByPID[pid] = append(tss.spansByPID[pid], ts)
		}
	}
	// Sort all spans by increasing startTimestamp, and assign a unique ID to each.
	nextID := queryID + 1
	// Iterate through PIDs in sorted order to keep IDs stable.
	var sortedPIDs = []PID{}
	for pid := range tss.spansByPID {
		sortedPIDs = append(sortedPIDs, pid)
	}
	sort.Slice(sortedPIDs, func(a, b int) bool {
		return sortedPIDs[a] < sortedPIDs[b]
	})
	for _, pid := range sortedPIDs {
		sort.Slice(tss.spansByPID[PID(pid)], func(a, b int) bool {
			return tss.spansByPID[PID(pid)][a].startTimestamp < tss.spansByPID[PID(pid)][b].startTimestamp
		})
		for _, ts := range tss.spansByPID[PID(pid)] {
			ts.id = nextID
			nextID++
		}
	}
	var ret map[PID][]*threadSpan
	ret, tss.spansByPID = tss.spansByPID, map[PID][]*threadSpan{}
	return ret, nil
}
