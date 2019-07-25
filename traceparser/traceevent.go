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
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// TraceEvent holds a single trace event as unmarshalled from the raw binary trace output
type TraceEvent struct {
	// The timestamp in the trace of this event.
	Timestamp uint64
	// The CPU that this event was recorded on
	CPU int64
	// A mapping of text property names to property values, normally strings.
	// If there is a dynamic array field, it will be stored as a binary blob in TextProperties.
	TextProperties map[string]string
	// A mapping of numeric properties to property values.
	NumberProperties map[string]int64
	// The Format ID of this event. Should be an event ID defined in a loaded format file.
	FormatID uint16
	// True if this Event fell outside of the known-valid range of a trace which
	// experienced buffer overruns.
	Clipped bool
	// An error, if one occurred while creating this TraceEvent
	Err error
}

// NewTraceEvent creates a new TraceEvent
func NewTraceEvent(cpu int64) *TraceEvent {
	return &TraceEvent{
		CPU:              cpu,
		TextProperties:   make(map[string]string),
		NumberProperties: make(map[string]int64),
	}
}

// SaveFieldValue saves a byte array into a TraceEvent's field.
// The byte array will be converted to the type specified in the field definition.
func (t *TraceEvent) SaveFieldValue(field *FormatField, buf []byte, endianness binary.ByteOrder) error {
	if field.IsDynamicArray {
		// Store binary blob as a string, which is allowed in Go.
		t.TextProperties[field.Name] = string(buf)
		return nil
	}

	if field.ProtoType == "string" {
		// Convert to string and remove extra trailing null bytes
		t.TextProperties[field.Name] = strings.Split(string(buf), "\x00")[0]
	} else if field.ProtoType == "int64" {
		// Pad to 8 bytes
		if len(buf) < 8 {
			padding := [8]byte{}
			switch endianness {
			case binary.LittleEndian:
				copy(padding[:(8-len(buf))], buf)
			case binary.BigEndian:
				return errors.New("big endian is not supported")
			default:
				return errors.New("unknown endianness")
			}
			buf = padding[:]
		}
		t.NumberProperties[field.Name] = int64(endianness.Uint64(buf))
	} else {
		return fmt.Errorf("unknown field type %s. only string and int64 are supported", field.ProtoType)
	}
	return nil
}
