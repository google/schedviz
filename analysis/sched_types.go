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

// Unknown thread index, thread state, PID, CPU, or priority.
const Unknown = -1

// ThreadState specifies the state of a thread at an instant in time.
type ThreadState int8

const (
	// UnknownState threads are of an indeterminate state, because there is not
	// yet enough information to infer their state.
	UnknownState ThreadState = iota + Unknown
	// RunningState threads are switched-in on a CPU.
	RunningState
	// WaitingState threads are in RUNNABLE state but are not switched in.
	WaitingState
	// SleepingState threads are in a non-RUNNABLE state (usually INTERRUPTIBLE) and
	// are not switched in.
	SleepingState
)

func (ts ThreadState) String() string {
	switch ts {
	case UnknownState:
		return "Unknown"
	case RunningState:
		return "Running"
	case WaitingState:
		return "Waiting"
	case SleepingState:
		return "Sleeping"
	default:
		return "Invalid State"
	}
}

// mergeState accepts two ThreadStates, and returns their merger:
// * if both have the same value, returns that value,
// * if one is unknown, returns the other,
// * if both are known and they disagree, returns an error.
func mergeState(a, b ThreadState) (ThreadState, error) {
	if a != UnknownState && b != UnknownState && a != b {
		return UnknownState, status.Errorf(codes.Internal, "can't merge different unknown states")
	}
	if a == UnknownState {
		return b, nil
	}
	return a, nil
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
	PID      PID
	Command  string
	Priority Priority
}

func (t Thread) String() string {
	return fmt.Sprintf("%s (%s, %s)", t.PID, t.Command, t.Priority)
}

// ThreadResidency describes a duration of time a thread held a state on a CPU.
type ThreadResidency struct {
	Thread *Thread
	// The duration of the residency.  If StartTimestamp is Unknown, reflects a
	// cumulative duration.
	Duration                     Duration
	State                        ThreadState
	DroppedEventIDs              []int
	IncludesSyntheticTransitions bool
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
	StartTimestamp      trace.Timestamp
	Duration            Duration
	CPU                 CPUID
	ThreadResidencies   []*ThreadResidency
	MergedIntervalCount int
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
