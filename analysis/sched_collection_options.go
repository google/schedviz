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
	log "github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	elpb "github.com/google/schedviz/analysis/event_loaders_go_proto"
)

type collectionOptions struct {
	// If true, all timestamps in the collection will be normalized to that of the
	// first unclipped event.
	normalizeTimestamps bool
	// If true, thread command names in intervals will be as precise as possible:
	// events lacking commands will be populated with thread command names from
	// earlier events referring to the same PID, and intervals will be split on
	// changes in thread command name, even if nothing else changed.
	preciseCommands bool
	// As preciseCommands, but for thread priorities.
	precisePriorities bool
	// The event loaders to use with this collection.
	loaders EventLoaders
}

// Option specifies an option that may be specified for a Collection at its
// creation.
type Option func(o *collectionOptions) error

// NormalizeTimestamps specifies whether to adjust event timestamps.  Called
// with true, adjusts all timestamps in the Collection such that the first
// unclipped event (whether sched or otherwise) has Timestamp 0, and all other
// events are adjusted by the same amount.  Called with false, it leaves all
// timestamps unmodified.
// If unspecified, timestamps are not normalized.
func NormalizeTimestamps(b bool) Option {
	return func(o *collectionOptions) error {
		o.normalizeTimestamps = b
		return nil
	}
}

// PreciseCommands specifies whether thread command names in intervals should
// be as precise as possible: events lacking thread command names will be
// populated with commands from earlier events referring to the same PID, and
// intervals will be split on changes in thread command, even if nothing else
// changed.
func PreciseCommands(b bool) Option {
	return func(o *collectionOptions) error {
		o.preciseCommands = b
		return nil
	}
}

// PrecisePriorities specifies whether thread priorities in intervals should
// be as precise as possible: events lacking thread priorities will be
// populated with priorities from earlier events referring to the same PID, and
// intervals will be split on changes in thread priority, even if nothing else
// changed.
func PrecisePriorities(b bool) Option {
	return func(o *collectionOptions) error {
		o.precisePriorities = b
		return nil
	}
}

// UsingEventLoadersType specifies the event loaders, by their LoaderType, to
// use while loading this collection.  Overrides the EventSet's default event
// loader.
func UsingEventLoadersType(elt elpb.LoadersType) Option {
	return func(o *collectionOptions) error {
		log.Infof("Using event loader type %s", elt)
		el, err := EventLoader(elt)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid argument to UsingEventLoadersType: %v", err)
		}
		o.loaders = el
		return nil
	}
}

// UsingEventLoaders specifies the event loaders to use while loading this
// collection.  Overrides the EventSet's default event loader.
func UsingEventLoaders(el EventLoaders) Option {
	return func(o *collectionOptions) error {
		log.Infof("Using custom event loader")
		if el == nil {
			return status.Errorf(codes.InvalidArgument, "nil argument to UsingEventLoaders")
		}
		o.loaders = el
		return nil
	}
}
