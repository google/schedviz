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
// Package testhelpers contains helpers for tests
package testhelpers

import (
	"os"
	"path"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
)

// DiffProto compares the string representations of two protos
func DiffProto(t *testing.T, a, b proto.Message) (diff string, equal bool) {
	t.Helper()
	equal = proto.Equal(a, b)
	if !equal {
		diff = cmp.Diff(a, b)
	}
	return
}

// GetRunFilesPath returns the path of the runfiles directory when running under Bazel
func GetRunFilesPath() string {
	// Bazel stores test location in these environment variables
	runFiles := path.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"))
	return runFiles
}
