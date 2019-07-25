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
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseFormat(t *testing.T) {
	headerFormat := `
Header:
	field: u64 timestamp;	offset:0;	size:8;	signed:0;
	field: local_t commit;	offset:8;	size:8;	signed:1;
	field: int overwrite;	offset:8;	size:1;	signed:1;
	field: char data;	offset:16;	size:4080;	signed:1;
`

	formats := []string{`
name: sched_switch
ID: 314
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;

	field:char prev_comm[16];	offset:8;	size:16;	signed:1;
	field:pid_t prev_pid;	offset:24;	size:4;	signed:1;

print fmt: "prev_comm=%s prev_pid=%d prev_prio=%d prev_state=%s%s ==> next_comm=%s next_pid=%d next_prio=%d", REC->prev_comm, REC->prev_pid, REC->prev_prio, (REC->prev_state & ((((0x0000 | 0x0001 | 0x0002 | 0x0004 | 0x0008 | 0x0010 | 0x0020 | 0x0040) + 1) << 1) - 1)) ? __print_flags(REC->prev_state & ((((0x0000 | 0x0001 | 0x0002 | 0x0004 | 0x0008 | 0x0010 | 0x0020 | 0x0040) + 1) << 1) - 1), "|", { 0x01, "S" }, { 0x02, "D" }, { 0x04, "T" }, { 0x08, "t" }, { 0x10, "X" }, { 0x20, "Z" }, { 0x40, "P" }, { 0x80, "I" }) : "R", REC->prev_state & (((0x0000 | 0x0001 | 0x0002 | 0x0004 | 0x0008 | 0x0010 | 0x0020 | 0x0040) + 1) << 1) ? "+" : "", REC->next_comm, REC->next_pid, REC->next_prio
`,
		`
name: special_event
ID: 1942
format:
	field:unsigned short common_type;	offset:0;	size:2;	signed:0;
	field:unsigned char common_flags;	offset:2;	size:1;	signed:0;
	field:unsigned char common_preempt_count;	offset:3;	size:1;	signed:0;
	field:int common_pid;	offset:4;	size:4;	signed:1;

	field:__data_loc uint8[] event;	offset:8;	size:4;	signed:0;

print fmt: "%s", print_special_evt(p, (__get_dynamic_array(event)))
`}

	want := TraceParser{
		HeaderFormat: Format{
			Fields: []*FormatField{
				{FieldType: "u64 timestamp", Name: "timestamp", ProtoType: "int64", Offset: 0, Size: 8, NumElements: 1, ElementSize: 8, Signed: false, IsDynamicArray: false},
				{FieldType: "local_t commit", Name: "commit", ProtoType: "int64", Offset: 8, Size: 8, NumElements: 1, ElementSize: 8, Signed: true, IsDynamicArray: false},
				{FieldType: "int overwrite", Name: "overwrite", ProtoType: "int64", Offset: 8, Size: 1, NumElements: 1, ElementSize: 1, Signed: true, IsDynamicArray: false},
				{FieldType: "char data", Name: "data", ProtoType: "string", Offset: 16, Size: 4080, NumElements: 1, ElementSize: 4080, Signed: true, IsDynamicArray: false},
			},
		},
		Formats: map[uint16]*EventFormat{
			314: {
				Name: "sched_switch",
				ID:   314,
				Format: Format{
					CommonFields: []*FormatField{
						{FieldType: "unsigned short common_type", Name: "common_type", ProtoType: "int64", Offset: 0, Size: 2, NumElements: 1, ElementSize: 2, Signed: false, IsDynamicArray: false},
						{FieldType: "unsigned char common_flags", Name: "common_flags", ProtoType: "string", Offset: 2, Size: 1, NumElements: 1, ElementSize: 1, Signed: false, IsDynamicArray: false},
					},
					Fields: []*FormatField{
						{FieldType: "char prev_comm[16]", Name: "prev_comm", ProtoType: "string", Offset: 8, Size: 16, NumElements: 16, ElementSize: 1, Signed: true, IsDynamicArray: false},
						{FieldType: "pid_t prev_pid", Name: "prev_pid", ProtoType: "int64", Offset: 24, Size: 4, NumElements: 1, ElementSize: 4, Signed: true, IsDynamicArray: false},
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

	got, err := New(headerFormat, formats)
	if err != nil {
		t.Fatalf("Error in traceparser.New(): %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestParseFormatFile: Diff -want +got:\n%s", diff)
	}

}

func TestFieldParsing(t *testing.T) {
	tests := []struct {
		in  string
		out FormatField
		err error
	}{
		{
			in: "	field: void * alarm;	offset:0;	size:8;	signed:0;",
			out: FormatField{FieldType: "void * alarm", Name: "alarm", ProtoType: "int64", Offset: 0, Size: 8, NumElements: 1, ElementSize: 8, Signed: false, IsDynamicArray: false},
		},
		{
			in: "	field: char comm[16];	offset:0;	size:16;	signed:0;",
			out: FormatField{FieldType: "char comm[16]", Name: "comm", ProtoType: "string", Offset: 0, Size: 16, NumElements: 16, ElementSize: 1, Signed: false, IsDynamicArray: false},
		},
		{
			in: "	field: __data_loc char[] dev;	offset:0;	size:16;	signed:0;",
			// Note: don't currently compute number of elements by size/type size.
			// Instead, we look at the number in the brackets.
			out: FormatField{FieldType: "__data_loc char[] dev", Name: "dev", ProtoType: "string", Offset: 0, Size: 16, NumElements: 1, ElementSize: 16, Signed: false, IsDynamicArray: false},
		},
		{
			in: "	field: s64 now;	offset:0;	size:8;	signed:0;",
			out: FormatField{FieldType: "s64 now", Name: "now", ProtoType: "int64", Offset: 0, Size: 8, NumElements: 1, ElementSize: 8, Signed: false, IsDynamicArray: false},
		},
		{
			in: "	field: ext4_fsblk_t pblk;	offset:0;	size:8;	signed:0;",
			out: FormatField{FieldType: "ext4_fsblk_t pblk", Name: "pblk", ProtoType: "int64", Offset: 0, Size: 8, NumElements: 1, ElementSize: 8, Signed: false, IsDynamicArray: false},
		},
		{
			in: "	field: __u8 saddr_v6[16];	offset:0;	size:128;	signed:0;",
			out: FormatField{FieldType: "__u8 saddr_v6[16]", Name: "saddr_v6", ProtoType: "int64", Offset: 0, Size: 128, NumElements: 16, ElementSize: 8, Signed: false, IsDynamicArray: false},
		},
		{
			in: "	field: struct dbc_request * req;	offset:0;	size:16;	signed:0;",
			out: FormatField{FieldType: "struct dbc_request * req", Name: "req", ProtoType: "int64", Offset: 0, Size: 16, NumElements: 1, ElementSize: 16, Signed: false, IsDynamicArray: false},
		},
		{
			in: "	field: abc def hgijk lmnop;	offset:0;	size:128;	signed:0;",
			err: errors.New("\"abc def hgijk lmnop\" does not appear to be a C declaration expression"),
		},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("TestFieldParsing Case: %d", i), func(t *testing.T) {
			field, err := parseField(test.in)
			if err != nil {
				if test.err != nil {
					if diff := cmp.Diff(test.err.Error(), err.Error()); diff != "" {
						t.Fatalf("Input: %s\nDiff -want +got:\n%s", test.in, diff)
					}
					return
				}
				t.Fatal(err)
			}

			if diff := cmp.Diff(test.out, *field); diff != "" {
				t.Fatalf("Input: %s\nDiff -want +got:\n%s", test.in, diff)
			}
		})
	}
}
