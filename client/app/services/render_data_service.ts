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
import {HttpClient, HttpErrorResponse} from '@angular/common/http';
import {Injectable} from '@angular/core';
import {Observable, of} from 'rxjs';
import {catchError, map} from 'rxjs/operators';

import {CollectionParameters, CpuInterval, Layer, SchedEvent, ThreadInterval, ThreadState, WaitingThreadInterval} from '../models';
import * as services from '../models/render_data_services';
import {Viewport} from '../util';

/**
 * A render service fetches intervals and events as needed for heatmap
 * rendering, on viewport change.
 */
export interface RenderDataService {
  getCpuIntervals(
      parameters: CollectionParameters, viewport: Viewport,
      minIntervalDuration: number, cpus: number[]): Observable<CpuInterval[]>;
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
      minIntervalDuration: number, cpus: number[]): Observable<CpuInterval[]> {
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
            map(res => res.cpuIntervals.map(
                    intervalsList => intervalsList.intervals)),
            map(intervalsByCpu => intervalsByCpu.map(
                    intervals => intervals.map(
                        interval => new CpuInterval(
                            parameters,
                            interval.cpu,
                            interval.startTimestampNs,
                            interval.endTimestampNs,
                            interval.runningCommand,
                            interval.runningPid,
                            interval.idleNs,
                            interval.waitingPidCount,
                            interval.waitingPids,
                            )))),
            map(nestedIntervals => {
              return new Array<CpuInterval>().concat(...nestedIntervals);
            }));
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
            map(res => res.pidIntervals.map(
                    intervalsList => intervalsList.intervals)),
            map(intervalsByCpu => intervalsByCpu.map(
                    intervals => intervals.map(
                        interval => interval.state ===
                                services.ThreadState.WAITING_STATE ?
                            new WaitingThreadInterval(
                                parameters, interval.cpu,
                                interval.startTimestampNs,
                                interval.endTimestampNs, interval.pid,
                                interval.command) :
                            new ThreadInterval(
                                parameters, interval.cpu,
                                interval.startTimestampNs,
                                interval.endTimestampNs, interval.pid,
                                interval.command,
                                HttpRenderDataService.getThreadState(
                                    interval.state))))),
            map(nestedIntervals => {
              layer.intervals =
                  new Array<ThreadInterval>().concat(...nestedIntervals);
              return layer;
            }));
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

  /**
   * Translates the ThreadState proto to a local string state flag.
   */
  static getThreadState(state: services.ThreadState) {
    switch (state) {
      case services.ThreadState.RUNNING_STATE:
        return ThreadState.RUNNING;
      case services.ThreadState.SLEEPING_STATE:
        return ThreadState.SLEEPING;
      case services.ThreadState.WAITING_STATE:
        return ThreadState.WAITING;
      default:
        return ThreadState.UNKNOWN;
    }
  }

  /**
   * Handle Http operation that failed.
   * Let the app continue.
   * @param operation - name of the operation that failed
   * @param result - optional value to return as the observable result
   */
  private handleError<T>(operation = 'operation', result?: T) {
    return (error: HttpErrorResponse): Observable<T> => {
      // TODO: send the error to remote logging infrastructure
      console.error(error);  // log to console instead

      // TODO: better job of transforming error for user consumption
      console.log(`${operation} failed: ${error.message}`);

      // Let the app keep running by returning an empty result.
      return of(result as T);
    };
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
      minIntervalDuration: number, cpus: number[]): Observable<CpuInterval[]> {
    const intervals = [];
    for (let cpu = 0; cpu < parameters.size; cpu++) {
      let timestamp = parameters.startTimeNs;
      let prevTimestamp = timestamp;
      while (timestamp < parameters.endTimeNs) {
        prevTimestamp = timestamp;
        const delta = 2 * Math.random() *
            (parameters.endTimeNs - parameters.startTimeNs) / parameters.size;
        timestamp += delta;
        timestamp = Math.min(timestamp, parameters.endTimeNs);
        const percentIdle = 0.5 * Math.random();
        const waitingPidCount = Math.floor(3 * Math.random());
        const idleTime = percentIdle * (timestamp - prevTimestamp);
        const interval = new CpuInterval(
            parameters, cpu, prevTimestamp, timestamp, 'foo', idleTime,
            waitingPidCount);
        intervals.push(interval);
      }
    }
    // TODO(sainsley): Store visible CPUs on CPU Layers and return rendered
    // layer as in PidIntervals callback.
    return of(intervals);
  }

  /** Returns set of mock Intervals for a few CPUs */
  getPidIntervals(
      parameters: CollectionParameters, layer: Layer, viewport: Viewport,
      minIntervalDuration: number): Observable<Layer> {
    const intervals = [];
    const cpuCount = Math.floor(5 * Math.random()) + 1;
    for (let i = 0; i < cpuCount; i++) {
      const cpu = Math.floor(Math.random() * parameters.size);
      let timestamp = parameters.startTimeNs;
      let prevTimestamp = timestamp;
      while (timestamp < parameters.endTimeNs) {
        const pid = Math.floor(Math.random() * 5000);
        prevTimestamp = timestamp;
        const delta = 3 * Math.random() *
            (parameters.endTimeNs - parameters.startTimeNs) / parameters.size;
        timestamp += delta;
        timestamp = Math.min(timestamp, parameters.endTimeNs);
        const interval = new ThreadInterval(
            parameters, cpu, prevTimestamp, timestamp, pid, 'bar');
        intervals.push(interval);
      }
    }
    layer.intervals = intervals;
    return of(layer);
  }

  getSchedEvents(
      parameters: CollectionParameters, layer: Layer, viewport: Viewport,
      cpus: number[]): Observable<Layer> {
    return of(layer);
  }
}
