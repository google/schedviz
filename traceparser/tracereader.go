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

// tracereader contains methods for reading binary trace data

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unsafe"

	log "github.com/golang/glog"
)

// SetNativeEndian makes the TraceParser parse binary data in the native endian byte order
// of this machine. Currently only little endian is supported.
func (tp *TraceParser) SetNativeEndian() error {
	// From https://github.com/tensorflow/tensorflow/blob/fe5e1f39590f5847a384dcccb33956a5c2606d16/tensorflow/go/tensor.go#L488-L505
	var nativeEndian binary.ByteOrder

	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xABCD)

	switch buf {
	case [2]byte{0xCD, 0xAB}:
		nativeEndian = binary.LittleEndian
	case [2]byte{0xAB, 0xCD}:
		nativeEndian = binary.BigEndian
	default:
		return errors.New("could not determine native endianness")
	}

	tp.Endianness = nativeEndian
	return nil
}

// SetBigEndian makes the TraceParser parse binary data in the big endian byte order
// Big endian is not currently supported.
func (tp *TraceParser) SetBigEndian() error {
	tp.Endianness = binary.BigEndian
	// Return a nil error for consistency with SetNativeEndian()
	return nil
}

// SetLittleEndian makes the TraceParser parse binary data in the little endian byte order
func (tp *TraceParser) SetLittleEndian() error {
	tp.Endianness = binary.LittleEndian
	// Return a nil error for consistency with SetNativeEndian()
	return nil
}

// TraceReader is an interface for an io.Reader that also provides a Discard function
// An example implementation of this interface is bufio.Reader
type TraceReader interface {
	//Read reads up to len(p) bytes into p. It returns the number of bytes read
	// (0 <= n <= len(p)) and any error encountered.
	Read(p []byte) (n int, err error)
	// Discard skips the next n bytes, returning the number of bytes discarded.
	//
	// If Discard skips fewer than n bytes, it also returns an error.
	// If 0 <= n <= b.Buffered(), Discard is guaranteed to succeed without
	// reading from the underlying io.Reader.
	Discard(n int) (discarded int, err error)
}

// ParseTrace accepts a TraceReader (such as a bufio.Reader) from which raw trace data may be read,
// the number of the CPU whose buffer is being read, and a callback that will take TraceEvents
// parsed from that raw trace data.  If the callback returns false or has a non-nil error,
// ParseTrace will return. If an error is returned by ParseTrace, the raw trace should be considered
// to be corrupted.
func (tp *TraceParser) ParseTrace(reader TraceReader, cpu int64, callback func(*TraceEvent) (bool, error)) error {
	if tp.Endianness == nil {
		if err := tp.SetNativeEndian(); err != nil {
			return err
		}
	}

	for {
		pageHeader, err := tp.readPageHeader(reader)
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}

		// Read data
		page, err := tp.readPageData(reader, pageHeader.Size())
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("failed to read page. caused by: %s", err)
			}
			return nil
		}

		var timeStamp = pageHeader.Timestamp

		// readEvent() advances the page start pointer, so stop when there can't be anything
		// contained in what's left
		for len(page) >= ringBufferEventHeaderSize {
			traceEvent := NewTraceEvent(cpu)
			rbEvent, err := tp.readEvent(&page)
			if err != nil {
				return err
			}

			rawTypeLen, err := rbEvent.TypeLen()
			if err != nil {
				return err
			}
			typeLen := ringBufferType(rawTypeLen)

			// Handle non-data events
			if typeLen == ringbufTypeTimeExtend {
				delta, err := rbEvent.TimestampOrExtendedTimeDelta()
				if err != nil {
					return err
				}
				timeStamp += delta
				continue
			} else if typeLen == ringbufTypeTimeStamp {
				// Sync time stamp with external clock.
				newTimestamp, err := rbEvent.TimestampOrExtendedTimeDelta()
				if err != nil {
					return err
				}
				timeStamp = newTimestamp
				continue
			} else if typeLen >= ringbufTypePadding {
				continue
			}

			eventData := rbEvent.Array

			// The format ID is the first two bytes in eventData
			id := tp.Endianness.Uint16(eventData)

			evtFmt := tp.Formats[id]
			if evtFmt == nil {
				return fmt.Errorf("no format found with id: %d", id)
			}
			eFormat := evtFmt.Format
			traceEvent.FormatID = id

			timeDelta, err := rbEvent.TimeDelta()
			if err != nil {
				return err
			}
			timeStamp += uint64(timeDelta)
			traceEvent.Timestamp = timeStamp

			// Read in each field using the offset and size we got from the format files.
			for _, field := range append(eFormat.CommonFields[1:], eFormat.Fields...) {
				buf := eventData[field.Offset:(field.Offset + field.Size)]
				if err := traceEvent.SaveFieldValue(field, buf, tp.Endianness); err != nil {
					return err
				}
				if field.IsDynamicArray {
					if field.Size != 4 {
						log.Warningf("field %q is used as a dynamic array, but its structure does not appear to match one. Size should be 4 bytes, but was %d bytes. skipping reading the array", field.Name, field.Size)
						continue
					}
					offset := tp.Endianness.Uint16(buf[:2])
					length := tp.Endianness.Uint16(buf[2:4])
					dynArrBuf := eventData[offset:(offset + length)]
					dynArrField := &FormatField{
						Name:           "__data_loc_" + field.Name,
						IsDynamicArray: true,
					}
					if err := traceEvent.SaveFieldValue(dynArrField, dynArrBuf, tp.Endianness); err != nil {
						return err
					}
				}
			}

			if cont, err := callback(traceEvent); !cont {
				return err
			}
		}

		// If there weren't enough events to fill up this page, and we aren't done reading all the
		// pages, then skip to the next page.
		if err = tp.skipToNextPage(reader, tp.HeaderFormat, pageHeader.Size()); err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

func (tp *TraceParser) readPageHeader(page io.Reader) (ringBufferPageHeader, error) {
	pageHeader := ringBufferPageHeader{}
	if err := binary.Read(page, tp.Endianness, &pageHeader); err != nil {
		return ringBufferPageHeader{}, err
	}
	return pageHeader, nil
}

func (tp *TraceParser) readPageData(reader io.Reader, dataSize uint64) ([]byte, error) {
	pageBuf := make([]byte, dataSize)
	n, err := reader.Read(pageBuf)
	if n != len(pageBuf) {
		return nil, fmt.Errorf("not enough bytes left in reader. wanted to read %d, but read %d", len(pageBuf), n)
	}
	if err != nil {
		return nil, err
	}

	return pageBuf, nil
}

func (tp *TraceParser) readEvent(buf *[]byte) (ringBufferEvent, error) {
	if len(*buf) < ringBufferEventHeaderSize {
		return ringBufferEvent{}, fmt.Errorf("not enough bytes to contain ring buffer event header. got: %d, want: %d", len(*buf), ringBufferEventHeaderSize)
	}

	rbEvent := ringBufferEvent{Bitfield: tp.Endianness.Uint32((*buf)[:4]), endianness: tp.Endianness}
	*buf = (*buf)[4:]
	// The length of the data is stored in the either the bitfield or in the first 4 bytes of the data
	rbEvent.Array = (*buf)[:4]
	eventLength, err := rbEvent.Length()
	if err != nil {
		return ringBufferEvent{}, fmt.Errorf("unable to get length of event. caused by: %s", err)
	}

	if uint32(len(*buf)) < eventLength {
		return ringBufferEvent{}, fmt.Errorf("not enough bytes to contain ring buffer data. got: %d, want: %d", len(*buf), eventLength)
	}

	rbEvent.Array = (*buf)[:eventLength]
	*buf = (*buf)[eventLength:]

	return rbEvent, nil
}

func (tp *TraceParser) skipToNextPage(reader TraceReader, headerFormat Format, bytesRead uint64) error {
	numRemainingBytes := int(headerFormat.Fields[3].Size - bytesRead)
	if numRemainingBytes > 0 {
		discarded, err := reader.Discard(numRemainingBytes)
		if discarded != numRemainingBytes {
			return fmt.Errorf("not enough bytes left in reader. wanted to discard %d, but discarded %d", numRemainingBytes, discarded)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
