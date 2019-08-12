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
	"errors"
	"fmt"
	"sort"

	"github.com/google/schedviz/tracedata/trace"
)

func pidMapKeys(m map[PID]struct{}) []PID {
	keys := make([]PID, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

type antagonistBuilder struct {
	pid            PID
	startTimestamp trace.Timestamp
	endTimestamp   trace.Timestamp
	stringTable    *stringTable
	victims        map[string]Thread
	antagonisms    []Antagonism
}

func newAntagonistBuilder(pid PID, startTimestamp, endTimestamp trace.Timestamp, sTbl *stringTable) *antagonistBuilder {
	return &antagonistBuilder{
		pid:            pid,
		startTimestamp: startTimestamp,
		endTimestamp:   endTimestamp,
		stringTable:    sTbl,
		victims:        make(map[string]Thread),
		antagonisms:    []Antagonism{},
	}
}

// addVictim adds a victim thread to the builder's victims list.
func (ab *antagonistBuilder) addVictim(span *threadSpan) error {
	if span.pid != ab.pid {
		return fmt.Errorf("'victim' span has wrong pid. want: %d, got: %d", ab.pid, span.pid)
	}
	cmd, err := ab.stringTable.stringByID(span.command)
	if err != nil {
		return fmt.Errorf("could not get victim command with string ID %d", span.command)
	}
	ab.victims[fmt.Sprintf("%d:%s", span.pid, cmd)] = Thread{
		Priority: span.priority,
		Command:  cmd,
		PID:      span.pid,
	}
	return nil
}

// RecordAntagonism saves a new antagonist into the builder state.
func (ab *antagonistBuilder) RecordAntagonism(waiting, antagonist *threadSpan) error {
	if antagonist.pid == ab.pid {
		return fmt.Errorf("PID %d is improperly antagonizing itself", ab.pid)
	}
	cmd, err := ab.stringTable.stringByID(antagonist.command)
	if err != nil {
		return fmt.Errorf("could not find antagonist command: %s", err)
	}

	startTimestamp := waiting.startTimestamp
	if startTimestamp < antagonist.startTimestamp {
		startTimestamp = antagonist.startTimestamp
	}
	if startTimestamp < ab.startTimestamp {
		startTimestamp = ab.startTimestamp
	}

	endTimestamp := antagonist.endTimestamp
	if endTimestamp < antagonist.endTimestamp {
		endTimestamp = antagonist.endTimestamp
	}
	if endTimestamp > ab.endTimestamp {
		endTimestamp = ab.endTimestamp
	}
	if startTimestamp >= endTimestamp {
		// The requested interval is negative, empty, or lies entirely outside of ab's range; do nothing.
		return nil
	}

	ab.antagonisms = append(ab.antagonisms, Antagonism{
		RunningThread: Thread{
			PID:      antagonist.pid,
			Command:  cmd,
			Priority: antagonist.priority,
		},
		CPU:            antagonist.cpu,
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	})
	return nil
}

// Antagonists returns a Antagonists that contains all of the recorded antagonists.
func (ab *antagonistBuilder) Antagonists() Antagonists {
	var victims = []Thread{}
	for _, v := range ab.victims {
		victims = append(victims, v)
	}
	return Antagonists{
		Victims:        victims,
		Antagonisms:    ab.antagonisms,
		StartTimestamp: ab.startTimestamp,
		EndTimestamp:   ab.endTimestamp,
	}
}

// Antagonists analyzes a single provided victim thread over a provided
// interval, returning a list of antagonisms -- intervals where other threads
// ran on the victim's core while the victim itself was waiting (runnable but
// not running.)  Its complexity is O(N) on the total events in the collection,
// as it looks at any given event at most twice -- once when iterating through
// per-PID events, and once when iterating through per-CPU events.
func (c *Collection) Antagonists(filters ...Filter) (Antagonists, error) {
	f := buildFilter(c, filters)
	pids := pidMapKeys(f.pids)
	if len(pids) != 1 {
		return Antagonists{}, errors.New("can only collect antagonists of a single PID")
	}
	pid := pids[0]
	if pid == 0 {
		return Antagonists{}, errors.New("antagonist analysis not available for PID 0")
	}

	ab := newAntagonistBuilder(pid, f.startTimestamp, f.endTimestamp, c.stringTable)

	pidSpans := c.spansByPID[pid]
	pidStart := sort.Search(len(pidSpans), func(i int) bool {
		return pidSpans[i].endTimestamp >= f.startTimestamp
	})
	for i := pidStart; i < len(pidSpans); i++ {
		pidSpan := pidSpans[i]
		if pidSpan.startTimestamp > f.endTimestamp {
			break
		}
		// Victim Thread is recorded even if the thread is never victimized (i.e. had to wait)
		if err := ab.addVictim(pidSpan); err != nil {
			return Antagonists{}, err
		}
		// If this span is waiting, get a list of all running spans on the same cpu.
		if pidSpan.state == WaitingState {
			runThreads := c.runningSpansByCPU[pidSpan.cpu]
			cpuStart := sort.Search(len(runThreads), func(j int) bool {
				return runThreads[j].endTimestamp >= pidSpan.startTimestamp
			})
			for j := cpuStart; j < len(runThreads); j++ {
				antagonist := runThreads[j]
				if antagonist.startTimestamp > pidSpan.endTimestamp {
					break
				}
				if antagonist.state != RunningState {
					return Antagonists{}, fmt.Errorf("antagonist %v was not running", antagonist)
				}
				if antagonist.pid == pid {
					// a thread can not antagonize itself
					continue
				}

				if err := ab.RecordAntagonism(pidSpan, antagonist); err != nil {
					return Antagonists{}, err
				}
			}
		}
	}

	return ab.Antagonists(), nil
}

// Utilization groups together several metrics describing the utilization or over-utilization of
// some portion of the system over some span of the trace. For example, if 2 CPUs were each
// overloaded for half a second, one with one waiting thread and the other with 2 working threads,
// while three other CPUs were idle, a Utilization structure with WallTime of 500 ms, PerCPUTime of
// 1 sec, PerThreadTime of 1.5 sec, and UtilizationFraction of 25%, is returned.
type Utilization struct {
	// WallTime accumulates the duration of time over which *any* CPU in the set
	// was idle (running swapper) while any other was overloaded (had waiting
	// threads).
	WallTime Duration
	// PerCPUTime accumulates the total per-CPU time during which some CPUs lay
	// idle while at least as many others were overloaded.
	PerCPUTime Duration
	// PerThreadTime accumulates the total per-thread time during which some
	// threads waited on overloaded CPUs while at least as many other CPUs lay
	// idle.
	PerThreadTime Duration
	// UtilizationFraction expresses the fraction of total requested CPU-time
	// spent not idle; that is, doing useful work.
	UtilizationFraction float64
}

// UtilizationMetrics returns a Utilization for the requested set of CPUs and over the requested
// duration.  Any CPUs in the requested set that lack any events are dropped, as we can't tell
// whether they were off, idle, or running a single thread the entire time.
func (c *Collection) UtilizationMetrics(filters ...Filter) (Utilization, error) {
	f := buildFilter(c, filters)
	provider, err := c.NewElementaryCPUIntervalProvider(true /*=diffOutput*/, filters...)
	if err != nil {
		return Utilization{}, err
	}
	um := Utilization{}
	eim := newElementaryIntervalMerger(f)
	cpuCount := len(f.cpus)
	var totalTime, idleTime Duration
	eiCount := 0
	for {
		elemInterval, err := provider.NextInterval()
		if err != nil {
			return Utilization{}, err
		}
		if elemInterval == nil {
			if totalTime > 0 {
				um.UtilizationFraction = 1 - float64(idleTime)/float64(totalTime)
			}
			return um, nil
		}
		eiCount++
		intervalDuration := Duration(elemInterval.EndTimestamp - elemInterval.StartTimestamp)
		totalTime += intervalDuration * Duration(cpuCount)
		waitingThreadCount := 0
		// Idle CPUs have nothing currently running on them.
		idleCPUs := map[CPUID]struct{}{}
		// Overloaded CPUs currently have something waiting to run on them.
		overloadedCPUs := map[CPUID]struct{}{}
		if err := eim.mergeDiff(elemInterval); err != nil {
			return Utilization{}, err
		}
		for _, csm := range eim.cpuStateMergers {
			// CPU is idle if either no thread was running on it (can happen when unused for all recorded
			// time before now) or if it has a process with PID 0.
			isIdle := csm.running == nil || csm.running.PID == 0
			// CPU is overloaded if it has any waiting threads
			isOverloaded := len(csm.waiting) > 0

			if isIdle && isOverloaded {
				// Don't count CPUs that are both idle and overloaded as either: the scheduler seldom reacts
				// instantly to an opportunity, but generally does react very quickly.
				continue
			}
			if isIdle {
				idleCPUs[csm.cpu] = struct{}{}
			}
			if isOverloaded {
				overloadedCPUs[csm.cpu] = struct{}{}
				waitingThreadCount += len(csm.waiting)
			}
		}

		idleCPUCount := len(idleCPUs)
		overloadedCPUCount := len(overloadedCPUs)

		idleTime += Duration(idleCPUCount) * intervalDuration

		// Check if any CPU is idle while some others are overloaded
		if idleCPUCount > 0 && overloadedCPUCount > 0 {
			um.WallTime += intervalDuration
			// Compute PerCPUTime
			minPerCPUCount := idleCPUCount
			if overloadedCPUCount < minPerCPUCount {
				minPerCPUCount = overloadedCPUCount
			}
			um.PerCPUTime += Duration(minPerCPUCount) * intervalDuration
			// Compute PerThreadTime
			minPerThreadCount := idleCPUCount
			if waitingThreadCount < minPerThreadCount {
				minPerThreadCount = waitingThreadCount
			}
			um.PerThreadTime += Duration(minPerThreadCount) * intervalDuration
		}
	}
}
