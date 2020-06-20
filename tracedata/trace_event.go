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
// Package trace provides interfaces for conveniently parsing tracepoint
// collections and accessing their events.  trace.Collection provides
// basic event accessors, while derived types embedding Collection can provide
// more specialized, event-specific iteration.
package trace

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

// Timestamp describes a trace event timestamp.  Its units of this timestamp
// depend on the trace clock specified during collection.  'local', the default
// trace clock, and 'global' both generate ns-based timestamps, but some
// clocks, such as 'x86_tsc', generate timestamps that are not in, but can be
// converted to, ns, and some, such as 'counter', are in units that cannot be
// converted to ns at all.
type Timestamp int64

// UnknownTimestamp represents an unspecified event timestamp.
const UnknownTimestamp Timestamp = -1

// Event describes a single trace event.  A Collection stores its constituent
// events in a much more compact, but less usable, format than this, so it is
// recommended to generate Events on demand (via Collection::EventByIndex)
// rather than persisting more than a few Events.
type Event struct {
	// An index uniquely identifying this Event within its Collection.
	Index int `json:"index"`
	// The name of the event's type.
	Name string `json:"name"`
	// The CPU that logged the event.  Note that the CPU that logs an event may be
	// otherwise unrelated to the event.
	CPU int64 `json:"cpu"`
	// The event timestamp.
	Timestamp Timestamp `json:"timestamp"`
	// True if this Event fell outside of the known-valid range of a trace which
	// experienced buffer overruns.  Some kinds of traces are only valid for
	// unclipped events.
	Clipped bool `json:"clipped"`
	// A map of text properties, indexed by name.
	TextProperties map[string]string `json:"textProperties"`
	// A map of numeric properties, indexed by name.
	NumberProperties map[string]int64 `json:"numberProperties"`
}

func isPrintable(data string) bool {
	for _, r := range data {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// String returns the supplied event formatted in a string, or an empty string
// if the provided event is nil.
func (ev Event) String() string {
	var out = []string{}
	out = append(out, fmt.Sprintf("%-18d (CPU %d) %s", ev.Timestamp, ev.CPU, ev.Name))
	var props sort.StringSlice
	for k, v := range ev.TextProperties {
		if !isPrintable(v) {
			v = "<binary>"
		}
		props = append(props, fmt.Sprintf("%s: %s", k, v))
	}
	for k, v := range ev.NumberProperties {
		props = append(props, fmt.Sprintf("%s: %d", k, v))
	}
	sort.Sort(props)
	out = append(out, props...)
	return strings.Join(out, " ")
}

type options struct {
	normalizationOffset Timestamp
}

// NormalizationOffset specifies the timestamp offset to which all event
// timestamps should be normalized.
func NormalizationOffset(normalizationOffset Timestamp) func(o *options) {
	return func(o *options) {
		o.normalizationOffset = normalizationOffset
	}
}

// NewCollection builds and returns a new trace.Collection based on the
// tracepoint event set in es, or nil and an error if one could not be created.
func NewCollection(es *eventpb.EventSet, opts ...func(o *options)) (*Collection, error) {
	sort.Slice(es.Event, func(a, b int) bool {
		return es.Event[a].TimestampNs < es.Event[b].TimestampNs
	})
	o := &options{
		normalizationOffset: 0,
	}
	for _, opt := range opts {
		opt(o)
	}
	c := &Collection{
		eventSet: es,
		o:        o,
	}
	if err := c.init(); err != nil {
		return nil, err
	}
	return c, nil
}

// Collection provides convenience accessors for event traces stored in eventpb.EventSets.
type Collection struct {
	o              *options
	eventSet       *eventpb.EventSet
	startTimestamp Timestamp
	endTimestamp   Timestamp
}

// RawEventSet returns the EventSet proto contained in this collection
func (tc Collection) RawEventSet() *eventpb.EventSet {
	return tc.eventSet
}

// eventDescriptorByID returns the EventDescriptor associated with the provided
// ID, or nil if there is no such EventDescriptor.
func (tc Collection) eventDescriptorByID(id int64) *eventpb.EventDescriptor {
	if tc.eventSet == nil || id < 0 || id >= int64(len(tc.eventSet.EventDescriptor)) {
		return nil
	}
	return tc.eventSet.EventDescriptor[id]
}

// stringByID returns the string table entry at the provided ID, or "<INVALID>"
// if there is no such string.
func (tc Collection) stringByID(id int64) string {
	if tc.eventSet == nil || id < 0 || id >= int64(len(tc.eventSet.StringTable)) {
		return "<INVALID>"
	}
	return tc.eventSet.StringTable[id]
}

// clear clears the Collection's state.
func (tc *Collection) clear() {
	tc.eventSet = nil
	tc.startTimestamp = 0
	tc.endTimestamp = 0
}

// init initializes the Collection with the provided EventSet, returning an error if the
// initialization was unsuccessful.
func (tc *Collection) init() error {
	// Ensure that the EventSet is well formed, with no duplicate events.  Any
	// empty event names are ignored.
	ens := tc.EventNames()
	enm := make(map[string]bool)
	for _, en := range ens {
		if en == "" {
			continue
		}
		_, ok := enm[en]
		if ok {
			tc.clear()
			return fmt.Errorf("collection init failed: duplicate event '%s'", en)
		}
		enm[en] = true
	}
	if !tc.Valid() {
		return errors.New("invalid collection (are there any events?)")
	}
	var err error
	var event *Event
	if event, err = tc.EventByIndex(0); err != nil {
		tc.clear()
		return err
	}
	tc.startTimestamp = event.Timestamp
	tc.startTimestamp = tc.startTimestamp - tc.o.normalizationOffset
	if event, err = tc.EventByIndex(tc.EventCount() - 1); err != nil {
		tc.clear()
		return err
	}
	tc.endTimestamp = event.Timestamp
	tc.endTimestamp = tc.endTimestamp - tc.o.normalizationOffset
	return nil
}

// EventCount returns the number of events in the managed EventSet, or 0 if none is managed.
func (tc Collection) EventCount() int {
	return len(tc.eventSet.Event)
}

// Valid returns whether tc is a valid initialized Collection.
func (tc Collection) Valid() bool {
	return tc.eventSet != nil && tc.EventCount() > 0
}

// Interval returns the first and last timestamps of the events present in
// this Collection.  Only valid if tc.Valid() is true.
func (tc Collection) Interval() (startTimestamp Timestamp, endTimestamp Timestamp) {
	return tc.startTimestamp, tc.endTimestamp
}

// EventByIndex returns the event with the provided ID in the collection as an
// Event, or nil if there is no such event or the nth event is malformed.
func (tc Collection) EventByIndex(id int) (*Event, error) {
	if !tc.Valid() {
		return nil, errors.New("invalid collection")
	}
	if id < 0 || id >= tc.EventCount() {
		return nil, status.Errorf(codes.NotFound, "event %d not found", id)
	}
	ev := tc.eventSet.Event[id]
	ed := tc.eventDescriptorByID(ev.EventDescriptor)
	if ed == nil || len(ed.PropertyDescriptor) != len(ev.Property) {
		pc := 0
		if ed != nil {
			pc = len(ev.Property)
		}
		return nil, fmt.Errorf("mismatch between expected (%d) and actual (%d) property count", len(ed.PropertyDescriptor), pc)
	}
	nev := &Event{
		Index:            id,
		CPU:              ev.Cpu,
		Name:             tc.stringByID(ed.Name),
		Timestamp:        Timestamp(ev.TimestampNs) - tc.o.normalizationOffset,
		Clipped:          ev.Clipped,
		TextProperties:   make(map[string]string),
		NumberProperties: make(map[string]int64),
	}
	for i := range ev.Property {
		pd := ed.PropertyDescriptor[i]
		pdname := tc.stringByID(pd.Name)
		if pd.Type == eventpb.EventDescriptor_PropertyDescriptor_TEXT {
			nev.TextProperties[pdname] = tc.stringByID(ev.Property[i])
		} else {
			nev.NumberProperties[pdname] = ev.Property[i]
		}
	}
	return nev, nil
}

// EventNames returns a list of names of the events present in this Collection.
func (tc Collection) EventNames() sort.StringSlice {
	if !tc.Valid() {
		return nil
	}
	var ens sort.StringSlice
	for _, ed := range tc.eventSet.EventDescriptor {
		ens = append(ens, tc.stringByID(ed.Name))
	}
	sort.Sort(ens)
	return ens
}
