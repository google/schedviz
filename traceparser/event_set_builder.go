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
package traceparser

// protoconverter converts TraceEvents into a compact proto representation

import (
	"errors"
	"fmt"
	"sort"

	"github.com/golang/protobuf/proto"
	elpb "github.com/google/schedviz/analysis/event_loaders_go_proto"
	"github.com/google/schedviz/tracedata/clipping"
	pb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

// EventSetBuilder is a builder for constructing EventSet protobufs.
// To start constructing a new EventSet, call either NewEventSetBuilder() or
// NewEventSetBuilderWithFormats() depending on if the formats are currently available. If more
// formats need to be added, AddFormat() can be used.
// Use AddTraceEvent() to add events as they come in instead of keeping a collection of TraceEvents
// around. AddTraceEvent() compresses the TraceEvents when it stores them in the EventSet, and is
// much more efficient than a raw TraceEvent.
// To the final constructed EventSet protobuf, read the EventSet property. Once this is done, the
// EventSetBuilder should no longer be used. Make sure that all events and formats are added before
// fetching the EventSet.
type EventSetBuilder struct {
	eventSet             *pb.EventSet
	formats              map[uint16]*EventFormat
	eventDescriptorMap   map[uint16]*pb.EventDescriptor
	eventDescriptorTable map[*pb.EventDescriptor]int64
	strTable             map[string]int64
	overwrite            bool
	overflowedCPUs       map[int64]struct{}
}

// NewEventSetBuilder constructs a new builder for making EventSet proto
// Optionally, a TraceParser can be provided. If one is provided, its formats will be added as
// EventDescriptors to the EventSet. If it is not provided (i.e. nil is passed), than no formats
// will be initially added to the EventSet.
func NewEventSetBuilder(tp *TraceParser) *EventSetBuilder {
	formats := make(map[uint16]*EventFormat)
	esb := EventSetBuilder{
		formats: formats,
		eventSet: &pb.EventSet{
			Event:       []*pb.Event{},
			StringTable: []string{},
		},
		// Map of Format ID to event descriptor. Used so we can look up the appropriate event descriptor
		// when all we have is a Format
		eventDescriptorMap: make(map[uint16]*pb.EventDescriptor),
		// Map of event descriptor to ID in event descriptor table. Will be transformed into the
		// actual table later.
		eventDescriptorTable: make(map[*pb.EventDescriptor]int64),
		// Map of string to ID in string table. Will be transformed into actual string table later.
		strTable: make(map[string]int64),
		// If true (default) then the oldest events might have been discarded by the
		// kernel if the trace was longer than the buffers could contain.
		// If false, then the newest events might have been discarded instead.
		// Corresponds with the "overwrite" FTrace option.
		overwrite: true,
		// The set of cpus that overflowed. This is used along with the overwrite field to decide which events need to be marked as clipped.
		overflowedCPUs: make(map[int64]struct{}),
	}
	// Add an empty string to the start of the string table so that an omitted string
	// (default value "") is in parity with an omitted string index (default value 0)
	esb.addString("")
	// Create an event descriptor for each format if a TraceParser is provided.
	if tp != nil {
		// Perform a deterministic iteration
		var keys []uint16
		for key := range tp.Formats {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})
		for _, key := range keys {
			esb.AddFormat(tp.Formats[key])
		}
	}
	return &esb
}

// SetOverwrite configures the overwrite property of the eventSetBuilder
func (esb *EventSetBuilder) SetOverwrite(option bool) {
	esb.overwrite = option
}

// SetOverflowedCPUs lets the eventSetBuilder know which CPUs overflowed.
func (esb *EventSetBuilder) SetOverflowedCPUs(cpus map[int64]struct{}) {
	esb.overflowedCPUs = cpus
}

// SetDefaultEventLoadersType specifies the default event loaders that should
// be used to interpret traces.
func (esb *EventSetBuilder) SetDefaultEventLoadersType(elt elpb.LoadersType) {
	esb.eventSet.DefaultLoadersType = elt
}

// AddFormat adds a new event descriptor based off of the EventFormat to the EventSet being built
// by the EventSetBuilder.
func (esb *EventSetBuilder) AddFormat(eFormat *EventFormat) {
	esb.formats[eFormat.ID] = eFormat
	fields := append(eFormat.Format.CommonFields, eFormat.Format.Fields...)

	eventDescriptor := &pb.EventDescriptor{
		Name: esb.addString(eFormat.Name),
	}

	// For each field in the format, create a property in the event descriptor
	for _, field := range fields {
		propertyDescriptor := &pb.EventDescriptor_PropertyDescriptor{
			Name: esb.addString(field.Name),
			Type: convertProtoTypeToFieldType(field),
		}

		eventDescriptor.PropertyDescriptor = append(eventDescriptor.PropertyDescriptor, propertyDescriptor)

		if field.IsDynamicArray {
			dynArrDescriptor := &pb.EventDescriptor_PropertyDescriptor{
				Name: esb.addString("__data_loc_" + field.Name),
				Type: pb.EventDescriptor_PropertyDescriptor_TEXT,
			}
			eventDescriptor.PropertyDescriptor = append(eventDescriptor.PropertyDescriptor, dynArrDescriptor)
		}
	}

	// Store the event descriptor in the bookkeeping data structures.
	esb.eventDescriptorMap[eFormat.ID] = eventDescriptor
	esb.addEventDescriptor(eventDescriptor)
}

// AddTraceEvent adds a new trace event to the EventSet being built by the EventSetBuilder.
func (esb *EventSetBuilder) AddTraceEvent(traceEvent *TraceEvent) error {
	// Get the event descriptor
	ed, ok := esb.eventDescriptorMap[traceEvent.FormatID]
	if !ok {
		return fmt.Errorf("missing event descriptor for format %d", traceEvent.FormatID)
	}
	edIndex := esb.eventDescriptorTable[ed]

	// Fetch the format that this event has. Should match the event descriptor.
	eFormat, ok := esb.formats[traceEvent.FormatID]
	if !ok {
		return fmt.Errorf("missing format definition for format %d", traceEvent.FormatID)
	}
	fields := append(eFormat.Format.CommonFields, eFormat.Format.Fields...)

	// Fetch properties out of the events and store them in the properties field of the proto.
	// Each item in the properties array has a corresponding property descriptor in the event
	// descriptor. If the properties are not in the same order (and the same type) as their property
	// descriptors, they won't be able to be correctly interpreted.
	var properties []int64
	for _, field := range fields {
		if field.ProtoType == "string" {
			// If the field is a string field, insert the value into the string table
			// and store its index in the properties table
			prop := traceEvent.TextProperties[field.Name]
			properties = append(properties, esb.addString(prop))
			if field.IsDynamicArray {
				dynProp := traceEvent.TextProperties["__data_loc_"+field.Name]
				properties = append(properties, esb.addString(dynProp))
			}
		} else if field.ProtoType == "int64" {
			// If the field is a number field, directly store the number in the properties table.
			properties = append(properties, traceEvent.NumberProperties[field.Name])
		} else {
			return fmt.Errorf("unknown ProtoType: %s", field.ProtoType)
		}
	}

	// Save the event in the event set.
	event := &pb.Event{
		EventDescriptor: edIndex,
		Cpu:             traceEvent.CPU,
		TimestampNs:     int64(traceEvent.Timestamp),
		Clipped:         traceEvent.Clipped,
		Property:        properties,
	}
	esb.eventSet.Event = append(esb.eventSet.Event, event)
	return nil
}

// Clone makes a copy of the EventSetBuilder
func (esb *EventSetBuilder) Clone() (*EventSetBuilder, error) {
	clonedEventSet, ok := proto.Clone(esb.eventSet).(*pb.EventSet)
	if !ok {
		return nil, errors.New("failed to clone EventSetBuilder")
	}

	formats := make(map[uint16]*EventFormat)
	newEsb := EventSetBuilder{
		formats:              formats,
		eventSet:             clonedEventSet,
		eventDescriptorMap:   make(map[uint16]*pb.EventDescriptor),
		eventDescriptorTable: make(map[*pb.EventDescriptor]int64),
		strTable:             make(map[string]int64),
	}

	for k, v := range esb.formats {
		// Make a copy of the original format
		format := *v
		newEsb.formats[k] = &format
	}
	for k, eventDescriptor := range esb.eventDescriptorMap {
		// Make a copy of the event descriptor
		clonedEventDescriptor, ok := proto.Clone(eventDescriptor).(*pb.EventDescriptor)
		if !ok {
			return nil, errors.New("failed to clone EventDescriptor")
		}

		newEsb.eventDescriptorMap[k] = clonedEventDescriptor
		// Set the corresponding eventDescriptorTable entry to the same number as before
		newEsb.eventDescriptorTable[clonedEventDescriptor] = esb.eventDescriptorTable[eventDescriptor]
	}
	for k, v := range esb.strTable {
		newEsb.strTable[k] = v
	}

	return &newEsb, nil
}

// clip sets the clipping boolean on the events that have been processed
// according to the builder's overwrite value and overflowedCPUs. We cannot know
// which events must be clipped until after all events are parsed, so this must
// be called after parsing events.
func (esb *EventSetBuilder) clip() {
	if esb.overwrite {
		clipping.ClipFromStartOfTrace(esb.eventSet, esb.overflowedCPUs)
	} else {
		clipping.ClipFromEndOfTrace(esb.eventSet, esb.overflowedCPUs)
	}
}

// Finalize creates and returns the final event set. This method should only be called once after
// all events and data have been processed.
func (esb *EventSetBuilder) Finalize() *pb.EventSet {
	esb.clip()
	return esb.eventSet
}

// convertProtoTypeToFieldType converts TraceEvent's ProtoType into the proto's FieldType enum
func convertProtoTypeToFieldType(field *FormatField) pb.EventDescriptor_PropertyDescriptor_FieldType {
	switch field.ProtoType {
	case "string":
		return pb.EventDescriptor_PropertyDescriptor_TEXT
	case "int64":
		return pb.EventDescriptor_PropertyDescriptor_NUMBER
	default:
		return pb.EventDescriptor_PropertyDescriptor_UNKNOWN
	}
}

// addString inserts a key into the string table and returns the index that it was inserted into
func (esb *EventSetBuilder) addString(key string) int64 {
	curr, ok := esb.strTable[key]
	if !ok {
		curr = int64(len(esb.strTable))
		esb.strTable[key] = curr
		// Insert into actual table as well
		esb.eventSet.StringTable = append(esb.eventSet.StringTable, key)
	}
	return curr
}

// addEventDescriptor inserts a key into the event descriptor table and returns
// the index that it was inserted into
func (esb *EventSetBuilder) addEventDescriptor(key *pb.EventDescriptor) int64 {
	curr, ok := esb.eventDescriptorTable[key]
	if !ok {
		curr = int64(len(esb.eventDescriptorTable))
		esb.eventDescriptorTable[key] = curr
		// Insert into actual table as well
		esb.eventSet.EventDescriptor = append(esb.eventSet.EventDescriptor, key)
	}
	return curr
}
