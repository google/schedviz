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
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/google/schedviz/tracedata/trace"
)

// mergeCPU accepts two CPUIDs, and returns their merger:
// * if both have the same value, returns that value,
// * if one is unknown, returns the other,
// * if both are known and they disagree, returns an error.
func mergeCPU(a, b CPUID) (CPUID, error) {
	if a != UnknownCPU && b != UnknownCPU && a != b {
		return UnknownCPU, status.Errorf(codes.Internal, "can't merge different unknown CPUs")
	}
	if a == UnknownCPU {
		return b, nil
	}
	return a, nil
}

// Unknown thread index, PID, CPU, or priority.
const Unknown = -1


// ThreadState specifies the state of a thread at an instant in time.
type ThreadState int8

// ThreadTransitions may specify any combination of RunningState, WaitingState,
// and SleepingState.
const (
	// UnknownState threads are of an indeterminate state, because there is not
	// yet enough information to infer their state.  When used in ThreadTransitions,
	// it is a synonym for AnyState.
	UnknownState ThreadState = 1 << iota
	// RunningState threads are switched-in on a CPU.
	RunningState
	// WaitingState threads are in RUNNABLE state but are not switched in.
	WaitingState
	// SleepingState threads are in a non-RUNNABLE state (usually INTERRUPTIBLE) and
	// are not switched in.
	SleepingState
)


// AnyState is the superposition of all possible thread states -- Running,
// Waiting, and Sleeping.
var AnyState ThreadState = RunningState | WaitingState | SleepingState

func (ts ThreadState) String() string {
	var ret []string
	if ts&UnknownState != 0 {
		ret = append(ret, "Unknown")
	}
	if ts&RunningState != 0 {
		ret = append(ret, "Running")
	}
	if ts&WaitingState != 0 {
		ret = append(ret, "Waiting")
	}
	if ts&SleepingState != 0 {
		ret = append(ret, "Sleeping")
	}
	if len(ret) == 0 {
		ret = []string{"no known state"}
	}
	return strings.Join(ret, " || ")
}

// isKnown returns true iff the receiver is a single known state -- Running,
// Waiting, or Sleeping.
func (ts ThreadState) isKnown() bool {
	return ts == RunningState || ts == WaitingState || ts == SleepingState
}

// mergeState accepts two ThreadStates, and returns their intersection.
// If the intersection is nil, returns false, otherwise returns true.
func mergeState(a, b ThreadState) (ThreadState, bool) {
	ret := a & b
	if ret == 0 {
		return 0, false
	}
	return ret, true
}

// PID specifies a kernel thread ID.  Valid PIDs are nonnegative.
type PID int64

// UnknownPID represents an indeterminate PID value.
const UnknownPID PID = Unknown

// Valid returns true iff the provided PID is valid.
func (p PID) Valid() bool {
	return p > UnknownPID
}

func (p PID) String() string {
	if p.Valid() {
		return fmt.Sprintf("PID %7d", p)
	}
	return unknownString
}

// CPUID specifies a CPU number.  Valid CPUIDs are nonnegative
type CPUID int64

// UnknownCPU represents an indeterminate CPU value.
const UnknownCPU CPUID = Unknown

// Valid returns true iff the provided CPUID is valid.
func (c CPUID) Valid() bool {
	return c > UnknownCPU
}

func (c CPUID) String() string {
	if c.Valid() {
		return fmt.Sprintf("CPU %3d", c)
	}
	return unknownString
}

// Priority specifies a thread priority.  Valid Priorities are nonnegative.
type Priority int64

// UnknownPriority represents an indeterminate thread priority.
const UnknownPriority Priority = Unknown

// Valid returns true iff the provided Priority is valid.
func (p Priority) Valid() bool {
	return p > UnknownPriority
}

func (p Priority) String() string {
	if p.Valid() {
		return fmt.Sprintf("prio %4d", p)
	}
	return unknownString
}

// UnknownCommand represents an indeterminate thread command name.
const UnknownCommand stringID = Unknown

// UnknownTimestamp represents an unspecified event timestamp.
const UnknownTimestamp = trace.UnknownTimestamp

// Duration is a delta between two trace.Timestamps.
type Duration trace.Timestamp

// UnknownDuration represents an unknown interval or span duration.
const UnknownDuration Duration = Unknown

func (d Duration) add(other Duration) Duration {
	if d == UnknownDuration || other == UnknownDuration {
		return UnknownDuration
	}
	return d + other
}

func duration(startTimestamp, endTimestamp trace.Timestamp) Duration {
	if startTimestamp == UnknownTimestamp || endTimestamp == UnknownTimestamp ||
		endTimestamp < startTimestamp {
		return UnknownDuration
	}
	return Duration(endTimestamp - startTimestamp)
}

// Thread describes a single thread's PID, command string, and priority.
type Thread struct {
	PID      PID      `json:"pid"`
	Command  string   `json:"command"`
	Priority Priority `json:"priority"`
}

func (t Thread) String() string {
	return fmt.Sprintf("%s (%s, %s)", t.PID, t.Command, t.Priority)
}

// ThreadResidency describes a duration of time a thread held a state on a CPU.
type ThreadResidency struct {
	Thread *Thread `json:"thread"`
	// The duration of the residency in ns. If StartTimestamp is Unknown, reflects a
	// cumulative duration.
	Duration        Duration    `json:"duration"`
	State           ThreadState `json:"state"`
	DroppedEventIDs []int       `json:"droppedEventIDs"`
	// Set to true if this was constructed from at least one synthetic transitions
	// i.e. a transition that was not in the raw event set.
	IncludesSyntheticTransitions bool `json:"includesSyntheticTransitions"`
}

func (tr *ThreadResidency) merge(other *ThreadResidency) error {
	if tr.Thread.PID != other.Thread.PID || tr.State != other.State {
		return status.Errorf(codes.Internal, "can't merge ThreadResidencies with different PIDs or states")
	}
	tr.Duration = tr.Duration.add(other.Duration)
	eventIds := map[int]struct{}{}
	for _, deid := range tr.DroppedEventIDs {
		eventIds[deid] = struct{}{}
	}
	for _, deid := range other.DroppedEventIDs {
		eventIds[deid] = struct{}{}
	}
	tr.DroppedEventIDs = nil
	for deid := range eventIds {
		tr.DroppedEventIDs = append(tr.DroppedEventIDs, deid)
	}
	tr.IncludesSyntheticTransitions = tr.IncludesSyntheticTransitions || other.IncludesSyntheticTransitions
	return nil
}

func (tr *ThreadResidency) String() string {
	var evIDs = []string{}
	for _, evID := range tr.DroppedEventIDs {
		evIDs = append(evIDs, strconv.Itoa(evID))
	}
	ret := fmt.Sprintf("Thread %s, Duration %d, State %s, Dropped [%s]", tr.Thread, tr.Duration, tr.State,
		strings.Join(evIDs, ","))
	if tr.IncludesSyntheticTransitions {
		ret += " (includes synthetic transitions)"
	}
	return ret
}

// Interval represents a duration of time starting at a specific moment on a
// CPU within a trace.
type Interval struct {
	StartTimestamp      trace.Timestamp    `json:"startTimestamp"`
	Duration            Duration           `json:"duration"`
	CPU                 CPUID              `json:"cpu"`
	ThreadResidencies   []*ThreadResidency `json:"threadResidencies"`
	MergedIntervalCount int                `json:"mergedIntervalCount"`
}

func (ti *Interval) String() string {
	ret := fmt.Sprintf("[%d] StartTimestamp: %d, Duration: %d, %s, \n",
		ti.MergedIntervalCount,
		ti.StartTimestamp, ti.Duration, ti.CPU)
	for _, tr := range ti.ThreadResidencies {
		ret = ret + fmt.Sprintf("  * %s\n", tr)
	}
	return ret
}

// Antagonism is an interval during which a single thread running on a
// single CPU antagonized the waiting victim.
type Antagonism struct {
	RunningThread  *Thread          `json:"runningThread"`
	CPU            CPUID           `json:"cpu"`
	StartTimestamp trace.Timestamp `json:"startTimestamp"`
	EndTimestamp   trace.Timestamp `json:"endTimestamp"`
}

// Antagonists are threads that are running instead of the victim thread
type Antagonists struct {
	Victims     []*Thread     `json:"victims"`
	Antagonisms []*Antagonism `json:"antagonisms"`
	// The time range over which these antagonists were gathered.
	StartTimestamp trace.Timestamp `json:"startTimestamp"`
	EndTimestamp   trace.Timestamp `json:"endTimestamp"`
}

// Metrics holds a set of aggregated metrics for some or all of the sched trace.
type Metrics struct {
	// The number of migrations observed in the aggregated trace.  If CPU
	// filtering was used generating this Metric, only migrations inbound to a
	// filtered-in CPU are aggregated.
	MigrationCount int `json:"migrationCount"`
	// The number of wakeups observed in the aggregated trace.
	WakeupCount int `json:"wakeupCount"`
	// Aggregated thread-state times over the aggregated trace.
	UnknownTimeNs Duration `json:"unknownTimeNs"`
	RunTimeNs     Duration `json:"runTimeNs"`
	WaitTimeNs    Duration `json:"waitTimeNs"`
	SleepTimeNs   Duration `json:"sleepTimeNs"`
	// Unique PIDs, COMMs, priorities, and CPUs observed in the aggregated trace.
	// Note that these fields are not correlated; if portions of trace containing
	// execution from several different PIDs are aggregated together in a metric,
	// all of their PIDs, commands, and priorities will be present here, and the
	// Metrics can reveal which PIDs were present, but it will not be possible to
	// tell from the Metrics which commands go with which PIDs, and so forth.
	// TODO(sabarabc) Create maps from PID -> ([]command, []priority),
	//  command -> ([]PID, []priority), and priority -> ([]PID, []command)
	//  so that we can tell which of these are correlated.
	Pids       []PID      `json:"pids"`
	Commands   []string   `json:"commands"`
	Priorities []Priority `json:"priorities"`
	Cpus       []CPUID    `json:"cpus"`
	// The time range over which these metrics were aggregated.
	StartTimestampNs trace.Timestamp `json:"startTimestampNs"`
	EndTimestampNs   trace.Timestamp `json:"endTimestampNs"`
}
