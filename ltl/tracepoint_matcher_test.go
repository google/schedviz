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
	"reflect"
	"sort"
	"testing"

	"github.com/ilhamster/ltl/pkg/binder"
	be "github.com/ilhamster/ltl/pkg/bindingenvironment"
	"github.com/ilhamster/ltl/pkg/ltl"
	ops "github.com/ilhamster/ltl/pkg/operators"
	esb "github.com/google/schedviz/tracedata/eventsetbuilder"
	"github.com/google/schedviz/tracedata/trace"
)

// tracepointMatcherGenerator returns a function that builds TracepointMatchers for
// tests, bootstrapped with the appropriate trace.Collection and testing helper.
func tracepointMatcherGenerator(collection *trace.Collection, t *testing.T) func(s string) ltl.Operator {
	t.Helper()
	return func(s string) ltl.Operator {
		tm, err := Generator(collection)(s)
		if err != nil {
			t.Fatalf("got unparseable matcher %q in test case not testing parsing, but wanted parse success", err)
		}
		return tm
	}
}

// tracepointIntervalGenerator returns a function that, given a test case number,
// extracts the corresponding events from the larger collection and returns them
// as a list of LTL tokens. A test case number n corresponds to all events in the
// range [n*100, n*100+100).
func tracepointIntervalGenerator(collection *trace.Collection, t *testing.T) func(i int) []ltl.Token {
	t.Helper()
	return func(testCaseNumber int) []ltl.Token {
		startTimestamp := trace.Timestamp(testCaseNumber * 100)
		endTimestamp := trace.Timestamp(testCaseNumber*100 + 99)
		startIdx := sort.Search(collection.EventCount(), func(idx int) bool {
			ev, err := collection.EventByIndex(idx)
			if err != nil {
				return false
			}
			return ev.Timestamp >= startTimestamp
		})
		tokens := []ltl.Token{}
		for i := startIdx; i < collection.EventCount(); i++ {
			ev, err := collection.EventByIndex(i)
			if err != nil {
				t.Fatalf("got unexpected error %q retrieving event at index %d, but wanted no error", err, i)
			}
			if ev.Timestamp <= endTimestamp {
				tokens = append(tokens, TracepointToken(ev.Index))
			} else {
				break
			}
		}
		if len(tokens) == 0 {
			t.Log("Warning: returned no events while retrieving from collection. (Are your test event timestamps correct?)")
		}
		return tokens
	}
}

func TestTracepointMatcher(t *testing.T) {
	type testInput struct {
		// input is a sequence of LTL tokens (in our case, TracepointTokens).
		input []ltl.Token
		// wantMatch is whether the query is expected to match on the input.
		wantMatch bool
		// wantCaptureTimestamps are the timestamps of input tokens expected
		// to be involved in the match. Nil if no match/captures are expected.
		wantCaptureTimestamps []int
	}

	type testCase struct {
		// query is an LTL operator describing a pattern of tokens.
		query  ltl.Operator
		inputs []testInput
	}

	wantMatchWithCaptures := func(tokens []ltl.Token, wantCaptureTimestamps []int) testInput {
		return testInput{tokens, true, wantCaptureTimestamps}
	}
	wantNoMatch := func(tokens []ltl.Token) testInput {
		return testInput{tokens, false, nil}
	}

	// Build a set of tracepoint events. Each individual test case within this test
	// operates over its own temporal slice of the set, with the first test case
	// operating on all events between 0 and 99, the second operating on all events
	// between 100 and 199, and so forth.
	allEvents := esb.NewBuilder().
		WithEventDescriptor("sys:sysenter").
		WithEventDescriptor("sys:sysexit").
		WithEventDescriptor("sys:something").
		WithEventDescriptor("5").
		WithEventDescriptor("sched:sched_switch", esb.Text("foo"), esb.Number("baz")).
		WithEventDescriptor("sched:sched_wakeup_new", esb.Text("foo")).
		WithEventDescriptor("sched:sched_migrate_task").
		WithEventDescriptor("sys:anything", esb.Text("foo"), esb.Text("bar")).
		WithEventDescriptor("sys:start", esb.Text("foo"), esb.Text("bar")).
		WithEventDescriptor("sys:branch", esb.Number("foo")).
		// Testcase 0
		WithEvent("sys:sysenter", 6, 0, false).
		WithEvent("sys:sysexit", 7, 1, false).
		// Testcase 1
		WithEvent("sys:sysenter", 6, 100, false).
		WithEvent("sys:sysexit", 257, 101, false).
		// Testcase 2
		WithEvent("sys:sysexit", 7, 200, false).
		WithEvent("sys:sysenter", 6, 201, false).
		// Testcase 3
		WithEvent("sys:sysenter", 6, 300, false).
		// Testcase 4
		WithEvent("sys:sysenter", 6, 400, false).
		WithEvent("sys:something", 6, 401, false).
		WithEvent("sys:sysexit", 6, 402, false).
		// Testcase 5
		WithEvent("sys:sysenter", 6, 500, false).
		WithEvent("sys:something", 6, 501, false).
		WithEvent("sys:sysexit", 7, 502, false).
		// Testcase 6
		WithEvent("sys:sysenter", 6, 600, false).
		WithEvent("sys:something", 6, 601, false).
		// Testcase 7
		WithEvent("sched:sched_switch", 6, 700, false, "bar", 36).
		WithEvent("sys:something", 6, 701, false).
		// Testcase 8
		WithEvent("sched:sched_switch", 6, 800, false, "bar", 36).
		WithEvent("sys:sysexit", 6, 801, false).
		// Testcase 9
		WithEvent("sched:sched_switch", 6, 900, false, "bar", 36).
		WithEvent("sys:sysenter", 6, 901, false).
		// Testcase 10
		WithEvent("sched:sched_switch", 6, 1000, false, "notbar", 36).
		WithEvent("sys:something", 6, 1001, false).
		// Testcase 11
		WithEvent("sched:sched_wakeup_new", 13, 1100, false, "bar").
		WithEvent("sched:sched_migrate_task", 13, 1101, false).
		// Testcase 12
		WithEvent("sched:sched_wakeup_new", 13, 1200, false, "bar").
		WithEvent("sched:sched_switch", 13, 1201, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1202, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1203, false, "", 0).
		WithEvent("sched:sched_migrate_task", 13, 1204, false).
		// Testcase 13
		WithEvent("sched:sched_wakeup_new", 13, 1300, false, "bar").
		WithEvent("sched:sched_switch", 13, 1301, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1302, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1303, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1304, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1305, false, "", 0).
		WithEvent("sched:sched_migrate_task", 13, 1306, false).
		// Testcase 14
		WithEvent("sched:sched_wakeup_new", 13, 1400, false, "bar").
		WithEvent("sched:sched_switch", 13, 1401, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1402, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1403, false, "", 0).
		// Testcase 15
		WithEvent("sched:sched_wakeup_new", 13, 1500, false, "bar").
		WithEvent("sched:sched_switch", 13, 1501, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1502, false, "", 0).
		WithEvent("sched:sched_switch", 13, 1503, false, "", 0).
		WithEvent("sched:sched_migrate_task", 14, 1504, false).
		// Testcase 16
		WithEvent("sys:sysenter", 6, 1600, false).
		WithEvent("sys:something", 6, 1601, false).
		// Testcase 17
		WithEvent("sys:sysenter", 6, 1700, false).
		WithEvent("sys:something", 6, 1701, false).
		// Testcase 18
		WithEvent("sys:something", 8, 1800, false).
		WithEvent("sys:something", 7, 1801, false).
		WithEvent("sys:something", 8, 1802, false).
		// Testcase 19
		WithEvent("sys:something", 3, 1900, false).
		WithEvent("sys:something", 7, 1901, false).
		WithEvent("sys:something", 3, 1902, false).
		// Testcase 20
		WithEvent("sys:something", 2, 2000, false).
		WithEvent("sys:something", 7, 2001, false).
		WithEvent("sys:something", 3, 2002, false).
		// Testcase 21
		WithEvent("sys:something", 3, 2100, false).
		WithEvent("sys:something", 1, 2101, false).
		WithEvent("sys:something", 3, 2102, false).
		// Testcase 22
		WithEvent("sys:anything", 6, 2200, false, "bar", "zal").
		WithEvent("sys:anything", 23, 2201, false, "baz", "zal").
		WithEvent("sys:anything", 5, 2202, false, "notbar", "bar").
		// Testcase 23
		WithEvent("sys:anything", 6, 2300, false, "bar", "zal").
		WithEvent("sys:anything", 23, 2301, false, "baz", "zal").
		WithEvent("sys:anything", 23, 2302, false, "baz", "zal").
		WithEvent("sys:anything", 23, 2303, false, "baz", "zal").
		WithEvent("sys:anything", 5, 2304, false, "notbar", "bar").
		// Testcase 24
		WithEvent("sys:start", 6, 2400, false, "bar", "zal").
		WithEvent("sys:anything", 23, 2401, false, "baz", "zal").
		WithEvent("sys:start", 23, 2402, false, "baz", "zal").
		WithEvent("sys:start", 23, 2403, false, "baz", "zal").
		WithEvent("sys:start", 5, 2404, false, "notbar", "bar").
		// Testcase 25
		WithEvent("sys:anything", 6, 2500, false, "bar", "zal").
		WithEvent("sys:anything", 23, 2501, false, "baz", "zal").
		WithEvent("sys:anything", 5, 2502, false, "notbar", "alsonotbar").
		// Testcase 26
		WithEvent("sys:start", 6, 2600, false, "bar", "zal").
		WithEvent("sys:sysenter", 23, 2601, false).
		WithEvent("sys:start", 23, 2602, false, "baz", "zal").
		WithEvent("sys:start", 23, 2603, false, "baz", "zal").
		WithEvent("sys:start", 5, 2604, false, "notbar", "bar").
		// Testcase 27
		WithEvent("sys:sysenter", 6, 2700, false).
		WithEvent("sys:sysenter", 23, 2701, false).
		// Testcase 28
		WithEvent("sys:sysenter", 6, 2800, false).
		WithEvent("sys:sysexit", 23, 2801, false).
		// Testcase 29
		WithEvent("sys:branch", 2901, 2900, false, 3).
		WithEvent("sys:branch", 2900, 2901, false, 3).
		// Testcase 30
		WithEvent("sys:branch", 3000, 3000, false, 3).
		WithEvent("sys:branch", 3000, 3001, false, 3).
		// Testcase 31
		WithEvent("sched:sched_wakeup_new", 5, 3100, false, "baz").
		WithEvent("sched:sched_wakeup_new", 5, 3101, false, "bar").
		// Testcase 32
		WithEvent("sched:sched_wakeup_new", 5, 3200, false, "baz").
		// Testcase 33
		WithEvent("sys:sysenter", 5, 3300, false).
		WithEvent("5", 5, 3301, true)

	// Transform events to usable type.
	eventSet, errs := allEvents.EventSet()
	if errs != nil && len(errs) > 0 {
		t.Fatalf("got unexpected errors '%q' while creating event set , but wanted no errors ", errs)
	}
	collection, err := trace.NewCollection(eventSet)
	if err != nil {
		t.Fatalf("got unexpected error %q in parsing eventSet to collection, but wanted no error", err)
	}

	tm := tracepointMatcherGenerator(collection, t)
	tokensForTestCaseNumber := tracepointIntervalGenerator(collection, t)

	tests := []testCase{
		{
			// Match series of tracepoint events where the first event has the name `sys:syseenter`
			// and is immediately followed by an event with the name `sys:sysexit`.
			query: ops.Then(tm("event.name=sys:sysenter"), tm("event.name=sys:sysexit")),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(0), []int{0, 1}),
				wantMatchWithCaptures(tokensForTestCaseNumber(1), []int{100, 101}),
				wantNoMatch(tokensForTestCaseNumber(2)),
				wantNoMatch(tokensForTestCaseNumber(3)),
			},
		},
		{
			// Match series of tracepoint events where the first event was emitted by cpu 6 and
			// has the name `sys:sysenter` and is eventually followed by an event emitted by
			// cpu 6 with the name `sys:sysexit`.
			query: ops.Then(ops.And(tm("event.name=sys:sysenter"), tm("event.cpu=6")),
				ops.Eventually(ops.And(tm("event.name=sys:sysexit"), tm("event.cpu=6")))),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(4), []int{400, 402}),
				wantNoMatch(tokensForTestCaseNumber(5)),
				wantNoMatch(tokensForTestCaseNumber(6)),
			},
		},
		{
			// Match series of tracepoint events where the first event has the name `sched:sched_switch`,
			// the text_property "foo":"bar", and the number_property[baz]=36. The event that immediately
			// follows such an event must not have the name `sys:sysenter`.
			query: ops.Then(ops.And(ops.And(tm("event.name=sched:sched_switch"),
				tm("event.text_properties[foo]=bar")), tm("event.number_properties[baz]=36")),
				ops.Not(tm("event.name=sys:sysenter"))),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(7), []int{700, 701}),
				wantMatchWithCaptures(tokensForTestCaseNumber(8), []int{800, 801}),
				wantNoMatch(tokensForTestCaseNumber(9)),
				wantNoMatch(tokensForTestCaseNumber(10)),
			},
		},
		{
			// Match series of tracepoint events where the first event is emitted by cpu 13, has the name
			// `sched:sched_wakeup`, and text_property "foo":"bar". Then eventually cpu 13 emits
			// a tracepoint event within 5 events without the name `sched:sched_switch`.
			query: ops.Then(ops.And(ops.And(tm("event.name=sched:sched_wakeup_new"),
				tm("event.text_properties[foo]=bar")), tm("event.cpu=13")), ops.Limit(5,
				ops.Eventually(ops.And(ops.Not(tm("event.name=sched:sched_switch")), tm("event.cpu=13"))))),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(11), []int{1100, 1101}),
				wantMatchWithCaptures(tokensForTestCaseNumber(12), []int{1200, 1204}),
				wantNoMatch(tokensForTestCaseNumber(13)),
				wantNoMatch(tokensForTestCaseNumber(14)),
				wantNoMatch(tokensForTestCaseNumber(15)),
			},
		},
		{
			// Match series of tracepoint events where the first event happened at timestamp 1600
			// and is immediately followed by an event emitted at timestamp 1601.
			query: ops.Then(tm("event.timestamp=1600"), tm("event.timestamp=1601")),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(16), []int{1600, 1601}),
				wantNoMatch(tokensForTestCaseNumber(17)),
			},
		},
		{
			// Match series of tracepoint events where an event happening on cpu 7 is immediately preceded
			// and followed by an event that happened on the same cpu (e.g. both on cpu 9 or both on cpu 28).
			query: ops.Then(tm("$cpu<-event.cpu"), ops.Then(tm("event.cpu=7"), tm("event.cpu=$cpu"))),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(18), []int{1800, 1801, 1802}),
				wantMatchWithCaptures(tokensForTestCaseNumber(19), []int{1900, 1901, 1902}),
				wantNoMatch(tokensForTestCaseNumber(20)),
				wantNoMatch(tokensForTestCaseNumber(21)),
			},
		},
		{
			// Match series of tracepoint events where an event has the text_property "foo":someSpecificVal, is
			// followed immediately by an event with the name "sys:anything", and is then eventually
			// followed by an event with the property "bar":someSpecificVal (the same one from the earlier "foo")
			query: ops.Then(tm("$fooVal<-event.text_properties[foo]"), ops.Then(tm("event.name=sys:anything"),
				ops.Eventually(tm("event.text_properties[bar]=$fooVal")))),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(22), []int{2200, 2201, 2202}),
				wantMatchWithCaptures(tokensForTestCaseNumber(23), []int{2300, 2301, 2304}),
				wantMatchWithCaptures(tokensForTestCaseNumber(24), []int{2400, 2401, 2404}),
				wantNoMatch(tokensForTestCaseNumber(25)),
				wantNoMatch(tokensForTestCaseNumber(26)),
			},
		},
		{
			// Match series of tracepoint events where two events with the same name
			// appear one after another.
			query: ops.Then(tm("$name<-event.name"), tm("event.name=$name")),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(27), []int{2700, 2701}),
				wantNoMatch(tokensForTestCaseNumber(28)),
			},
		},
		{
			// Match series of tracepoint events where (oddly enough), the cpu matches the timestamp of the
			// event that comes after it, and the timestamp matches the cpu that comes after it. Additionally,
			// the two events have the same number_properties[foo] value.
			query: ops.Then(ops.And(tm("$cpu<-event.cpu"), ops.And(tm("$time<-event.timestamp"),
				tm("$foo<-event.number_properties[foo]"))), ops.And(tm("event.cpu=$time"),
				ops.And(tm("event.timestamp=$cpu"), tm("event.number_properties[foo]=$foo")))),
			inputs: []testInput{
				wantMatchWithCaptures(tokensForTestCaseNumber(29), []int{2900, 2901}),
				wantNoMatch(tokensForTestCaseNumber(30)),
			},
		},
		{
			// Query that will not match, because it references a binding that is never resolved.
			query: ops.Then(tm("event.name=sys:something"), tm("event.text_properties[foo]=$bar")),
			inputs: []testInput{
				wantNoMatch(tokensForTestCaseNumber(31)),
			},
		},
		{
			// Query that will not match, because it registers a binding that is never referenced.
			query: ops.Then(tm("event.name=sys:something"), tm("$bar<-event.text_properties[foo]")),
			inputs: []testInput{
				wantNoMatch(tokensForTestCaseNumber(32)),
			},
		},
		{
			// Query that will not match, because it mixes binding types - an int is not comparable to a string.
			query: ops.Then(tm("$cpuNum<-event.cpu"), tm("event.name=$cpuNum")),
			inputs: []testInput{
				wantNoMatch(tokensForTestCaseNumber(33)),
			},
		},
	}

	testCounter := -1
	for _, testCase := range tests {
		for _, testInput := range testCase.inputs {
			testCounter++
			testTitle := fmt.Sprintf("Test#%d:%v; shouldMatch:%v, shouldCapture:%v", testCounter,
				ops.PrettyPrint(testCase.query, ops.Inline()), testInput.wantMatch, testInput.wantCaptureTimestamps)
			t.Run(testTitle, func(t *testing.T) {
				op := testCase.query
				var env ltl.Environment
				for index, tok := range testInput.input {
					if op == nil && testInput.wantMatch {
						t.Fatalf("got op = nil, but wanted match")
					}
					op, env = ltl.Match(op, tok)
					if env.Err() != nil {
						t.Fatalf("got unexpected error %s at index %d, but wanted no error", env.Err(), index)
					}
				}
				if testInput.wantMatch != env.Matching() {
					t.Fatalf("got match state %t, but wanted match state %t", env.Matching(), testInput.wantMatch)
				}
				if testInput.wantCaptureTimestamps != nil {
					capturesMap := be.Captures(env).Get(true)
					gotCaptureTimestamps := []int{}
					for capture := range capturesMap {
						ttok, ok := capture.(TracepointToken)
						if !ok {
							t.Fatalf("got unexpected token capture of type %T, but wanted type TracepointToken", capture)
						}
						ev, err := collection.EventByIndex(int(ttok))
						if err != nil {
							t.Fatalf("got unexpected err %q while looking up TracepointToken in collection, but wanted no error", err)
						}
						gotCaptureTimestamps = append(gotCaptureTimestamps, int(ev.Timestamp))
					}
					sort.Ints(gotCaptureTimestamps)
					if !reflect.DeepEqual(gotCaptureTimestamps, testInput.wantCaptureTimestamps) {
						t.Fatalf("got captures %v but wanted %v", gotCaptureTimestamps, testInput.wantCaptureTimestamps)
					}
				}
			})
		}
	}
}

// A fakeToken that satisfies the ltl.Operator interface.
type fakeToken string

func (ft fakeToken) EOI() bool {
	return false
}

func (ft fakeToken) String() string {
	return string(ft)
}

func fakeTokensToLtlTokens(fts []fakeToken) []ltl.Token {
	toks := []ltl.Token{}
	for _, ft := range fts {
		toks = append(toks, ft)
	}
	return toks
}

// errorTestInput is a test that should error. It relaxes the way inputs are
// passed to tests so that we can intentionally misuse TracepointMatchers
// and ensure edge case errors are being generated correctly.
type errorTestInput struct {
	trainToks []ltl.Token
	inputToks []ltl.Token
}

func createTracepointTokens(b esb.Builder, t *testing.T) []ltl.Token {
	t.Helper()
	eventSet, errs := b.EventSet()
	if errs != nil && len(errs) > 0 {
		t.Fatalf("got unexpected eventset errors %q, but wanted no errors ", errs)
	}
	eventCollection, err := trace.NewCollection(eventSet)
	if err != nil {
		t.Fatalf("got unexpected error %q in parsing eventSet to eventCollection, but wanted no error", err)
	}
	var toks []ltl.Token
	for i := 0; i < eventCollection.EventCount(); i++ {
		event, err := eventCollection.EventByIndex(i)
		if err != nil {
			t.Fatalf("got unexpected error %q in accessing event in collection, but wanted no error", err)
		}
		toks = append(toks, newToken(*event))
	}
	return toks
}

func TestTracepointMatcherErrors(t *testing.T) {
	type testCase struct {
		testInput errorTestInput
		ops       []ltl.Operator
	}

	// We should not include all test events in this set as the point of
	// these tests is to "train" the TracepointMatcher on one set of events,
	// but then present it with a different set of events to match on.
	trainEvents := *esb.NewBuilder().
		WithEventDescriptor("sys:enter").
		WithEvent("sys:enter", 13, 1, false)

	eventSet, errs := trainEvents.EventSet()
	if errs != nil && len(errs) > 0 {
		t.Fatalf("got unexpected eventset errors %q, but wanted no errors ", errs)
	}
	trainCollection, err := trace.NewCollection(eventSet)
	if err != nil {
		t.Fatalf("got unexpected error %q in parsing eventSet to eventCollection, but wanted no error", err)
	}

	tm := tracepointMatcherGenerator(trainCollection, t)

	var trainToks []ltl.Token
	for i := 0; i < trainCollection.EventCount(); i++ {
		event, err := trainCollection.EventByIndex(i)
		if err != nil {
			t.Fatalf("got unexpected error %q in accessing event in collection, but wanted no error", err)
		}
		trainToks = append(trainToks, newToken(*event))
	}

	tests := []testCase{
		{
			// Generate matchers with one token set, and then try to do a literal match on a
			// token set of a different token type (fakeToken instead of TracepointToken).
			testInput: errorTestInput{trainToks, fakeTokensToLtlTokens([]fakeToken{"some", "incorrect", "tokens"})},
			ops:       []ltl.Operator{tm("event.name=sys:enter")},
		},
		{
			// Run query on more input tokens than provided when generating matchers,
			// where the match is found in the events beyond those initially provided.
			testInput: errorTestInput{trainToks,
				createTracepointTokens(*esb.NewBuilder().
					WithEventDescriptor("sys:enter").
					WithEventDescriptor("sys:exit", esb.Text("foo"), esb.Number("foo")).
					WithEvent("sys:enter", 13, 1, false).
					WithEvent("sys:exit", 17, 2, true, "bar", 3), t)},
			ops: []ltl.Operator{
				ops.Eventually(tm("event.cpu=17")),
				ops.Eventually(tm("event.timestamp=5")),
				ops.Eventually(tm("event.name=sys.exit")),
				ops.Eventually(tm("event.number_properties[foo]=3")),
				ops.Eventually(tm("event.text_properties[foo]=bar")),
			},
		},
		{
			// Assign bind from an attribute that does not exist on the "run-time"
			// (i.e. input token) type.
			testInput: errorTestInput{trainToks, fakeTokensToLtlTokens([]fakeToken{"some", "incorrect", "tokens"})},
			ops: []ltl.Operator{
				tm("$fooVal<-event.number_properties[foo]"),
				tm("$fooVal<-event.text_properties[foo]"),
				tm("$cpu<-event.cpu"),
				tm("$ts<-event.timestamp"),
				tm("$name<-event.name"),
			},
		},
		{
			// Assign bind from an attribute selector that does not exist on the input token.
			testInput: errorTestInput{trainToks,
				createTracepointTokens(*esb.NewBuilder().
					WithEventDescriptor("sys:enter").
					WithEvent("sys:enter", 13, 1, false).
					WithEvent("sys:enter", 85, 2, false), t)},
			ops: []ltl.Operator{
				tm("$fooVal<-event.number_properties[bar]"),
				tm("$fooVal<-event.text_properties[bar]"),
			},
		},
		{
			// Run query on more input tokens than provided when generating matchers,
			// where the match is found in the events beyond those initially provided.
			testInput: errorTestInput{trainToks,
				createTracepointTokens(*esb.NewBuilder().
					WithEventDescriptor("sys:enter").
					WithEventDescriptor("sys:exit", esb.Text("foo"), esb.Number("foo")).
					WithEvent("sys:enter", 13, 1, false).
					WithEvent("sys:exit", 17, 2, true, "bar", 3), t)},
			ops: []ltl.Operator{
				ops.Eventually(tm("event.cpu=17")),
				ops.Eventually(tm("event.timestamp=5")),
				ops.Eventually(tm("event.name=sys.exit")),
				ops.Eventually(tm("event.number_properties[foo]=3")),
				ops.Eventually(tm("event.text_properties[foo]=bar")),
			},
		},
		{
			// Run bindings query on more input tokens than provided while generating matchers,
			// where the match is found in the events beyond those initially provided.
			testInput: errorTestInput{trainToks,
				createTracepointTokens(*esb.NewBuilder().
					WithEventDescriptor("sys:enter").
					WithEventDescriptor("sys:exit", esb.Text("foo"), esb.Number("foo")).
					WithEvent("sys:enter", 13, 1, false).
					WithEvent("sys:exit", 17, 2, false, "bar", 3).
					WithEvent("sys:exit", 17, 2, false, "bar", 3).
					WithEvent("sys:enter", 13, 3, false), t)},
			ops: []ltl.Operator{
				ops.Then(tm("$name<-event.name"), tm("event.name=$name")),
				ops.Then(tm("$cpu<-event.cpu"), tm("event.cpu=$cpu")),
				ops.Then(tm("$ts<-event.timestamp"), tm("event.timestamp=$ts")),
				ops.Then(tm("$foo<-event.text_properties[foo]"), tm("event.text_properties[foo]=$foo")),
				ops.Then(tm("$foo<-event.number_properties[foo]"), tm("event.number_properties[foo]=$foo")),
			},
		},
		{
			// Pass the same set of tokens for both training and running the query,
			// (which would normally run fine), but bind on clipped, which is not
			// currently supported as a binding attribute.
			testInput: errorTestInput{trainToks, trainToks},
			ops: []ltl.Operator{
				tm("$cl<-event.clipped"),
			},
		},
	}

	for testNum, test := range tests {
		testInput := test.testInput
		for _, testOp := range test.ops {
			testTitle := fmt.Sprintf("Test#%d: %v -> want error", testNum,
				ops.PrettyPrint(testOp, ops.Inline()))
			t.Run(testTitle, func(t *testing.T) {
				op := testOp
				gotError := false
				var env ltl.Environment
				for _, tok := range testInput.inputToks {
					op, env = ltl.Match(op, tok)
					if env.Err() != nil {
						gotError = true
						break
					}
				}
				if !gotError {
					t.Fatalf("got no error, but wanted error")
				}
			})
		}
	}
}

type parseTest struct {
	input    string
	wantErr  bool
	wantType string
	events   *trace.Collection
}

// pt builds a parseTest object.
func pt(input string, wantErr bool, wantType string, b *esb.Builder, t *testing.T) parseTest {
	t.Helper()
	if b == nil {
		// If no builder is passed, don't bother generating an event collection.
		return parseTest{input, wantErr, wantType, nil}
	}
	eventSet, errs := b.EventSet()
	if errs != nil && eventSet == nil {
		t.Fatalf("got unexpected eventset errors %q, but wanted no errors ", errs)
	}
	eventCollection, err := trace.NewCollection(eventSet)
	if err != nil {
		t.Fatalf("got unexpected error %q in parsing eventSet to eventCollection, but wanted no error", err)
	}
	return parseTest{
		input:    input,
		wantErr:  wantErr,
		wantType: wantType,
		events:   eventCollection,
	}
}

func TestNewMatcherFromString(t *testing.T) {
	tests := []parseTest{
		// Success cases
		pt("event.name=sched:sched_wakeup_new", false, "*TracepointMatcher",
			esb.NewBuilder().
				WithEventDescriptor("sched:sched_wakeup_new").
				WithEvent("sched:sched_wakeup_new", 3, 1, false), t,
		),
		pt("event.text_properties[foo]=bar", false, "*TracepointMatcher",
			esb.NewBuilder().
				WithEventDescriptor("sys:something", esb.Text("foo")).
				WithEvent("sys:something", 3, 1, false, "bar"), t,
		),
		pt("event.cpu=13", false, "*TracepointMatcher",
			esb.NewBuilder().
				WithEventDescriptor("sys:something").
				WithEvent("sys:something", 13, 1, false), t,
		),
		pt("event.number_properties[bar]=2589283", false, "*TracepointMatcher",
			esb.NewBuilder().
				WithEventDescriptor("sys:something", esb.Number("bar")).
				WithEvent("sys:something", 3, 1, false, 2589283), t,
		),
		pt("event.clipped=true", false, "*TracepointMatcher",
			esb.NewBuilder().
				WithEventDescriptor("sys:something").
				WithEvent("sys:something", 3, 1, true), t,
		),
		pt("event.clipped=1", false, "*TracepointMatcher",
			esb.NewBuilder().
				WithEventDescriptor("sys:something").
				WithEvent("sys:something", 3, 1, true), t,
		),
		pt("$a<-event.cpu", false, "*binder.Binder", nil, t),
		pt("$something<-event.timestamp", false, "*binder.Binder", nil, t),
		pt("$foo<-event.text_properties[foo]", false, "*binder.Binder", nil, t),
		pt("$bar<-event.number_properties[bar]", false, "*binder.Binder", nil, t),
		pt("$clip<-event.clipped", false, "*binder.Binder", nil, t),
		pt("$name<-event.name", false, "*binder.Binder", nil, t),
		pt("event.name=$name", false, "*binder.Referencer", nil, t),
		pt("event.timestamp=$ts", false, "*binder.Referencer", nil, t),
		pt("event.clipped=$c", false, "*binder.Referencer", nil, t),
		pt("event.cpu=$cpuNum", false, "*binder.Referencer", nil, t),
		pt("event.text_properties[foo]=$fooVal", false, "*binder.Referencer", nil, t),
		pt("event.number_properties[bar]=$barVal", false, "*binder.Referencer", nil, t),
		// Failure cases
		pt("event.timestamp=garbage", true, "", nil, t),
		pt("event.cpu=garbage", true, "", nil, t),
		pt("event.number_properties[foo]=notanumber", true, "", nil, t),
		pt("cpu=8", true, "", nil, t),
		pt("event.cpu==garbage", true, "", nil, t),
		pt("event.cpugarbage", true, "", nil, t),
		pt("event.fakeproperty=blah", true, "", nil, t),
		pt("event.number_properties[foo=8", true, "", nil, t),
		pt("event.number_propertiesfoo]=8", true, "", nil, t),
		pt("event.number_properties=8", true, "", nil, t),
		pt("event.text_properties[foo=bar", true, "", nil, t),
		pt("event.text_propertiesfoo]=bar", true, "", nil, t),
		pt("event.text_properties=bar", true, "", nil, t),
		pt("event.clipped=flase", true, "", nil, t),
		pt("event.timestamp=", true, "", nil, t),
		pt("=something", true, "", nil, t),
		pt("event.cpu=92233720368547758079223372036854775807", true, "", nil, t),
		pt("event.timestamp=92233720368547758079223372036854775807", true, "", nil, t),
		pt("$a<-event.fakeproperty", true, "", nil, t),
	}

	for testNum, test := range tests {
		testTitle := fmt.Sprintf("Test#%d: %v", testNum, test.input)
		t.Run(testTitle, func(t *testing.T) {
			operator, err := Generator(test.events)(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("got no error, but wanted error. (parsed operator to: %v)", operator)
				}
				return
			}
			switch test.wantType {
			case "*TracepointMatcher":
				tpMatcher, ok := operator.(*TracepointMatcher)
				if !ok {
					t.Fatalf("got type %T, but wanted operator of type %s", operator, test.wantType)
				}
				ev, err := test.events.EventByIndex(0)
				if err != nil {
					t.Fatalf("got error %q while looking up the event at index 0 of a collection, but wanted no error",
						err)
				}
				matching := tpMatcher.matching(ev)
				if err != nil {
					t.Fatalf("got error %q while matching matcher %v against token %v, but wanted no error",
						err, tpMatcher, TracepointToken(0))
				}
				if !matching {
					tok, _ := test.events.EventByIndex(0)
					t.Fatalf("got matcher '%v' did not match token '%v', but wanted them to match", tpMatcher, tok)
				}
			case "*binder.Binder":
				if _, ok := operator.(*binder.Binder); !ok {
					t.Fatalf("got type %T, but wanted operator of type %s", operator, test.wantType)
				}
			case "*binder.Referencer":
				if _, ok := operator.(*binder.Referencer); !ok {
					t.Fatalf("got type %T, but wanted operator of type %s", operator, test.wantType)
				}
			default:
				t.Fatalf(`invalid test input: got wantType %q, but wanted one of
				['*TracepointMatcher', '*binder.Binder', '*binder.Referencer']`, test.wantType)
			}
		})
	}
}
