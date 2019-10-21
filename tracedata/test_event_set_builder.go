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
// Package testeventsetbuilder provides helpers for streamlined use of
// eventsetbuilder.Builders in tests.
package testeventsetbuilder

import (
	"testing"

	"github.com/google/schedviz/tracedata/eventsetbuilder"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

// TestProtobuf accepts a testing.T and apopulated Builder, and returns the
// Builder's EventSet.  If the Builder has errors, the test is failed with
// an appropriate message.
func TestProtobuf(t *testing.T, b *eventsetbuilder.Builder) *eventpb.EventSet {
	es, errs := b.EventSet()
	if len(errs) > 0 {
		t.Error("Errors building EventSet:")
		for err := range errs {
			t.Errorf("  %s", err)
		}
		t.Fatalf("Bailing...")
	}
	return es
}
