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
package trace

import (
	"reflect"
	"sort"
	"testing"

	builder "github.com/google/schedviz/tracedata/eventsetbuilder"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

var b = builder.NewBuilder().
	WithEventDescriptor("event1",
		builder.Number("numprop1"),
		builder.Number("numprop2")).
	WithEventDescriptor("event2",
		builder.Text("txtprop1"),
		builder.Text("txtprop2")).
	WithEventDescriptor("event3",
		builder.Number("numprop1"),
		builder.Text("txtprop1"),
		builder.Number("numprop2"),
		builder.Text("txtprop2"))

func populatedBuilder(t *testing.T) *builder.Builder {
	t.Helper()
	return b.TestClone(t).
		WithEvent("event1", 0, 1000, false, 100, 200).
		WithEvent("event1", 0, 2000, false, 100, 400).
		WithEvent("event2", 1, 3000, false, "thing1", "thing2").
		WithEvent("event3", 0, 4000, false, 50, "thing1", 150, "thing2")
}

func bustedEventSet(t *testing.T) *eventpb.EventSet {
	t.Helper()
	b := populatedBuilder(t)
	es := b.TestProtobuf(t)
	es.EventDescriptor = append(es.EventDescriptor, &eventpb.EventDescriptor{
		Name: es.EventDescriptor[0].Name,
		PropertyDescriptor: []*eventpb.EventDescriptor_PropertyDescriptor{
			{
				Name: es.EventDescriptor[0].PropertyDescriptor[0].Name,
				Type: eventpb.EventDescriptor_PropertyDescriptor_NUMBER,
			},
		},
	})
	return es
}

// TestInit tests ktrace.Collection initialization, and whole-collection
// statistics.
func TestInit(t *testing.T) {
	tests := []struct {
		description    string
		eventSet       *eventpb.EventSet
		wantErr        bool
		wantValid      bool
		wantEventCount int
		wantStart      Timestamp
		wantEnd        Timestamp
		wantEventNames sort.StringSlice
	}{{
		"normal init",
		populatedBuilder(t).TestProtobuf(t),
		false,
		true,
		4,
		Timestamp(1000),
		Timestamp(4000),
		sort.StringSlice{"event1", "event2", "event3"},
	}, {
		"empty init",
		builder.NewBuilder().TestProtobuf(t),
		true,
		false,
		0, 0, 0, sort.StringSlice{},
	}, {
		"duplicate event names",
		bustedEventSet(t),
		true,
		false,
		0, 0, 0, sort.StringSlice{},
	}}
	for _, test := range tests {
		c, err := NewCollection(test.eventSet)
		if test.wantErr != (err != nil) {
			t.Errorf("test %s: NewCollection() returned errorIsNonnil %t, want %t", test.description, err != nil, test.wantErr)
		}
		if c == nil {
			continue
		}
		if test.wantValid != c.Valid() {
			t.Errorf("test %s: new collection had unexpected validity: got %t, want %t", test.description, c.Valid(), test.wantValid)
		}
		if !c.Valid() {
			continue
		}
		if test.wantEventCount != c.EventCount() {
			t.Errorf("test %s: c.EventCount() returned %d, want %d", test.description, c.EventCount(), test.wantEventCount)
		}
		start, end := c.Interval()
		if test.wantStart != start || test.wantEnd != end {
			t.Errorf("test %s: c.IntervalNs() returned (%d, %d), want (%d, %d)", test.description, start, end, test.wantStart, test.wantEnd)
		}
		if !reflect.DeepEqual(test.wantEventNames, c.EventNames()) {
			t.Errorf("test %s: c.EventNames() returned %v, want %v", test.description, c.EventNames(), test.wantEventNames)
		}
	}
}

// TestEvent tests that ktrace.Events are properly formed and returned.
func TestEvent(t *testing.T) {
	c, err := NewCollection(populatedBuilder(t).TestProtobuf(t))
	if err != nil || !c.Valid() {
		t.Fatal("Broken collection, can't proceed.")
	}
	event, err := c.EventByIndex(3)
	if err != nil {
		t.Fatalf("Unexpected error on EventByIndex: %s", err)
	}
	es := event.String()
	wantEs := "4000               (CPU 0) event3 " +
		"numprop1: 50 " +
		"numprop2: 150 " +
		"txtprop1: thing1 " +
		"txtprop2: thing2"
	if es != wantEs {
		t.Errorf("event.String() returned %s, want %s", es, wantEs)
	}
}
