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
import {flatMap, map} from 'rxjs/operators';

import {CollectionParameters, FtraceInterval, Thread, ThreadInterval, UtilizationMetrics} from '../models';

import * as services from '../models/metrics_services';
import {ThreadState} from '../models/render_data_services';
import {Viewport} from '../util';

/**
 * Metrics service fetches thread, CPU, event, etc. metrics for tabular display.
 */
export interface MetricsService {
  getThreadSummaries(
      parameters: CollectionParameters, viewport: Viewport,
      cpus: number[]): Observable<Thread[]>;
  getPerThreadEvents(
      parameters: CollectionParameters, viewport: Viewport,
      thread: Thread): Observable<FtraceInterval[]>;
  getThreadAntagonists(
      parameters: CollectionParameters, viewport: Viewport,
      thread: Thread): Observable<ThreadInterval[]>;
  getUtilizationMetrics(
      collectionName: string, cpus: number[], startTimeNs: number,
      endTimeNs: number): Observable<UtilizationMetrics|{}>;
}

/**
 * A metrics service that fetches data from the SchedViz server.
 */
@Injectable({providedIn: 'root'})
export class HttpMetricsService implements MetricsService {
  private readonly threadSummariesUrl = '/get_thread_summaries';
  private readonly perThreadEventsUrl = '/get_per_thread_event_series';
  private readonly threadAntagonistsUrl = '/get_antagonists';
  private readonly utilizationMetricsUrl = '/get_utilization_metrics';

  constructor(private readonly http: HttpClient) {}

  getThreadSummaries(
      parameters: CollectionParameters, viewport: Viewport,
      cpus: number[]): Observable<Thread[]> {
    const leftNs = Math.floor(
        viewport.left * (parameters.endTimeNs - parameters.startTimeNs));
    const rightNs = Math.ceil(
        viewport.right * (parameters.endTimeNs - parameters.startTimeNs));
    const req: services.ThreadSummariesRequest = {
      collectionName: parameters.name,
      startTimestampNs: parameters.startTimeNs + leftNs,
      endTimestampNs: parameters.startTimeNs + rightNs,
      cpus,
    };
    return this.http
        .post<services.ThreadSummariesResponse>(this.threadSummariesUrl, req)
        .pipe(
            map(res => res.metrics),
            map(summaries => summaries.map(
                    summary => new Thread(
                        parameters,
                        Number(summary.pids[0]),
                        summary.cpus,
                        summary.commands.length ? summary.commands[0] : '',
                        Number(summary.wakeupCount),
                        Number(summary.migrationCount),
                        Number(summary.runTimeNs),
                        Number(summary.waitTimeNs),
                        Number(summary.sleepTimeNs),
                        Number(summary.unknownTimeNs),
                        ))));
  }


  getPerThreadEvents(
      parameters: CollectionParameters, viewport: Viewport,
      thread: Thread): Observable<FtraceInterval[]> {
    const leftNs = Math.floor(
        viewport.left * (parameters.endTimeNs - parameters.startTimeNs));
    const rightNs = Math.ceil(
        viewport.right * (parameters.endTimeNs - parameters.startTimeNs));
    const perThreadEventSeriesReq: services.PerThreadEventSeriesRequest = {
      collectionName: parameters.name,
      pids: [thread.pid],
      startTimestampNs: parameters.startTimeNs + leftNs,
      endTimestampNs: parameters.startTimeNs + rightNs,
    };
    return this.http
        .post<services.PerThreadEventSeriesResponse>(
            this.perThreadEventsUrl, perThreadEventSeriesReq)
        .pipe(
            // TODO(sainsley): generalize for multiple threads
            map(res => res.eventSeries[0].events),
            map(events => events.map(
                    event => new FtraceInterval(parameters, event))));
  }

  getThreadAntagonists(
      parameters: CollectionParameters, viewport: Viewport,
      thread: Thread): Observable<ThreadInterval[]> {
    const leftNs = Math.floor(
        viewport.left * (parameters.endTimeNs - parameters.startTimeNs));
    const rightNs = Math.ceil(
        viewport.right * (parameters.endTimeNs - parameters.startTimeNs));
    const antagonistsReq: services.AntagonistsRequest = {
      collectionName: parameters.name,
      pids: [thread.pid],
      startTimestampNs: parameters.startTimeNs + leftNs,
      endTimestampNs: parameters.startTimeNs + rightNs,
    };

    return this.http
        .post<services.AntagonistsResponse>(
            this.threadAntagonistsUrl, antagonistsReq)
        .pipe(
            map(proto => proto.antagonists),
            map(threads => threads.map((thread) => {
              return thread.antagonisms.map(
                  ant => new ThreadInterval(
                      parameters, ant.cpu, ant.startTimestamp, ant.endTimestamp,
                      ant.runningThread.pid, ant.runningThread.command, [{
                        thread: ant.runningThread,
                        // Antagonists must be running to be antagonizing
                        state: ThreadState.RUNNING_STATE,
                        duration: ant.endTimestamp - ant.startTimestamp,
                        includesSyntheticTransitions: false,
                        droppedEventIDs: [],
                      }]));
            })),
            map(nestedIntervals => {
              return new Array<ThreadInterval>().concat(...nestedIntervals);
            }));
  }

  getUtilizationMetrics(
      collectionName: string, cpus: number[], startTimeNs: number,
      endTimeNs: number): Observable<UtilizationMetrics|{}> {
    const utilizationMetricsReq: services.UtilizationMetricsRequest = {
      collectionName,
      cpus,
      startTimestampNs: startTimeNs,
      endTimestampNs: endTimeNs,
    };

    return this.http
        .post<services.UtilizationMetricsResponse>(
            this.utilizationMetricsUrl, utilizationMetricsReq)
        .pipe(map(res => UtilizationMetrics.fromJSON(res)));
  }
}
// END-INTERNAL

/**
 * A metrics service that returns mock data.
 */
@Injectable({providedIn: 'root'})
export class LocalMetricsService implements MetricsService {
  private static getRandomInt(max: number) {
    return Math.floor(Math.random() * Math.floor(max));
  }

  private static getRandomFloat(max: number) {
    return Math.random() * max;
  }

  private static getRandomCommand() {
    let text = '';
    const possible = 'abcdefghijklmnopqrstuvwxyz0123456789';
    const commandLen = 5 + LocalMetricsService.getRandomInt(8);
    for (let i = 0; i < commandLen; i++) {
      text += possible.charAt(Math.floor(Math.random() * possible.length));
    }
    return text;
  }

  /** GET intervals from the server */
  getThreadSummaries(parameters: CollectionParameters, viewport: Viewport):
      Observable<Thread[]> {
    const threadCount = (LocalMetricsService.getRandomInt(1000) + 200) *
        viewport.width * viewport.height;
    const threadData: Thread[] = [];
    for (let i = 0; i < threadCount; i++) {
      const cpus = [];
      const cpuCount = parameters.size;
      for (let ii = 0; ii < cpuCount; ii++) {
        cpus.push(LocalMetricsService.getRandomInt(cpuCount));
      }
      threadData.push(new Thread(
          parameters, LocalMetricsService.getRandomInt(10000), cpus,
          LocalMetricsService.getRandomCommand(),
          LocalMetricsService.getRandomInt(100),
          LocalMetricsService.getRandomInt(100),
          LocalMetricsService.getRandomFloat(500000),
          LocalMetricsService.getRandomFloat(500000),
          LocalMetricsService.getRandomFloat(500000),
          LocalMetricsService.getRandomFloat(500000)));
    }
    return of(threadData);
  }

  getPerThreadEvents(
      parameters: CollectionParameters, viewport: Viewport,
      thread: Thread): Observable<FtraceInterval[]> {
    const foo: FtraceInterval[] = [];
    return of(foo);
  }

  getThreadAntagonists(
      parameters: CollectionParameters, viewport: Viewport,
      thread: Thread): Observable<ThreadInterval[]> {
    const foo: ThreadInterval[] = [];
    return of(foo);
  }


  getUtilizationMetrics(
      collectionName: string, cpus: number[], startTimeNs: number,
      endTimeNs: number): Observable<UtilizationMetrics|{}> {
    const mockValue = cpus.length;
    return of(
        new UtilizationMetrics(mockValue, mockValue, mockValue, mockValue));
  }
}
