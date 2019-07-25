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

// percpuevents provides tools for working with arbitrary tracepoint
// events: fetching specific events and their metadata, by name, time range,
// and CPU.

import (
	"math"
	"sort"

	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/google/schedviz/tracedata/trace"
)

// CPULookupFunc is a function type used to look up the CPUs associated with an
// Event.  It should return an empty CPU list if the provided event should not
// be included in the PerCPUCollection, and should return an error only when a valid
// collection cannot be created with this event.
type CPULookupFunc func(*trace.Event) ([]CPUID, error)

// UnclippedReportingCPU returns the CPU that reported (=logged in its buffer)
// the provided Event, if it is unclipped.  It is the default CPULookupFunc.
var UnclippedReportingCPU CPULookupFunc = func(event *trace.Event) ([]CPUID, error) {
	if event.Clipped {
		return nil, nil
	}
	return []CPUID{CPUID(event.CPU)}, nil
}

type eventStub struct {
	// The index of the corresponding event in the PerCPUCollection's trace.PerCPUCollection.
	index int
	// The timestamp (normalized, if normalization was requested) of this event.
	timestamp      trace.Timestamp
	eventTypeIndex int
}

// PerCPUCollection extends trace.PerCPUCollection by associating events with one or more
// CPUs, and providing accessors to find events by CPU set and time range.
type PerCPUCollection struct {
	*trace.Collection
	normalizeTimestamps   bool
	minUnclippedTimestamp trace.Timestamp
	maxUnclippedTimestamp trace.Timestamp
	// A mapping of CPU to time-ordered slices of eventStubs for each indexed
	// Event.  Events without any stub in this map are not available for search.
	perCPUEventStubs    map[CPUID][]*eventStub
	eventTypes          []string
	eventTypesToIndices map[string]int
	indicesToEventTypes map[int]string
}

type perCPUCollectionBuilder struct {
	coll                *PerCPUCollection
	eventTypesToIndices map[string]int
	indicesToEventTypes map[int]string
}

// NewPerCPUCollection creates and returns a new PerCPUCollection from the
// provided EventSet.
// If normalizeTimestamps is true, then all event timestamps are normalized to
// the earliest unclipped timestamp in the collection.
// cpuLookup is the function to use to determine the CPU(s) a given event
// involves.  If any application of cpuLookup returns an error, collection
// creation will fail.  cpuLookup will be called on events in increasing order
// of their timestamp.
// Two special cases apply to cpuLookup:
//  * If cpuLookup returns no CPUs, the event will not be associated with any
//    CPU, and will not be returned when searching the PerCPUCollection.  Thus, a
//    CPULookupFunc returning no CPUs on a given event will effectively filter
//    that event out of the collection.
//  * If cpuLookup is nil, all event types are handled using the default lookup
//    function, UnclippedReportingCPU.
func NewPerCPUCollection(es *eventpb.EventSet, normalizeTimestamps bool, cpuLookup CPULookupFunc) (*PerCPUCollection, error) {
	var err error
	cb := &perCPUCollectionBuilder{
		coll: &PerCPUCollection{
			perCPUEventStubs:      make(map[CPUID][]*eventStub),
			normalizeTimestamps:   normalizeTimestamps,
			minUnclippedTimestamp: math.MaxInt64,
			maxUnclippedTimestamp: math.MinInt64,
		},
		eventTypesToIndices: make(map[string]int),
		indicesToEventTypes: make(map[int]string),
	}
	cb.coll.Collection, err = trace.NewCollection(es)
	if err != nil {
		return nil, err
	}
	if err := cb.addEvents(cpuLookup); err != nil {
		return nil, err
	}
	cb.coll.eventTypesToIndices = cb.eventTypesToIndices
	cb.coll.indicesToEventTypes = cb.indicesToEventTypes
	return cb.coll, nil
}

func (cb *perCPUCollectionBuilder) lookupEventTypeIndex(eventType string) int {
	idx, ok := cb.eventTypesToIndices[eventType]
	if ok {
		return idx
	}
	idx = len(cb.coll.eventTypes)
	cb.eventTypesToIndices[eventType] = idx
	cb.indicesToEventTypes[idx] = eventType
	cb.coll.eventTypes = append(cb.coll.eventTypes, eventType)
	return idx
}

func (cb *perCPUCollectionBuilder) addEvents(cpuLookup CPULookupFunc) error {
	// We should use the default lookup function, UnclippedReportingCPU, if
	// cpuLookup is nil.
	if cpuLookup == nil {
		cpuLookup = UnclippedReportingCPU
	}
	// Perform a first pass over all events to assemble all the eventStubs, and
	// to find the unclipped timestamp range.
	for index := 0; index < cb.coll.Collection.EventCount(); index++ {
		event, err := cb.coll.Collection.EventByIndex(index)
		if err != nil {
			return err
		}
		// Timestamp normalization is to the first unclipped event in the collection.
		// This permits aligning these events with other event sources such as
		// sched.PerCPUCollection.
		if !event.Clipped && event.Timestamp < cb.coll.minUnclippedTimestamp {
			cb.coll.minUnclippedTimestamp = event.Timestamp
		}
		if !event.Clipped && event.Timestamp > cb.coll.maxUnclippedTimestamp {
			cb.coll.maxUnclippedTimestamp = event.Timestamp
		}
		cpus, err := cpuLookup(event)
		if err != nil {
			return err
		}
		for _, cpu := range cpus {
			newStub := &eventStub{
				index:          index,
				timestamp:      event.Timestamp,
				eventTypeIndex: cb.lookupEventTypeIndex(event.Name),
			}
			if _, ok := cb.coll.perCPUEventStubs[cpu]; ok {
				cb.coll.perCPUEventStubs[cpu] = append(cb.coll.perCPUEventStubs[cpu], newStub)
			} else {
				cb.coll.perCPUEventStubs[cpu] = []*eventStub{newStub}
			}
		}
	}
	if cb.coll.normalizeTimestamps {
		for _, stubs := range cb.coll.perCPUEventStubs {
			for _, stub := range stubs {
				stub.timestamp = cb.coll.normalizeTimestamp(stub.timestamp)
			}
		}
	}
	return nil
}

func (c *PerCPUCollection) normalizeTimestamp(ts trace.Timestamp) trace.Timestamp {
	if c.normalizeTimestamps {
		return ts - c.minUnclippedTimestamp
	}
	return ts
}

// UnclippedInterval returns the minimal interval containing all unclipped
// events.
func (c *PerCPUCollection) UnclippedInterval() (startTimestamp, endTimestamp trace.Timestamp) {
	return c.normalizeTimestamp(c.minUnclippedTimestamp), c.normalizeTimestamp(c.maxUnclippedTimestamp)
}

// CPUs returns a sorted list of CPUs which have Events in this PerCPUCollection.
func (c *PerCPUCollection) CPUs() []CPUID {
	var cpus = []CPUID{}
	for cpu := range c.perCPUEventStubs {
		cpus = append(cpus, cpu)
	}
	sort.Slice(cpus, func(a, b int) bool {
		return cpus[a] < cpus[b]
	})
	return cpus
}

// EventIndices returns a timestamp-ordered list of the event indices between
// the specified timestamps on the specified CPUs.  Any requested CPUs with no
// events are ignored.
func (c *PerCPUCollection) EventIndices(filterFuncs ...Filter) []int {
	// Assemble a filter from the provided filters
	f := &filter{
		startTimestamp: UnknownTimestamp,
		endTimestamp:   UnknownTimestamp,
		eventTypes:     map[string]struct{}{},
		cpus:           map[CPUID]struct{}{},
		pids:           map[PID]struct{}{},
	}
	for _, filterFunc := range filterFuncs {
		filterFunc(f)
	}
	if len(f.cpus) == 0 {
		for _, cpu := range c.CPUs() {
			f.cpus[cpu] = struct{}{}
		}
	}
	if f.startTimestamp == trace.UnknownTimestamp {
		f.startTimestamp, _ = c.UnclippedInterval()
	}
	if f.endTimestamp == trace.UnknownTimestamp {
		_, f.endTimestamp = c.UnclippedInterval()
	}
	var stubs = []*eventStub{}
	for cpu := range f.cpus {
		cpuStubs := c.perCPUEventStubs[cpu]
		// Find the index of the first stub at or after startTimestamp.
		startIndex := sort.Search(len(cpuStubs), func(i int) bool {
			return cpuStubs[i].timestamp >= f.startTimestamp
		})
		// Find the index of the first stub after endTimestamp.
		endIndex := sort.Search(len(cpuStubs), func(i int) bool {
			return cpuStubs[i].timestamp > f.endTimestamp
		})
		stubs = append(stubs, cpuStubs[startIndex:endIndex]...)
	}
	sort.Slice(stubs, func(a, b int) bool {
		return stubs[a].timestamp < stubs[b].timestamp
	})
	var indices = []int{}
	for _, stub := range stubs {
		// Only include this event if either the filter does not include event types
		// or this event is one of the requested types.
		_, includeEvent := f.eventTypes[c.indicesToEventTypes[stub.eventTypeIndex]]
		if len(f.eventTypes) == 0 || includeEvent {
			indices = append(indices, stub.index)
		}
	}
	return indices
}

// EventIndexOnCPUAfter returns the index of the first event on the specified
// CPU after the specified time, and whether such an event was found.
func (c *PerCPUCollection) EventIndexOnCPUAfter(CPU CPUID, timestamp trace.Timestamp) (int, bool) {
	cpuStubs := c.perCPUEventStubs[CPU]
	stubIndex := sort.Search(len(cpuStubs), func(i int) bool {
		return cpuStubs[i].timestamp > timestamp
	})
	if stubIndex == len(cpuStubs) {
		return 0, false
	}
	return cpuStubs[stubIndex].index, true
}

// EventIndexOnCPUBefore returns the index of the last event on the specified
// CPU before the specified time, and whether such an event was found.
func (c *PerCPUCollection) EventIndexOnCPUBefore(CPU CPUID, timestamp trace.Timestamp) (int, bool) {
	cpuStubs := c.perCPUEventStubs[CPU]
	// Find the first stub at or after the timestamp, then if it is found and is
	// not zero, return the one before that.
	stubIndex := sort.Search(len(cpuStubs), func(i int) bool {
		return cpuStubs[i].timestamp >= timestamp
	})
	if stubIndex == 0 {
		return 0, false
	}
	stubIndex--
	return cpuStubs[stubIndex].index, true
}

// Event returns the event at the requested index, after normalizing its
// timestamp.  The returned Event is not owned and may be altered.
func (c *PerCPUCollection) Event(index int) (*trace.Event, error) {
	ev, err := c.Collection.EventByIndex(index)
	if err != nil {
		return nil, err
	}
	ev.Timestamp = c.normalizeTimestamp(ev.Timestamp)
	return ev, nil
}
