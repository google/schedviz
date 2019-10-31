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

import {HttpErrorResponse} from '@angular/common/http';
import {ChangeDetectionStrategy, Component, ElementRef, Inject, Input, OnDestroy, OnInit, ViewChild} from '@angular/core';
import {MatSnackBar} from '@angular/material/snack-bar';
import {Sort} from '@angular/material/sort';
import {BehaviorSubject, Subscription} from 'rxjs';
import {debounceTime} from 'rxjs/operators';

import {CollectionParameters, FtraceInterval, Interval, Layer, SchedEvent, Thread, ThreadInterval} from '../models';
import {MetricsService} from '../services/metrics_service';
import {RenderDataService} from '../services/render_data_service';
import {createHttpErrorMessage, SystemTopology, Viewport} from '../util';

/**
 * The sidebar component holds the thread list and settings tab
 */
@Component({
  selector: 'sidebar',
  templateUrl: 'sidebar.ng.html',
  styleUrls: ['sidebar.css'],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Sidebar implements OnInit, OnDestroy {
  @ViewChild('tabGroup', {static: false}) tabGroup!: ElementRef;
  @Input() parameters!: BehaviorSubject<CollectionParameters|undefined>;
  @Input() systemTopology!: SystemTopology;
  @Input() preview!: BehaviorSubject<Interval|undefined>;
  @Input() layers!: BehaviorSubject<Array<BehaviorSubject<Layer>>>;
  @Input() viewport!: BehaviorSubject<Viewport>;
  @Input() tab!: BehaviorSubject<number>;
  @Input() filter!: BehaviorSubject<string>;
  @Input() threadSort!: BehaviorSubject<Sort>;
  @Input() showMigrations!: BehaviorSubject<boolean>;
  @Input() showSleeping!: BehaviorSubject<boolean>;
  @Input() maxIntervalCount!: BehaviorSubject<number>;
  @Input() cpuFilter!: BehaviorSubject<string>;
  @Input() expandedThread!: BehaviorSubject<string|undefined>;

  expandedFtraceIntervals = new BehaviorSubject<FtraceInterval[]>([]);
  expandedThreadAntagonists = new BehaviorSubject<ThreadInterval[]>([]);
  expandedThreadIntervals = new BehaviorSubject<ThreadInterval[]>([]);
  selectedSchedEvents = new BehaviorSubject<SchedEvent[]>([]);
  eventTypesSubject = new BehaviorSubject<string[]>([]);
  threads = new BehaviorSubject<Thread[]>([]);
  threadsPending = true;

  threadSubscription?: Subscription;

  constructor(
      @Inject('MetricsService') public metricsService: MetricsService,
      @Inject('RenderDataService') public renderDataService: RenderDataService,
      private readonly snackBar: MatSnackBar) {}

  ngOnInit() {
    // Check required inputs
    if (!this.parameters) {
      throw new Error('Missing required CollectionParameters');
    }
    if (!this.systemTopology) {
      throw new Error('Missing required SystemTopology');
    }
    if (!this.preview) {
      throw new Error('Missing Observable for preview');
    }
    if (!this.layers) {
      throw new Error('Missing Observable for layers');
    }
    if (!this.viewport) {
      throw new Error('Missing Observable for viewport');
    }
    if (!this.tab) {
      throw new Error('Missing Observable for tab index');
    }
    if (!this.filter) {
      throw new Error('Missing Observable for PID|Command filter');
    }
    if (!this.showMigrations) {
      throw new Error('Missing Observable for migrations render flag');
    }
    if (!this.showSleeping) {
      throw new Error('Missing Observable for sleep state render flag');
    }
    if (!this.maxIntervalCount) {
      throw new Error('Missing Observable for maxIntervalCount');
    }
    if (!this.cpuFilter) {
      throw new Error('Missing Observable for cpu filter');
    }
    if (!this.threadSort) {
      throw new Error('Missing Observable for thread table sort');
    }
    if (!this.expandedThread) {
      throw new Error('Missing Observable for expandedThread');
    }
    // Fetch expanded thread data (if missing) on expanded thread change
    this.expandedThread.subscribe((thread) => {
      this.getExpandedThreadData(thread);
    });
    // TODO(sainsley): Update badge icon on layers change
    // Refetch thread data on threads change.
    this.viewport
        .pipe(debounceTime(300))  // wait at least 300ms between emits
        .subscribe((viewport) => {
          const parameters = this.parameters.value;
          if (parameters) {
            this.getThreadSummaries(parameters, viewport);
          }
        });

    const parameters = this.parameters.value;
    if (parameters) {
      this.eventTypesSubject.next(parameters.ftraceEventTypes);
    }

  }

  ngOnDestroy() {
    // TODO(sainsley): Use switchMap to avoid manually managing subscriptions
    if (this.threadSubscription) {
      this.threadSubscription.unsubscribe();
    }
    this.threads.complete();
    this.expandedThread.complete();
    this.expandedFtraceIntervals.complete();
    this.expandedThreadAntagonists.complete();
    this.expandedThreadIntervals.complete();
    this.selectedSchedEvents.complete();
  }


  /**
   * Fetches thread summaries from backend on viewport change.
   */
  getThreadSummaries(parameters: CollectionParameters, viewport: Viewport):
      void {
    if (this.threadSubscription) {
      this.threadSubscription.unsubscribe();
    }
    this.threadsPending = true;
    const cpuFilter = this.cpuFilter.value;
    const cpus = this.systemTopology.getVisibleCpuIds(viewport, cpuFilter);
    this.threadSubscription =
        this.metricsService.getThreadSummaries(parameters, viewport, cpus)
            .subscribe(
                threads => {
                  this.threads.next(threads);
                  // Recompute expanded threads if the currently expanded thread
                  // is also in the new thread list.
                  if (threads.find(
                          t => t.label &&
                              t.label === this.expandedThread.value)) {
                    this.expandedThread.next(this.expandedThread.value);
                  }
                  this.threadsPending = false;
                },
                (err: HttpErrorResponse) => {
                  const errMsg = createHttpErrorMessage(
                      `Failed to get thread summaries for ${parameters.name}`,
                      err);
                  this.snackBar.open(errMsg, 'Dismiss');
                });
  }

  /**
   * Fetches any missing thread data info on expanded thread change.
   */
  getExpandedThreadData(threadLabel: string|undefined) {
    const parameters = this.parameters.value;
    const threads = this.threads.value;
    if (!threadLabel || !parameters || !threads) {
      return;
    }
    const thread = threads.find(t => t.label === threadLabel);
    if (!thread) {
      return;
    }
    // Fetch events, if missing
    if (!thread.events.length) {
      thread.eventsPending = true;
      this.metricsService
          .getPerThreadEvents(parameters, this.viewport.value, thread)
          .subscribe(
              events => {
                thread.events = events;
                thread.eventsPending = false;
                this.expandedFtraceIntervals.next(events);
              },
              (err: HttpErrorResponse) => {
                thread.eventsPending = false;
                const errMsg = createHttpErrorMessage(
                    `Failed to get thread events for PID: ${thread.pid}`, err);
                this.snackBar.open(errMsg, 'Dismiss');
              });
    } else {
      this.expandedFtraceIntervals.next(thread.events);
    }
    // Fetch thread intervals
    if (!thread.intervals.length) {
      thread.intervalsPending = true;
      const tmpLayers = [new Layer('tmp', 'Thread', [thread.pid])];
      this.renderDataService
          .getPidIntervals(parameters, tmpLayers, this.viewport.value, 0)
          .subscribe(
              layers => {
                thread.intervalsPending = false;
                thread.intervals =
                    layers.map(layer => layer.intervals as ThreadInterval[])
                        .reduce((acc, curr) => acc.concat(curr), []);
                this.expandedThreadIntervals.next(thread.intervals);
              },
              (err: HttpErrorResponse) => {
                thread.intervalsPending = false;
                const errMsg = createHttpErrorMessage(
                    `Failed to get thread intervals for PID: ${thread.pid}`,
                    err);
                this.snackBar.open(errMsg, 'Dismiss');
              });
    } else {
      this.expandedThreadIntervals.next(thread.intervals);
    }
    // Fetch antagonists, if missing
    if (!thread.antagonists.length) {
      thread.antagonistsPending = true;
      this.metricsService
          .getThreadAntagonists(parameters, this.viewport.value, thread)
          .subscribe(
              ants => {
                thread.antagonists = ants;
                thread.antagonistsPending = false;
                this.expandedThreadAntagonists.next(thread.antagonists);
              },
              (err: HttpErrorResponse) => {
                thread.antagonistsPending = false;
                const errMsg = createHttpErrorMessage(
                    `Failed to get thread antagonists for PID: ${thread.pid}`,
                    err);
                this.snackBar.open(errMsg, 'Dismiss');
              });
    } else {
      this.expandedThreadAntagonists.next(thread.antagonists);
    }
  }
}
