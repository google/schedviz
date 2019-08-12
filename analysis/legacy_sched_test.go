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
package legacysched

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/analysis/schedtestcommon"
	"github.com/google/schedviz/server/models"
)

func TestPIDIntervals(t *testing.T) {
	coll, err := NewCollection(schedtestcommon.TestTrace1(t), true /*=normalizeTimestamps*/)
	if err != nil {
		t.Fatalf("Unexpected collection creation error %s", err)
	}
	tests := []struct {
		description         string
		pid                 int64
		startTimestamp      time.Duration
		endTimestamp        time.Duration
		minIntervalDuration time.Duration
		wantIntervals       []models.PIDInterval
	}{{
		description:         "PID 100, full range, unmerged",
		pid:                 100,
		startTimestamp:      -1,
		endTimestamp:        -1,
		minIntervalDuration: 0,
		wantIntervals: []models.PIDInterval{
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.UnknownState,
				StartTimestampNs:    0,
				EndTimestampNs:      0,
				MergedIntervalCount: 1,
			},
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.WaitingState,
				StartTimestampNs:    0,
				EndTimestampNs:      10,
				MergedIntervalCount: 1,
			},
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.RunningState,
				StartTimestampNs:    10,
				EndTimestampNs:      100,
				MergedIntervalCount: 1,
			},
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.WaitingState,
				StartTimestampNs:    100,
				EndTimestampNs:      101,
				MergedIntervalCount: 1,
			},
		},
	}, {
		description:         "PID 100, full range, merged",
		pid:                 100,
		startTimestamp:      -1,
		endTimestamp:        -1,
		minIntervalDuration: 50,
		wantIntervals: []models.PIDInterval{
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.UnknownState,
				StartTimestampNs:    0,
				EndTimestampNs:      10,
				MergedIntervalCount: 2,
			},
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.RunningState,
				StartTimestampNs:    10,
				EndTimestampNs:      100,
				MergedIntervalCount: 1,
			},
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.WaitingState,
				StartTimestampNs:    100,
				EndTimestampNs:      101,
				MergedIntervalCount: 1,
			},
		},
	}, {
		description:         "PID 100, partial range, unmerged",
		pid:                 100,
		startTimestamp:      5,
		endTimestamp:        25,
		minIntervalDuration: 0,
		wantIntervals: []models.PIDInterval{
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.WaitingState,
				StartTimestampNs:    0,
				EndTimestampNs:      10,
				MergedIntervalCount: 1,
			},
			{
				CPU:                 1,
				Pid:                 100,
				Command:             "Process1",
				Priority:            50,
				State:               sched.RunningState,
				StartTimestampNs:    10,
				EndTimestampNs:      100,
				MergedIntervalCount: 1,
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			gotIntervals, err := coll.PIDIntervals(
				test.pid, test.startTimestamp, test.endTimestamp,
				test.minIntervalDuration)
			if err != nil {
				t.Fatalf("Expected PIDIntervals to yield no errors but got %s", err)
			}
			if diff := cmp.Diff(test.wantIntervals, gotIntervals); diff != "" {
				t.Errorf("PIDIntervals() == %v: diff -want +got %s", test.wantIntervals, diff)
			}
		})
	}
}

func TestPIDSummary(t *testing.T) {
	coll, err := NewCollection(schedtestcommon.TestTrace1(t), true /*=normalizeTimestamps*/)
	if err != nil {
		t.Fatalf("Broken collection, can't proceed: `%s'", err)
	}
	tests := []struct {
		description    string
		cpus           []int64
		startTimestamp time.Duration
		endTimestamp   time.Duration
		wantMs         []models.Metrics
	}{{
		description:    "Full time range",
		startTimestamp: -1,
		endTimestamp:   -1,
		wantMs: []models.Metrics{
			{
				// Wakeup, switch-in at 1010, switch-out at 1100
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        90,
				WaitTimeNs:       10,
				Pids:             []int64{100},
				Commands:         []string{"Process1"},
				Cpus:             []int64{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				// Not counted because wakeup is first event, and therefore can't be inferred.
				WakeupCount: 0,
				Priorities:  []int64{50},
			},
			{
				// Switch-out SLEEPING at 1000, wakeup at 1040, migrate at 1080, switch-in at 1100.
				MigrationCount:   1,
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       60,
				SleepTimeNs:      40,
				Pids:             []int64{200},
				Commands:         []string{"Process2"},
				Cpus:             []int64{1, 2},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []int64{50},
			},
			{
				// Switch-in at 1000, switch-out at 1010, wakeup at 1090, switch-in at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        10,
				WaitTimeNs:       10,
				SleepTimeNs:      80,
				Pids:             []int64{300},
				Commands:         []string{"Process3"},
				Cpus:             []int64{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []int64{50},
			},
			{
				// Initial, switch-out at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        100,
				WaitTimeNs:       0,
				Pids:             []int64{400},
				Commands:         []string{"Process4"},
				Cpus:             []int64{2},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				Priorities:       []int64{50},
			},
		},
	}, {
		description:    "Full time range, CPU filtered",
		cpus:           []int64{1},
		startTimestamp: -1,
		endTimestamp:   -1,
		wantMs: []models.Metrics{
			{
				// Wakeup, switch-in at 1010, switch-out at 1100
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        90,
				WaitTimeNs:       10,
				Pids:             []int64{100},
				Commands:         []string{"Process1"},
				Cpus:             []int64{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				// Not counted because wakeup is first event, and therefore can't be inferred.
				WakeupCount: 0,
				Priorities:  []int64{50},
			},
			{
				// Switch-out and wakeup
				MigrationCount:   0, // Only migrations-in count.
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       40, // After the wakeup and before the migrate-out.
				SleepTimeNs:      40, // Up to the wakeup
				Pids:             []int64{200},
				Commands:         []string{"Process2"},
				Cpus:             []int64{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []int64{50},
			},
			{
				// Switch-in at 1000, switch-out at 1010, wakeup at 1090, switch-in at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        10,
				WaitTimeNs:       10,
				SleepTimeNs:      80,
				Pids:             []int64{300},
				Commands:         []string{"Process3"},
				Cpus:             []int64{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []int64{50},
			},
		},
	}, {
		description:    "Time filtered",
		startTimestamp: 50,
		endTimestamp:   100,
		wantMs: []models.Metrics{
			{
				// Switch-out at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        50, // Running even though no events within the range.
				WaitTimeNs:       0,
				Pids:             []int64{100},
				Commands:         []string{"Process1"},
				Cpus:             []int64{1},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []int64{50},
			},
			{
				// Migrate at 1080, switch-in at 1100.
				MigrationCount:   1,
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       50,
				Pids:             []int64{200},
				Commands:         []string{"Process2"},
				Cpus:             []int64{1, 2},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []int64{50},
			},
			{
				// Wakeup at 1090, Switch-in at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       10,
				SleepTimeNs:      40, // Sleeping even though no events within the range.
				Pids:             []int64{300},
				Commands:         []string{"Process3"},
				Cpus:             []int64{1},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []int64{50},
			},
			{
				// Switch-out at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        50,
				WaitTimeNs:       0,
				Pids:             []int64{400},
				Commands:         []string{"Process4"},
				Cpus:             []int64{2},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []int64{50},
			},
		},
	}, {
		description:    "Time and CPU filtered",
		cpus:           []int64{2},
		startTimestamp: 50,
		endTimestamp:   100,
		wantMs: []models.Metrics{
			{
				// Migrate at 1080, switch-in at 1100.
				MigrationCount:   1, // Migrates-in count.
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       20,
				Pids:             []int64{200},
				Commands:         []string{"Process2"},
				Cpus:             []int64{2},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []int64{50},
			},
			{
				// Switch-out at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        50,
				WaitTimeNs:       0,
				Pids:             []int64{400},
				Commands:         []string{"Process4"},
				Cpus:             []int64{2},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []int64{50},
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			metrics, err := coll.ThreadSummaries(test.cpus, test.startTimestamp, test.endTimestamp)
			if err != nil {
				t.Fatalf("ThreadSummaries(%v, %v, %v) threw %v", test.cpus, test.startTimestamp, test.endTimestamp, err)
			}
			if len(metrics) != len(test.wantMs) {
				t.Fatalf("PIDSummaryList(%v, %s, %s) returned %d metrics, expected %d", test.cpus, test.startTimestamp, test.endTimestamp, len(metrics), len(test.wantMs))
			}
			if diff := cmp.Diff(test.wantMs, metrics); diff != "" {
				t.Errorf("PIDSummaryList(%v, %s, %s) = %v, Diff -want +got: %v", test.cpus, test.startTimestamp, test.endTimestamp, metrics, diff)
			}
		})
	}
}
