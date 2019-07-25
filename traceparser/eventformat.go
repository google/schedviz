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

// EventFormat represents a TraceFS event's format
type EventFormat struct {
	Name   string
	ID     uint16
	Format Format
}

// Format is a collection of fields contained within an event format
type Format struct {
	// CommonFields are fields that are common to all Events
	CommonFields []*FormatField
	// Fields are fields that are unique to each Event type
	Fields []*FormatField
}

// FormatField describes a single field within a format
type FormatField struct {
	// FieldType is the C declaration of the field
	FieldType string
	// Name is the name of the field. Extracted from the name in the C declaration in FieldType
	Name string
	// ProtoType is the "proto" type of the field. Extracted from FieldType.
	// If the type in FieldType is char or char[], ProtoType will be string. Otherwise, it'll be int64
	ProtoType string
	// The offset from the beginning of the event in bytes
	Offset uint64
	// Size is the size of this field in bytes
	Size uint64
	// NumElements is the number of elements in this type. Only relevant for array types.
	NumElements uint64
	// ElementSize is the size of each element in bytes. Only relevant for array types.
	ElementSize uint64
	// Signed states if this type is signed or unsigned. Only relevant for numeric types.
	Signed bool
	// IsDynamicArray is true if this field contains a struct describing a dynamic array
	// The struct is of the form:
	// struct {
	//    uint16 offset
	//    uint16 length
	// }
	IsDynamicArray bool
}
