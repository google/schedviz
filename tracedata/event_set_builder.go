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
// Package eventsetbuilder provides utilities for programmatically assembling
// tracepoint collections as eventpb.EventSets.
package eventsetbuilder

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	tp "github.com/google/schedviz/traceparser/traceparser"
)

// PropertyDescriptor describes a single property in an event descriptor.
type PropertyDescriptor struct {
	name string
	t    string
}

// Number returns a number-type PropertyDescriptor with the provided name.
func Number(name string) PropertyDescriptor {
	return PropertyDescriptor{
		name: name,
		t:    "int64",
	}
}

// Text returns a text-type PropertyDescriptor with the provided name.
func Text(name string) PropertyDescriptor {
	return PropertyDescriptor{
		name: name,
		t:    "string",
	}
}

// Builder allows successive programmatic assembly of new eventpb.EventSets.
// Construct event sets by creating an Builder (New), then adding event
// descriptors (withEventDescriptor) and events (withEvent) to it.  Then, in
// test, call TestProtobuf() on the builder, passing it the test object, to get
// its EventSet.
type Builder struct {
	esb                *tp.EventSetBuilder
	eventFormatsByName map[string]*tp.EventFormat
	errs               []error
}

// NewBuilder constructs and returns a new, empty Builder.
func NewBuilder() *Builder {
	esb := tp.NewEventSetBuilder(nil)
	return &Builder{
		esb:                esb,
		eventFormatsByName: make(map[string]*tp.EventFormat),
		errs:               []error{},
	}
}

// Clone returns a cloned copy of the receiver.
func (b *Builder) Clone() (*Builder, error) {
	newEsb, err := b.esb.Clone()
	if err != nil {
		return nil, errors.New("failed to clone Builder")
	}
	newB := Builder{
		esb:                newEsb,
		eventFormatsByName: make(map[string]*tp.EventFormat),
	}
	for k, v := range b.eventFormatsByName {
		newB.eventFormatsByName[k] = v
	}
	for i, v := range b.errs {
		newB.errs[i] = v
	}
	return &newB, nil
}

// TestClone returns a cloned copy of the receiver, failing on the provided
// testing.T if an error is encountered.
func (b Builder) TestClone(t *testing.T) *Builder {
	newEsb, err := b.Clone()
	if err != nil {
		t.Fatalf("Failed to clone Builder: %s", err)
	}
	return newEsb
}

// WithEventDescriptor adds the provided event descriptor (a name and a series
// of PropertyDescriptors) to the receiving Builder, returning that
// Builder to facilitate chaining.
func (b *Builder) WithEventDescriptor(name string, propertyDescriptors ...PropertyDescriptor) *Builder {
	eventFormat := &tp.EventFormat{
		Name: name,
		ID:   uint16(len(b.eventFormatsByName)),
		Format: tp.Format{
			Fields: make([]*tp.FormatField, len(propertyDescriptors)),
		},
	}
	b.eventFormatsByName[name] = eventFormat
	for i, prop := range propertyDescriptors {
		field := &tp.FormatField{
			Name:      prop.name,
			ProtoType: prop.t,
		}
		eventFormat.Format.Fields[i] = field
	}
	b.esb.AddFormat(eventFormat)
	return b
}

// WithEvent adds the provided event to the receiving Builder,
// returning that Builder to facilitate chaining.
func (b *Builder) WithEvent(eventName string, cpu int64, timestampNs int64, clipped bool, props ...interface{}) *Builder {
	eventFormat := b.eventFormatsByName[eventName]
	if eventFormat == nil {
		b.errs = append(b.errs, fmt.Errorf("expected event descriptor for %s to be stored", eventName))
		return b
	}
	traceEvent := &tp.TraceEvent{
		FormatID:         eventFormat.ID,
		CPU:              cpu,
		Clipped:          clipped,
		Timestamp:        uint64(timestampNs),
		NumberProperties: make(map[string]int64),
		TextProperties:   make(map[string]string),
	}
	if len(props) != len(eventFormat.Format.Fields) {
		err := fmt.Errorf("expected %d properties, but only got %d", len(props), len(eventFormat.Format.Fields))
		b.errs = append(b.errs, err)
		return b
	}
	for i, prop := range props {
		field := eventFormat.Format.Fields[i]
		switch v := prop.(type) {
		case int:
			if field.ProtoType != "int64" {
				b.errs = append(b.errs, fmt.Errorf("expected integer argument for property %d", i))
				return b
			}
			traceEvent.NumberProperties[field.Name] = int64(v)
		case string:
			if field.ProtoType != "string" {
				b.errs = append(b.errs, fmt.Errorf("expected string argument for property %d", i))
				return b
			}
			traceEvent.TextProperties[field.Name] = v
		default:
			b.errs = append(b.errs, fmt.Errorf("unknown type for property %d", i))
			return b
		}
	}
	if err := b.esb.AddTraceEvent(traceEvent); err != nil {
		b.errs = append(b.errs, err)
	}
	return b
}

// TestProtobuf returns the EventSet built by the Builder.  If the
// builder is in error, it fails on the provided testing.T.
func (b *Builder) TestProtobuf(t *testing.T) *eventpb.EventSet {
	if len(b.errs) > 0 {
		var errStrs []string
		for _, err := range b.errs {
			errStrs = append(errStrs, err.Error())
		}
		t.Fatalf("Failed to construct EventSet protobuf: %s", strings.Join(errStrs, ", "))
	}
	return b.esb.EventSet
}
