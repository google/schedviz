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
// Package tracepointmatcher provides a terminal tracepoint-matching ltl.Operator.
// The Operator consumes `trace.Event` tokens until the tracepoint query is fully matched.
package tracepointmatcher

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	be "github.com/ilhamster/ltl/pkg/bindingenvironment"
	"github.com/ilhamster/ltl/pkg/ltl"
	"github.com/google/schedviz/tracedata/trace"
)

const (
	// Name is the string reference to a trace.Event's `Name` field.
	Name string = "name"
	// CPU is the string reference to a trace.Event's `CPU` field.
	CPU string = "cpu"
	// Timestamp is the string reference to a trace.Event's `Timestamp` field.
	Timestamp string = "timestamp"
	// Clipped is the string reference to a trace.Event's `Clipped` field.
	Clipped string = "clipped"
	// TextProperties is the string reference to a trace.Event's `TextProperties` field.
	TextProperties string = "text_properties"
	// NumberProperties is the string reference to a trace.Event's `NumberProperties` field.
	NumberProperties string = "number_properties"
)

var (
	// matchExprRe matches the general format of a matcher expression,
	// either attribute=value or bindingName<-attribute.
	matchExprRe = regexp.MustCompile(`^(?:(.+)=(.+))|(?:\$(\w+)<-(.+))$`)

	// fieldNamesRe matches the specific allowed attribute names and format.
	// Maps to the LHS of an attribute matcher and the RHS of a binding matcher.
	// Note that this could be expanded to include the expected character types
	// of the associated value, but we would still need to parse values to their
	// real types later and might as well handle the error there, especially considering
	// that this inclusion would make the regexp not reusable for binding matchers.
	fieldNamesRe = regexp.MustCompile(`^event\.(text_properties\[\w+\]|number_properties\[\w+\]|name|cpu|timestamp|clipped)$`)

	// extractFieldsRe provides captures to extract an attribute's name and
	// the optionally provided attribute selector.
	extractFieldsRe = regexp.MustCompile(`^event\.(\w+)(?:\[(\w+)\])?$`)
)

// TracepointToken wraps the index of a trace.Event in order to implement
// additional functions such as EOI() for the ltl.Operator interface.
type TracepointToken int

// EOI (End of Input) is always false for tracepoints.
// Planned for removal by LTL library, so will not be needed in future.
func (t TracepointToken) EOI() bool {
	return false
}

func (t TracepointToken) String() string {
	return strconv.Itoa(int(t))
}

// TracepointMatcher is a tracepoint-matching ltl.Operator.
type TracepointMatcher struct {
	// sourceInput is the original string input used to produce the matcher.
	sourceInput string
	// col is the collection of events where the matcher looks up the
	// tokens it receives to check if they are a match.
	col *trace.Collection
	// matching reports whether the current TracepointMatcher matches a
	// given trace.Event. The implementation is dependent on which field
	// of a trace.Event the matcher is targeting.
	matching func(ev *trace.Event) bool
}

func (tm TracepointMatcher) String() string {
	return fmt.Sprintf("[%s]", tm.sourceInput)
}

// Reducible returns true for all TracepointMatchers.
func (tm TracepointMatcher) Reducible() bool {
	return true
}

// newAttributeMatcher generates a TracepointMatcher based on a literal value it should match.
func newAttributeMatcher(col *trace.Collection, tpm *TracepointMatcher, lhs string, rhs string) (*TracepointMatcher, error) {
	attributeName, attributeSelector, attributeValue := "", "", rhs

	if !fieldNamesRe.MatchString(lhs) {
		return nil, fmt.Errorf("invalid attribute or format '%s'", lhs)
	}

	parsedAttributes := extractFieldsRe.FindStringSubmatch(lhs)
	attributeName, attributeSelector = parsedAttributes[1], parsedAttributes[2]

	switch attributeName {
	case TextProperties:
		tpm.matching = func(ev *trace.Event) bool {
			gotValue, ok := ev.TextProperties[attributeSelector]
			return ok && gotValue == attributeValue
		}
	case NumberProperties:
		expectedValueNum, err := strconv.ParseInt(attributeValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number for attribute '%s', got '%s'", attributeName, attributeValue)
		}
		tpm.matching = func(ev *trace.Event) bool {
			gotValue, ok := ev.NumberProperties[attributeSelector]
			return ok && gotValue == expectedValueNum
		}
	case Name:
		tpm.matching = func(ev *trace.Event) bool {
			return ev.Name == attributeValue
		}
	case CPU:
		expectedValueNum, err := strconv.ParseInt(attributeValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number for attribute '%s', got '%s'", attributeName, attributeValue)
		}
		tpm.matching = func(ev *trace.Event) bool {
			return ev.CPU == expectedValueNum
		}
	case Timestamp:
		expectedValueNum, err := strconv.ParseInt(attributeValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number for attribute '%s', got '%s'", attributeName, attributeValue)
		}
		tpm.matching = func(ev *trace.Event) bool {
			return ev.Timestamp == trace.Timestamp(expectedValueNum)
		}
	case Clipped:
		expectedValueBool, err := strconv.ParseBool(attributeValue)
		if err != nil {
			return nil, fmt.Errorf("expected boolean for attribute '%s', got '%s'", attributeName, attributeValue)
		}
		tpm.matching = func(ev *trace.Event) bool {
			return ev.Clipped == expectedValueBool
		}
	}

	return tpm, nil
}

// newMatcherFromString "parses" a string to a TracepointMatcher.
// Returns either a valid *TracepointMatcher, or an error if parsing fails.
func newMatcherFromString(col *trace.Collection, s string) (ltl.Operator, error) {
	if !matchExprRe.MatchString(s) {
		return nil, fmt.Errorf("expected format 'attribute=value', 'attribute[selector]=value', or 'name<-value', but got '%s'", s)
	}

	captures := matchExprRe.FindStringSubmatch(s)
	attributeLHS, attributeRHS := captures[1], captures[2]
	bindingLHS, bindingRHS := captures[3], captures[4]

	tpm := &TracepointMatcher{
		sourceInput: s,
		col:         col,
		matching:    nil, // = not matching by default
	}

	// A literal matcher like `event.name=sys:enter`
	if attributeLHS != "" && attributeRHS != "" && !strings.HasPrefix(attributeRHS, "$") {
		return newAttributeMatcher(col, tpm, attributeLHS, attributeRHS)
	}

	// TODO(mirrorkeydev): add support for bindings
	return nil, fmt.Errorf("TODO: add support for processing bindings like %s<-%s", bindingLHS, bindingRHS)
}

// newToken converts a trace.Event to a TracepointToken by
// extracting its index. This is done both to fulfill the
// ltl.Operator interface and to allow the TracepointToken
// to be hashable (requirement of the LTL bindings library).
func newToken(tp trace.Event) TracepointToken {
	return TracepointToken(tp.Index)
}

func (tm *TracepointMatcher) matchInternal(ttok TracepointToken) (ltl.Operator, ltl.Environment) {
	if tm == nil {
		return nil, be.New(be.Matching(false))
	}

	ev, err := tm.col.EventByIndex(int(ttok))
	if err != nil {
		return nil, ltl.ErrEnv(err)
	}

	matching := tm.matching(ev)
	opts := []be.Option{be.Matching(matching), be.Captured(ttok)}
	env := be.New(opts...)
	return nil, env
}

// Match performs an LTL match on the receiving TracepointMatcher.
func (tm *TracepointMatcher) Match(tok ltl.Token) (ltl.Operator, ltl.Environment) {
	ev, ok := tok.(TracepointToken)
	if !ok {
		return nil, ltl.ErrEnv(fmt.Errorf("got token of type %T but expected TracepointToken", tok))
	}
	return tm.matchInternal(ev)
}

// Generator returns a generator function producing tracepoint matchers.
// The returned function accepts a string and returns a matcher for that
// string (and possibly an error).
func Generator() func(col *trace.Collection, s string) (ltl.Operator, error) {
	return func(col *trace.Collection, s string) (ltl.Operator, error) {
		return newMatcherFromString(col, s)
	}
}
