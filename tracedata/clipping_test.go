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
package clipping

import (
	"testing"

	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

func createEvent(cpu int64, timestamp int64) *eventpb.Event {
	return &eventpb.Event{
		Cpu:         cpu,
		TimestampNs: timestamp,
	}
}

// Creates sample events to test clipping.
// If 2 is overflowed, then the first 3 or last 3
// should be clipped.
// Else if 1 is overflowed, then the first or last
// should be clipped.
// Else nothing should be clipped.
func sampleClippingData() *eventpb.EventSet {
	events := []*eventpb.Event{
		createEvent(3, 1000),
		createEvent(1, 1500),
		createEvent(1, 2000),
		createEvent(2, 3000),
		createEvent(1, 3500),
		createEvent(2, 4000),
		createEvent(1, 4500),
		createEvent(2, 5000),
		createEvent(1, 5500),
		createEvent(1, 6000),
		createEvent(3, 7000),
	}

	set := eventpb.EventSet{
		Event: events,
	}
	return &set
}

func TestStartClipping(t *testing.T) {
	es := sampleClippingData()
	ClipFromStartOfTrace(es, map[int64]struct{}{1: {}, 2: {}})
	for ix, ee := range es.Event {
		if (ix <= 2) != ee.Clipped {
			t.Error("Incorrect start clipping, only the early events should be clipped.")
		}
	}

	es = sampleClippingData()
	ClipFromStartOfTrace(es, map[int64]struct{}{1: {}, 3: {}})
	for ix, ee := range es.Event {
		if (ix == 0) != ee.Clipped {
			t.Error("Incorrect start clipping, only the early events should be clipped.")
		}
	}

	es = sampleClippingData()
	ClipFromStartOfTrace(es, map[int64]struct{}{3: {}})
	for _, ee := range es.Event {
		if ee.Clipped {
			t.Error("No events should be clipped for just CPU three.")
		}
	}
}

func TestEndClipping(t *testing.T) {
	es := sampleClippingData()
	ClipFromEndOfTrace(es, map[int64]struct{}{1: {}, 2: {}})
	for ix, ee := range es.Event {
		if (ix >= len(es.Event)-3) != ee.Clipped {
			t.Error("Incorrect start clipping, only the early events should be clipped.")
		}
	}

	es = sampleClippingData()
	ClipFromEndOfTrace(es, map[int64]struct{}{1: {}, 3: {}})
	for ix, ee := range es.Event {
		if (ix == len(es.Event)-1) != ee.Clipped {
			t.Error("Incorrect start clipping, only the early events should be clipped.")
		}
	}

	es = sampleClippingData()
	ClipFromEndOfTrace(es, map[int64]struct{}{3: {}})
	for _, ee := range es.Event {
		if ee.Clipped {
			t.Error("No events should be clipped for just CPU three.")
		}
	}
}

func TestNoEventsFromClippedCPU(t *testing.T) {
	es := sampleClippingData()
	ClipFromEndOfTrace(es, map[int64]struct{}{})
	for _, ee := range es.Event {
		if ee.Clipped == true {
			t.Error("No Events should be clipped.")
		}
	}

	es = sampleClippingData()
	ClipFromStartOfTrace(es, map[int64]struct{}{})
	for _, ee := range es.Event {
		if ee.Clipped == true {
			t.Error("No Events should be clipped.")
		}
	}

	es = sampleClippingData()
	err := ClipFromStartOfTrace(es, map[int64]struct{}{5: {}})
	if err == nil {
		t.Error("Expected error when overflowed CPU has no events.")
	}

	es = sampleClippingData()
	err = ClipFromEndOfTrace(es, map[int64]struct{}{5: {}})
	if err == nil {
		t.Error("Expected error when overflowed CPU has no events.")
	}
}
