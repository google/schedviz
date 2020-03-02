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
import {ChangeDetectorRef, Component, Input, OnInit} from '@angular/core';
import {Sort} from '@angular/material/sort';
import {BehaviorSubject, ReplaySubject} from 'rxjs';
import {takeUntil} from 'rxjs/operators';

import {FtraceInterval} from '../../models';
import {ColorService} from '../../services/color_service';
import * as Duration from '../../util/duration';

import {jumpToTime} from './jump_to_time';
import {SelectableTable} from './selectable_table';

/**
 * The EventTable displays thread events in an Angular 2 material table.
 */
@Component({
  selector: 'event-table',
  styleUrls: ['thread_table.css'],
  templateUrl: 'event_table.ng.html',
})
export class EventTable extends SelectableTable implements OnInit {
  @Input() jumpToTimeNs!: ReplaySubject<number>;
  sort = new BehaviorSubject<Sort>({active: '', direction: ''});

  constructor(
      public colorService: ColorService, protected cdr: ChangeDetectorRef) {
    super(colorService, cdr);
    this.displayedColumns = ['startTimeNs', 'description'];
  }

  ngOnInit() {
    super.ngOnInit();

    if (!this.jumpToTimeNs) {
      throw new Error('Missing Observable for jump to time');
    }

    this.dataSource.sortingDataAccessor = (data, sortHeaderId) =>
        this.getSortingValue(data as FtraceInterval, sortHeaderId);

    this.jumpToTimeNs.pipe(takeUntil(this.unsub$)).subscribe((timeNs) => {
      jumpToTime(this.dataSource, timeNs);
    });
  }

  /**
   * Helper method to format durations as trimmed, human readable strings.
   * @param durationNs the duration value in nanoseconds
   */
  formatTime(durationNs: number) {
    return Duration.getHumanReadableDurationFromNs(durationNs);
  }

  getSortingValue(interval: FtraceInterval, sortHeaderId: string): string
      |number {
    switch (sortHeaderId) {
      case 'startTimeNs':
        return interval.startTimeNs;
      default:
        this.outputErrorThrottled(`Unknown header: ${sortHeaderId}`);
        return '';
    }
  }
}
