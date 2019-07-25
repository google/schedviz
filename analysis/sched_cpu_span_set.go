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
	"sort"

	"github.com/Workiva/go-datastructures/augmentedtree"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// cpuSpans stores per-CPU sets of running, sleeping, and waiting threadSpans.
type cpuSpans struct {
	runningSpans  []*threadSpan
	sleepingSpans []*threadSpan
	waitingSpans  []*threadSpan
}

func (cs *cpuSpans) addSpan(span *threadSpan) {
	switch span.state {
	case RunningState:
		cs.runningSpans = append(cs.runningSpans, span)
	case SleepingState:
		cs.sleepingSpans = append(cs.sleepingSpans, span)
	case WaitingState:
		cs.waitingSpans = append(cs.waitingSpans, span)
	}
}

func sortSpans(tss []*threadSpan) {
	sort.Slice(tss, func(a, b int) bool {
		return tss[a].less(tss[b])
	})
}

// sort sorts each group of spans in the receiver by increasing start
// timestamp.
func (cs *cpuSpans) sort() {
	sortSpans(cs.runningSpans)
	sortSpans(cs.waitingSpans)
	sortSpans(cs.sleepingSpans)
}

// finalize sorts the cpuSpans, then confirms that there are no
// anomalies in it.  Any anomalies result in returned errors.
func (cs *cpuSpans) finalize() error {
	cs.sort()
	// Ensure that no CPU ever has more than one running thread.
	var lastSpan *threadSpan
	for _, ts := range cs.runningSpans {
		if lastSpan != nil && lastSpan.endTimestamp > ts.startTimestamp {
			return status.Errorf(codes.InvalidArgument, "multiple running threads on %s at timestamp %d: [%s %s]", ts.cpu, ts.startTimestamp, lastSpan, ts)
		}
		lastSpan = ts
	}
	return nil
}

type cpuSpanSet struct {
	cpuSpansByCPU map[CPUID]*cpuSpans
}

func newCPUSpanSet() *cpuSpanSet {
	return &cpuSpanSet{
		cpuSpansByCPU: map[CPUID]*cpuSpans{},
	}
}

func (css *cpuSpanSet) cpuSpans(cpu CPUID) *cpuSpans {
	cs, ok := css.cpuSpansByCPU[cpu]
	if !ok {
		cs = &cpuSpans{}
		css.cpuSpansByCPU[cpu] = cs
	}
	return cs
}

// addSpan adds the provided span to its appropriate cpuSpans.
func (css *cpuSpanSet) addSpan(span *threadSpan) {
	css.cpuSpans(span.cpu).addSpan(span)
}

func (css *cpuSpanSet) cpuTrees() (runningSpansByCPU map[CPUID][]*threadSpan, sleepingSpansByCPU, waitingSpansByCPU map[CPUID]augmentedtree.Tree, err error) {
	runningSpansByCPU = map[CPUID][]*threadSpan{}
	sleepingSpansByCPU = map[CPUID]augmentedtree.Tree{}
	waitingSpansByCPU = map[CPUID]augmentedtree.Tree{}
	for cpu, css := range css.cpuSpansByCPU {
		if err = css.finalize(); err != nil {
			return
		}
		runningSpansByCPU[cpu] = css.runningSpans
		sleeping := augmentedtree.New(1)
		for _, span := range css.sleepingSpans {
			sleeping.Add(span)
		}
		sleepingSpansByCPU[cpu] = sleeping
		waiting := augmentedtree.New(1)
		for _, span := range css.waitingSpans {
			waiting.Add(span)
		}
		waitingSpansByCPU[cpu] = waiting
	}
	return
}
