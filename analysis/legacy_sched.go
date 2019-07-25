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
// Package legacysched exposes a legacy sched analysis library interface for
// SchedViz.  This library is born deprecated; SchedViz v2 should migrate to
// use the native sched analysis library.
package legacysched

import (
	"fmt"
	"sort"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/google/schedviz/tracedata/trace"
)

// Collection wraps sched.Collection and provides a legacy SchedViz interface.
type Collection struct {
	c *sched.Collection
}

// NewCollection creates a new Collection, following the legacy SchedViz
// interface.
func NewCollection(es *eventpb.EventSet, normalizeTimestamps bool) (*Collection, error) {
	c, err := sched.NewCollection(es, sched.DefaultEventLoaders(), sched.NormalizeTimestamps(normalizeTimestamps))
	if err != nil {
		return nil, err
	}
	return &Collection{
		c: c,
	}, nil
}

// WrapCollection creates a new Collection that wraps an existing sched.Collection
func WrapCollection(c *sched.Collection) *Collection {
	return &Collection{c}
}

func mapTimestamp(timestamp time.Duration) trace.Timestamp {
	if timestamp < 0 {
		return sched.UnknownTimestamp
	}
	return trace.Timestamp(timestamp)
}

// ExpandCPUList takes a slice of CPUs, and either returns that slice,
// if it is not empty, or if it is empty, a slice of all CPUs in the
// collection.
func (c *Collection) ExpandCPUList(cpus []int64) []int64 {
	if len(cpus) == 0 {
		return c.CPUs()
	}
	return cpus
}

// CPUs returns slice of CPUs observed in the collection, in increasing order.
func (c *Collection) CPUs() []int64 {
	var cpus = []int64{}
	for cpu := range c.c.CPUs() {
		cpus = append(cpus, int64(cpu))
	}
	sort.Slice(cpus, func(i, j int) bool {
		return cpus[i] < cpus[j]
	})

	return cpus
}

// PIDIntervals returns a slice of models.PIDIntervals reflecting, in
// increasing temporal order, the lifecycle of the provided PID over the
// provided interval.  Adjacent intervals on the same CPU shorter than
// minIntervalDuration are merged together, and are ended as soon as the thread
// migrates to a different CPU or the merged interval exceeds
// minIntervalDuration.  Intervals that cross the requested start or end
// timestamp are left untrimmed; if using this data to aggregate metrics over
// the interval, they should be trimmed before aggregating.
// PIDIntervals maintains the legacy SchedViz interface.
func (c *Collection) PIDIntervals(PID int64, startTimestamp, endTimestamp, minIntervalDuration time.Duration) ([]models.PIDInterval, error) {
	ivals, err := c.c.ThreadIntervals(
		sched.TimeRange(mapTimestamp(startTimestamp), mapTimestamp(endTimestamp)),
		sched.MinIntervalDuration(sched.Duration(minIntervalDuration)),
		sched.PIDs(sched.PID(PID)),
		sched.TruncateToTimeRange(false))
	if err != nil {
		return nil, err
	}
	var ret = []models.PIDInterval{}
	for _, ival := range ivals {
		if len(ival.ThreadResidencies) == 0 {
			return nil, status.Errorf(codes.Internal, "thread interval has no thread residencies")
		}
		pi := models.PIDInterval{}
		pi.StartTimestampNs = int64(ival.StartTimestamp)
		pi.EndTimestampNs = int64(ival.StartTimestamp) + int64(ival.Duration)
		pi.CPU = int64(ival.CPU)
		pi.MergedIntervalCount = int64(ival.MergedIntervalCount)
		// PostWakeup is only meaningful in unmerged intervals, and is unused in the
		// frontend.
		pi.PostWakeup = false
		firstResidency := ival.ThreadResidencies[0]
		if len(ival.ThreadResidencies) == 1 {
			switch firstResidency.State {
			case sched.RunningState:
				pi.State = models.ThreadStateRunningState
			case sched.SleepingState:
				pi.State = models.ThreadStateSleepingState
			case sched.WaitingState:
				pi.State = models.ThreadStateWaitingState
			default:
				pi.State = models.ThreadStateUnknownState
			}
		} else {
			pi.State = models.ThreadStateUnknownState
		}
		pi.Pid = int64(firstResidency.Thread.PID)
		pi.Command = firstResidency.Thread.Command
		pi.Priority = int64(firstResidency.Thread.Priority)
		ret = append(ret, pi)
	}
	return ret, nil
}

// CPUIntervals returns a slice of models.CPUIntervals reflecting, in increasing temporal order, the
// running and waiting processes on the requested CPU over a specified range. Adjacent intervals
// shorter than minIntervalDuration are merged together. New CPUIntervals being added when the
// merged interval's duration exceeds the minIntervalDuration.
func (c *Collection) CPUIntervals(CPU int64, startTimestamp, endTimestamp, minIntervalDuration time.Duration) ([]models.CPUInterval, error) {
	ivals, err := c.c.CPUIntervals(
		sched.TimeRange(mapTimestamp(startTimestamp), mapTimestamp(endTimestamp)),
		sched.MinIntervalDuration(sched.Duration(minIntervalDuration)),
		sched.CPUs(sched.CPUID(CPU)))
	if err != nil {
		return nil, err
	}
	var ret = []models.CPUInterval{}
	for _, ival := range ivals {
		ci := models.CPUInterval{
			StartTimestampNs: int64(ival.StartTimestamp),
			EndTimestampNs:   int64(ival.StartTimestamp) + int64(ival.Duration),
			CPU:              int64(ival.CPU),
		}
		seenRunning := false
		var runningThread *sched.Thread
		var waitingPIDs = []int64{}
		var waitingDuration sched.Duration
		for _, tr := range ival.ThreadResidencies {
			switch tr.State {
			case sched.RunningState:
				if seenRunning {
					// If this CPU interval contains multiple running threads, we can't cite
					// any single running thread.
					runningThread = nil
				} else {
					runningThread = tr.Thread
					seenRunning = true
				}
			case sched.WaitingState:
				waitingPIDs = append(waitingPIDs, int64(tr.Thread.PID))
				waitingDuration += tr.Duration
			}
		}
		ci.WaitingPids = waitingPIDs
		if runningThread != nil {
			ci.RunningPid = int64(runningThread.PID)
			ci.RunningCommand = runningThread.Command
			ci.RunningPriority = int64(runningThread.Priority)
		} else {
			ci.RunningPid = -1
		}
		ci.MergedIntervalCount = int32(ival.MergedIntervalCount)
		if !seenRunning {
			ci.IdleNs = int64(ival.Duration)
		}
		if ival.Duration > 0 {
			ci.WaitingPidCount = float32(waitingDuration) / float32(ival.Duration)
		} else {
			ci.WaitingPidCount = float32(len(waitingPIDs))
		}
		// Don't add running PID 0 (swapper)
		if ci.RunningPid != 0 {
			ret = append(ret, ci)
		}
	}
	return ret, nil
}

func involvesPID(ev *trace.Event, pid int64) bool {
	nextPID, hasNextPID := ev.NumberProperties["next_pid"]
	prevPID, hasPrevPID := ev.NumberProperties["prev_pid"]
	normalPID, hasNormalPID := ev.NumberProperties["pid"]
	return (hasNextPID && nextPID == pid) ||
		(hasPrevPID && prevPID == pid) ||
		(hasNormalPID && normalPID == pid)
}

func getNumberProp(ev *trace.Event, name string) (int64, error) {
	if val, ok := ev.NumberProperties[name]; ok {
		return val, nil
	}
	return 0, fmt.Errorf("%d event (type: %q) does not have %q property", ev.Index, ev.Name, name)
}

func getTextProp(ev *trace.Event, name string) (string, error) {
	if val, ok := ev.TextProperties[name]; ok {
		return val, nil
	}
	return "", fmt.Errorf("%d event (type: %q) does not have %q property", ev.Index, ev.Name, name)
}

func processRawSchedSwitch(ev *trace.Event) (*models.Event, error) {
	prevComm, err := getTextProp(ev, "prev_comm")
	if err != nil {
		return nil, err
	}
	prevPID, err := getNumberProp(ev, "prev_pid")
	if err != nil {
		return nil, err
	}
	prevPrio, err := getNumberProp(ev, "prev_prio")
	if err != nil {
		return nil, err
	}
	prevTaskState, err := getNumberProp(ev, "prev_state")
	if err != nil {
		return nil, err
	}
	nextComm, err := getTextProp(ev, "next_comm")
	if err != nil {
		return nil, err
	}
	nextPid, err := getNumberProp(ev, "next_pid")
	if err != nil {
		return nil, err
	}
	nextPrio, err := getNumberProp(ev, "next_prio")
	if err != nil {
		return nil, err
	}
	return &models.Event{
		EventType:     models.EventTypeSwitch,
		PrevCommand:   prevComm,
		PrevPid:       prevPID,
		PrevPriority:  prevPrio,
		PrevTaskState: prevTaskState,
		Command:       nextComm,
		Pid:           nextPid,
		Priority:      nextPrio,
		CPU:           ev.CPU,
	}, nil
}

func processRawSchedMigrateTask(ev *trace.Event) (*models.Event, error) {
	comm, err := getTextProp(ev, "comm")
	if err != nil {
		return nil, err
	}
	pid, err := getNumberProp(ev, "pid")
	if err != nil {
		return nil, err
	}
	prio, err := getNumberProp(ev, "prio")
	if err != nil {
		return nil, err
	}
	origCPU, err := getNumberProp(ev, "orig_cpu")
	if err != nil {
		return nil, err
	}
	destCPU, err := getNumberProp(ev, "dest_cpu")
	if err != nil {
		return nil, err
	}
	return &models.Event{
		EventType: models.EventTypeMigrateTask,
		Command:   comm,
		Pid:       pid,
		Priority:  prio,
		PrevCPU:   origCPU,
		CPU:       destCPU,
	}, nil
}

func processRawSchedWakeup(ev *trace.Event) (*models.Event, error) {
	comm, err := getTextProp(ev, "comm")
	if err != nil {
		return nil, err
	}
	pid, err := getNumberProp(ev, "pid")
	if err != nil {
		return nil, err
	}
	prio, err := getNumberProp(ev, "prio")
	if err != nil {
		return nil, err
	}
	targetCPU, err := getNumberProp(ev, "target_cpu")
	if err != nil {
		return nil, err
	}

	typ := models.EventTypeWakeup
	if ev.Name == "sched_wakeup_new" {
		typ = models.EventTypeWakeupNew
	}

	return &models.Event{
		EventType: typ,
		Command:   comm,
		Pid:       pid,
		Priority:  prio,
		CPU:       targetCPU,
	}, nil
}

func processRawSchedWait(ev *trace.Event) (*models.Event, error) {
	comm, err := getTextProp(ev, "comm")
	if err != nil {
		return nil, err
	}
	pid, err := getNumberProp(ev, "pid")
	if err != nil {
		return nil, err
	}
	prio, err := getNumberProp(ev, "prio")
	if err != nil {
		return nil, err
	}

	typ := models.EventTypeProcessWait
	if ev.Name == "sched_wait_task" {
		typ = models.EventTypeWaitTask
	}

	return &models.Event{
		EventType: typ,
		Command:   comm,
		Pid:       pid,
		Priority:  prio,
		CPU:       ev.CPU,
	}, nil
}

// PerThreadEventSeries returns all events in a specified collection occurring on a specified PID
// in a specified interval, in increasing temporal order.
func (c *Collection) PerThreadEventSeries(pid int64, startTimestamp, endTimestamp time.Duration) ([]models.Event, error) {
	events, err := c.c.GetRawEvents(
		sched.TimeRange(trace.Timestamp(startTimestamp), trace.Timestamp(endTimestamp)),
		sched.EventTypes("sched_switch", "sched_migrate_task", "sched_wakeup", "sched_wakeup_new", "sched_process_wait", "sched_wait_task"),
	)
	if err != nil {
		return nil, err
	}

	var ret = []models.Event{}
	for _, ev := range events {
		if !involvesPID(ev, pid) {
			continue
		}

		var protoEv *models.Event
		switch ev.Name {
		case "sched_switch":
			protoEv, err = processRawSchedSwitch(ev)
			if err != nil {
				return nil, err
			}
		case "sched_migrate_task":
			protoEv, err = processRawSchedMigrateTask(ev)
			if err != nil {
				return nil, err
			}
		case "sched_wakeup", "sched_wakeup_new":
			protoEv, err = processRawSchedWakeup(ev)
			if err != nil {
				return nil, err
			}
		case "sched_process_wait", "sched_wait_task":
			protoEv, err = processRawSchedWait(ev)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown event type %q", ev.Name)
		}

		protoEv.UniqueID = int64(ev.Index)
		protoEv.TimestampNs = int64(ev.Timestamp)

		ret = append(ret, *protoEv)
	}
	return ret, nil
}

// ThreadSummaries returns a set of thread summaries for a specified collection over a specified interval.
func (c *Collection) ThreadSummaries(cpus []int64, startTimestamp, endTimestamp time.Duration) ([]models.Metrics, error) {
	pidsAndComms, err := c.c.PIDsAndComms()
	if err != nil {
		return nil, err
	}
	var pids = []sched.PID{}
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

	var cpuIDs = []sched.CPUID{}
	for _, cpu := range cpus {
		cpuIDs = append(cpuIDs, sched.CPUID(cpu))
	}
	cpuFilter := sched.CPUs(cpuIDs...)
	filterCPUs := c.c.CPUs(cpuFilter)

	var pidMetrics = []models.Metrics{}
	// Get timestamps from collection if not provided.
	startTS := trace.Timestamp(startTimestamp)
	if startTS <= trace.UnknownTimestamp {
		collectionStartTS, _ := c.c.Interval()
		startTS = collectionStartTS
	}
	endTS := trace.Timestamp(endTimestamp)
	if endTS <= trace.UnknownTimestamp {
		_, collectionEndTS := c.c.Interval()
		endTS = collectionEndTS
	}

	for _, pid := range pids {
		filters := []sched.Filter{sched.PIDs(pid), sched.TimeRange(startTS, endTS)}
		// Don't use CPU filter for ThreadIntervals or else migrates will be filtered out.
		threadIntervals, err := c.c.ThreadIntervals(filters...)
		if err != nil {
			return nil, err
		}
		// Compute metrics on the thread intervals
		metric := c.newMetric(append(filters, cpuFilter)...)
		var lastInterval *sched.Interval

		for _, interval := range threadIntervals {
			if err := metric.recordInterval(filterCPUs, startTS, endTS, lastInterval, interval); err != nil {
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
	filters       []sched.Filter
	pids          map[sched.PID]struct{}
	cpus          map[sched.CPUID]struct{}
	priorities    map[sched.Priority]struct{}
	commands      map[string]struct{}
	s             models.Metrics
	c             *sched.Collection
	intervalCount int
}

func (c *Collection) newMetric(filters ...sched.Filter) metric {
	m := metric{
		filters:    filters,
		c:          c.c,
		cpus:       map[sched.CPUID]struct{}{},
		pids:       map[sched.PID]struct{}{},
		priorities: map[sched.Priority]struct{}{},
		commands:   map[string]struct{}{},
	}

	startTS, endTS := m.c.Interval(m.filters...)
	m.s = models.Metrics{
		StartTimestampNs: int64(startTS),
		EndTimestampNs:   int64(endTS),
	}
	return m
}

func isMigration(last, curr *sched.Interval) bool {
	return last != nil && last.CPU != curr.CPU
}

func isWakeup(last, curr *sched.Interval) bool {
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
	return lastTR.State == sched.SleepingState &&
		(currTR.State == sched.WaitingState || currTR.State == sched.RunningState)
}

func (m *metric) recordInterval(filterCPUs map[sched.CPUID]struct{}, startTimestamp, endTimestamp trace.Timestamp, last, curr *sched.Interval) error {
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

func (m *metric) recordDuration(dur trace.Timestamp, state sched.ThreadState) {
	duration := int64(dur)
	switch state {
	case sched.UnknownState:
		m.s.UnknownTimeNs += duration
	case sched.RunningState:
		m.s.RunTimeNs += duration
	case sched.WaitingState:
		m.s.WaitTimeNs += duration
	case sched.SleepingState:
		m.s.SleepTimeNs += duration
	}
}

func (m *metric) finalize() models.Metrics {
	m.s.Cpus = nil
	for cpu := range m.cpus {
		m.s.Cpus = append(m.s.Cpus, int64(cpu))
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
		m.s.Pids = append(m.s.Pids, int64(pid))
	}
	m.s.Priorities = nil
	for priority := range m.priorities {
		m.s.Priorities = append(m.s.Priorities, int64(priority))
	}
	return m.s
}

// UtilizationMetrics returns a set of metrics describing the utilization or over-utilization of
// some portion of the system over some span of the trace. These metrics are described in
// the sched.Utilization struct.
func (c *Collection) UtilizationMetrics(cpus []int64, startTimestamp, endTimestamp time.Duration) (*models.UtilizationMetrics, error) {
	var cpuIDs = []sched.CPUID{}
	for _, cpu := range cpus {
		cpuIDs = append(cpuIDs, sched.CPUID(cpu))
	}

	um, err := c.c.UtilizationMetrics(sched.CPUs(cpuIDs...), sched.TimeRange(trace.Timestamp(startTimestamp), trace.Timestamp(endTimestamp)), sched.TruncateToTimeRange(true))
	if err != nil {
		return nil, err
	}
	return &models.UtilizationMetrics{
		WallTime:               int64(um.WallTime),
		PerCPUTime:             int64(um.PerCPUTime),
		PerThreadTime:          int64(um.PerThreadTime),
		CPUUtilizationFraction: um.UtilizationFraction,
	}, nil
}
