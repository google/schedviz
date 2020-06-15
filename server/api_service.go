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
// Package apiservice contains wrappers around the analysis library
package apiservice

import (
	"context"
	"fmt"
	"sync"
	"time"


	"github.com/golang/sync/errgroup"

	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"
	"github.com/google/schedviz/server/storageservice"
	"github.com/google/schedviz/tracedata/trace"
)


// APIService contains wrappers around the analysis library
type APIService struct {
	StorageService storageservice.StorageService
}

func (as *APIService) fetchCollection(ctx context.Context, collectionName string) (*storageservice.CachedCollection, error) {
	if len(collectionName) == 0 {
		return nil, missingFieldError("collection_name")
	}
	return as.StorageService.GetCollection(ctx, collectionName)
}

// GetCPUIntervals returns CPU intervals for the specified collection.
func (as *APIService) GetCPUIntervals(ctx context.Context, req *models.CPUIntervalsRequest) (*models.CPUIntervalsResponse, error) {
	c, err := as.fetchCollection(ctx, req.CollectionName)
	if err != nil {
		return nil, err
	}
	res := &models.CPUIntervalsResponse{
		CollectionName: req.CollectionName,
		Intervals:      make([]models.CPUIntervals, len(req.CPUs)),
	}

	var g errgroup.Group
	for i, cpu := range req.CPUs {
		i := i
		filters := []sched.Filter{
			sched.TimeRange(trace.Timestamp(req.StartTimestampNs), trace.Timestamp(req.EndTimestampNs)),
			sched.MinIntervalDuration(sched.Duration(req.MinIntervalDurationNs)),
			sched.CPUs(sched.CPUID(cpu)),
		}

		res.Intervals[i].CPU = cpu

		g.Go(func() error {
			cpuIntervals, err := c.Collection.CPUIntervals(false /*=splitOnWaitingPIDChange*/, filters...)
			if err != nil {
				return err
			}
			res.Intervals[i].Running = cpuIntervals
			return nil
		})

		g.Go(func() error {
			waitingIntervals, err := c.Collection.CPUIntervals(true /*=splitOnWaitingPIDChange*/, filters...)
			if err != nil {
				return err
			}
			res.Intervals[i].Waiting = waitingIntervals
			return nil
		})

	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return res, nil
}

// GetPIDIntervals returns PID intervals for the specified collection and PIDs.
func (as *APIService) GetPIDIntervals(ctx context.Context, req *models.PidIntervalsRequest) (*models.PIDntervalsResponse, error) {
	c, err := as.fetchCollection(ctx, req.CollectionName)
	if err != nil {
		return nil, err
	}
	res := &models.PIDntervalsResponse{
		CollectionName: req.CollectionName,
		PIDIntervals:   make([]models.PIDIntervals, len(req.Pids)),
	}

	var g errgroup.Group
	for i, pid := range req.Pids {
		i, pid := i, pid
		g.Go(func() error {
			pidIntervals, err := c.Collection.ThreadIntervals(
				sched.PIDs(sched.PID(pid)),
				sched.TimeRange(trace.Timestamp(req.StartTimestampNs), trace.Timestamp(req.EndTimestampNs)),
				sched.MinIntervalDuration(sched.Duration(req.MinIntervalDurationNs)),
				sched.TruncateToTimeRange(false))
			if err != nil {
				return fmt.Errorf("error occurred getting intervals for PID: %d, %v", pid, err)
			}
			res.PIDIntervals[i] = models.PIDIntervals{
				PID:       pid,
				Intervals: pidIntervals,
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return res, nil
}

// GetAntagonists returns a set of antagonist information for a specified collection, from a
// specified set of threads and over a specified interval.
func (as *APIService) GetAntagonists(ctx context.Context, req *models.AntagonistsRequest) (*models.AntagonistsResponse, error) {
	c, err := as.fetchCollection(ctx, req.CollectionName)
	if err != nil {
		return nil, err
	}
	res := &models.AntagonistsResponse{
		CollectionName: req.CollectionName,
	}
	for _, pid := range req.Pids {
		ants, err := c.Collection.Antagonists(
			sched.PIDs(sched.PID(pid)),
			sched.StartTimestamp(trace.Timestamp(req.StartTimestampNs)),
			sched.EndTimestamp(trace.Timestamp(req.EndTimestampNs)))
		if err != nil {
			return nil, fmt.Errorf("error fetching antagonists for pid: %d. caused by: %s", pid, err)
		}
		res.Antagonists = append(res.Antagonists, &ants)
	}
	return res, nil
}

// GetPerThreadEventSeries returns all events in a specified collection occurring on a specified PID
// in a specified interval, in increasing temporal order.
func (as *APIService) GetPerThreadEventSeries(ctx context.Context, req *models.PerThreadEventSeriesRequest) (*models.PerThreadEventSeriesResponse, error) {
	c, err := as.fetchCollection(ctx, req.CollectionName)
	if err != nil {
		return nil, err
	}
	var g errgroup.Group
	ess := []*models.PerThreadEventSeries{}
	m := sync.Mutex{}
	for _, pid := range req.Pids {
		// Create a copy of pid
		pid := pid
		g.Go(func() error {
			events, err := c.Collection.PerThreadEventSeries(pid, time.Duration(req.StartTimestampNs), time.Duration(req.EndTimestampNs))
			if err != nil {
				return fmt.Errorf("error occurred getting thread events for PID: %d, %v", pid, err)
			}
			m.Lock()
			defer m.Unlock()
			ess = append(ess, &models.PerThreadEventSeries{
				Pid:    pid,
				Events: events,
			})
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return &models.PerThreadEventSeriesResponse{
		CollectionName: req.CollectionName,
		EventSeries:    ess,
	}, nil
}

// GetThreadSummaries returns a set of thread summaries for a specified collection over a specified
// interval.
func (as *APIService) GetThreadSummaries(ctx context.Context, req *models.ThreadSummariesRequest) (*models.ThreadSummariesResponse, error) {
	c, err := as.fetchCollection(ctx, req.CollectionName)
	if err != nil {
		return nil, err
	}
	threadSummaries, err := c.Collection.ThreadSummaries(
		sched.CPUs(req.Cpus...),
		sched.TimeRange(trace.Timestamp(req.StartTimestampNs), trace.Timestamp(req.EndTimestampNs)))
	if err != nil {
		return nil, err
	}
	return &models.ThreadSummariesResponse{
		CollectionName: req.CollectionName,
		Metrics:        threadSummaries,
	}, nil
}

// GetUtilizationMetrics returns a set of metrics describing the utilization or over-utilization of
// some portion of the system over some span of the trace.
// These metrics are described in the sched.Utilization struct.
func (as *APIService) GetUtilizationMetrics(ctx context.Context, req *models.UtilizationMetricsRequest) (*models.UtilizationMetricsResponse, error) {
	c, err := as.fetchCollection(ctx, req.CollectionName)
	if err != nil {
		return nil, err
	}
	um, err := c.Collection.UtilizationMetrics(sched.CPUs(req.Cpus...), sched.TimeRange(req.StartTimestampNs, req.EndTimestampNs), sched.TruncateToTimeRange(true))
	if err != nil {
		return nil, err
	}
	return &models.UtilizationMetricsResponse{
		Request:            req,
		UtilizationMetrics: &um,
	}, nil
}

// GetSystemTopology returns the system topology of the machine that the collection was recorded on.
func (as *APIService) GetSystemTopology(ctx context.Context, collectionName string) (*models.SystemTopology, error) {
	c, err := as.fetchCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}

	return &c.SystemTopology, err
}


func missingFieldError(fieldName string) error {
	return fmt.Errorf("missing required field %q", fieldName)
}
