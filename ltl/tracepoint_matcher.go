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

	be "github.com/ilhamster/ltl/pkg/bindingenvironment"
	"github.com/ilhamster/ltl/pkg/ltl"
	"github.com/google/schedviz/tracedata/trace"
)

// matchExprRe matches the general format of a matcher expression,
// either attribute=value or bindingName<-attribute.
var matchExprRe = regexp.MustCompile(`^(?:(.+)=(.+))|(?:(.+)<-(.+))$`)

// fieldNamesRe matches the specific allowed attribute names and format.
// Maps to the LHS of an attribute matcher and the RHS of a binding matcher.
// Note that this could be expanded to include the expected character types
// of the associated value, but we would still need to parse values to their
// real types later and might as well handle the error there, especially considering
// that this inclusion would make the regexp not reusable for binding matchers.
var fieldNamesRe = regexp.MustCompile(`^event.(text_properties\[\w+\]|number_properties\[\w+\]|name|cpu|timestamp|clipped)$`)

// extractFieldsRe provides captures to extract an attribute's name and
// the optionally provided attribute selector.
var extractFieldsRe = regexp.MustCompile(`^event.(\w+)(?:\[(\w+)\])?$`)

// TracepointToken wraps a trace.Event in order to implement
// additional functions such as EOI() for ltl.Operator.
type TracepointToken trace.Event

// EOI (End of Input) is always false for tracepoints.
// Planned for removal by LTL library, so will not be needed in future.
func (t TracepointToken) EOI() bool {
	return false
}

func (t TracepointToken) String() string {
	te := trace.Event(t)
	return te.String()
}

// TracepointMatcher is a tracepoint-matching ltl.Operator.
type TracepointMatcher struct {
	sourceInput string
	matching    func(ev trace.Event) bool
}

func (tm TracepointMatcher) String() string {
	return fmt.Sprintf("[%s]", tm.sourceInput)
}

// Reducible returns true for all TracepointMatchers.
func (tm *TracepointMatcher) Reducible() bool {
	return true
}

func newAttributeMatcher(tpm *TracepointMatcher, lhs string, RHS string) (*TracepointMatcher, error) {
	attributeName, attributeSelector, attributeValue := "", "", RHS

	if !fieldNamesRe.MatchString(lhs) {
		return nil, fmt.Errorf("invalid attribute or format '%s'", lhs)
	}

	parsedAttributes := extractFieldsRe.FindStringSubmatch(lhs)
	attributeName, attributeSelector = parsedAttributes[1], parsedAttributes[2]

	switch attributeName {
	case "text_properties":
		tpm.matching = func(ev trace.Event) bool {
			gotValue, ok := ev.TextProperties[attributeSelector]
			return ok && gotValue == attributeValue
		}
	case "number_properties":
		expectedValueNum, err := strconv.ParseInt(attributeValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number for attribute '%s', got '%s'", attributeName, attributeValue)
		}
		tpm.matching = func(ev trace.Event) bool {
			gotValue, ok := ev.NumberProperties[attributeSelector]
			return ok && gotValue == expectedValueNum
		}
	case "name":
		tpm.matching = func(ev trace.Event) bool {
			return ev.Name == attributeValue
		}
	case "cpu":
		expectedValueNum, err := strconv.ParseInt(attributeValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number for attribute '%s', got '%s'", attributeName, attributeValue)
		}
		tpm.matching = func(ev trace.Event) bool {
			return ev.CPU == expectedValueNum
		}
	case "timestamp":
		expectedValueNum, err := strconv.ParseInt(attributeValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number for attribute '%s', got '%s'", attributeName, attributeValue)
		}
		tpm.matching = func(ev trace.Event) bool {
			return ev.Timestamp == trace.Timestamp(expectedValueNum)
		}
	case "clipped":
		expectedValueBool, err := strconv.ParseBool(attributeValue)
		if err != nil {
			return nil, fmt.Errorf("expected boolean for attribute '%s', got '%s'", attributeName, attributeValue)
		}
		tpm.matching = func(ev trace.Event) bool {
			return ev.Clipped == expectedValueBool
		}
	}

	return tpm, nil
}

func newBindingMatcher(tpm *TracepointMatcher, LHS string, RHS string) (*TracepointMatcher, error) {
	bindingName, bindingValue := LHS, RHS

	if !fieldNamesRe.MatchString(bindingValue) {
		return nil, fmt.Errorf("invalid binding value or format '%s'", bindingValue)
	}

	parsedAttributes := extractFieldsRe.FindStringSubmatch(bindingValue)
	bindingAttributeName, bindingAttributeSelector := parsedAttributes[1], parsedAttributes[2]

	// TODO(mirrorkeydev): add support for bindings
	return nil, fmt.Errorf("TODO: add support for processing bindings like %s<-%s[%s]",
		bindingName, bindingAttributeName, bindingAttributeSelector)
}

// newMatcherFromString "parses" a string to a TracepointMatcher.
// Returns either a valid *TracepointMatcher, or an error if parsing fails.
func newMatcherFromString(s string) (*TracepointMatcher, error) {
	if !matchExprRe.MatchString(s) {
		return nil, fmt.Errorf("expected format 'attribute=value', 'attribute[selector]=value', or 'name<-value', but got '%s'", s)
	}

	captures := matchExprRe.FindStringSubmatch(s)
	attributeLHS, attributeRHS := captures[1], captures[2]
	bindingLHS, bindingRHS := captures[3], captures[4]

	tpm := &TracepointMatcher{
		sourceInput: s,
		matching: func(ev trace.Event) bool {
			return false // Matches nothing by default
		},
	}

	if attributeLHS != "" && attributeRHS != "" {
		return newAttributeMatcher(tpm, attributeLHS, attributeRHS)
	}

	return newBindingMatcher(tpm, bindingLHS, bindingRHS)
}

// newToken wraps a trace.Event in a TracepointToken
// so that it fulfills the ltl.Operator interface.
func newToken(tp trace.Event) TracepointToken {
	return TracepointToken(tp)
}

func (tm *TracepointMatcher) matchInternal(ttok TracepointToken) (ltl.Operator, ltl.Environment) {
	if tm == nil {
		return nil, be.New(be.Matching(false))
	}
	return nil, be.New(be.Matching(tm.matching(trace.Event(ttok))))
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
// The returned function accepts a string and returns a
// matcher for that string (and possibly an error).
func Generator() func(s string) (ltl.Operator, error) {
	return func(s string) (ltl.Operator, error) {
		return newMatcherFromString(s)
	}
}
