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
)

func TestPIDSummary(t *testing.T) {
	coll, err := NewCollection(schedtestcommon.TestTrace1(t),
		NormalizeTimestamps(true))

	if err != nil {
		t.Fatalf("Broken collection, can't proceed: `%s'", err)
	}
	tests := []struct {
		description string
		filters     []Filter
		wantMs      []*Metrics
	}{{
		description: "Full time range",
		wantMs: []*Metrics{
			{
				// Wakeup, switch-in at 1010, switch-out at 1100
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        90,
				WaitTimeNs:       10,
				Pids:             []PID{100},
				Commands:         []string{"Process1"},
				Cpus:             []CPUID{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []Priority{50},
			},
			{
				// Switch-out SLEEPING at 1000, wakeup at 1040, migrate at 1080, switch-in at 1100.
				MigrationCount:   1,
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       60,
				SleepTimeNs:      40,
				Pids:             []PID{200},
				Commands:         []string{"Process2"},
				Cpus:             []CPUID{1, 2},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []Priority{50},
			},
			{
				// Switch-in at 1000, switch-out at 1010, wakeup at 1090, switch-in at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        10,
				WaitTimeNs:       10,
				SleepTimeNs:      80,
				Pids:             []PID{300},
				Commands:         []string{"Process3"},
				Cpus:             []CPUID{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []Priority{50},
			},
			{
				// Initial, switch-out at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        100,
				WaitTimeNs:       0,
				Pids:             []PID{400},
				Commands:         []string{"Process4"},
				Cpus:             []CPUID{2},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				Priorities:       []Priority{50},
			},
		},
	}, {
		description: "Full time range, CPU filtered",
		filters:     []Filter{CPUs(1)},
		wantMs: []*Metrics{
			{
				// Wakeup, switch-in at 1010, switch-out at 1100
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        90,
				WaitTimeNs:       10,
				Pids:             []PID{100},
				Commands:         []string{"Process1"},
				Cpus:             []CPUID{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []Priority{50},
			},
			{
				// Switch-out and wakeup
				MigrationCount:   0, // Only migrations-in count.
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       40, // After the wakeup and before the migrate-out.
				SleepTimeNs:      40, // Up to the wakeup
				Pids:             []PID{200},
				Commands:         []string{"Process2"},
				Cpus:             []CPUID{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []Priority{50},
			},
			{
				// Switch-in at 1000, switch-out at 1010, wakeup at 1090, switch-in at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        10,
				WaitTimeNs:       10,
				SleepTimeNs:      80,
				Pids:             []PID{300},
				Commands:         []string{"Process3"},
				Cpus:             []CPUID{1},
				StartTimestampNs: 0,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []Priority{50},
			},
		},
	}, {
		description: "Time filtered",
		filters:     []Filter{TimeRange(50, 100)},
		wantMs: []*Metrics{
			{
				// Switch-out at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        50, // Running even though no events within the range.
				WaitTimeNs:       0,
				Pids:             []PID{100},
				Commands:         []string{"Process1"},
				Cpus:             []CPUID{1},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []Priority{50},
			},
			{
				// Migrate at 1080, switch-in at 1100.
				MigrationCount:   1,
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       50,
				Pids:             []PID{200},
				Commands:         []string{"Process2"},
				Cpus:             []CPUID{1, 2},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []Priority{50},
			},
			{
				// Wakeup at 1090, Switch-in at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       10,
				SleepTimeNs:      40, // Sleeping even though no events within the range.
				Pids:             []PID{300},
				Commands:         []string{"Process3"},
				Cpus:             []CPUID{1},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				WakeupCount:      1,
				Priorities:       []Priority{50},
			},
			{
				// Switch-out at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        50,
				WaitTimeNs:       0,
				Pids:             []PID{400},
				Commands:         []string{"Process4"},
				Cpus:             []CPUID{2},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []Priority{50},
			},
		},
	}, {
		description: "Time and CPU filtered",
		filters:     []Filter{CPUs(2), TimeRange(50, 100)},
		wantMs: []*Metrics{
			{
				// Migrate at 1080, switch-in at 1100.
				MigrationCount:   1, // Migrates-in count.
				UnknownTimeNs:    0,
				RunTimeNs:        0,
				WaitTimeNs:       20,
				Pids:             []PID{200},
				Commands:         []string{"Process2"},
				Cpus:             []CPUID{2},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []Priority{50},
			},
			{
				// Switch-out at 1100.
				MigrationCount:   0,
				UnknownTimeNs:    0,
				RunTimeNs:        50,
				WaitTimeNs:       0,
				Pids:             []PID{400},
				Commands:         []string{"Process4"},
				Cpus:             []CPUID{2},
				StartTimestampNs: 50,
				EndTimestampNs:   100,
				Priorities:       []Priority{50},
			},
		},
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			metrics, err := coll.ThreadSummaries(test.filters...)
			if err != nil {
				t.Fatalf("ThreadSummaries() threw %v", err)
			}
			if len(metrics) != len(test.wantMs) {
				t.Fatalf("PIDSummaryList() returned %d metrics, expected %d", len(metrics), len(test.wantMs))
			}
			if diff := cmp.Diff(test.wantMs, metrics); diff != "" {
				t.Errorf("PIDSummaryList() = %v, Diff -want +got: %v", metrics, diff)
			}
		})
	}
}
