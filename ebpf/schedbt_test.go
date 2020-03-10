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
package schedbt

import (
	"bufio"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/golang/protobuf/proto"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
	"github.com/google/schedviz/tracedata/testeventsetbuilder"
)

func TestParsing(t *testing.T) {
	tests := []struct {
		description string
		input       string
		want        *eventpb.EventSet
	}{{
		"Valid trace",
		`Attaching 5 probes...
ST:0:beef
P:a:64:70:Thread 1
P:a:c8:70:Thread 2
S:a:0:64:0:c8
M:14:0:0:1:64
W:1e:0:1:64`,
		testeventsetbuilder.TestProtobuf(t, emptyEventSet().
			WithEvent("sched_switch", 0, 0xbeef+10, false,
				100, "Thread 1", 112, 0,
				200, "Thread 2", 112).
			WithEvent("sched_migrate_task", 0, 0xbeef+20, false,
				100, "Thread 1", 112,
				0, 1).
			WithEvent("sched_wakeup", 0, 0xbeef+30, false,
				100, "Thread 1", 112, 1)),
	}, {
		"Process name with colons",
		`Attaching 5 probes...
ST:0:beef
P:a:64:70:Thread:1
M:a:0:0:1:64`,
		testeventsetbuilder.TestProtobuf(t, emptyEventSet().
			WithEvent("sched_migrate_task", 0, 0xbeef+10, false,
				100, "Thread:1", 112,
				0, 1)),
	}}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			p := NewParser()
			r := strings.NewReader(test.input)
			bufr := bufio.NewReader(r)
			if err := p.Parse(bufr); err != nil {
				t.Fatalf("Parse() yielded unexpected error %v", err)
			}
			got, err := p.EventSet()
			if err != nil {
				t.Fatalf("EventSet() yielded unexpected error %v", err)
			}
			if d := cmp.Diff(test.want, got, cmp.Comparer(proto.Equal)); d != "" {
				t.Errorf("Parser produced %s, diff(want->got) %s", proto.MarshalTextString(got), d)
			}
		})
	}
}
