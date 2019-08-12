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
import {HttpClient} from '@angular/common/http';
import {Injectable} from '@angular/core';
import {Observable, of} from 'rxjs';
import {map} from 'rxjs/operators';

import {CollectionParameters, CpuInterval, CpuIntervalCollection, Layer, SchedEvent, ThreadInterval, WaitingCpuInterval} from '../models';
import * as services from '../models/render_data_services';
import {Viewport} from '../util';

/**
 * A render service fetches intervals and events as needed for heatmap
 * rendering, on viewport change.
 */
export interface RenderDataService {
  getCpuIntervals(
      parameters: CollectionParameters, viewport: Viewport,
      minIntervalDuration: number,
      cpus: number[]): Observable<CpuIntervalCollection[]>;
  getPidIntervals(
      parameters: CollectionParameters, layer: Layer, viewport: Viewport,
      minIntervalDuration: number): Observable<Layer>;
  getSchedEvents(
      parameters: CollectionParameters, layer: Layer, viewport: Viewport,
      cpus: number[]): Observable<Layer>;
}

/**
 * A render service that fetches data from the SchedViz server.
 */
@Injectable({providedIn: 'root'})
export class HttpRenderDataService implements RenderDataService {
  private readonly cpuIntervalsUrl = '/get_cpu_intervals';
  private readonly pidIntervalsUrl = '/get_pid_intervals';
  private readonly ftraceEventsUrl = '/get_ftrace_events';

  constructor(private readonly http: HttpClient) {}

  /** Get CPU intervals from the server via POST */
  getCpuIntervals(
      parameters: CollectionParameters, viewport: Viewport,
      minIntervalDuration: number,
      cpus: number[]): Observable<CpuIntervalCollection[]> {
    const cpuIntervalsReq: services.CpuIntervalsRequest = {
      collectionName: parameters.name,
      minIntervalDurationNs: minIntervalDuration,
      cpus,
    };
    const leftNs = Math.floor(
        viewport.left * (parameters.endTimeNs - parameters.startTimeNs));
    const rightNs = Math.ceil(
        viewport.right * (parameters.endTimeNs - parameters.startTimeNs));
    cpuIntervalsReq.startTimestampNs = parameters.startTimeNs + leftNs;
    cpuIntervalsReq.endTimestampNs = parameters.startTimeNs + rightNs;

    return this.http
        .post<services.CpuIntervalsResponse>(
            this.cpuIntervalsUrl, cpuIntervalsReq)
        .pipe(
            map(res => res.intervals),
            map(intervalsByCpu => intervalsByCpu.map(
                    ({cpu, running, waiting}) => new CpuIntervalCollection(
                        cpu,
                        running.map(
                            interval =>
                                constructCpuInterval(parameters, interval)),
                        waiting.map(
                            interval => constructWaitingCpuInterval(
                                parameters, interval)),
                        ))),
        );
  }

  /** Get PID intervals from the server via POST */
  getPidIntervals(
      parameters: CollectionParameters, layer: Layer, viewport: Viewport,
      minIntervalDuration: number): Observable<Layer> {
    const pidList = layer.ids;
    const leftNs = Math.floor(
        viewport.left * (parameters.endTimeNs - parameters.startTimeNs));
    const rightNs = Math.ceil(
        viewport.right * (parameters.endTimeNs - parameters.startTimeNs));
    const pidIntervalsReq: services.PidIntervalsRequest = {
      collectionName: parameters.name,
      startTimestampNs: parameters.startTimeNs + leftNs,
      endTimestampNs: parameters.startTimeNs + rightNs,
      minIntervalDurationNs: minIntervalDuration,
      pids: pidList,
    };

    return this.http
        .post<services.PidIntervalsResponse>(
            this.pidIntervalsUrl, pidIntervalsReq)
        .pipe(
            map(res => res.pidIntervals.map(intervalsList => {
              const pid = intervalsList.pid;
              return intervalsList.intervals
                  .map(interval => {
                    // Double check that there is only one command
                    const commands =
                        Array.from(new Set(interval.threadResidencies.map(
                            tr => tr.thread.command)));
                    if (commands.length !== 1) {
                      return;
                    }
                    return new ThreadInterval(
                        parameters,
                        interval.cpu,
                        interval.startTimestamp,
                        interval.startTimestamp + interval.duration,
                        pid,
                        commands[0],
                        interval.threadResidencies,
                    );
                  })
                  .filter((i): i is ThreadInterval => i != null);
            })),
            map(nestedIntervals =>
                    new Array<ThreadInterval>().concat(...nestedIntervals)),
            map(intervals => {
              layer.intervals = intervals;
              return layer;
            }),
        );
  }

  getSchedEvents(
      parameters: CollectionParameters, layer: Layer, viewport: Viewport,
      cpus: number[]): Observable<Layer> {
    const leftNs = Math.floor(
        viewport.left * (parameters.endTimeNs - parameters.startTimeNs));
    const rightNs = Math.ceil(
        viewport.right * (parameters.endTimeNs - parameters.startTimeNs));

    const ftraceEventsReq: services.FtraceEventsRequest = {
      collectionName: parameters.name,
      startTimestamp: parameters.startTimeNs + leftNs,
      endTimestamp: parameters.startTimeNs + rightNs,
      cpus,
      eventTypes: [layer.name],
    };

    return this.http
        .post<services.FtraceEventsResponse>(
            this.ftraceEventsUrl, ftraceEventsReq)
        .pipe(
            map(res => Object.values(res.eventsByCpu)),
            map(eventList => eventList.map(events => events.map(event => {
              const properties = new Map<string, string>();
              const eventTextProps = event.textProperties;
              for (const [key, value] of Object.entries(eventTextProps)) {
                properties.set(key, value);
              }
              const eventNumProps = event.numberProperties;
              for (const [key, value] of Object.entries(eventNumProps)) {
                properties.set(key, `${value}`);
              }
              return new SchedEvent(
                  parameters,
                  event.index,
                  event.name,
                  event.cpu,
                  event.timestamp,
                  properties,
              );
            }))),
            map(nestedEvents => {
              layer.intervals = new Array<SchedEvent>().concat(...nestedEvents);
              return layer;
            }));
  }
}
// END-INTERNAL

/**
 * A render service that returns mock data.
 */
@Injectable({providedIn: 'root'})
export class LocalRenderDataService implements RenderDataService {
  /** Returns set of mock Intervals for all CPUs */
  getCpuIntervals(
      parameters: CollectionParameters, viewport: Viewport,
      minIntervalDuration: number,
      cpus: number[]): Observable<CpuIntervalCollection[]> {
    const collection = [];
    const duration = parameters.endTimeNs - parameters.startTimeNs;
    for (let cpu = 0; cpu < parameters.size; cpu++) {
      const runningIntervals: CpuInterval[] = [];
      const waitingIntervals: WaitingCpuInterval[] = [];
      let timestamp = parameters.startTimeNs;
      let prevTimestamp = timestamp;
      while (timestamp < parameters.endTimeNs) {
        prevTimestamp = timestamp;
        const delta = Math.max(minIntervalDuration, duration / 100);
        timestamp += delta;
        timestamp = Math.min(timestamp, parameters.endTimeNs);
        const percentIdle = 0.5 * Math.random();
        const runningTime = (1 - percentIdle) * (timestamp - prevTimestamp);

        const running = [{
          thread: {command: 'foo', pid: 0, priority: 100},
          state: services.ThreadState.RUNNING_STATE,
          duration: runningTime,
          droppedEventIDs: [],
          includesSyntheticTransitions: false,
        }];

        const runningInterval = new CpuInterval(
            parameters, cpu, prevTimestamp, timestamp, running, []);
        runningIntervals.push(runningInterval);

        const waitingInterval = new WaitingCpuInterval(
            parameters, cpu, prevTimestamp, timestamp, running, []);
        waitingIntervals.push(waitingInterval);
      }
      collection.push(
          new CpuIntervalCollection(cpu, runningIntervals, waitingIntervals));
    }
    // TODO(sainsley): Store visible CPUs on CPU Layers and return rendered
    // layer as in PidIntervals callback.
    return of(collection);
  }

  /** Returns set of mock Intervals for a few CPUs */
  getPidIntervals(
      parameters: CollectionParameters, layer: Layer, viewport: Viewport,
      minIntervalDuration: number): Observable<Layer> {
    const intervals = [];
    const cpuCount = parameters.size;
    for (let i = 0; i < cpuCount; i++) {
      const cpu = Math.floor(Math.random() * parameters.size);
      let timestamp = parameters.startTimeNs;
      let prevTimestamp = timestamp;
      while (timestamp < parameters.endTimeNs) {
        const pid = Math.floor(Math.random() * 5000);
        prevTimestamp = timestamp;
        const delta = minIntervalDuration;
        timestamp += delta;
        timestamp = Math.min(timestamp, parameters.endTimeNs);
        const interval = new ThreadInterval(
            parameters, cpu, prevTimestamp, timestamp, pid, 'bar', [{
              thread: {command: 'foo', pid: 0, priority: 100},
              state: services.ThreadState.RUNNING_STATE,
              duration: timestamp - prevTimestamp,
              droppedEventIDs: [],
              includesSyntheticTransitions: false,
            }]);
        intervals.push(interval);
      }
    }
    layer.intervals = intervals;
    return of(layer);
  }

  getSchedEvents(
      parameters: CollectionParameters, layer: Layer, viewport: Viewport,
      cpus: number[]): Observable<Layer> {
    const intervals = [];
    const cpuCount = parameters.size;
    for (let i = 0; i < cpuCount; i++) {
      const cpu = Math.floor(Math.random() * parameters.size);
      let timestamp = parameters.startTimeNs;
      while (timestamp < parameters.endTimeNs) {
        const delta = 3 * Math.random() *
            (parameters.endTimeNs - parameters.startTimeNs) / parameters.size;
        timestamp += delta;
        timestamp = Math.min(timestamp, parameters.endTimeNs);
        const interval =
            new SchedEvent(parameters, 1234, 'event_type', cpu, timestamp);
        intervals.push(interval);
      }
    }
    layer.intervals = intervals;
    return of(layer);
  }
}

function getThreadResidenciesByType(
    threadResidencies: services.ThreadResidency[]) {
  return threadResidencies.reduce((acc, tr) => {
    acc[tr.state] = acc[tr.state] || [];
    acc[tr.state]!.push(tr);
    return acc;
  }, {} as {[k in services.ThreadState]?: services.ThreadResidency[]});
}

function constructCpuInterval(
    parameters: CollectionParameters, interval: services.Interval) {
  const threadResidenciesByType =
      getThreadResidenciesByType(interval.threadResidencies);
  return new CpuInterval(
      parameters,
      interval.cpu,
      interval.startTimestamp,
      interval.startTimestamp + interval.duration,
      threadResidenciesByType[services.ThreadState.RUNNING_STATE] || [],
      threadResidenciesByType[services.ThreadState.WAITING_STATE] || [],
  );
}

function constructWaitingCpuInterval(
    parameters: CollectionParameters, interval: services.Interval) {
  const threadResidenciesByType =
      getThreadResidenciesByType(interval.threadResidencies);
  return new WaitingCpuInterval(
      parameters,
      interval.cpu,
      interval.startTimestamp,
      interval.startTimestamp + interval.duration,
      threadResidenciesByType[services.ThreadState.RUNNING_STATE] || [],
      threadResidenciesByType[services.ThreadState.WAITING_STATE] || [],
  );
}
