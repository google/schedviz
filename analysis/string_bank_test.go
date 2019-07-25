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
package sched

import (
	"strings"
	"testing"
)

func TestStringBank(t *testing.T) {
	strs := []string{"a", "b", "such a long string amaze wow", "ελληνικά"}
	sb := newStringBank()
	var strIDs = []stringID{}
	// Generate a known-absent index by summing all real indices and adding 1
	var absentID = stringID(1)
	for _, str := range strs {
		id := sb.stringIDByString(str)
		strIDs = append(strIDs, id)
		absentID += id
	}
	// Ensure each index maps to the expected string
	for i, str := range strs {
		fetchedStr, err := sb.stringByID(stringID(i))
		if err != nil {
			t.Fatalf("Expected index %d to return string %s, but got error %v", i, str, err)
		}
		if strings.Compare(fetchedStr, str) != 0 {
			t.Errorf("stringByID(%d) = %s, want %s", i, fetchedStr, str)
		}
	}
	// Ensure that the absent index is in fact absent.
	fetchedStr, err := sb.stringByID(absentID)
	if err == nil {
		t.Errorf("stringByID(%d) = %s, but should have been absent", absentID, fetchedStr)
	}
}
