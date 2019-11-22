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

// formatparser contains a parser for TraceFS format files

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	nameRe   = regexp.MustCompile(`name:[ \t]*(\w+)`)
	idRe     = regexp.MustCompile(`ID:[ \t]*(\d+)`)
	fieldRe  = regexp.MustCompile(`field:[ \t]*([^;]+);[ \t]*offset:[ \t]*(\d+);[ \t]*size:[ \t]*(\d+);[ \t]*(?:signed:[ \t]*(\d+);)?`)
	typeRe   = regexp.MustCompile(`^((?:\w+\s+)?\w+(?:\s+\**\s*)?(?:\[])?)\s+(\w+)\s*(?:\[\s*(\d+)\s*])?$`)
	charRe   = regexp.MustCompile(`\bchar\b`)
	dynArrRe = regexp.MustCompile(`^__data_loc\b`)
)

type parseState int

const (
	findName parseState = iota
	findID
	findFormat
	findCommonField
	findField
	done
)

// parseRegularFormats parses TraceFS Formats into an EventFormat structs.
// formatFiles is list of contents of format files.
// This function returns a map from event type to EventFormat structs.
// format files look like this:
/**
name: some_name
ID: 123
format:
	field: C Type;	offset: 0;	size: 123;	signed: 0; <-- These are the common fields
	... (more fields)
BLANK LINE
	field: C Type;	offset: 0;	size: 123;	signed: 0; <-- These are the event specific fields
	... (more fields)
BLANK LINE
print fmt: C String, printf format parameters <-- Essentially arguments to C's printf()
*/
func parseRegularFormats(formatFiles []string) (map[uint16]*EventFormat, error) {
	var ret = make(map[uint16]*EventFormat, len(formatFiles))

	for _, formatFileContent := range formatFiles {
		scanner := bufio.NewScanner(strings.NewReader(formatFileContent))

		evtFmt := EventFormat{}
		state := findName

	scan:
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if trimmed == "" && state != findCommonField {
				// Skip empty or whitespace-only lines.
				continue
			}

			switch state {
			case findName:
				name, err := parseName(line)
				if err != nil {
					return nil, err
				}
				evtFmt.Name = name
				state = findID
			case findID:
				id, err := parseID(line)
				if err != nil {
					return nil, err
				}
				evtFmt.ID = id
				state = findFormat
			case findFormat:
				if trimmed == "format:" {
					// Start checking for field lines
					evtFmt.Format = Format{}
					state = findCommonField
				} else {
					return nil, fmt.Errorf("expected \"format:\", but got \"%s\" instead", line)
				}
			case findCommonField:
				newCommonField, err := parseField(line)
				if err != nil {
					state = findField
					continue
				}
				evtFmt.Format.CommonFields = append(evtFmt.Format.CommonFields, newCommonField)
			case findField:
				newField, err := parseField(line)
				if err != nil {
					state = done
					continue
				}
				evtFmt.Format.Fields = append(evtFmt.Format.Fields, newField)
			case done:
				break scan
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("unable to read format. caused by: %v", err)
		}

		ret[evtFmt.ID] = &evtFmt
	}

	return ret, nil
}

// parseHeaderFormat parses the header_page TraceFS format file into an format struct.
// Header format files look like this:
/**
Header:
	field: C Type;	offset: 0;	size: 123;	signed: 0;
	... (more fields)
*/
func parseHeaderFormat(headerFileContent string) (*Format, error) {
	scanner := bufio.NewScanner(strings.NewReader(headerFileContent))
	ret := Format{}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "Header:" {
			// Skip empty, whitespace-only, and header lines.
			continue
		}

		newField, err := parseField(line)
		if err != nil {
			return nil, err
		}
		ret.Fields = append(ret.Fields, newField)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("unable to read header format file. caused by: %v", err)
	}

	return &ret, nil
}

func parseName(line string) (string, error) {
	matches := nameRe.FindSubmatch([]byte(line))
	if matches == nil {
		return "", fmt.Errorf("unexpected string \"%s\"", line)
	}
	strMatch := string(matches[1])
	return strMatch, nil
}

func parseID(line string) (uint16, error) {
	matches := idRe.FindSubmatch([]byte(line))
	if matches == nil {
		return 0, fmt.Errorf("unexpected string \"%s\"", line)
	}
	strMatch := string(matches[1])
	id, err := strconv.ParseUint(strMatch, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing ID: %s", err)
	}
	return uint16(id), nil
}

func parseField(line string) (*FormatField, error) {
	matches := fieldRe.FindSubmatch([]byte(line))
	if matches == nil {
		return nil, fmt.Errorf("unexpected string \"%s\"", line)
	}
	fieldType := string(matches[1])

	size, err := strconv.ParseUint(string(matches[3]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing size for field: %s", err)
	}

	field, err := constructFormatField(fieldType, size)
	if err != nil {
		return nil, err
	}

	offset, err := strconv.ParseUint(string(matches[2]), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing size for field %s: %s", field.Name, err)
	}
	field.Offset = offset

	// Some kernels don't have signed in their field formats.
	if matches[4] != nil {
		signed, err := strconv.ParseUint(string(matches[4]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing signed for field %s: %s", field.Name, err)
		}
		if signed == 0 {
			field.Signed = false
		} else {
			field.Signed = true
		}
	}

	return &field, nil
}

func constructFormatField(fieldType string, size uint64) (FormatField, error) {
	field := FormatField{FieldType: fieldType, Size: size}
	// For now, just check if char array or not.
	// If char array - return string, else assume int64.
	matches := typeRe.FindSubmatch([]byte(fieldType))
	if matches == nil {
		return FormatField{}, fmt.Errorf("\"%s\" does not appear to be a C declaration expression", fieldType)
	}

	field.Name = string(matches[2])

	cType := matches[1]
	// Treat fields of char type that are more than one byte long as strings;
	// char types that are one byte long will be treated as integers.
	// This is needed because many char fields in some events are used as
	// bitfields and can therefore contain non-UTF8 code point values, which can
	// not be stored in a proto string.
	if charRe.Match(cType) && size > 1 {
		field.ProtoType = "string"
	} else if dynArrRe.Match(cType) {
		// If this field's type includes "__data_loc", then it describes a dynamic array.
		// If the type is "__data_loc char []" then it is a dynamic string, which we currently don't
		// handle, so in this case, don't treat the field as a dynamic array.
		field.ProtoType = "string"
		field.IsDynamicArray = true
	} else {
		field.ProtoType = "int64"
	}

	if matches[3] != nil {
		numElems, err := strconv.ParseUint(string(matches[3]), 10, 32)
		if err != nil {
			return FormatField{}, fmt.Errorf("unable to parse numElems for field %s. caused by: %s", field.Name, err)
		}

		if numElems == 0 {
			return FormatField{}, fmt.Errorf("field \"%s\" has is a zero length array, which is not valid", fieldType)
		}

		field.NumElements = numElems
		elementSize := size / numElems
		field.ElementSize = elementSize
	} else {
		field.NumElements = 1
		field.ElementSize = size
	}

	return field, nil
}
