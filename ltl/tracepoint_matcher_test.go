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
package tracepointmatcher

import (
	"fmt"
	"testing"

	"github.com/ilhamster/ltl/pkg/ltl"
	ops "github.com/ilhamster/ltl/pkg/operators"
	esb "github.com/google/schedviz/tracedata/eventsetbuilder"
	"github.com/google/schedviz/tracedata/trace"
)

type testInput struct {
	input     esb.Builder
	toks      []ltl.Token
	wantMatch bool
}

func tm(s string, t *testing.T) ltl.Operator {
	t.Helper()
	tm, err := newMatcherFromString(s)
	if err != nil {
		t.Fatalf("got unparseable matcher '%s' in test case not testing parsing, but wanted parse success", err)
	}
	return tm
}

func createTracepointTokens(b esb.Builder, t *testing.T) []ltl.Token {
	t.Helper()
	eventSet, errs := b.EventSet()
	if errs != nil && len(errs) > 0 {
		t.Fatalf("got unexpected eventset errors '%q', but wanted no errors ", errs)
	}
	eventCollection, err := trace.NewCollection(eventSet)
	if err != nil {
		t.Fatalf("got unexpected error '%s' in parsing eventSet to eventCollection, but wanted no error", err)
	}
	var toks []ltl.Token
	for i := 0; i < eventCollection.EventCount(); i++ {
		event, err := eventCollection.EventByIndex(i)
		if err != nil {
			t.Fatalf("got unexpected error '%s' in accessing event in collection, but wanted no error", err)
		}
		toks = append(toks, newToken(*event))
	}
	return toks
}

// A matching set of events
func m(b esb.Builder, t *testing.T) testInput {
	t.Helper()
	return testInput{b, createTracepointTokens(b, t), true}
}

// A non-matching set of events
func nm(b esb.Builder, t *testing.T) testInput {
	t.Helper()
	return testInput{b, createTracepointTokens(b, t), false}
}

func TestTracepointMatcher(t *testing.T) {
	type testCase struct {
		op         ltl.Operator
		testInputs []testInput
	}
	tc := func(op ltl.Operator, testInputs ...testInput) testCase {
		return testCase{
			op:         op,
			testInputs: testInputs,
		}
	}
	tests := []testCase{
		// Match series of tracepoint events where the first event has the name `sys:syseenter`
		// and is immediately followed by an event with the name `sys:sysexit`.
		tc(ops.Then(tm("event.name=sys:sysenter", t), tm("event.name=sys:sysexit", t)),
			m(*esb.NewBuilder().
				WithEventDescriptor("sys:sysenter").
				WithEventDescriptor("sys:sysexit").
				WithEvent("sys:sysenter", 6, 1, false).
				WithEvent("sys:sysexit", 7, 2, false),
				t),
			m(*esb.NewBuilder().
				WithEventDescriptor("sys:sysenter").
				WithEventDescriptor("sys:sysexit").
				WithEvent("sys:sysenter", 6, 1, false).
				WithEvent("sys:sysexit", 257, 2, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sys:sysenter").
				WithEventDescriptor("sys:sysexit").
				WithEvent("sys:sysexit", 7, 1, false).
				WithEvent("sys:sysenter", 6, 2, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sys:sysenter").
				WithEvent("sys:sysenter", 6, 1, false).
				WithEvent("sys:sysenter", 6, 2, false),
				t),
		),
		// Match series of tracepoint events where the first event was emitted by cpu 6 and
		// has the name `sched:sched_switch` and is eventually followed by an event emitted
		// by cpu 6 with the name `sys:sysexit`.
		tc(ops.Then(ops.And(tm("event.name=sched:sched_switch", t), tm("event.cpu=6", t)),
			ops.Eventually(ops.And(tm("event.name=sys:sysexit", t), tm("event.cpu=6", t)))),
			m(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sys:something").
				WithEventDescriptor("sys:sysexit").
				WithEvent("sched:sched_switch", 6, 1, false).
				WithEvent("sys:something", 6, 2, false).
				WithEvent("sys:sysexit", 6, 3, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sys:something").
				WithEventDescriptor("sys:sysexit").
				WithEvent("sched:sched_switch", 6, 1, false).
				WithEvent("sys:something", 6, 2, false).
				WithEvent("sys:sysexit", 7, 3, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sys:something").
				WithEvent("sched:sched_switch", 6, 1, false).
				WithEvent("sys:something", 6, 2, false),
				t),
		),
		// Match series of tracepoint events where the first event has the name `sched:sched_switch`,
		// the text_property "foo":"bar", and the number_property[baz]=36. The event that immediately
		// follows such an event must not have the name `sched:sched_wakeup`.
		tc(ops.Then(ops.And(
			ops.And(tm("event.name=sched:sched_switch", t), tm("event.text_properties[foo]=bar", t)),
			tm("event.number_properties[baz]=36", t)),
			ops.Not(tm("event.name=sched:sched_wakeup", t))),
			m(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch", esb.Text("foo"), esb.Number("baz")).
				WithEventDescriptor("sys:something").
				WithEvent("sched:sched_switch", 6, 1, false, "bar", 36).
				WithEvent("sys:something", 6, 2, false),
				t),
			m(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch", esb.Text("foo"), esb.Number("baz")).
				WithEventDescriptor("sys:garbage").
				WithEvent("sched:sched_switch", 6, 1, false, "bar", 36).
				WithEvent("sys:garbage", 6, 2, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch", esb.Text("foo"), esb.Number("baz")).
				WithEventDescriptor("sched:sched_wakeup").
				WithEvent("sched:sched_switch", 6, 1, false, "bar", 36).
				WithEvent("sched:sched_wakeup", 6, 2, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch", esb.Text("foo"), esb.Number("baz")).
				WithEventDescriptor("sys:something").
				WithEvent("sched:sched_switch", 6, 1, false, "notbar", 36).
				WithEvent("sys:something", 6, 2, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch", esb.Number("qux"), esb.Number("baz")).
				WithEventDescriptor("sys:something").
				WithEvent("sched:sched_switch", 6, 1, false, 89, 36).
				WithEvent("sys:something", 6, 2, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch", esb.Text("foo"), esb.Number("notbaz")).
				WithEventDescriptor("sys:something").
				WithEvent("sched:sched_switch", 6, 1, false, "bar", 36).
				WithEvent("sys:something", 6, 2, false),
				t),
		),
		// Match series of tracepoint events where the first event is emitted by cpu 13, has the name
		// `sched:sched_wakeup`, and text_property "foo":"bar". Then eventually cpu 13 emits
		// a tracepoint event within 5 events without the name `sched:sched_switch`.
		tc(ops.Then(ops.And(ops.And(tm("event.name=sched:sched_wakeup_new", t), tm("event.text_properties[foo]=bar", t)), tm("event.cpu=13", t)),
			ops.Limit(5, ops.Eventually(ops.And(ops.Not(tm("event.name=sched:sched_switch", t)), tm("event.cpu=13", t))))),
			m(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_wakeup_new", esb.Text("foo")).
				WithEventDescriptor("sched:sched_migrate_task").
				WithEvent("sched:sched_wakeup_new", 13, 1, false, "bar").
				WithEvent("sched:sched_migrate_task", 13, 2, false),
				t),
			m(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_wakeup_new", esb.Text("foo")).
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sched:sched_migrate_task").
				WithEvent("sched:sched_wakeup_new", 13, 1, false, "bar").
				WithEvent("sched:sched_switch", 13, 2, false).
				WithEvent("sched:sched_switch", 13, 3, false).
				WithEvent("sched:sched_switch", 13, 4, false).
				WithEvent("sched:sched_migrate_task", 13, 5, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_wakeup_new", esb.Text("foo")).
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sched:sched_migrate_task").
				WithEvent("sched:sched_wakeup_new", 13, 1, false, "bar").
				WithEvent("sched:sched_switch", 13, 2, false).
				WithEvent("sched:sched_switch", 13, 3, false).
				WithEvent("sched:sched_switch", 13, 4, false).
				WithEvent("sched:sched_switch", 13, 5, false).
				WithEvent("sched:sched_switch", 13, 6, false).
				WithEvent("sched:sched_migrate_task", 13, 7, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_wakeup_new", esb.Text("foo")).
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sched:sched_migrate_task").
				WithEvent("sched:sched_wakeup_new", 13, 1, false, "bar").
				WithEvent("sched:sched_switch", 13, 2, false).
				WithEvent("sched:sched_switch", 13, 3, false).
				WithEvent("sched:sched_switch", 13, 4, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_wakeup_new", esb.Text("foo")).
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sched:sched_migrate_task").
				WithEvent("sched:sched_wakeup_new", 13, 1, false, "bar").
				WithEvent("sched:sched_switch", 13, 2, false).
				WithEvent("sched:sched_switch", 13, 3, false).
				WithEvent("sched:sched_switch", 13, 4, false).
				WithEvent("sched:sched_migrate_task", 14, 7, false),
				t),
		),
		// Match series of tracepoint events where the first event happened at timestamp 1 and was
		// clipped, and is immediately followed by an event emitted at timestamp 2 that isn't clipped.
		tc(ops.Then(ops.And(tm("event.timestamp=1", t), tm("event.clipped=true", t)),
			ops.And(tm("event.timestamp=2", t), tm("event.clipped=false", t))),
			m(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sys:something").
				WithEvent("sched:sched_switch", 6, 1, true).
				WithEvent("sys:something", 6, 2, false),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sys:something").
				WithEvent("sched:sched_switch", 6, 1, true).
				WithEvent("sys:something", 6, 2, true),
				t),
			nm(*esb.NewBuilder().
				WithEventDescriptor("sched:sched_switch").
				WithEventDescriptor("sys:something").
				WithEvent("sched:sched_switch", 6, 1, true).
				WithEvent("sys:something", 6, 3, false),
				t),
		),
	}
	for testNum, test := range tests {
		for testInputNum, testInput := range test.testInputs {
			testTitle := fmt.Sprintf("Test#%d.%d: %v -> want match: %v", testNum, testInputNum,
				ops.PrettyPrint(test.op, ops.Inline()), testInput.wantMatch)
			t.Run(testTitle, func(t *testing.T) {
				op := test.op
				var env ltl.Environment
				for index, tok := range testInput.toks {
					if op == nil && testInput.wantMatch {
						t.Fatalf("got op = nil, but wanted match")
					}
					op, env = ltl.Match(op, tok)
					if env.Err() != nil {
						t.Fatalf("got unexpected error %s at index %d, but wanted no error", env.Err(), index)
					}
				}
				if testInput.wantMatch != env.Matching() {
					t.Fatalf("got %t, but wanted match state %t", env.Matching(), testInput.wantMatch)
				}
			})
		}
	}
}

type parseTest struct {
	input         string
	wantErr       bool
	matchingEvent *trace.Event
}

func pt(input string, wantErr bool, matchingEvent *trace.Event) parseTest {
	return parseTest{
		input:         input,
		wantErr:       wantErr,
		matchingEvent: matchingEvent,
	}
}

func TestNewMatcherFromString(t *testing.T) {
	tests := []parseTest{
		// Success cases
		pt("event.name=sched:sched_wakeup_new", false,
			&trace.Event{
				Name: "sched:sched_wakeup_new",
			}),
		pt("event.text_properties[foo]=bar", false,
			&trace.Event{
				TextProperties: map[string]string{"foo": "bar"},
			}),
		pt("event.cpu=13", false,
			&trace.Event{
				CPU: 13,
			}),
		pt("event.number_properties[bar]=2589283", false,
			&trace.Event{
				NumberProperties: map[string]int64{"bar": 2589283},
			}),
		pt("event.clipped=true", false,
			&trace.Event{
				Clipped: true,
			}),
		pt("event.clipped=1", false,
			&trace.Event{
				Clipped: true,
			}),
		// Failure cases
		pt("event.timestamp=garbage", true, nil),
		pt("event.cpu=garbage", true, nil),
		pt("event.number_properties[foo]=notanumber", true, nil),
		pt("cpu=8", true, nil),
		pt("event.cpu==garbage", true, nil),
		pt("event.cpugarbage", true, nil),
		pt("event.fakeproperty=blah", true, nil),
		pt("event.number_properties[foo=8", true, nil),
		pt("event.number_propertiesfoo]=8", true, nil),
		pt("event.number_properties=8", true, nil),
		pt("event.text_properties[foo=bar", true, nil),
		pt("event.text_propertiesfoo]=bar", true, nil),
		pt("event.text_properties=bar", true, nil),
		pt("event.clipped=flase", true, nil),
		pt("event.timestamp=", true, nil),
		pt("=something", true, nil),
		pt("event.cpu=92233720368547758079223372036854775807", true, nil),
		pt("event.timestamp=92233720368547758079223372036854775807", true, nil),
		pt("$a<-event.fakeproperty", true, nil),

		// Expected failure while bindings aren't implemented.
		// TODO(mirrorkeydev): remove once bindings are implemented
		pt("$a<-event.cpu", true, nil),
	}

	for testNum, test := range tests {
		testTitle := fmt.Sprintf("Test#%d: %v", testNum, test.input)
		t.Run(testTitle, func(t *testing.T) {
			matcher, err := Generator()(test.input)
			if !test.wantErr {
				if err != nil {
					t.Fatalf("got error %s, but wanted no error", err)
				}
				tpMatcher, ok := matcher.(*TracepointMatcher)
				if !ok {
					t.Fatalf("got type %T, but wanted matcher of type *TracepointMatcher", matcher)
				}
				if !tpMatcher.matching(*test.matchingEvent) {
					t.Fatalf("got matcher '%v' did not match token '%v', but wanted them to match", tpMatcher, *test.matchingEvent)
				}
			} else if err == nil {
				t.Fatalf("got no error, but wanted error. (parsed matcher to: %v)", matcher)
			}
		})
	}
}
