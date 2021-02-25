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

// PIDsAndComms returns the PIDs and their commands observed in the filtered-in
// portion of the collection.
// FILTERS:
//   PIDs: Only the filtered-in PIDs are included.
//   TimeRange, StartTimestamp, EndTimestamp: Only threads observed in the
//       filtered-in range are returned.
func (c *Collection) PIDsAndComms(filters ...Filter) (map[PID][]string, error) {
	f := buildFilter(c, filters)
	pidToCommSet := map[PID]map[string]struct{}{}
	for pid := range f.pids {
		pidToCommSet[pid] = map[string]struct{}{}
		ts := c.spansByPID[pid]
		// Binary search the filter time range to quickly find the subsequence of
		// spans we care about.
		startIdx := sort.Search(len(ts), func(i int) bool {
			return ts[i].endTimestamp >= f.startTimestamp
		})
		endIdx := sort.Search(len(ts), func(i int) bool {
			return ts[i].startTimestamp > f.endTimestamp
		})
		for _, span := range ts[startIdx:endIdx] {
			if !f.spanFilteredIn(span) || span.command == UnknownCommand {
				continue
			}
			comm, err := c.LookupCommand(span.command)
			if err != nil {
				return nil, err
			}
			pidToCommSet[pid][comm] = struct{}{}
		}
	}
	ret := map[PID][]string{}
	for pid, comms := range pidToCommSet {
		for comm := range comms {
			ret[pid] = append(ret[pid], comm)
		}
		sort.Slice(ret[pid], func(a, b int) bool {
			return ret[pid][a] < ret[pid][b]
		})
	}
	return ret, nil
}

type threadIntervalBuilder struct {
	c                   *Collection
	f                   *filter
	startTimestamp      trace.Timestamp
	duration            Duration
	thread              *Thread
	cpu                 CPUID
	threadResidencies   map[ThreadState]*ThreadResidency
	mergedIntervalCount int
}

func (c *Collection) newThreadIntervalBuilder(f *filter) *threadIntervalBuilder {
	tib := &threadIntervalBuilder{
		c: c,
		f: f,
	}
	tib.fetchAndReset()
	return tib
}

func (tib *threadIntervalBuilder) fetchAndReset() *Interval {
	var ret *Interval
	if tib.startTimestamp != UnknownTimestamp {
		ret = &Interval{
			StartTimestamp:      tib.startTimestamp,
			Duration:            tib.duration,
			CPU:                 tib.cpu,
			MergedIntervalCount: tib.mergedIntervalCount,
			ThreadResidencies:   []*ThreadResidency{},
		}
		// Add residencies in a fixed order.
		for _, state := range []ThreadState{RunningState, SleepingState, UnknownState, WaitingState} {
			if tr, ok := tib.threadResidencies[state]; ok {
				ret.ThreadResidencies = append(ret.ThreadResidencies, tr)
			}
		}

	}
	tib.startTimestamp = UnknownTimestamp
	tib.duration = UnknownDuration
	tib.thread = &Thread{
		PID:      UnknownPID,
		Priority: UnknownPriority,
	}
	tib.cpu = UnknownCPU
	tib.threadResidencies = map[ThreadState]*ThreadResidency{}
	tib.mergedIntervalCount = 0
	return ret
}

func (tib *threadIntervalBuilder) addSpan(span *threadSpan) (*Interval, error) {
	var ret *Interval
	if !tib.f.spanFilteredIn(span) {
		return nil, nil
	}
	// If the current interval is already long enough, emit it.
	if (tib.duration != UnknownDuration && tib.duration+span.duration() > tib.f.minIntervalDuration) ||
		(tib.cpu != UnknownCPU && tib.cpu != span.cpu) {
		ret = tib.fetchAndReset()
	}
	// Adjust start and end timestamps according to the filter.
	startTimestamp := span.startTimestamp
	endTimestamp := span.endTimestamp
	// Our known state runs 1ns beyond the end of the trace, since any subsequent
	// change would take effect at the next ns after the end of the trace.  If
	// we have such a span, truncate the end timestamp to the end of the trace.
	if span.endTimestamp > tib.c.endTimestamp {
		endTimestamp = tib.c.endTimestamp
	}
	if tib.f.truncateToTimeRange {
		if startTimestamp < tib.f.startTimestamp {
			startTimestamp = tib.f.startTimestamp
		}
		if endTimestamp > tib.f.endTimestamp {
			endTimestamp = tib.f.endTimestamp
		}
	}
	// If there's no interval currently being built, set one up.
	if tib.startTimestamp == UnknownTimestamp {
		comm, err := tib.c.LookupCommand(span.command)
		if err != nil {
			return nil, err
		}
		tib.thread.PID = span.pid
		tib.thread.Command = comm
		tib.thread.Priority = span.priority
		tib.startTimestamp = startTimestamp
		tib.duration = 0
		tib.cpu = span.cpu
	}
	if tib.thread.PID != span.pid {
		return nil, status.Errorf(codes.Internal, "can't merge a threadSpan from PID %d into a Interval with PID %d", span.pid, tib.thread.PID)
	}
	ti, ok := tib.threadResidencies[span.state]
	if !ok {
		ti = &ThreadResidency{
			Thread: tib.thread,
			State:  span.state,
		}
		tib.threadResidencies[span.state] = ti
	}
	dur := duration(startTimestamp, endTimestamp)
	ti.Duration = ti.Duration.add(dur)
	ti.DroppedEventIDs = append(ti.DroppedEventIDs, span.droppedEventIDs...)
	ti.IncludesSyntheticTransitions = ti.IncludesSyntheticTransitions || span.syntheticStart || span.syntheticEnd
	tib.duration = tib.duration.add(dur)
	tib.mergedIntervalCount++
	return ret, nil
}

// ThreadIntervals returns a slice of Intervals representing, in increasing
// temporal order, the filtered-in scheduling intervals pertaining to a single
// thread.  Intervals are split by a change of thread state or CPU.
// If the filter specifies to truncate to time range, intervals spanning one
// or both extremities of the filtered-in time range will be truncated to the
// extremities; otherwise they will be left whole.  The latter can be useful
// for visualizers wishing to report the true length of a given interval even
// when part of it lies outside the viewport.
// FILTERS:
//   PIDs: ThreadIntervals expects a single PID to be filtered in; this PID is
//       the one for which intervals will be returned.
//   CPUs: Intervals are restricted to only the specified CPUs.
//   TimeRange, StartTimestamp, EndTimestamp: ThreadIntervals are produced for
//       the filtered-in time range.
//   ThreadState: Intervals are restricted to only the specified thread states.
//   TruncateToTimeRange: If true, returned intervals will be clipped to the
//       filtered time range.
//   MinIntervalDuration: adjacent thread intervals are aggregated until they
//       exceed the specified MinIntervalDuration.  When this occurs, multiple
//       ThreadResidencies within the Interval will reflect what states the PID
//       held, and for how long, but not precisely win.  Intervals with
//       different CPUs are never aggregated.
func (c *Collection) ThreadIntervals(filters ...Filter) ([]*Interval, error) {
	f := buildFilter(c, filters)
	var pids = []PID{}
	for pid := range f.pids {
		pids = append(pids, pid)
	}
	if len(pids) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "exactly one PID required for ThreadIntervals")
	}
	pid := pids[0]
	tib := c.newThreadIntervalBuilder(f)
	var tis = []*Interval{}
	span := c.spansByPID[pid]
	// Binary search the filter time range to quickly find the subsequence of
	// spans we care about.
	startIdx := sort.Search(len(span), func(i int) bool {
		return span[i].endTimestamp >= f.startTimestamp
	})
	endIdx := sort.Search(len(span), func(i int) bool {
		return span[i].startTimestamp > f.endTimestamp
	})
	for _, span := range span[startIdx:endIdx] {
		ti, err := tib.addSpan(span)
		if err != nil {
			return nil, err
		}
		if ti != nil {
			tis = append(tis, ti)
		}
	}
	ti := tib.fetchAndReset()
	if ti != nil {
		tis = append(tis, ti)
	}
	return tis, nil
}

type cpuIntervalBuilder struct {
	f                       *filter
	splitOnWaitingPIDChange bool
	startTimestamp          trace.Timestamp
	duration                Duration
	cpu                     CPUID
	waitingPIDs             map[PID]struct{}
	runningPID              PID
	threadResidencies       map[ThreadState]map[PID]*ThreadResidency
	mergedIntervalCount     int
}

func (c *Collection) newCPUIntervalBuilder(splitOnWaitingPIDChange bool, f *filter) *cpuIntervalBuilder {
	cib := &cpuIntervalBuilder{
		f:                       f,
		splitOnWaitingPIDChange: splitOnWaitingPIDChange,
	}
	cib.fetchAndReset()
	return cib
}

func (cib *cpuIntervalBuilder) fetchAndReset() *Interval {
	var ret *Interval
	if cib.startTimestamp != UnknownTimestamp {
		ret = &Interval{
			StartTimestamp:      cib.startTimestamp,
			Duration:            cib.duration,
			CPU:                 cib.cpu,
			MergedIntervalCount: cib.mergedIntervalCount,
			ThreadResidencies:   []*ThreadResidency{},
		}
		// Add residencies in a fixed order.
		for _, state := range []ThreadState{RunningState, SleepingState, UnknownState, WaitingState} {
			var pids = []PID{}
			for pid := range cib.threadResidencies[state] {
				pids = append(pids, pid)
			}
			sort.Slice(pids, func(a, b int) bool {
				return pids[a] < pids[b]
			})
			for _, pid := range pids {
				ret.ThreadResidencies = append(ret.ThreadResidencies, cib.threadResidencies[state][pid])
			}
		}
	}
	cib.startTimestamp = UnknownTimestamp
	cib.duration = UnknownDuration
	cib.cpu = UnknownCPU
	cib.waitingPIDs = map[PID]struct{}{}
	cib.runningPID = UnknownPID
	cib.threadResidencies = map[ThreadState]map[PID]*ThreadResidency{
		RunningState: {},
		WaitingState: {},
	}
	cib.mergedIntervalCount = 0
	return ret
}

func elementaryCPUIntervalToThreadResidencies(eci *ElementaryCPUInterval) []*ThreadResidency {
	var ret = []*ThreadResidency{}
	for _, cpuState := range eci.CPUStates {
		if cpuState.Running != nil {
			ret = append(ret, &ThreadResidency{
				Thread:   cpuState.Running,
				Duration: eci.duration(),
				State:    RunningState,
			})
		}
		for _, waiting := range cpuState.Waiting {
			ret = append(ret, &ThreadResidency{
				Thread:   waiting,
				Duration: eci.duration(),
				State:    WaitingState,
			})
		}
	}
	return ret
}

func (cib *cpuIntervalBuilder) addElementaryInterval(eci *ElementaryCPUInterval) (*Interval, error) {
	if len(eci.CPUStates) != 1 {
		return nil, status.Errorf(codes.Internal, "expected 1 CPUState in elementary interval, got %d", len(eci.CPUStates))
	}
	var ret *Interval
	cpuState := eci.CPUStates[0]
	// The ongoing CPU interval may be split if the running PID changes
	split := false
	if (cpuState.Running != nil || cib.runningPID != UnknownPID) &&
		((cpuState.Running == nil && cib.runningPID != UnknownPID) ||
			(cpuState.Running != nil && cib.runningPID == UnknownPID) ||
			(cpuState.Running.PID != cib.runningPID)) {
		split = true
	}
	if cib.splitOnWaitingPIDChange {
		if len(cpuState.Waiting) != len(cib.waitingPIDs) {
			split = true
		} else {
			for _, t := range cpuState.Waiting {
				if _, ok := cib.waitingPIDs[t.PID]; !ok {
					split = true
					break
				}
			}
		}
	}
	// If the current interval is already long enough and a split is in order,
	// emit it.
	if cib.duration != UnknownDuration &&
		cib.duration+eci.duration() > cib.f.minIntervalDuration &&
		split {
		ret = cib.fetchAndReset()
	}
	// If there's no interval currently being built, set one up.
	if cib.startTimestamp == UnknownTimestamp {
		cib.startTimestamp = eci.StartTimestamp
		cib.duration = 0
		cib.cpu = cpuState.CPU
	}
	if cib.cpu != cpuState.CPU {
		return nil, status.Errorf(codes.Internal, "can't merge an ElementaryCPUInterval from CPU %d into a Interval with CPU %d", cpuState.CPU, cib.cpu)
	}
	if cpuState.Running == nil {
		cib.runningPID = UnknownPID
	} else {
		cib.runningPID = cpuState.Running.PID
	}
	cib.waitingPIDs = map[PID]struct{}{}
	for _, t := range cpuState.Waiting {
		cib.waitingPIDs[t.PID] = struct{}{}
	}
	for _, newTr := range elementaryCPUIntervalToThreadResidencies(eci) {
		tr, ok := cib.threadResidencies[newTr.State][newTr.Thread.PID]
		if !ok {
			cib.threadResidencies[newTr.State][newTr.Thread.PID] = newTr
			tr = newTr
		} else {
			if err := tr.merge(newTr); err != nil {
				return nil, err
			}
		}
	}
	cib.duration = cib.duration.add(eci.duration())
	cib.mergedIntervalCount++
	return ret, nil
}

// CPUIntervals returns a slice of Intervals representing, in increasing
// temporal order, the filtered-in scheduling intervals pertaining to a single
// CPU.  Intervals are split on a change of running thread and, if
// splitOnWaitingPIDChange is true, when a thread starts or stops waiting on
// the CPU.
// If the filter specifies to truncate to time range, intervals spanning one
// or both extremities of the filtered-in time range will be truncated to the
// extremities; otherwise they will be left whole.  The latter can be useful
// for visualizers wishing to report the true length of a given interval even
// when part of it lies outside the viewport.
// If the filter specifies a minimum time interval greater than 1, smaller
// adjacent intervals will be aggregated together until they exceed that
// minimum interval.  When this occurs, multiple ThreadResidencies within the
// Interval will reflect what PIDs held what states on the CPU, and for how
// long, but not precisely when.
// FILTERS:
//   CPUIntervals performs its calculations over elementary CPU intervals,
//   so it honors the same filters as NewElementaryCPUIntervalProvider:
//   TruncateToTimeRange: If true, returned elementary intervals will be
//       clipped to the filtered time range.
//   TimeRange, StartTimestamp, EndTimestamp: Intervals are generated over the
//       filtered-in time range.
//   CPUs: Intervals are restricted to the specified CPUs.
//   PIDs: Intervals are restricted to the specified PIDs.
//   ThreadStates: Elementary intervals only include running and waiting
//       threads
//   It also supports:
//   MinIntervalDuration: adjacent CPU intervals are aggregated until they
//       exceed the specified MinIntervalDuration.  When this occurs, multiple
//       ThreadResidencies within the Interval will reflect what PIDs held what
//       states on the CPU, and for how long, but not precisely when.
func (c *Collection) CPUIntervals(splitOnWaitingPIDChange bool, filters ...Filter) ([]*Interval, error) {
	f := buildFilter(c, filters)
	if len(f.cpus) > 1 {
		return nil, status.Errorf(codes.InvalidArgument, "exactly one CPU required for Intervals")
	}
	provider, err := c.NewElementaryCPUIntervalProvider(false /*=diffOutput*/, filters...)
	if err != nil {
		return nil, err
	}
	var ret = []*Interval{}
	if len(f.cpus) == 0 {
		return ret, nil
	}
	cib := c.newCPUIntervalBuilder(splitOnWaitingPIDChange, f)
	for {
		elemInterval, err := provider.NextInterval()
		if err != nil {
			return nil, err
		}
		if elemInterval == nil {
			ret = append(ret, cib.fetchAndReset())
			return ret, nil
		}
		ti, err := cib.addElementaryInterval(elemInterval)
		if err != nil {
			return nil, err
		}
		if ti != nil {
			ret = append(ret, ti)
		}
	}
}
