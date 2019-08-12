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

	"github.com/Workiva/go-datastructures/augmentedtree"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/google/schedviz/tracedata/trace"
)

// Collection provides an interface for processing EventSets containing
// scheduling information and serving queries over that scheduling information.
type Collection struct {
	stringTable *stringTable
	// If timestamp normalization is requested, the duration to subtract from each
	// event's timestamp to normalize it.  This is the actual timestamp of the
	// first valid, unclipped, sched event.
	normalizationOffset trace.Timestamp
	// A mapping from CPU to vector of threadSpans, reflecting what was
	// known to be running on each CPU at each moment.
	runningSpansByCPU map[CPUID][]*threadSpan
	// A mapping from CPU to interval tree.  Each CPU's one-dimensional interval
	// tree contains intervals during which PIDs slept, and can be queried for
	// the set of sleeping PIDs at any moment.
	sleepingSpansByCPU map[CPUID]augmentedtree.Tree
	// A mapping from CPU to interval tree.  Each CPU's one-dimensional interval
	// tree contains intervals during which PIDs waited, and can be queried for
	// the set of waiting PIDs at any moment.
	waitingSpansByCPU map[CPUID]augmentedtree.Tree
	spansByPID        map[PID][]*threadSpan
	options           *collectionOptions
	// cpus is a cached copy of all CPUs in the collection.
	cpus map[CPUID]struct{}
	// pids is a cached copy of all PIDs in the collection.
	pids map[PID]struct{}
	// Cached start and end timestamps of the collection.
	startTimestamp trace.Timestamp
	endTimestamp   trace.Timestamp
	// Trace collection containing the event set
	TraceCollection *trace.Collection
	// Thread transitions generated from the events loaded into this collection
	ThreadTransitions []*threadTransition
	// A mapping from dropped event IDs to the number of transitions that
	// dropped them.
	droppedEventCountsByID map[int]int
}

// NewCollection builds and returns a new sched.Collection based on the ktrace
// event set in es, or nil and an error if one could not be created.  If the
// normalizeTimestamps argument is true, all valid, unclipped, sched event
// timestamps will be normalized to the first valid, unclipped, sched event's.
func NewCollection(es *eventpb.EventSet, eventLoaders map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error, options ...Option) (*Collection, error) {
	c := &Collection{
		normalizationOffset:    Unknown,
		runningSpansByCPU:      make(map[CPUID][]*threadSpan),
		sleepingSpansByCPU:     make(map[CPUID]augmentedtree.Tree),
		waitingSpansByCPU:      make(map[CPUID]augmentedtree.Tree),
		options:                &collectionOptions{},
		cpus:                   map[CPUID]struct{}{},
		pids:                   map[PID]struct{}{},
		droppedEventCountsByID: map[int]int{},
	}
	for _, option := range options {
		option(c.options)
	}
	if err := c.buildSpansByPID(es, eventLoaders); err != nil {
		return nil, err
	}
	if err := c.buildSpansByCPU(); err != nil {
		return nil, err
	}
	return c, nil
}

// buildSpansByPID loads the events in the provided EventSet as
// threadTransitions,, infers any CPU or state information they are missing,
// and convolutes them into threadSpans.
func (c *Collection) buildSpansByPID(es *eventpb.EventSet, eventLoaders map[string]func(*trace.Event, *ThreadTransitionSetBuilder) error) error {
	stringBank := newStringBank()
	c.stringTable = stringBank.stringTable
	eventLoader, err := newEventLoader(eventLoaders, stringBank)
	if err != nil {
		return fmt.Errorf("failed to use eventLoaders: %s", err)
	}
	coll, err := trace.NewCollection(es)
	if err != nil {
		return err
	}
	c.TraceCollection = coll
	var ts *threadSpanSet
	if err := c.setTimestampNormalize(coll); err != nil {
		return err
	}
	for eventIndex := 0; eventIndex < coll.EventCount(); eventIndex++ {
		ev, err := coll.EventByIndex(eventIndex)
		if err != nil {
			return err
		}
		// Bypass clipped events.
		if ev.Clipped {
			continue
		}
		// Adjust the event's timestamp.
		ev.Timestamp = ev.Timestamp - c.normalizationOffset
		c.endTimestamp = ev.Timestamp
		// Initialize the threadIntervalBuilder on first use.
		if ts == nil {
			c.startTimestamp = ev.Timestamp
			ts = newThreadSpanSet(c.startTimestamp, c.options)
		}
		// Translate the event into ThreadTransitions.
		tts, err := eventLoader.threadTransitions(ev)
		if err != nil {
			return err
		}
		c.ThreadTransitions = tts
		// Add those transitions to the threadIntervalBuilder.
		for _, tt := range tts {
			if err := ts.addTransition(tt); err != nil {
				return err
			}
		}
	}
	if ts == nil {
		return status.Errorf(codes.InvalidArgument, "no usable events in collection")
	}
	// End any still-open per-thread spans with a Timestamp just past the last
	// unclipped Timestamp observed in the trace.  This indicates that the
	// behavior in the span is ongoing past the end of the trace.
	c.spansByPID, err = ts.threadSpans(c.endTimestamp + 1)
	c.droppedEventCountsByID = ts.droppedEventCountsByID
	return err
}

// buildSpansByCPU iterates over all per-PID threadSpans, assembling a new
// slice of running spans and interval trees of all sleeping and waiting spans
// for each CPU.  It must be invoked after buildSpansByPID has successfully
// completed.
func (c *Collection) buildSpansByCPU() error {
	cib := newCPUSpanSet()
	for _, spans := range c.spansByPID {
		for _, span := range spans {
			c.pids[span.pid] = struct{}{}
			c.cpus[span.cpu] = struct{}{}
			cib.addSpan(span)
		}
	}
	var err error
	c.runningSpansByCPU, c.sleepingSpansByCPU, c.waitingSpansByCPU, err = cib.cpuTrees()
	return err
}

// LookupCommand returns the command for the provided stringID.  If the provided
// stringID does not have a valid lookup,
func (c *Collection) LookupCommand(command stringID) (string, error) {
	if command == UnknownCommand {
		return "<unknown>", nil
	}
	commStr, err := c.stringTable.stringByID(command)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to find command for id %d", command)
	}
	return commStr, nil
}

func cpuLookupFunc(ev *trace.Event) ([]CPUID, error) {
	if ev.Clipped {
		return nil, nil
	}
	switch ev.Name {
	case "sched_migrate_task":
		prevCPU, ok := ev.NumberProperties["orig_cpu"]
		if !ok {
			return nil, status.Errorf(codes.Internal, "sched_migrate_task lacks orig_cpu field")
		}
		return []CPUID{CPUID(ev.CPU), CPUID(prevCPU)}, nil
	case "sched_wakeup", "sched_wakeup_new":
		targetCPU, ok := ev.NumberProperties["target_cpu"]
		if !ok {
			return nil, status.Errorf(codes.Internal, "%s lacks target_cpu field", ev.Name)
		}
		return []CPUID{CPUID(targetCPU)}, nil
	}
	return []CPUID{CPUID(ev.CPU)}, nil
}

// GetRawEvents returns the raw events in this collection
// Timestamps are normalized if timestamp normalization is enabled on the collection.
func (c *Collection) GetRawEvents(filters ...Filter) ([]*trace.Event, error) {
	perCPUColl, err := NewPerCPUCollection(c, cpuLookupFunc)
	if err != nil {
		return nil, err
	}

	var events = []*trace.Event{}

	for _, eventIndex := range perCPUColl.EventIndices(filters...) {
		ev, err := c.TraceCollection.EventByIndex(eventIndex)
		if err != nil {
			return nil, err
		}
		// Adjust the event's timestamp.
		ev.Timestamp = ev.Timestamp - c.normalizationOffset
		events = append(events, ev)
	}

	return events, nil
}

func (c *Collection) setTimestampNormalize(coll *trace.Collection) error {
	if coll.EventCount() == 0 {
		return nil
	}
	if c.normalizationOffset != Unknown {
		return nil
	}
	// If the normalization offset is Unknown, set it.
	if !c.options.normalizeTimestamps {
		c.normalizationOffset = 0
		return nil
	}

	for index := 0; index < coll.EventCount(); index++ {
		ev, err := coll.EventByIndex(index)
		if err != nil {
			return err
		}
		// Timestamp normalization is to the first unclipped event in the collection.
		if ev.Clipped {
			continue
		}

		c.normalizationOffset = ev.Timestamp
		return nil
	}
	return nil
}

// Interval returns the first and last timestamps of the events present in this Collection.
// Only valid if tc.Valid() is true.
func (c *Collection) Interval(filters ...Filter) (startTS, endTS trace.Timestamp) {
	f := buildFilter(c, filters)

	return f.startTimestamp, f.endTimestamp
}

// CPUs returns the CPUs that the collection covers after the provided filters have been applied.
func (c *Collection) CPUs(filters ...Filter) map[CPUID]struct{} {
	f := buildFilter(c, filters)
	return f.cpus
}

// NormalizationOffset returns the duration to subtract from each event's timestamp to normalize it.
// This is the actual timestamp of the first valid, unclipped, sched event if timestamp
// normalization is enabled, or zero if it is not.
func (c *Collection) NormalizationOffset() trace.Timestamp {
	return c.normalizationOffset
}

// DroppedEventIDs returns the IDs, or indices, of events dropped during CPU
// and state inference.
func (c *Collection) DroppedEventIDs() []int {
	var ret = []int{}
	for eventID := range c.droppedEventCountsByID {
		ret = append(ret, eventID)
	}
	sort.Slice(ret, func(a, b int) bool {
		return ret[a] < ret[b]
	})
	return ret
}

// ExpandCPUs takes a slice of CPUs, and either returns that slice,
// if it is not empty, or if it is empty, a slice of all CPUs in the
// collection.
func (c *Collection) ExpandCPUs(cpus []int64) []int64 {
	if len(cpus) == 0 {
		// Return a slice of CPUs observed in the collection, in increasing order.
		var cpus = []int64{}
		for cpu := range c.CPUs() {
			cpus = append(cpus, int64(cpu))
		}
		sort.Slice(cpus, func(i, j int) bool {
			return cpus[i] < cpus[j]
		})

		return cpus
	}
	return cpus
}
