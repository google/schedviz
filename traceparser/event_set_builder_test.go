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

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/golang/protobuf/proto"

	pb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

func testEventSetBuilder() *EventSetBuilder {
	var tp = &TraceParser{
		Formats: map[uint16]*EventFormat{
			314: {
				Name: "sched_switch",
				ID:   314,
				Format: Format{
					CommonFields: []*FormatField{
						{FieldType: "unsigned short common_type", Name: "common_type", ProtoType: "int64", Size: 2, NumElements: 1, ElementSize: 2},
						{FieldType: "unsigned char common_flags", Name: "common_flags", ProtoType: "string", Offset: 2, Size: 1, NumElements: 1, ElementSize: 1},
						{FieldType: "unsigned char common_preempt_count", Name: "common_preempt_count", ProtoType: "string", Offset: 3, Size: 1, NumElements: 1, ElementSize: 1},
						{FieldType: "int common_pid", Name: "common_pid", ProtoType: "int64", Offset: 4, Size: 4, NumElements: 1, ElementSize: 4, Signed: true},
					},
					Fields: []*FormatField{
						{FieldType: "char prev_comm[16]", Name: "prev_comm", ProtoType: "string", Offset: 8, Size: 16, NumElements: 16, ElementSize: 1, Signed: true},
						{FieldType: "pid_t prev_pid", Name: "prev_pid", ProtoType: "int64", Offset: 24, Size: 4, NumElements: 1, ElementSize: 4, Signed: true},
						{FieldType: "int prev_prio", Name: "prev_prio", ProtoType: "int64", Offset: 28, Size: 4, NumElements: 1, ElementSize: 4, Signed: true},
						{FieldType: "long prev_prio", Name: "prev_state", ProtoType: "int64", Offset: 32, Size: 8, NumElements: 1, ElementSize: 8, Signed: true},
						{FieldType: "char next_comm[16]", Name: "next_comm", ProtoType: "string", Offset: 40, Size: 16, NumElements: 16, ElementSize: 1, Signed: true},
						{FieldType: "pid_t next_pid", Name: "next_pid", ProtoType: "int64", Offset: 56, Size: 4, NumElements: 1, ElementSize: 4, Signed: true},
						{FieldType: "int next_prio", Name: "next_prio", ProtoType: "int64", Offset: 60, Size: 4, NumElements: 1, ElementSize: 4, Signed: true},
					},
				},
			},
			1942: {
				Name: "special_event",
				ID:   1942,
				Format: Format{
					CommonFields: []*FormatField{
						{FieldType: "unsigned short common_type", Name: "common_type", ProtoType: "int64", Offset: 0, Size: 2, NumElements: 1, ElementSize: 2, Signed: false, IsDynamicArray: false},
						{FieldType: "unsigned char common_flags", Name: "common_flags", ProtoType: "string", Offset: 2, Size: 1, NumElements: 1, ElementSize: 1, Signed: false, IsDynamicArray: false},
						{FieldType: "unsigned char common_preempt_count", Name: "common_preempt_count", ProtoType: "string", Offset: 3, Size: 1, NumElements: 1, ElementSize: 1, Signed: false, IsDynamicArray: false},
						{FieldType: "int common_pid", Name: "common_pid", ProtoType: "int64", Offset: 4, Size: 4, NumElements: 1, ElementSize: 4, Signed: true, IsDynamicArray: false},
					},
					Fields: []*FormatField{
						{FieldType: "__data_loc uint8[] event", Name: "event", ProtoType: "string", Offset: 8, Size: 4, NumElements: 1, ElementSize: 4, Signed: false, IsDynamicArray: true},
					},
				},
			},
		},
	}
	return NewEventSetBuilder(tp)
}

var traceEvents = []*TraceEvent{
	{
		Timestamp: 1040483711613818,
		TextProperties: map[string]string{
			"common_flags":         "\x01",
			"common_preempt_count": "",
			"__data_loc_event":     "def",
			"event":                "abc",
		},
		NumberProperties: map[string]int64{
			"common_pid": 0,
			"prev_pid":   0,
			"prev_prio":  120,
			"prev_state": 0,
			"next_pid":   166549,
			"next_prio":  120,
		},
		FormatID: 1942,
		CPU:      1,
	},
	{
		Timestamp: 1040483711613819,
		TextProperties: map[string]string{
			"common_flags":         "\x01",
			"common_preempt_count": "",
			"prev_comm":            "swapper/0",
			"next_comm":            "cat e.sh minal-",
		},
		NumberProperties: map[string]int64{
			"common_pid": 0,
			"prev_pid":   0,
			"prev_prio":  120,
			"prev_state": 0,
			"next_pid":   166549,
			"next_prio":  120,
		},
		FormatID: 314,
	},
	{
		Timestamp: 1040483711630169,
		TextProperties: map[string]string{
			"common_flags":         "\x01",
			"common_preempt_count": "",
			"prev_comm":            "cat e.sh minal-",
			"next_comm":            "swapper/0",
		},
		NumberProperties: map[string]int64{
			"common_pid": 166549,
			"prev_pid":   166549,
			"prev_prio":  120,
			"prev_state": 1,
			"next_pid":   0,
			"next_prio":  120,
		},
		FormatID: 314,
	},
	{
		Timestamp: 1040483711647349,
		TextProperties: map[string]string{
			"common_flags":         "\x01",
			"common_preempt_count": "",
			"prev_comm":            "swapper/0",
			"next_comm":            "cat e.sh minal-",
		},
		NumberProperties: map[string]int64{
			"common_pid": 0,
			"prev_pid":   0,
			"prev_prio":  120,
			"prev_state": 0,
			"next_pid":   166549,
			"next_prio":  120,
		},
		FormatID: 314,
		CPU:      1,
	},
}

var eventSet = &pb.EventSet{
	StringTable: []string{"", "sched_switch", "common_type", "common_flags", "common_preempt_count", "common_pid", "prev_comm", "prev_pid", "prev_prio", "prev_state", "next_comm", "next_pid", "next_prio", "special_event", "event", "__data_loc_event", "\x01", "abc", "def", "swapper/0", "cat e.sh minal-"},
	EventDescriptor: []*pb.EventDescriptor{
		{
			Name: 1,
			PropertyDescriptor: []*pb.EventDescriptor_PropertyDescriptor{
				{Name: 2, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
				{Name: 3, Type: pb.EventDescriptor_PropertyDescriptor_TEXT},
				{Name: 4, Type: pb.EventDescriptor_PropertyDescriptor_TEXT},
				{Name: 5, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
				{Name: 6, Type: pb.EventDescriptor_PropertyDescriptor_TEXT},
				{Name: 7, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
				{Name: 8, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
				{Name: 9, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
				{Name: 10, Type: pb.EventDescriptor_PropertyDescriptor_TEXT},
				{Name: 11, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
				{Name: 12, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
			},
		},
		{
			Name: 13,
			PropertyDescriptor: []*pb.EventDescriptor_PropertyDescriptor{
				{Name: 2, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
				{Name: 3, Type: pb.EventDescriptor_PropertyDescriptor_TEXT},
				{Name: 4, Type: pb.EventDescriptor_PropertyDescriptor_TEXT},
				{Name: 5, Type: pb.EventDescriptor_PropertyDescriptor_NUMBER},
				{Name: 14, Type: pb.EventDescriptor_PropertyDescriptor_TEXT},
				{Name: 15, Type: pb.EventDescriptor_PropertyDescriptor_TEXT},
			},
		},
	},
	Event: []*pb.Event{
		{
			EventDescriptor: 1,
			Cpu:             1,
			TimestampNs:     1040483711613818,
			Clipped:         false,
			Property:        []int64{0, 16, 0, 0, 17, 18},
		},
		{
			EventDescriptor: 0,
			Cpu:             0,
			TimestampNs:     1040483711613819,
			Clipped:         false,
			Property:        []int64{0, 16, 0, 0, 19, 0, 120, 0, 20, 166549, 120},
		},
		{
			EventDescriptor: 0,
			Cpu:             0,
			TimestampNs:     1040483711630169,
			Clipped:         false,
			Property:        []int64{0, 16, 0, 166549, 20, 166549, 120, 1, 19, 0, 120},
		},
		{
			EventDescriptor: 0,
			Cpu:             1,
			TimestampNs:     1040483711647349,
			Clipped:         false,
			Property:        []int64{0, 16, 0, 0, 19, 0, 120, 0, 20, 166549, 120},
		},
	},
}

func TestEventSetBuilder(t *testing.T) {
	esb := testEventSetBuilder()
	for _, traceEvent := range traceEvents {
		if err := esb.AddTraceEvent(traceEvent); err != nil {
			t.Fatalf("error in AddTraceEvent: %s", err)
		}
	}

	got, err := esb.Finalize()

	if err != nil {
		t.Fatalf("unexpected error finalizing events: %s", err)
	}

	if diff := cmp.Diff(eventSet, got, cmp.Comparer(proto.Equal)); diff != "" {
		t.Fatalf("TestEventSetBuilder: Diff -want +got:\n%s", diff)
	}
}

func TestEventSetBuilder_ClipStart(t *testing.T) {
	esb := testEventSetBuilder()
	esb.SetOverwrite(true)
	esb.SetOverflowedCPUs(map[int64]struct{}{0: {}, 1: {}})
	for _, traceEvent := range traceEvents {
		if err := esb.AddTraceEvent(traceEvent); err != nil {
			t.Fatalf("error in AddTraceEvent: %s", err)
		}
	}

	got, err := esb.Finalize()

	if err != nil {
		t.Fatalf("unexpected error finalizing events: %s", err)
	}

	for idx, event := range got.Event {
		if event.GetClipped() != (idx == 0) {
			t.Fatalf("Only the first event should be clipped")
		}
	}
}

func TestEventSetBuilder_ClipEnd(t *testing.T) {
	esb := testEventSetBuilder()
	esb.SetOverwrite(false)
	esb.SetOverflowedCPUs(map[int64]struct{}{0: {}, 1: {}})
	for _, traceEvent := range traceEvents {
		if err := esb.AddTraceEvent(traceEvent); err != nil {
			t.Fatalf("error in AddTraceEvent: %s", err)
		}
	}

	got, err := esb.Finalize()

	if err != nil {
		t.Fatalf("unexpected error finalizing events: %s", err)
	}

	for idx, event := range got.Event {
		if event.GetClipped() != (idx == len(got.Event)-1) {
			t.Fatalf("Only the last event should be clipped")
		}
	}
}

func TestEventSetBuilder_Clone(t *testing.T) {
	want, ok := proto.Clone(eventSet).(*pb.EventSet)
	if !ok {
		t.Fatalf("failed to clone eventSet")
	}
	want.Event = append(want.Event, want.Event[0])
	// Finalize on the esb will sort items, so this is
	// needed to ensure our expectations are true at the
	// end of the test.
	sort.Slice(want.Event, func(i, j int) bool {
		return want.Event[i].TimestampNs < want.Event[j].TimestampNs
	})

	esb := testEventSetBuilder()
	for _, traceEvent := range traceEvents {
		if err := esb.AddTraceEvent(traceEvent); err != nil {
			t.Fatalf("error in AddTraceEvent: %s", err)
		}
	}

	original, err := esb.Finalize()

	if err != nil {
		t.Fatalf("unexpected error finalizing events: %s", err)
	}

	clonedEsb, err := esb.Clone()
	if err != nil {
		t.Fatalf("error cloning event set builder: %s", err)
	}
	if err := clonedEsb.AddTraceEvent(traceEvents[0]); err != nil {
		t.Fatalf("error in AddTraceEvent: %s", err)
	}
	cloned, err := clonedEsb.Finalize()

	if err != nil {
		t.Fatalf("unexpected error finalizing events: %s", err)
	}

	if diff := cmp.Diff(eventSet, original, cmp.Comparer(proto.Equal)); diff != "" {
		t.Fatalf("TestEventSetBuilder_Clone: Diff -want +got:\n%s", diff)
	}
	if diff := cmp.Diff(cloned, want, cmp.Comparer(proto.Equal)); diff != "" {
		t.Fatalf("TestEventSetBuilder_Clone: Diff -want +got:\n%s", diff)
	}
}
