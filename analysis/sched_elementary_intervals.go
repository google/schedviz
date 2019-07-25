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
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/google/schedviz/tracedata/trace"
)

// CPUStateMergeType specifies how a given CPUState's data should be used:
// whether it is complete on its own, should add to a running diff, or should
// be removed from a running diff.
type CPUStateMergeType int8

const (
	// Full signifies that its CPUState contains full information of running and
	// waiting Threads.
	Full CPUStateMergeType = iota
	// Add signifies that its CPUState adds its running and waiting Threads to
	// previously-accumulated state.
	Add
	// Remove signifies that its CPUState removes its running and waiting Threads
	// from previously-accumulated state.
	Remove
)

func (mt CPUStateMergeType) String() string {
	switch mt {
	case Full:
		return "(Full)"
	case Add:
		return "(Add)"
	case Remove:
		return "(Remove)"
	default:
		return "(UNKNOWN)"
	}
}

// CPUState records the instantaneous scheduling state of a CPU: what threads
// are running and waiting.
type CPUState struct {
	CPU       CPUID
	Running   *Thread
	Waiting   []*Thread
	MergeType CPUStateMergeType
}

func (cs *CPUState) String() string {
	var waitingThreads = []string{}
	for _, wt := range cs.Waiting {
		waitingThreads = append(waitingThreads, wt.String())
	}
	return fmt.Sprintf("%s Running: %s, Waiting: [%s] %s", cs.CPU, cs.Running, strings.Join(waitingThreads, ", "), cs.MergeType)
}

// ElementaryCPUInterval describes an elementary interval over one or more CPUs
// in the trace.  These are maximal intervals over which the running and
// waiting threads on all requested CPUs remain constant.
type ElementaryCPUInterval struct {
	StartTimestamp trace.Timestamp
	EndTimestamp   trace.Timestamp
	CPUStates      []*CPUState
}

func (eci *ElementaryCPUInterval) String() string {
	var cpuStates = []string{}
	for _, cs := range eci.CPUStates {
		cpuStates = append(cpuStates, cs.String())
	}
	return fmt.Sprintf("%d-%d [%s]", eci.StartTimestamp, eci.EndTimestamp, strings.Join(cpuStates, ", "))
}

func (eci *ElementaryCPUInterval) duration() Duration {
	return duration(eci.StartTimestamp, eci.EndTimestamp)
}

// elementaryCPUIntervalBuilder helps satisfy a ElementaryCPUIntervals query by
// maintaining running and waiting thread state while progressively
// constructing the requested intervals.  To assemble a sequence of elementary
// CPU intervals, construct a new elementaryCPUIntervalBuilder b using
// newElementaryCPUBuilder, then repeatedly call b.next(), consuming the
// returned *ElementaryCPUInterval until it is nil.
// Internally, a builder maintains, as state, a current timestamp, and indices
// into slices of running and waiting threads (the latter sorted both by wait
// start and wait end.)  At each call to next, the builder identifies the next
// timestamp after its current timestamp in which anything interesting
// happened, then constructs and returns a new ElementaryCPUInterval reflecting
// the running and waiting state between the current and next timestamp.  It
// then updates the current timestamp, and advances the 'next index' pointers
// appropriately.
// 'Anything interesting' can be:
//   * a thread starting to wait on some requested CPU,
//   * a thread ending a wait on some requested CPU,
//   * a thread starting to run on some requested CPU, or
//   * a thread stopping running on some requested CPU.
// Two special cases merit mention.  SchedViz prefers CPU intervals to have
// 'ragged edges', such that intervals that span the requested start or end
// times are not truncated to the requested edge but may extend beyond it.
// If this is requested, it requires some special treatment at the start and
// at the end of the elementary interval creation phase.  Finally, the last
// returned interval may be of zero width, if the requested end timestamp
// coincides with something interesting happening and ragged edges were not
// requested.  In this case, the zero-width interval gives the state of the
// requested CPUs immediately after the requested interval end.
//
// elementaryCPUIntervalBuilder produces diffed output, in which each interval
// only records the changes since the last interval -- the running and waiting
// threads added or removed.  The first interval is always fully-specified,
// with all state introduced as CPUStateMergeType Add.
type elementaryCPUIntervalBuilder struct {
	c *Collection
	f *filter
	// The timestamp range over which to provide elementary intervals.  If the
	// last interval we would emit ends past endTimestamp, its endpoint is
	// truncated to endTimestamp.
	startTimestamp trace.Timestamp
	endTimestamp   trace.Timestamp
	// Two lists of waiting threadSpans on requested CPUs, one ordered by
	// startTimestamp increasing, and the other ordered by endTimestamp
	// increasing.
	waitingIntervalsByStart []*threadSpan
	waitingIntervalsByEnd   []*threadSpan
	// The current index into the above waiting slices.
	nextWaitingIntervalByStartIdx int
	nextWaitingIntervalByEndIdx   int

	// As with waiting; running threadSpans ordered by start and end time.
	runningIntervalsByStart       []*threadSpan
	runningIntervalsByEnd         []*threadSpan
	nextRunningIntervalByStartIdx int
	nextRunningIntervalByEndIdx   int

	// The current time offset into this analysis.  Initially startTimestamp.
	currentTimestamp trace.Timestamp
	// Running and waiting intervals starting and ending this cycle.
	startingWaitingIntervals []*threadSpan
	endingWaitingIntervals   []*threadSpan
	startingRunningIntervals []*threadSpan
	endingRunningIntervals   []*threadSpan
	// Set to true when the last requested interval has been returned.  If true,
	// only nil intervals will be returned thenceforward.
	finished bool
}

// Create a new elementaryCPUIntervalBuilder over the requested CPUs and time
// range.
func (c *Collection) newElementaryCPUIntervalBuilder(f *filter) (*elementaryCPUIntervalBuilder, error) {
	if len(f.cpus) > 1 && !f.truncateToTimeRange {
		return nil, status.Errorf(codes.Internal, "can't provide untruncated-to-time-range elementary CPU interval edges over more than one CPU")
	}
	startTimestamp := f.startTimestamp
	if !f.truncateToTimeRange {
		newStartTimestamp := startTimestamp
		// Find the earliest start time among all waiting threads.
		for cpu := range f.cpus {
			waitingTree, ok := c.waitingSpansByCPU[cpu]
			if !ok {
				continue
			}
			query := &threadSpan{
				startTimestamp: startTimestamp,
				endTimestamp:   startTimestamp,
				id:             queryID,
			}
			waitingIntervals := waitingTree.Query(query)
			for _, interval := range waitingIntervals {
				ts := interval.(*threadSpan)
				if ts.startTimestamp < newStartTimestamp {
					newStartTimestamp = ts.startTimestamp
				}
			}
		}
		// And the earliest start time among all running threads.
		for cpu := range f.cpus {
			runningThreads, ok := c.runningSpansByCPU[cpu]
			if !ok {
				continue
			}
			// Find the first index i in runningThreads at which
			//   runningThreads[i].endTimestamp >= startTimestamp
			startIdx := sort.Search(len(runningThreads), func(i int) bool {
				return runningThreads[i].endTimestamp >= startTimestamp
			})
			if startIdx < len(runningThreads) && runningThreads[startIdx].startTimestamp < newStartTimestamp {
				newStartTimestamp = runningThreads[startIdx].startTimestamp
			}
		}
		startTimestamp = newStartTimestamp
	}
	ret := &elementaryCPUIntervalBuilder{
		c:                c,
		f:                f,
		startTimestamp:   startTimestamp,
		endTimestamp:     f.endTimestamp,
		currentTimestamp: startTimestamp,
		finished:         startTimestamp > f.endTimestamp,
	}
	// Populate and sort the waiting interval slices.
	for cpu := range f.cpus {
		waitingTree, ok := c.waitingSpansByCPU[cpu]
		if !ok {
			continue
		}
		query := &threadSpan{
			startTimestamp: startTimestamp,
			endTimestamp:   f.endTimestamp,
			id:             queryID,
		}
		waitingIntervals := waitingTree.Query(query)
		for _, interval := range waitingIntervals {
			ts := interval.(*threadSpan)
			ret.waitingIntervalsByStart = append(ret.waitingIntervalsByStart, ts)
			ret.waitingIntervalsByEnd = append(ret.waitingIntervalsByEnd, ts)
		}
	}
	sort.Slice(ret.waitingIntervalsByStart, func(i, j int) bool {
		return ret.waitingIntervalsByStart[i].startTimestamp < ret.waitingIntervalsByStart[j].startTimestamp
	})
	sort.Slice(ret.waitingIntervalsByEnd, func(i, j int) bool {
		return ret.waitingIntervalsByEnd[i].endTimestamp < ret.waitingIntervalsByEnd[j].endTimestamp
	})
	// Populate running intervals.
	for cpu := range f.cpus {
		runningThreads, ok := c.runningSpansByCPU[cpu]
		if !ok {
			continue
		}
		// Find the first index i in runningThreads at which
		//   runningThreads[i].endTimestamp >= startTimestamp
		startIdx := sort.Search(len(runningThreads), func(i int) bool {
			return runningThreads[i].endTimestamp >= startTimestamp
		})
		// Find the first index i in runningThreads at which
		//   runningThreads[i].startTimestamp > endTimestamp
		endIdx := sort.Search(len(runningThreads), func(i int) bool {
			return runningThreads[i].startTimestamp > f.endTimestamp
		})
		ret.runningIntervalsByStart = append(ret.runningIntervalsByStart, runningThreads[startIdx:endIdx]...)
		ret.runningIntervalsByEnd = append(ret.runningIntervalsByEnd, runningThreads[startIdx:endIdx]...)
	}
	sort.Slice(ret.runningIntervalsByStart, func(i, j int) bool {
		return ret.runningIntervalsByStart[i].startTimestamp < ret.runningIntervalsByStart[j].startTimestamp
	})
	sort.Slice(ret.runningIntervalsByEnd, func(i, j int) bool {
		return ret.runningIntervalsByEnd[i].endTimestamp < ret.runningIntervalsByEnd[j].endTimestamp
	})
	// Set the initial running and waiting threads.
	ret.updateRunningThreads()
	ret.updateWaitingThreads()
	return ret, nil
}

// Returns the next timestamp after currentTimestamp at which something
// interesting is happening on this set of CPUs: a waiting interval starts,
// ends, or we move out of or into a new running interval.  If no more
// interesting things are happening, returns UnknownTimestamp.
func (b *elementaryCPUIntervalBuilder) nextTimestamp() trace.Timestamp {
	var ts = UnknownTimestamp
	// Is the next thing happening a waiting interval starting?  Waiting
	// intervals begin at the instant of their startTimestamp, so if one starts
	// at currentTimestamp, it isn't the *next* one.
	var nextStartingWaitingSpan *threadSpan
	if b.nextWaitingIntervalByStartIdx < len(b.waitingIntervalsByStart) {
		nextStartingWaitingSpan = b.waitingIntervalsByStart[b.nextWaitingIntervalByStartIdx]
	}
	if nextStartingWaitingSpan != nil && nextStartingWaitingSpan.startTimestamp > b.currentTimestamp {
		ts = nextStartingWaitingSpan.startTimestamp
	}
	// Or is it a waiting interval ending?  Waiting intervals end at the
	// instant of their endTimestamp, so if one ends at currentTimestamp, it isn't
	// the *next* one.
	var nextEndingWaitingSpan *threadSpan
	if b.nextWaitingIntervalByEndIdx < len(b.waitingIntervalsByEnd) {
		nextEndingWaitingSpan = b.waitingIntervalsByEnd[b.nextWaitingIntervalByEndIdx]
	}
	if nextEndingWaitingSpan != nil && (ts == UnknownTimestamp ||
		(nextEndingWaitingSpan.endTimestamp < ts &&
			nextEndingWaitingSpan.endTimestamp > b.currentTimestamp)) {
		ts = nextEndingWaitingSpan.endTimestamp
	}
	// Or is it a change in running thread?
	var nextStartingRunningSpan *threadSpan
	if b.nextRunningIntervalByStartIdx < len(b.runningIntervalsByStart) {
		nextStartingRunningSpan = b.runningIntervalsByStart[b.nextRunningIntervalByStartIdx]
	}
	if nextStartingRunningSpan != nil && (ts == UnknownTimestamp ||
		(nextStartingRunningSpan.startTimestamp < ts &&
			nextStartingRunningSpan.startTimestamp > b.currentTimestamp)) {
		ts = nextStartingRunningSpan.startTimestamp
	}
	var nextEndingRunningSpan *threadSpan
	if b.nextRunningIntervalByEndIdx < len(b.runningIntervalsByEnd) {
		nextEndingRunningSpan = b.runningIntervalsByEnd[b.nextRunningIntervalByEndIdx]
	}
	if nextEndingRunningSpan != nil && (ts == UnknownTimestamp ||
		(nextEndingRunningSpan.endTimestamp < ts &&
			nextEndingRunningSpan.endTimestamp > b.currentTimestamp)) {
		ts = nextEndingRunningSpan.endTimestamp
	}
	return ts
}

// Returns a Thread initialized from the provided threadSpan.
func (b *elementaryCPUIntervalBuilder) threadFromThreadSpan(ts *threadSpan) (*Thread, error) {
	if ts == nil {
		return nil, nil
	}
	ret := &Thread{
		PID: ts.pid,
	}
	comm, err := b.c.LookupCommand(ts.command)
	if err != nil {
		return nil, err
	}
	ret.Command = comm
	ret.Priority = ts.priority
	return ret, nil
}

// Updates the set of per-CPU running threads, removing any that have stopped
// running by currentTimestamp and then adding any that started running by
// currentTimestamp.
func (b *elementaryCPUIntervalBuilder) updateRunningThreads() {
	b.startingRunningIntervals = nil
	b.endingRunningIntervals = nil
	// Add any newly-running threads, and remove any waiting ones.
	for _, runningThreadSpan := range b.runningIntervalsByStart[b.nextRunningIntervalByStartIdx:] {
		if runningThreadSpan.startTimestamp <= b.currentTimestamp {
			// Running spans that have already ended are no longer running.
			if runningThreadSpan.endTimestamp > b.currentTimestamp {
				b.startingRunningIntervals = append(b.startingRunningIntervals, runningThreadSpan)
			}
		} else {
			break
		}
	}
	for _, runningThreadSpan := range b.runningIntervalsByEnd[b.nextRunningIntervalByEndIdx:] {
		if runningThreadSpan.endTimestamp <= b.currentTimestamp && runningThreadSpan.endTimestamp > b.startTimestamp {
			b.endingRunningIntervals = append(b.endingRunningIntervals, runningThreadSpan)
		} else {
			break
		}
	}
	// Remove any running intervals we've already passed.
	for b.nextRunningIntervalByStartIdx < len(b.runningIntervalsByStart) &&
		b.runningIntervalsByStart[b.nextRunningIntervalByStartIdx].startTimestamp <= b.currentTimestamp {
		b.nextRunningIntervalByStartIdx++
	}
	for b.nextRunningIntervalByEndIdx < len(b.runningIntervalsByEnd) &&
		b.runningIntervalsByEnd[b.nextRunningIntervalByEndIdx].endTimestamp <= b.currentTimestamp {
		b.nextRunningIntervalByEndIdx++
	}
}

// Updates the set of per-CPU waiting threads, adding those that started
// waiting by currentTimestamp and removing those that stopped waiting by
// currentTimestamp.
func (b *elementaryCPUIntervalBuilder) updateWaitingThreads() {
	b.startingWaitingIntervals = nil
	b.endingWaitingIntervals = nil
	for _, waitingThreadSpan := range b.waitingIntervalsByStart[b.nextWaitingIntervalByStartIdx:] {
		if waitingThreadSpan.startTimestamp <= b.currentTimestamp {
			// Waiting spans that have already ended are no longer waiting.
			if waitingThreadSpan.endTimestamp > b.currentTimestamp {
				b.startingWaitingIntervals = append(b.startingWaitingIntervals, waitingThreadSpan)
			}
		} else {
			break
		}
	}
	for _, waitingThreadSpan := range b.waitingIntervalsByEnd[b.nextWaitingIntervalByEndIdx:] {
		if waitingThreadSpan.endTimestamp <= b.currentTimestamp && waitingThreadSpan.endTimestamp > b.startTimestamp {
			b.endingWaitingIntervals = append(b.endingWaitingIntervals, waitingThreadSpan)
		} else {
			break
		}
	}
	// Remove any waiting intervals we've already passed.
	for b.nextWaitingIntervalByStartIdx < len(b.waitingIntervalsByStart) &&
		b.waitingIntervalsByStart[b.nextWaitingIntervalByStartIdx].startTimestamp <= b.currentTimestamp {
		b.nextWaitingIntervalByStartIdx++
	}
	for b.nextWaitingIntervalByEndIdx < len(b.waitingIntervalsByEnd) &&
		b.waitingIntervalsByEnd[b.nextWaitingIntervalByEndIdx].endTimestamp <= b.currentTimestamp {
		b.nextWaitingIntervalByEndIdx++
	}
}

func (b *elementaryCPUIntervalBuilder) setCurrentTimestamp(ts trace.Timestamp) {
	b.currentTimestamp = ts
	b.updateRunningThreads()
	b.updateWaitingThreads()
}

func (b *elementaryCPUIntervalBuilder) nextInterval() (trace.Timestamp, trace.Timestamp) {
	// Advance to the next time something interesting happens.
	startTimestamp := b.currentTimestamp
	endTimestamp := b.nextTimestamp()
	// If we've got nothing else interesting, or the next interesting thing is
	// after our requested interval, we're finished.
	if endTimestamp == UnknownTimestamp || endTimestamp > b.endTimestamp {
		b.finished = true
		// If ragged edges were requested, leave them be, otherwise trim to the
		// requested endpoint.
		if b.f.truncateToTimeRange || endTimestamp == UnknownTimestamp {
			endTimestamp = b.endTimestamp
		}
		// If the next interesting thing lies beyond the collection's endpoint --
		// which is possible because intervals continuing beyond the end of the
		// trace are given an end timestamp of (collection endpoint + 1) -- adjust
		// down to the collection's endpoint.
		if endTimestamp > b.c.endTimestamp {
			endTimestamp = b.c.endTimestamp
		}
	}
	return startTimestamp, endTimestamp
}

// Updates running and waiting threads in the provided ElementaryCpuInterval,
// providing it with the changes since the last Interval this Builder emitted.
// The first Interval output by a Builder contains the full scheduling state of
// the system at that point as Added state.
func (b *elementaryCPUIntervalBuilder) updateRunningAndWaiting(resp *ElementaryCPUInterval) error {
	cpuStates := map[CPUStateMergeType]map[CPUID]*CPUState{
		Add:    {},
		Remove: {},
	}
	addThreadSpan := func(ts *threadSpan, mergeType CPUStateMergeType) error {
		thread, err := b.threadFromThreadSpan(ts)
		if err != nil {
			return err
		}
		cs, ok := cpuStates[mergeType][ts.cpu]
		if !ok {
			cs = &CPUState{
				CPU:       ts.cpu,
				MergeType: mergeType,
			}
			cpuStates[mergeType][ts.cpu] = cs
		}
		switch ts.state {
		case RunningState:
			if cs.Running != nil {
				return status.Errorf(codes.Internal, "observed multiple running threads on CPU %d at time range %d-%d", ts.cpu, ts.startTimestamp, ts.endTimestamp)
			}
			cs.Running = thread
		case WaitingState:
			cs.Waiting = append(cs.Waiting, thread)
		default:
			return status.Errorf(codes.Internal, "observed unexpected span state %s on CPU %d at time range %d-%d", ts.state, ts.cpu, ts.startTimestamp, ts.endTimestamp)
		}
		return nil
	}
	for _, ts := range b.endingRunningIntervals {
		if err := addThreadSpan(ts, Remove); err != nil {
			return err
		}
	}
	for _, ts := range b.startingRunningIntervals {
		if err := addThreadSpan(ts, Add); err != nil {
			return err
		}
	}
	for _, ts := range b.endingWaitingIntervals {
		if err := addThreadSpan(ts, Remove); err != nil {
			return err
		}
	}
	for _, ts := range b.startingWaitingIntervals {
		if err := addThreadSpan(ts, Add); err != nil {
			return err
		}
	}
	for _, csMap := range cpuStates {
		for _, cs := range csMap {
			resp.CPUStates = append(resp.CPUStates, cs)
		}
	}
	// Sort CPUStates first by state (Remove first), then by CPU increasing.
	sort.Slice(resp.CPUStates, func(a, b int) bool {
		csA, csB := resp.CPUStates[a], resp.CPUStates[b]
		return (csA.MergeType > csB.MergeType) ||
			csA.CPU < csB.CPU
	})
	return nil
}

// Returns the interval starting at currentTimestamp, or nil if there are no
// more intervals in the requested range.  Updates currentTimestamp.
func (b *elementaryCPUIntervalBuilder) next() (*ElementaryCPUInterval, error) {
	if b.finished {
		return nil, nil
	}
	startTimestamp, endTimestamp := b.nextInterval()
	ret := &ElementaryCPUInterval{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}
	if err := b.updateRunningAndWaiting(ret); err != nil {
		return nil, err
	}
	b.setCurrentTimestamp(endTimestamp)
	return ret, nil
}

type cpuStateMerger struct {
	cpu     CPUID
	running *Thread
	waiting map[PID]*Thread
}

func newCPUStateMerger(cpu CPUID) *cpuStateMerger {
	return &cpuStateMerger{
		cpu:     cpu,
		running: nil,
		waiting: map[PID]*Thread{},
	}
}

func (csm *cpuStateMerger) mergeDiff(diff *CPUState) error {
	if diff.CPU != csm.cpu {
		return status.Errorf(codes.Internal, "merge CPU mismatch: want %s, got %s", csm.cpu, diff.CPU)
	}
	switch diff.MergeType {
	case Full:
		return status.Errorf(codes.Internal, "mergeDiff accepts a diff, not a full, CPUState")
	case Add:
		if diff.Running != nil && csm.running != nil {
			return status.Errorf(codes.Internal, "can't merge two running threads")
		}
		if diff.Running != nil {
			csm.running = diff.Running
		}
		for _, waiting := range diff.Waiting {
			if _, ok := csm.waiting[waiting.PID]; ok {
				return status.Errorf(codes.Internal, "can't add an already-waiting thread")
			}
			csm.waiting[waiting.PID] = waiting
		}
	case Remove:
		if diff.Running != nil && (csm.running == nil || csm.running.PID != diff.Running.PID) {
			return status.Errorf(codes.Internal, "can't remove a nonrunning thread")
		}
		if diff.Running != nil {
			csm.running = nil
		}
		for _, waiting := range diff.Waiting {
			if _, ok := csm.waiting[waiting.PID]; !ok {
				return status.Errorf(codes.Internal, "can't remove a nonwaiting thread")
			}
			delete(csm.waiting, waiting.PID)
		}
	}
	return nil
}

func (csm *cpuStateMerger) cpuState() *CPUState {
	ret := &CPUState{
		CPU:       csm.cpu,
		Running:   csm.running,
		MergeType: Full,
	}
	for _, waiting := range csm.waiting {
		ret.Waiting = append(ret.Waiting, waiting)
	}
	return ret
}

type elementaryIntervalMerger struct {
	cpuStateMergers map[CPUID]*cpuStateMerger
}

func newElementaryIntervalMerger(f *filter) *elementaryIntervalMerger {
	ret := &elementaryIntervalMerger{
		cpuStateMergers: map[CPUID]*cpuStateMerger{},
	}
	for cpu := range f.cpus {
		ret.cpuStateMergers[cpu] = newCPUStateMerger(cpu)
	}
	return ret
}

func (eim *elementaryIntervalMerger) mergeDiff(eci *ElementaryCPUInterval) error {
	for _, cs := range eci.CPUStates {
		csm := eim.cpuStateMergers[cs.CPU]
		if csm == nil {
			return status.Errorf(codes.Internal, "merging unexpected CPUState from CPU %d", cs.CPU)
		}
		if err := csm.mergeDiff(cs); err != nil {
			return err
		}
	}
	return nil
}

func (eim *elementaryIntervalMerger) elementaryCPUInterval(eci *ElementaryCPUInterval) (*ElementaryCPUInterval, error) {
	if err := eim.mergeDiff(eci); err != nil {
		return nil, err
	}
	ret := &ElementaryCPUInterval{
		StartTimestamp: eci.StartTimestamp,
		EndTimestamp:   eci.EndTimestamp,
	}
	for _, csm := range eim.cpuStateMergers {
		ret.CPUStates = append(ret.CPUStates, csm.cpuState())
	}
	return ret, nil
}

// ElementaryCPUIntervalProvider is a wrapper providing a simple interface for
// fetching elementary CPU intervals one-at-a-time.
type ElementaryCPUIntervalProvider struct {
	eib    *elementaryCPUIntervalBuilder
	merger *elementaryIntervalMerger
}

// NewElementaryCPUIntervalProvider returns a new ElementaryCPUIntervalProvider
// that can be used to iterate through filtered-in elementary intervals.
func (c *Collection) NewElementaryCPUIntervalProvider(diffOutput bool, filters ...Filter) (*ElementaryCPUIntervalProvider, error) {
	f := buildFilter(c, filters)
	ret := &ElementaryCPUIntervalProvider{}
	var err error
	ret.eib, err = c.newElementaryCPUIntervalBuilder(f)
	if err != nil {
		return nil, err
	}
	if !diffOutput {
		ret.merger = newElementaryIntervalMerger(f)
	}
	return ret, nil
}

// NextInterval returns the next elementary CPU interval in the current
// traversal, or nil if there are no more intervals.
func (s *ElementaryCPUIntervalProvider) NextInterval() (*ElementaryCPUInterval, error) {
	nextInterval, err := s.eib.next()
	if err != nil {
		return nil, err
	}
	if nextInterval != nil && s.merger != nil {
		nextInterval, err = s.merger.elementaryCPUInterval(nextInterval)
		if err != nil {
			return nil, err
		}
	}
	return nextInterval, nil
}
