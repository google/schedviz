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
import {AfterViewInit, ChangeDetectionStrategy, ChangeDetectorRef, Component, OnInit} from '@angular/core';

import {ColorService} from '../../services/color_service';

import {SelectableTable} from './selectable_table';

/**
 * The SchedEventsTable displays a list of available ktraced event types for
 * this collection and a button to create a dedicated layer for a given type.
 */
@Component({
  selector: 'sched-events-table',
  styleUrls: ['thread_table.css'],
  templateUrl: 'sched_events_table.ng.html',
  host: {'(window:resize)': 'onResize()'},
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class SchedEventsTable extends SelectableTable implements OnInit,
                                                                 AfterViewInit {
  rowHeightPx = 24;
  constructor(
      public colorService: ColorService, protected cdr: ChangeDetectorRef) {
    super(colorService, cdr);
    this.displayedColumns = this.displayedColumns.concat(['name']);
  }

  ngOnInit() {
    super.ngOnInit();
  }

  ngAfterViewInit() {
    this.data.subscribe(() => {
      this.onResize();
    });
  }

  toggleEventType(row: string) {
    super.toggleLayer(row, 'SchedEvent');
  }

  onResize() {
    const clientHeight = this.table.nativeElement.clientHeight;
    if (!clientHeight) {
      return;
    }
    // Update pageSize
    const headerFooterSize = 107;  // 51px for header, 56px for footer
    const tableHeightPx = clientHeight - headerFooterSize;
    const pageSize = Math.floor(tableHeightPx / this.rowHeightPx) - 1;
    this.paginator._changePageSize(pageSize);
    // Detect changes again because when this is called from ngAfterViewInit(),
    // the change cycle has already occurred.
    this.cdr.detectChanges();
  }
}
