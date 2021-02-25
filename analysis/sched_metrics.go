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
	"time"

	"github.com/google/schedviz/tracedata/trace"
)

func involvesPID(ev *trace.Event, pid PID) bool {
	nextPID, hasNextPID := ev.NumberProperties["next_pid"]
	prevPID, hasPrevPID := ev.NumberProperties["prev_pid"]
	normalPID, hasNormalPID := ev.NumberProperties["pid"]
	return (hasNextPID && PID(nextPID) == pid) ||
		(hasPrevPID && PID(prevPID) == pid) ||
		(hasNormalPID && PID(normalPID) == pid)
}

// PerThreadEventSeries returns all events in a specified collection occurring on a specified PID
// in a specified interval, in increasing temporal order.
func (c *Collection) PerThreadEventSeries(pid PID, startTimestamp, endTimestamp time.Duration) ([]*trace.Event, error) {
	events, err := c.GetRawEvents(
		TimeRange(trace.Timestamp(startTimestamp), trace.Timestamp(endTimestamp)),
		EventTypes("sched_switch", "sched_migrate_task", "sched_wakeup", "sched_wakeup_new", "sched_process_wait", "sched_wait_task"),
	)
	if err != nil {
		return nil, err
	}

	// Filter out events that don't involve this PID.
	ret := []*trace.Event{}
	for i := range events {
		ev := events[i]
		if involvesPID(ev, pid) {
			ret = append(ret, ev)
		}
	}

	return ret, nil
}

// ThreadSummaries returns a set of thread summaries for a specified collection
// over a specified interval.
// FILTERS:
//   ThreadSummaries performs its calculations over thread intervals, so it
//   honors the same filters as ThreadIntervals, in the same ways.  However, it
//   calls ThreadIntervals for each individual thread in the filtered-in PID
//   set.
func (c *Collection) ThreadSummaries(filters ...Filter) ([]*Metrics, error) {
	f := buildFilter(c, filters)
	pidsAndComms, err := c.PIDsAndComms()
	if err != nil {
		return nil, err
	}
	var pids = []PID{}
	for pid := range pidsAndComms {
		// Do not attempt to summarize PID 0.
		if pid == 0 {
			continue
		}
		pids = append(pids, pid)
	}

	// Sort pids for deterministic ordering
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})

	var cpuIDs = []CPUID{}
	for cpu := range f.cpus {
		cpuIDs = append(cpuIDs, CPUID(cpu))
	}
	cpuFilter := CPUs(cpuIDs...)
	filterCPUs := c.CPUs(cpuFilter)

	var pidMetrics = []*Metrics{}

	for _, pid := range pids {
		filters := []Filter{PIDs(pid), TimeRange(f.startTimestamp, f.endTimestamp)}
		// Don't use CPU filter for ThreadIntervals or else migrates will be filtered out.
		threadIntervals, err := c.ThreadIntervals(filters...)
		if err != nil {
			return nil, err
		}
		// Compute metrics on the thread intervals
		metric := newMetric(c, append(filters, cpuFilter)...)
		var lastInterval *Interval

		for _, interval := range threadIntervals {
			if err := metric.recordInterval(filterCPUs, f.startTimestamp, f.endTimestamp, lastInterval, interval); err != nil {
				return nil, err
			}
			lastInterval = interval
		}
		if metric.intervalCount > 0 {
			metric.intervalCount = 0
			pidMetrics = append(pidMetrics, metric.finalize())
		}
	}

	return pidMetrics, nil
}

type metric struct {
	filters       []Filter
	pids          map[PID]struct{}
	cpus          map[CPUID]struct{}
	priorities    map[Priority]struct{}
	commands      map[string]struct{}
	s             *Metrics
	c             *Collection
	intervalCount int
}

func newMetric(c *Collection, filters ...Filter) metric {
	m := metric{
		filters:    filters,
		c:          c,
		cpus:       map[CPUID]struct{}{},
		pids:       map[PID]struct{}{},
		priorities: map[Priority]struct{}{},
		commands:   map[string]struct{}{},
	}

	startTS, endTS := m.c.Interval(m.filters...)
	m.s = &Metrics{
		StartTimestampNs: startTS,
		EndTimestampNs:   endTS,
	}
	return m
}
func isMigration(last, curr *Interval) bool {
	return last != nil && last.CPU != curr.CPU
}

func isWakeup(last, curr *Interval) bool {
	if last == nil {
		return false
	}
	if len(last.ThreadResidencies) != 1 || len(curr.ThreadResidencies) != 1 {
		// There should never be more than one thread residency per interval due to the PID filter,
		// but if there somehow is one, don't even try to compute if the interval is a wakeup or not.
		return false
	}
	lastTR := last.ThreadResidencies[0]
	currTR := curr.ThreadResidencies[0]
	return lastTR.State == SleepingState &&
		(currTR.State == WaitingState || currTR.State == RunningState)
}

func (m *metric) recordInterval(filterCPUs map[CPUID]struct{}, startTimestamp, endTimestamp trace.Timestamp, last, curr *Interval) error {
	// If filterCPUs is set and the interval's CPU is filtered, don't record anything
	if _, ok := filterCPUs[curr.CPU]; len(filterCPUs) > 0 && !ok {
		return nil
	}
	m.intervalCount++

	if curr.StartTimestamp+trace.Timestamp(curr.Duration) <= endTimestamp {
		endTimestamp = curr.StartTimestamp + trace.Timestamp(curr.Duration)
	}
	if curr.StartTimestamp > startTimestamp {
		startTimestamp = curr.StartTimestamp
	}

	m.cpus[curr.CPU] = struct{}{}
	// Should only ever have one ThreadResidency, but iterate just in case.
	for _, tr := range curr.ThreadResidencies {
		thread := tr.Thread
		m.pids[thread.PID] = struct{}{}
		m.priorities[thread.Priority] = struct{}{}
		m.commands[thread.Command] = struct{}{}
		m.recordDuration(endTimestamp-startTimestamp, tr.State)
	}

	if isMigration(last, curr) {
		m.s.MigrationCount++
	}
	if isWakeup(last, curr) {
		m.s.WakeupCount++
	}

	return nil
}

func (m *metric) recordDuration(dur trace.Timestamp, state ThreadState) {
	duration := Duration(dur)
	switch state {
	case RunningState:
		m.s.RunTimeNs += duration
	case WaitingState:
		m.s.WaitTimeNs += duration
	case SleepingState:
		m.s.SleepTimeNs += duration
	default:
		m.s.UnknownTimeNs += duration
	}
}

func (m *metric) finalize() *Metrics {
	m.s.Cpus = nil
	for cpu := range m.cpus {
		m.s.Cpus = append(m.s.Cpus, cpu)
	}
	sort.Slice(m.s.Cpus, func(i, j int) bool {
		return m.s.Cpus[i] < m.s.Cpus[j]
	})
	m.s.Commands = nil
	for command := range m.commands {
		m.s.Commands = append(m.s.Commands, command)
	}
	m.s.Pids = nil
	for pid := range m.pids {
		m.s.Pids = append(m.s.Pids, pid)
	}
	m.s.Priorities = nil
	for priority := range m.priorities {
		m.s.Priorities = append(m.s.Priorities, priority)
	}
	return m.s
}
