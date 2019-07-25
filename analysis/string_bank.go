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
// Package sched provides interfaces and helper functions for scheduling
// tracepoint collections.  It understands the sched:: tracepoint events
// sched_migrate_task, sched_wait_task, sched_wakeup, sched_wakeup_new,
// and sched_switch.
package sched

import (
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stringID identifies a unique string in a string bank.
type stringID int

const (
	// A string describing unknown CPUs, priorities, commands, and other fields.
	unknownString = "<unknown>"
)

// stringTable permits string lookup by unique stringID.  It does not support
// insertion (pushBack) concurrently with other insertions or lookups
// (stringByID), but lookups may be concurrent.
type stringTable struct {
	strings []string
}

func (st stringTable) stringByID(id stringID) (string, error) {
	if id < 0 || id >= stringID(len(st.strings)) {
		return "", status.Errorf(codes.NotFound, "string %d not found", id)
	}
	return st.strings[id], nil
}

// pushBack adds the provided string to the end of the stringTable, and returns
// its stringID.  It does not ensure appended strings are unique.
func (st *stringTable) pushBack(str string) stringID {
	newID := stringID(len(st.strings))
	st.strings = append(st.strings, str)
	return newID
}

// stringBank compacts a set of often-repeated strings, such as command names
// in scheduling events, by giving each unique string a unique identifier
// number.  String IDs may be fetched (and generated for new strings) with
// stringIDByString(), and strings may be fetched by ID with stringByID().
// stringBank is thread-safe for both lookup and insertion.
type stringBank struct {
	stringTable *stringTable
	stringIDs   map[string]stringID
	mutex       sync.RWMutex
}

func newStringBank() *stringBank {
	return &stringBank{
		stringTable: &stringTable{},
		stringIDs:   make(map[string]stringID),
	}
}

// stringByID returns the string stored in the string bank at the provided
// index, or an error if not present.
func (sb *stringBank) stringByID(id stringID) (string, error) {
	sb.mutex.RLock()
	defer sb.mutex.RUnlock()
	return sb.stringTable.stringByID(id)
}

// stringIDByString returns the index into the string bank for the supplied
// string, adding it to the bank if necessary.
func (sb *stringBank) stringIDByString(str string) stringID {
	// Read-only fast path.
	if id, ok := func(str string) (stringID, bool) {
		sb.mutex.RLock()
		defer sb.mutex.RUnlock()
		id, ok := sb.stringIDs[str]
		if ok {
			return id, true
		}
		return 0, false
	}(str); ok {
		return id
	}
	// Read/write slow path.
	sb.mutex.Lock()
	defer sb.mutex.Unlock()
	// See if someone wrote this value while we were waiting on the lock.
	id, ok := sb.stringIDs[str]
	if ok {
		return id
	}
	// No?  OK, let's put it in.
	id = sb.stringTable.pushBack(str)
	sb.stringIDs[str] = id
	return id
}
