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
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/google/schedviz/analysis/schedtestcommon"
	"github.com/google/schedviz/tracedata/testeventsetbuilder"
	"github.com/google/schedviz/tracedata/trace"
)

func TestAntagonists(t *testing.T) {
	c, err := NewCollection(
		testeventsetbuilder.TestProtobuf(t,
			schedtestcommon.UnpopulatedBuilder().
				// PID 300 switches in on CPU 1 at time 1000, PID 200 switches out SLEEPING
				WithEvent("sched_switch", 1, 1000, false,
					200, "Process2", 50, schedtestcommon.Interruptible,
					300, "Process3", 50).
				// PID 100 wakes up on CPU 1 at time 1000
				WithEvent("sched_wakeup", 0, 1000, false,
					100, "Process1", 50, 1).
				// PID 100 switches in on CPU 1 at time 1010; PID 300 switches out SLEEPING
				WithEvent("sched_switch", 1, 1010, false,
					300, "Process3", 50, schedtestcommon.Interruptible,
					100, "Process1", 50).
				// PID 200 wakes up on CPU 1 at time 1040
				WithEvent("sched_wakeup", 0, 1040, false,
					200, "Process2", 50, 1).
				// PID 200 migrates from CPU 1 to CPU 2 at time 1080
				WithEvent("sched_migrate_task", 0, 1080, false,
					200, "Process2", 50,
					1, 2).
				// PID 300 wakes up on CPU 1 at time 1090
				WithEvent("sched_wakeup", 0, 1090, false,
					300, "Process3", 50, 1).
				// PID 200 switches in on CPU 2 at time 1100; PID 400 switches out RUNNABLE
				WithEvent("sched_switch", 2, 1100, false,
					400, "Process4", 50, schedtestcommon.Runnable,
					200, "Process2", 50).
				// PID 300 switches in on CPU 1 at time 1100; PID 100 switches out RUNNABLE
				WithEvent("sched_switch", 1, 1100, false,
					100, "Process1", 50, schedtestcommon.Runnable,
					300, "Process3", 50).
				// PID 500 yields to PID 501 at time 99000, but this is clipped.
				WithEvent("sched_switch", 0, 99000, true,
					500, "InvalidProcess", 50, 130,
					501, "InvalidProcess", 50)),
		PreciseCommands(true),
		PrecisePriorities(true))
	if err != nil {
		t.Fatalf("Broken collection, can't proceed: %q", err)
	}
	tests := []struct {
		description     string
		pid             PID
		startTimestamp  trace.Timestamp
		endTimestamp    trace.Timestamp
		wantAntagonists Antagonists
	}{{
		description:    "single antagonist",
		pid:            300,
		startTimestamp: 1000,
		endTimestamp:   1100,
		wantAntagonists: Antagonists{
			Victims: []*Thread{{
				PID:      300,
				Command:  "Process3",
				Priority: 50,
			}},
			Antagonisms: []*Antagonism{{
				RunningThread: &Thread{
					PID:      100,
					Command:  "Process1",
					Priority: 50,
				},
				CPU:            1,
				StartTimestamp: 1090,
				EndTimestamp:   1100,
			}},
			StartTimestamp: 1000,
			EndTimestamp:   1100,
		},
	}, {
		description:    "partial wait (starting in wait)",
		pid:            300,
		startTimestamp: 1095,
		endTimestamp:   1100,
		wantAntagonists: Antagonists{
			Victims: []*Thread{{
				PID:      300,
				Command:  "Process3",
				Priority: 50,
			}},
			Antagonisms: []*Antagonism{{
				RunningThread: &Thread{
					PID:      100,
					Command:  "Process1",
					Priority: 50,
				},
				CPU:            1,
				StartTimestamp: 1095,
				EndTimestamp:   1100,
			}},
			StartTimestamp: 1095,
			EndTimestamp:   1100,
		},
	}, {
		description:    "partial wait (starting and ending in wait)",
		pid:            300,
		startTimestamp: 1095,
		endTimestamp:   1098,
		wantAntagonists: Antagonists{
			Victims: []*Thread{{
				PID:      300,
				Command:  "Process3",
				Priority: 50,
			}},
			Antagonisms: []*Antagonism{{
				RunningThread: &Thread{
					PID:      100,
					Command:  "Process1",
					Priority: 50,
				},
				CPU:            1,
				StartTimestamp: 1095,
				EndTimestamp:   1098,
			}},
			StartTimestamp: 1095,
			EndTimestamp:   1098,
		},
	}, {
		description:    "multiple antagonists",
		pid:            200,
		startTimestamp: 1000,
		endTimestamp:   1100,
		wantAntagonists: Antagonists{
			Victims: []*Thread{{
				PID:      200,
				Command:  "Process2",
				Priority: 50,
			}},
			Antagonisms: []*Antagonism{{
				RunningThread: &Thread{
					PID:      100,
					Command:  "Process1",
					Priority: 50,
				},
				CPU:            1,
				StartTimestamp: 1040,
				EndTimestamp:   1080,
			}, {
				RunningThread: &Thread{
					PID:      400,
					Command:  "Process4",
					Priority: 50,
				},
				CPU:            2,
				StartTimestamp: 1080,
				EndTimestamp:   1100,
			}},
			StartTimestamp: 1000,
			EndTimestamp:   1100,
		},
	}, {
		description:    "sleeping threads aren't victims",
		pid:            300,
		startTimestamp: 1000,
		endTimestamp:   1080,
		wantAntagonists: Antagonists{
			Victims: []*Thread{{
				PID:      300,
				Command:  "Process3",
				Priority: 50,
			}},
			Antagonisms:    []*Antagonism{},
			StartTimestamp: 1000,
			EndTimestamp:   1080,
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			ants, err := c.Antagonists(PIDs(test.pid), StartTimestamp(test.startTimestamp), EndTimestamp(test.endTimestamp))
			if err != nil {
				t.Errorf("Antagonists(%d, %v, %v) yielded unexpected error: %v", test.pid, test.startTimestamp, test.endTimestamp, err)
				return
			}
			if diff := cmp.Diff(test.wantAntagonists, ants); diff != "" {
				t.Errorf("Antagonists(%d, %v, %v) = %v; diff -want +got %v", test.pid, test.startTimestamp, test.endTimestamp, ants, diff)
			}
		})
	}
}

// TestUtilizationMetrics tests that UtilizationMetrics returns the expected
// wall-time and per-CPU-time for a trace interval.
func TestUtilizationMetrics(t *testing.T) {
	c, err := NewCollection(
		testeventsetbuilder.TestProtobuf(t,
			schedtestcommon.UnpopulatedBuilder().
				// PID 100 goes to sleep on CPU 1 at time 1000
				WithEvent("sched_switch", 1, 1000, false,
					100, "Process1", 50, schedtestcommon.Interruptible,
					0, "swapper/1", 0).
				// PID 200 switches in on CPU 2 at time 1000, while PID 300 switches out WAITING
				WithEvent("sched_switch", 2, 1000, false,
					300, "Process3", 50, schedtestcommon.Runnable,
					200, "Process2", 50).
				// PID 400 switches in on CPU 3 at time 1000, while PID 500 switches out WAITING
				WithEvent("sched_switch", 3, 1000, false,
					500, "Process5", 50, schedtestcommon.Runnable,
					400, "Process4", 50).
				// PID 700 goes to sleep on CPU 4 at time 1000.
				WithEvent("sched_switch", 4, 1000, false,
					700, "Process7", 50, schedtestcommon.Interruptible,
					0, "swapper/4", 0).
				// PID 600 switches in on CPU 3 at time 1010 while PID 400 switches out WAITING
				WithEvent("sched_switch", 3, 1010, false,
					400, "Process4", 50, schedtestcommon.Runnable,
					600, "Process6", 50).
				// PID 300 migrates from CPU 2 to CPU 1 at time 1040
				WithEvent("sched_migrate_task", 0, 1040, false,
					300, "Process3", 50,
					2, 1).
				// PID 300 switches in on CPU 1 at time 1050
				WithEvent("sched_switch", 1, 1050, false,
					0, "swapper/1", 0, schedtestcommon.Interruptible,
					300, "Process3", 50).
				// PID 300 goes to sleep on CPU 1 at time 1080
				WithEvent("sched_switch", 1, 1080, false,
					300, "Process3", 50, schedtestcommon.Interruptible,
					0, "swapper/1", 0).
				// PID 400 switches in on CPU 3 at time 1090, while PID 600 switches out SLEEPING
				WithEvent("sched_switch", 3, 1090, false,
					600, "Process6", 50, schedtestcommon.Interruptible,
					400, "Process4", 50).
				// PID 500 migrates from CPU 3 to CPU 2 at time 1100
				WithEvent("sched_migrate_task", 0, 1100, false,
					500, "Process5", 50,
					3, 2)),
		PreciseCommands(true),
		PrecisePriorities(true))
	if err != nil {
		t.Fatalf("Broken collection, can't proceed: %v", err)
	}
	tests := []struct {
		description string
		filters     []Filter
		cpus        []CPUID
		wantUM      Utilization
	}{{
		/* A map of this trace over time, broken out by CPU.  Each CPU is either
		   IDLE, meaning that it has nothing running and nothing waiting, SCHED,
			 indicating that it had nothing running but had a task waiting (and was
			 about to schedule the waiting task), or has a running non-swapper thread
			 and some number of waiting threads, designated by '#-W'.  Each thread
			 caption describes that thread's state from that time until .01µs-ε later.
			 At the bottom, a summary of wall-time (W), per-core time (pC) and per-
			 thread time (pT) for that interval is given.
							  1µs    1.01µs 1.02µs 1.03µs 1.04µs 1.05µs 1.06µs 1.07µs 1.08µs 1.09µs 1.1µs
				 CPU 1  IDLE   IDLE   IDLE   IDLE   SCHED  0-W    0-W    0-W    IDLE   IDLE   IDLE
				 CPU 2  1-W    1-W    1-W    1-W    0-W    0-W    0-W    0-W    0-W    0-W    1-W
				 CPU 3  1-W    2-W    2-W    2-W    2-W    2-W    2-W    2-W    2-W    1-W    0-W
				 CPU 4  IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE
			 W/pC/pT  1/2/2  1/2/2  1/2/2  1/2/2  1/1/1  1/1/1  1/1/1  1/1/1  1/1/2  1/1/1  1/1/1
		*/
		description: "all CPUs, all time",
		// SCHED time is not idle.
		wantUM: Utilization{
			WallTime:            100,
			PerCPUTime:          140,
			PerThreadTime:       150,
			UtilizationFraction: .6, // 24 .01µs intervals out of 40.
		},
	}, {
		/*			   1µs    1.01µs 1.02µs 1.03µs 1.04µs
		    CPU 1  IDLE   IDLE   IDLE   IDLE   SCHED
			  CPU 2  1-W    1-W    1-W    1-W    0-W
			  CPU 3  1-W    2-W    2-W    2-W    2-W
			  CPU 4  IDLE   IDLE   IDLE   IDLE   IDLE
			W/pC/pT  1/2/2  1/2/2  1/2/2  1/2/2  1/1/1
		*/
		description: "all CPUs, first 40ns",
		filters:     []Filter{EndTimestamp(1040)},
		wantUM: Utilization{
			WallTime:            40,
			PerCPUTime:          80,
			PerThreadTime:       80,
			UtilizationFraction: .5, // 8 .01µs intervals out of 16
		},
	}, {
		/*				 1.05µs 1.06µs 1.07µs 1.08µs 1.09µs 1.1µs
		    CPU 1  0-W    0-W    0-W    IDLE   IDLE   IDLE
			  CPU 2  0-W    0-W    0-W    0-W    0-W    1-W
			  CPU 3  2-W    2-W    2-W    2-W    1-W    0-W
			  CPU 4  IDLE   IDLE   IDLE   IDLE   IDLE   IDLE
			W/pC/pT  1/1/1  1/1/1  1/1/1  1/1/2  1/1/1  1/1/1
		*/
		description: "all CPUs, last 50ns",
		filters:     []Filter{StartTimestamp(1050)},
		wantUM: Utilization{
			WallTime:            50,
			PerCPUTime:          50,
			PerThreadTime:       60,
			UtilizationFraction: .65, // 13 .01µs intervals out of 20
		},
	}, {
		/*         1µs    1.01µs 1.02µs 1.03µs 1.04µs 1.05µs 1.06µs 1.07µs 1.08µs 1.09µs 1.1µs
		    CPU 1  IDLE   IDLE   IDLE   IDLE   SCHED  0-W    0-W    0-W    IDLE   IDLE   IDLE
			  CPU 2  1-W    1-W    1-W    1-W    0-W    0-W    0-W    0-W    0-W    0-W    1-W
			  CPU 3  1-W    2-W    2-W    2-W    2-W    2-W    2-W    2-W    2-W    1-W    0-W
			W/pC/pT  1/1/1  1/1/1  1/1/1  1/1/1  0/0/0  0/0/0  0/0/0  0/0/0  1/1/1  1/1/1  1/1/1
		*/
		description: "CPUs 1-3 CPUs, all time",
		filters:     []Filter{CPUs(1, 2, 3)},
		wantUM: Utilization{
			WallTime:            60,
			PerCPUTime:          60,
			PerThreadTime:       60,
			UtilizationFraction: .8, // 24 .01µs intervals out of 30
		},
	}, {
		/*
							  1µs    1.01µs 1.02µs 1.03µs 1.04µs 1.05µs 1.06µs 1.07µs 1.08µs 1.09µs 1.1µs
				 CPU 1  IDLE   IDLE   IDLE   IDLE   SCHED  0-W    0-W    0-W    IDLE   IDLE   IDLE
				 CPU 2  1-W    1-W    1-W    1-W    0-W    0-W    0-W    0-W    0-W    0-W    1-W
				 CPU 3  1-W    2-W    2-W    2-W    2-W    2-W    2-W    2-W    2-W    1-W    0-W
				 CPU 4  IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE   IDLE
			 W/pC/pT  1/2/2  1/2/2  1/2/2  1/2/2  1/1/1  1/1/1  1/1/1  1/1/1  1/1/2  1/1/1  1/1/1
		*/
		description: "all CPUs including an unattested one, all time",
		filters:     []Filter{CPUs(1, 2, 3, 4, 5)},
		wantUM: Utilization{
			// SCHED time is not idle.
			WallTime:            100,
			PerCPUTime:          140,
			PerThreadTime:       150,
			UtilizationFraction: .6, // 24 .01µs intervals out of 40.
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			test.filters = append(test.filters, TruncateToTimeRange(true))
			um, err := c.UtilizationMetrics(test.filters...)
			if err != nil {
				t.Fatalf("UtilizationMetrics() yielded unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.wantUM, um); diff != "" {
				t.Fatalf("UtilizationMetrics() = %v; Diff -want +got %v", um, diff)
			}
		})
	}
}

// TestThreadStats tests the aggregate thread statistics.
func TestThreadStats(t *testing.T) {
	c, err := NewCollection(schedtestcommon.TestTrace1(t), NormalizeTimestamps(false))
	if err != nil {
		t.Fatalf("Broken collection, can't proceed: `%s'", err)
	}
	tests := []struct {
		description     string
		PIDs            []PID
		CPUs            []CPUID
		startTimestamp  trace.Timestamp
		endTimestamp    trace.Timestamp
		wantThreadStats *ThreadStatistics
	}{{
		description:    "everything",
		startTimestamp: trace.UnknownTimestamp,
		endTimestamp:   trace.UnknownTimestamp,
		wantThreadStats: &ThreadStatistics{
			WaitTime:           80,  // 10 (PID 100) + 60 (PID 200) + 10 (PID 300)
			PostWakeupWaitTime: 80,  // All waits
			RunTime:            200, // All the time on CPUs 1 and 2
			SleepTime:          120, // 40 (PID 200) + 80 (PID 300)
			Wakeups:            5,   // PID 100 at start and end, PID 200 at 1040, PID 300 at 1090, PID 400 at end
			Migrations:         1,   // PID 200
		},
	}, {
		description:    "CPU 2",
		CPUs:           []CPUID{2},
		startTimestamp: trace.UnknownTimestamp,
		endTimestamp:   trace.UnknownTimestamp,
		wantThreadStats: &ThreadStatistics{
			WaitTime:           20, // PID 200
			PostWakeupWaitTime: 20,
			RunTime:            100,
			SleepTime:          0,
			Wakeups:            2, // PID 400 at end, PID 200 at 1080 when it arrived
			Migrations:         0, // No migrations among only 1 CPU
		},
	}, {
		description:    "PID 200",
		PIDs:           []PID{200},
		startTimestamp: trace.UnknownTimestamp,
		endTimestamp:   trace.UnknownTimestamp,
		wantThreadStats: &ThreadStatistics{
			WaitTime:           60,
			PostWakeupWaitTime: 60,
			RunTime:            0,
			SleepTime:          40,
			Wakeups:            1, // At 1040
			Migrations:         1,
		},
	}, {
		description:    "Time filtered",
		startTimestamp: 1045,
		endTimestamp:   1090,
		wantThreadStats: &ThreadStatistics{
			WaitTime:           45, // PID 200
			PostWakeupWaitTime: 45, // All waits
			RunTime:            90, // all the time on CPUs 1 and 2
			SleepTime:          45, // PID 300
			Wakeups:            2,  // PID 200 at its initial point, PID 300 at 1090
			Migrations:         1,  // PID 200
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			gotThreadStats, err := c.ThreadStats(PIDs(test.PIDs...), CPUs(test.CPUs...), TimeRange(test.startTimestamp, test.endTimestamp))
			if err != nil {
				t.Fatalf("Unexpected error %s", err)
			}
			diff := cmp.Diff(gotThreadStats, test.wantThreadStats)
			if len(diff) > 0 {
				t.Errorf("c.ThreadStats() = %v, diff(got->want) %v", gotThreadStats, diff)
			}
		})
	}
}
