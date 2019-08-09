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
import {ChangeDetectionStrategy, Component, Inject, Input, OnInit} from '@angular/core';
import {MatDialog} from '@angular/material/dialog';
import {MatSnackBar} from '@angular/material/snack-bar';
import {BehaviorSubject, combineLatest} from 'rxjs';
import {debounceTime} from 'rxjs/operators';

import {CollectionParameters, UtilizationMetrics} from '../../models';
import {MetricsService} from '../../services/metrics_service';
import {createHttpErrorMessage, Viewport} from '../../util';

/**
 * The MetricsOverlay shows various metrics about the current collection
 * as an overlay.
 */
@Component({
  selector: 'metrics-overlay',
  styleUrls: ['metrics_overlay.css'],
  templateUrl: 'metrics_overlay.ng.html',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class MetricsOverlay implements OnInit {
  @Input() visibleCpus!: BehaviorSubject<number[]>;
  @Input() parameters!: BehaviorSubject<CollectionParameters|undefined>;
  @Input() viewport!: BehaviorSubject<Viewport>;

  utilizationMetrics =
      new BehaviorSubject<UtilizationMetrics|undefined>(undefined);

  viewportDuration = new BehaviorSubject<number>(1);

  constructor(
      @Inject('MetricsService') public metricsService: MetricsService,
      public dialog: MatDialog,
      private readonly snackBar: MatSnackBar,
  ) {}

  ngOnInit() {
    if (!this.visibleCpus) {
      throw new Error('Missing required visibleCpus');
    }
    if (!this.parameters) {
      throw new Error('Missing required CollectionParameters');
    }
    if (!this.viewport) {
      throw new Error('Missing Observable for viewport');
    }
    combineLatest(this.visibleCpus, this.parameters, this.viewport)
        .pipe(debounceTime(500))
        .subscribe(() => {
          this.fetchMetrics();
        });
  }

  fetchMetrics() {
    const viewport = this.viewport.value;
    if (!this.parameters.value || !viewport) {
      return;
    }
    const {startTimeNs, endTimeNs} = this.parameters.value;
    const viewportStartTime = viewport.getStartTime(startTimeNs, endTimeNs);
    const viewportEndTime = viewport.getEndTime(startTimeNs, endTimeNs);
    const name = this.parameters.value.name;
    this.metricsService
        .getUtilizationMetrics(
            name,
            this.visibleCpus.value,
            viewportStartTime,
            viewportEndTime,
            )
        .subscribe(
            um => {
              if (um instanceof UtilizationMetrics) {
                this.utilizationMetrics.next(um);
                const viewportDuration = viewportEndTime - viewportStartTime;
                this.viewportDuration.next(viewportDuration);
              }
            },
            (err: HttpErrorResponse) => {
              const errMsg = createHttpErrorMessage(
                  `Failed to get utilization metrics for collection: ${name}`,
                  err);
              this.snackBar.open(errMsg, 'Dismiss');
            });
  }

  showHelp() {
    this.dialog.open(DialogMetricsHelp);
  }
}

/**
 * A dialog containing help text about Idle While Overloaded Metrics.
 */
@Component({
  selector: 'dialog-metrics-help',
  templateUrl: 'dialog_metrics_help.ng.html',
  styles: [`
    :host {
      display: block;
      max-width: 500px;
    }
  `],
})
export class DialogMetricsHelp {
}
