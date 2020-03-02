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

import {Interval, ThreadInterval} from '../../models';
import {ColorService} from '../../services/color_service';
import * as Duration from '../../util/duration';

import {jumpToTime} from './jump_to_time';
import {SelectableTable} from './selectable_table';

/**
 * The EventTable displays thread intervals in an Angular 2 material table.
 */
@Component({
  selector: 'interval-table',
  styleUrls: ['thread_table.css'],
  templateUrl: 'interval_table.ng.html',
})
export class IntervalTable extends SelectableTable implements OnInit {
  @Input() jumpToTimeNs!: ReplaySubject<number>;
  sort = new BehaviorSubject<Sort>({active: '', direction: ''});

  filter = new BehaviorSubject<string>('');
  hideResults = false;

  private readonly filterPredicate =
      (data: Interval, filter: string): boolean => {
        if (!filter) {
          return true;
        }
        const pred = (data as ThreadInterval)
                         .threadStatesToString()
                         .toLowerCase()
                         .includes(filter.toLowerCase());
        return this.hideResults ? !pred : pred;
      };

  constructor(
      public colorService: ColorService, protected cdr: ChangeDetectorRef) {
    super(colorService, cdr);
    this.displayedColumns =
        ['cpu', 'state', 'startTimeNs', 'endTimeNs', 'duration'];
  }

  ngOnInit() {
    super.ngOnInit();
    if (!this.jumpToTimeNs) {
      throw new Error('Missing Observable for jump to time');
    }

    this.dataSource.sortingDataAccessor = (data, sortHeaderId) =>
        this.getSortingValue(data as ThreadInterval, sortHeaderId);
    this.dataSource.filterPredicate = this.filterPredicate;

    this.jumpToTimeNs.pipe(takeUntil(this.unsub$)).subscribe((timeNs) => {
      jumpToTime(this.dataSource, timeNs);
    });

    this.filter.pipe(takeUntil(this.unsub$)).subscribe((filter: string) => {
      this.dataSource.filter = filter;
    });
    this.dataSource.sort!.sort(
        {id: 'startTimeNs', start: 'asc', disableClear: false});
  }

  /**
   * Helper method to format durations as trimmed, human readable strings.
   * @param durationNs the duration value in nanoseconds
   */
  formatTime(durationNs: number) {
    return Duration.getHumanReadableDurationFromNs(durationNs);
  }

  /**
   *  Toggles whether the filter input is applied in inverse.
   */
  toggleFiltering() {
    this.hideResults = !this.hideResults;
    this.filter.next(this.filter.value);
  }

  /**
   * Clears the current filter input, given the input element.
   */
  clearFilter() {
    this.filter.next('');
  }

  getSortingValue(interval: ThreadInterval, sortHeaderId: string): string
      |number {
    switch (sortHeaderId) {
      case 'cpu':
        return interval.cpu;
      case 'state':
        return interval.state;
      case 'startTimeNs':
        return interval.startTimeNs;
      case 'endTimeNs':
        return interval.endTimeNs;
      case 'duration':
        return interval.duration;
      default:
        this.outputErrorThrottled(`Unknown header: ${sortHeaderId}`);
        return '';
    }
  }
}
