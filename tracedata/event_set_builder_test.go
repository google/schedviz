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
package eventsetbuilder

import (
	"testing"

	"github.com/google/schedviz/testhelpers/testhelpers"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

// Tests that the eventSetBuilder creates the expected protos.
func TestBuilder(t *testing.T) {
	tests := []struct {
		description  string
		esb          *Builder
		wantErr      bool
		wantEventSet *eventpb.EventSet
	}{{
		description: "good eventset",
		esb: NewBuilder().
			WithEventDescriptor(
				"event_a",
				Number("num1"),
				Text("txt1"),
				Number("num2"),
				Text("txt2")).
			WithEventDescriptor(
				"event_b",
				Number("num1"),
				Number("num2")).
			WithEvent("event_a", 0, 100, false, 0, "hi", 1, "bye").
			WithEvent("event_a", 0, 200, false, 2, "this", 3, "that").
			WithEvent("event_b", 0, 300, false, 100, 200),
		wantErr: false,
		wantEventSet: &eventpb.EventSet{
			StringTable: []string{"", "event_a", "num1", "txt1", "num2", "txt2", "event_b", "hi", "bye", "this", "that"},
			EventDescriptor: []*eventpb.EventDescriptor{
				{
					Name: 1,
					PropertyDescriptor: []*eventpb.EventDescriptor_PropertyDescriptor{
						{
							Name: 2,
							Type: eventpb.EventDescriptor_PropertyDescriptor_NUMBER,
						},
						{
							Name: 3,
							Type: eventpb.EventDescriptor_PropertyDescriptor_TEXT,
						},
						{
							Name: 4,
							Type: eventpb.EventDescriptor_PropertyDescriptor_NUMBER,
						},
						{
							Name: 5,
							Type: eventpb.EventDescriptor_PropertyDescriptor_TEXT,
						},
					},
				},
				{
					Name: 6,
					PropertyDescriptor: []*eventpb.EventDescriptor_PropertyDescriptor{
						{
							Name: 2,
							Type: eventpb.EventDescriptor_PropertyDescriptor_NUMBER,
						},
						{
							Name: 4,
							Type: eventpb.EventDescriptor_PropertyDescriptor_NUMBER,
						},
					},
				},
			},
			Event: []*eventpb.Event{
				{
					EventDescriptor: 0,
					TimestampNs:     100,
					Property:        []int64{0, 7, 1, 8},
				},
				{
					EventDescriptor: 0,
					TimestampNs:     200,
					Property:        []int64{2, 9, 3, 10},
				},
				{
					EventDescriptor: 1,
					TimestampNs:     300,
					Property:        []int64{100, 200},
				},
			},
		},
	}, {
		description: "event with improper argument type",
		esb: NewBuilder().
			WithEventDescriptor(
				"event",
				Number("num1")).
			WithEvent("event", 0, 100, false, "not a number"),
		wantErr: true,
	}, {
		description: "event with improper event type",
		esb: NewBuilder().
			WithEventDescriptor(
				"event",
				Number("num1")).
			WithEvent("not an event", 0, 100, false, 1),
		wantErr: true,
	}, {
		description: "event missing properties",
		esb: NewBuilder().
			WithEventDescriptor(
				"event",
				Number("num1")).
			WithEvent("not an event", 0, 100, false),
		wantErr: true,
	},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			if len(test.esb.errs) == 0 && test.wantErr {
				t.Fatalf("eventSetBuilder generated no errors, but expected some")
			}
			if len(test.esb.errs) > 0 && !test.wantErr {
				t.Fatalf("eventSetBuilder generated %d errors (%#v), but expected none", len(test.esb.errs), test.esb.errs)
			}
			if len(test.esb.errs) > 0 || test.wantErr {
				return
			}
			gotEventSet, errs := test.esb.EventSet()
			if len(errs) > 0 {
				t.Error("Errors building EventSet:")
				for err := range errs {
					t.Errorf("  %s", err)
				}
				t.Fatalf("Bailing...")
			}
			if diff, eq := testhelpers.DiffProto(t, test.wantEventSet, gotEventSet); !eq {
				t.Errorf("eventSetBuilder.pb() = %#v,\ndiff (want->got) \n%s", gotEventSet, diff)
			}
		})
	}
}
