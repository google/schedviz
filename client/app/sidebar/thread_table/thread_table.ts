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
import {animate, state, style, transition, trigger} from '@angular/animations';
import {AfterViewInit, ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnInit} from '@angular/core';
import {FormControl} from '@angular/forms';
import {ErrorStateMatcher} from '@angular/material/core';
import {BehaviorSubject, Observable} from 'rxjs';

import {FtraceInterval, Interval, Layer, Thread, ThreadInterval} from '../../models';
import {ColorService} from '../../services/color_service';
import {timeInputToNs} from '../../util/duration';

import {jumpToRow} from './jump_to_time';
import {SelectableTable} from './selectable_table';

const EMBEDDED_PAGE_SIZE = 20;

/**
 * Matcher which enters an error state immediately, ignoring empty fields.
 */
class ImmediateErrorStateMatcher implements ErrorStateMatcher {
  isErrorState(control: FormControl|null): boolean {
    if (!control) {
      return false;
    }

    return control.invalid && control.value !== '';
  }
}

/**
 * The ThreadTable displays thread metrics in a sortable, searchable,
 * Angular 2 material table.
 */
@Component({
  selector: 'thread-table',
  styleUrls: ['thread_table.css'],
  templateUrl: 'thread_table.ng.html',
  animations: [trigger(
      'detailExpand',
      [
        state(
            'collapsed, void',
            style({height: '0px', minHeight: '0', display: 'none'})),
        state('expanded', style({height: '*'})),
        transition(
            'expanded <=> collapsed',
            animate('225ms cubic-bezier(0.4, 0.0, 0.2, 1)')),
        transition(
            'expanded <=> void',
            animate('225ms cubic-bezier(0.4, 0.0, 0.2, 1)'))
      ])],
  host: {'(window:resize)': 'onResize()'},
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class ThreadTable extends SelectableTable implements OnInit,
                                                            AfterViewInit {
  // True if this table is embedded within another table (some features limited)
  @Input() embedded = false;
  @Input() expandedThread!: BehaviorSubject<string|undefined>;
  @Input() expandedFtraceIntervals!: BehaviorSubject<FtraceInterval[]>;
  @Input() expandedThreadAntagonists!: BehaviorSubject<ThreadInterval[]>;
  @Input() expandedThreadIntervals!: BehaviorSubject<ThreadInterval[]>;
  @Input() filter!: BehaviorSubject<string>;
  @Input() tab!: BehaviorSubject<number>;
  @Input() jumpToTimeInput = new BehaviorSubject<string>('');
  jumpToTimeNs = new Observable<number>();
  jumpToTimeMatcher = new ImmediateErrorStateMatcher();
  aggregateWakeups = 0;
  aggregateMigrations = 0;
  aggregateWaitTime = 0;
  aggregateRunTime = 0;
  aggregateSleepTime = 0;
  aggregateUnknownTime = 0;
  rowHeightPx = 28;
  hideResults = false;

  filterPredicate = (data: Interval, filter: string): boolean => {
    if (!filter) {
      return true;
    }
    const filterLt = filter.toLowerCase();
    const thread = data as Thread;
    const pred = thread.command.toLowerCase().includes(filterLt) ||
        `${thread.pid}`.includes(filterLt);
    return this.hideResults ? !pred : pred;
  };

  constructor(
      public colorService: ColorService, protected cdr: ChangeDetectorRef) {
    super(colorService, cdr);
    this.displayedColumns = this.displayedColumns.concat([
      'pid', 'command', 'wakeups', 'migrations', 'waittime', 'runtime',
      'sleeptime', 'unknowntime'
    ]);
  }

  ngOnInit() {
    super.ngOnInit();
    // Check required inputs
    if (!this.filter) {
      throw new Error('Missing Observable for PID|Command filter');
    }
    if (!this.tab && !this.embedded) {
      throw new Error('Missing Observable for selected tab');
    }
    if (!this.expandedThread) {
      throw new Error('Missing Observable for expanded thread');
    }
    if (!this.expandedFtraceIntervals) {
      throw new Error('Missing Observable for expanded thread events');
    }
    if (!this.expandedThreadAntagonists) {
      throw new Error('Missing Observable for expanded thread antagonists');
    }
    if (!this.expandedThreadIntervals) {
      throw new Error('Missing Observable for expanded thread intervals');
    }
    this.dataSource.filterPredicate = this.filterPredicate;
    // Subscribe to filter changes
    this.filter.subscribe((filter: string) => {
      this.dataSource.filter = filter;
      this.updateAggregateMetrics(this.dataSource.filteredData as Thread[]);
    });

    this.jumpToTimeNs = this.jumpToTimeInput.pipe(timeInputToNs);
  }

  ngAfterViewInit() {
    // Update page size on data changes
    this.data.subscribe(() => {
      this.onResize();
      this.updateAggregateMetrics(this.dataSource.filteredData as Thread[]);
      if (this.data.value.length) {
        const row = this.dataSource.filteredData.findIndex(
            t => t.label && t.label === this.expandedThread.value);
        if (row > -1) {
          // Schedule task for next run of the event loop so the table
          // has time to render the new data.
          Promise.resolve().then(() => {
            jumpToRow(this.dataSource, row);
            this.cdr.detectChanges();
          });
        }
      }
    });
  }

  // TODO(sainsley): Fix issues with dynamic page sizes: b/129062093
  onResize() {
    if (!this.embedded) {
      const clientHeight = this.table.nativeElement.clientHeight;
      if (!clientHeight) {
        return;
      }
      // Update pageSize
      // 48px for filter, 51px for header, and 56px for footer
      const headerFooterSize = 155;
      const tableHeightPx = clientHeight - headerFooterSize;
      const pageSize = Math.floor(tableHeightPx / this.rowHeightPx) - 1;
      this.paginator._changePageSize(pageSize);
    } else {
      this.paginator._changePageSize(EMBEDDED_PAGE_SIZE);
    }
    // Detect changes again because when this is called from ngAfterViewInit(),
    // the change cycle has already occurred.
    this.cdr.detectChanges();
  }

  updateAggregateMetrics(threads: Thread[]) {
    this.aggregateWakeups = 0;
    this.aggregateMigrations = 0;
    this.aggregateWaitTime = 0;
    this.aggregateRunTime = 0;
    this.aggregateSleepTime = 0;
    this.aggregateUnknownTime = 0;

    for (const thread of threads) {
      this.aggregateMigrations += thread.migrations;
      this.aggregateWakeups += thread.wakeups;
      this.aggregateWaitTime += thread.waittime;
      this.aggregateRunTime += thread.runtime;
      this.aggregateSleepTime += thread.sleeptime;
      this.aggregateUnknownTime += thread.unknowntime;
    }
  }

  /**
   * Clears the current filter input, given the input element.
   */
  clearFilter(input: HTMLInputElement) {
    this.filter.next('');
  }

  /**
   *  Toggles whether the filter input is applied in inverse.
   */
  toggleFiltering(input: HTMLInputElement) {
    this.hideResults = !this.hideResults;
    this.filter.next(this.filter.value);
  }

  toggleSelection(row: Interval) {
    const thread = row as Thread;
    super.toggleSelection(thread);
  }

  /**
   * Creates a new layer for the current filtered dataset.
   */
  newFilterLayer() {
    const layers = this.layers.value;
    const filter =
        this.hideResults ? '!' + this.filter.value : this.filter.value;
    const rows = this.dataSource.filteredData;
    const name = rows.length === 1 ? rows[0].label : filter;
    const color = this.colorService.getColorFor(name);
    const layer = new Layer(name, 'Thread', rows.map(row => row.id), color);
    layers.unshift(new BehaviorSubject(layer));
    this.layers.next(layers);
  }

  expandThread(thread: Thread) {
    const expandedThreadLabel =
        this.expandedThread.value === thread.label ? undefined : thread.label;
    this.expandedThread.next(expandedThreadLabel);
  }

}
