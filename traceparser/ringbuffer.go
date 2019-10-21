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
)

/**
 * #################################################################################################
 * #                                        ringBufferEvent                                        #
 * #################################################################################################
 */

// ringBufferType is an enum of internal ring buffer types
type ringBufferType uint8

const (
	// ringbufTypeDataTypeLenMax (0 <= type_len <= 28)
	//      Data record
	//      If type_len is zero:
	//        array[0] holds the actual length
	//        array[1..(length+3)/4] holds data
	//        size = 4 + length (bytes)
	//      else
	//        length = type_len << 2
	//        array[0..(length+3)/4-1] holds data
	//        size = 4 + length (bytes)
	ringbufTypeDataTypeLenMax ringBufferType = 28
	// ringbufTypePadding Left over page padding or discarded event
	//      If time_delta is 0:
	//        array is ignored
	//        size is variable depending on how much
	//        padding is needed
	//      If time_delta is non zero:
	//        array[0] holds the actual length
	//        size = 4 + length (bytes)
	ringbufTypePadding ringBufferType = 29
	// ringbufTypeTimeExtend Extend the time delta
	//      event.time_delta contains bottom 27 bits
	//      array[0] = top (28 .. 59) bits of the time_delta
	//      size = 8 bytes
	ringbufTypeTimeExtend ringBufferType = 30
	// ringbufTypeTimeStamp Absolute timestamp
	//      Same format as TIME_EXTEND except that the
	//      value is an absolute timestamp, not a delta
	//      event.time_delta contains bottom 27 bits
	//      array[0] = top (28 .. 59) bits
	//      size = 8 bytes
	ringbufTypeTimeStamp ringBufferType = 31
)

const (
	// typeLenSize is the size in bits of the type_len field in a ring buffer event header
	typeLenSize = 5
	// timeDeltaSize is the size in bits of the time_delta field in a ring buffer event header
	timeDeltaSize = 27
)

const (
	// ringBufferEventHeaderSize is the size in bytes of the RingBufferHeader minus the size of its
	// array.
	ringBufferEventHeaderSize = (typeLenSize + timeDeltaSize) / 8 // Divide by 8 to convert to bytes
	// ringBufferTimeExtendLength is the size of the data stored in an ringbufTypeTimeExtend event
	ringBufferTimeExtendLength = 8
	// ringBufferTimeStampLength is the size of the data stored in an ringbufTypeTimeStamp event
	ringBufferTimeStampLength = 8
)

// ringBufferEvent contains:
// type_len : 5 bits
// time_delta : 27 bits (relative to base timestamp in page header)
// array : variable length. See ringBufferType's docs for details
type ringBufferEvent struct {
	Bitfield   uint32
	Array      []byte
	endianness binary.ByteOrder
}

// TypeLen returns the type_len field of a ringBufferEvent
// type_len is a field that describes both the type of the event and the length of its data array.
// If 0 <= type_len <= 28, then the event is a data event and type_len << 2 is the length of the
// data array. If type_len == 0 or type_len > 28, then type_len is just a type, and the length is
// contained elsewhere depending on the specific value of type_len. The various types that type_len
// can be are enumerated in the ringBufferType enum. See getLength() for more details.
func (r *ringBufferEvent) TypeLen() (uint8, error) {
	switch r.endianness {
	case binary.LittleEndian:
		// Get the lower 5 bits.
		return uint8(r.Bitfield & ((1 << typeLenSize) - 1)), nil
	case binary.BigEndian:
		return 0, errors.New("big endian is not supported")
	default:
		return 0, errors.New("unknown endianness")
	}
}

// TimeDelta returns the time_delta field of a ringBufferEvent
func (r *ringBufferEvent) TimeDelta() (uint32, error) {
	switch r.endianness {
	case binary.LittleEndian:
		// Get all but the lower 5 bits
		return uint32(r.Bitfield >> typeLenSize), nil
	case binary.BigEndian:
		return 0, errors.New("big endian is not supported")
	default:
		return 0, errors.New("unknown endianness")
	}
}

// TimestampOrExtendedTimeDelta returns the full time delta or stamp of a ringBufferEvent event
// which is the time_delta field prepended with the first element in the data array.
// This is only valid for ringbufTypeTimeExtend and ringbufTypeTimeStamp events
// If the event is a ringbufTypeTimeExtend event the return value represents an extended time delta.
// If the event is a ringbufTypeTimeStamp event the return value represents an absolute timestamp.
func (r *ringBufferEvent) TimestampOrExtendedTimeDelta() (uint64, error) {
	typeLen, err := r.TypeLen()
	if err != nil {
		return 0, err
	}
	if rbType := ringBufferType(typeLen); rbType != ringbufTypeTimeExtend && rbType != ringbufTypeTimeStamp {
		return 0, errors.New("TimestampOrExtendedTimeDelta() is only valid on time extend and timestamp events")
	}
	// See http://cs/kernel/kernel/trace/ring_buffer.c?l=3502-3507&rcl=b7717a6fe4424ca0a4656ee281c37979f22f2ffb
	// for the kernel version of this code
	data, err := r.DataFromArray()
	if err != nil {
		return 0, err
	}
	timeDelta, err := r.TimeDelta()
	if err != nil {
		return 0, err
	}

	delta := uint64(r.endianness.Uint32(data))
	delta <<= timeDeltaSize
	delta += uint64(timeDelta)
	return delta, nil
}

// DataFromArray returns the part of the array that contains data
// Depending on the value of type_len, the beginning of the data array is either bit 0 or bit 33
func (r *ringBufferEvent) DataFromArray() ([]byte, error) {
	typeLen, err := r.TypeLen()
	if err != nil {
		return nil, err
	}
	if ringBufferType(typeLen) > 0 {
		return r.Array, nil
	}
	return r.Array[4:], nil
}

// LenFromArray returns the first 32 bits of the data array, which is used as a length when
// type_len is ringbufTypePadding or 0
func (r *ringBufferEvent) LenFromArray() uint32 {
	return r.endianness.Uint32(r.Array)
}

// Length computes the length of a ringBufferEvent, which varies based on the value of type_len
func (r *ringBufferEvent) Length() (uint32, error) {
	// See http://cs/kernel/kernel/trace/ring_buffer.c?l=173-201&rcl=b7717a6fe4424ca0a4656ee281c37979f22f2ffb
	// for the kernel implementation of this function
	rawTypeLen, err := r.TypeLen()
	if err != nil {
		return 0, err
	}
	typeLen := ringBufferType(rawTypeLen)

	if typeLen == ringbufTypePadding {
		timeDelta, err := r.TimeDelta()
		if err != nil {
			return 0, err
		}
		if timeDelta == 0 {
			/* undefined */
			return 0, nil
		}
		return r.LenFromArray(), nil
	} else if typeLen == ringbufTypeTimeExtend {
		// Subtract the length of the header as the constant contains it
		return ringBufferTimeExtendLength - ringBufferEventHeaderSize, nil
	} else if typeLen == ringbufTypeTimeStamp {
		// Subtract the length of the header as the constant contains it
		return ringBufferTimeStampLength - ringBufferEventHeaderSize, nil
	} else if typeLen == 0 {
		return r.LenFromArray(), nil
	} else if 1 <= typeLen && typeLen <= ringbufTypeDataTypeLenMax {
		return uint32(typeLen << 2), nil
	} else {
		return 0, fmt.Errorf("unknown ring buffer type: %d", typeLen)
	}
}

/**
 * #################################################################################################
 * #                                      ringBufferPageHeader                                     #
 * #################################################################################################
 */

// ringBufferPageHeader represents the header of a page
type ringBufferPageHeader interface {
	Timestamp() uint64
	Commit() []byte
	Overwrite() uint8
	Size() uint64
	SetEndianness(e binary.ByteOrder)
	Endianness() binary.ByteOrder
	Data() interface{}
}

// ringBufferPageHeader64 is a ringBufferPageHeader with a 64 bit Commit field
// For internal use only
type ringBufferPageHeader64 struct {
	commit     [8]byte
	endianness binary.ByteOrder
	data       struct {
		// The base timestamp of this page. Time Deltas in events are relative to this.
		Timestamp uint64
		// First byte of Commit is the Overwrite indicator, rest of it is the size of the page data
		Commit uint64
	}
}

func (r *ringBufferPageHeader64) Timestamp() uint64 {
	return r.data.Timestamp
}

func (r *ringBufferPageHeader64) Commit() []byte {
	b := r.commit[:]
	r.Endianness().PutUint64(b, r.data.Commit)
	return b
}

// Overwrite returns if this page was overwritten (like during FDR mode)
func (r *ringBufferPageHeader64) Overwrite() uint8 {
	// Get the leftmost byte
	return uint8(r.data.Commit >> (7 * 8))
}

// Size returns the size in bytes of the data contained in this page
func (r *ringBufferPageHeader64) Size() uint64 {
	// Only the lower 20 bits contain the size.
	// See https://lkml.org/lkml/2019/5/23/1623
	return r.data.Commit & 0xfffff
}

func (r *ringBufferPageHeader64) SetEndianness(e binary.ByteOrder) {
	r.endianness = e
}

func (r *ringBufferPageHeader64) Endianness() binary.ByteOrder {
	return r.endianness
}

func (r *ringBufferPageHeader64) Data() interface{} {
	return &r.data
}

// ringBufferPageHeader32 is a ringBufferPageHeader with a 32 bit Commit field
// For internal use only
type ringBufferPageHeader32 struct {
	commit     [4]byte
	endianness binary.ByteOrder
	data       struct {
		// The base timestamp of this page. Time Deltas in events are relative to this.
		Timestamp uint64
		// First byte of Commit is the Overwrite indicator, rest of it is the size of the page data
		Commit uint32
	}
}

func (r *ringBufferPageHeader32) Timestamp() uint64 {
	return r.data.Timestamp
}

func (r *ringBufferPageHeader32) Commit() []byte {
	b := r.commit[:]
	r.Endianness().PutUint32(r.commit[:], r.data.Commit)
	return b
}

// Overwrite returns if this page was overwritten (like during FDR mode)
func (r *ringBufferPageHeader32) Overwrite() uint8 {
	// Get the leftmost byte
	return uint8(r.data.Commit >> (3 * 8))
}

// Size returns the size in bytes of the data contained in this page
func (r *ringBufferPageHeader32) Size() uint64 {
	// Only the lower 20 bits contain the size.
	// See https://lkml.org/lkml/2019/5/23/1623
	return uint64(r.data.Commit & 0xfffff)
}

func (r *ringBufferPageHeader32) SetEndianness(e binary.ByteOrder) {
	r.endianness = e
}

func (r *ringBufferPageHeader32) Endianness() binary.ByteOrder {
	return r.endianness
}

func (r *ringBufferPageHeader32) Data() interface{} {
	return &r.data
}
