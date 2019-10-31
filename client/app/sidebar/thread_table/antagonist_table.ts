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
import {ChangeDetectorRef, Component, Input, OnDestroy, OnInit} from '@angular/core';
import {BehaviorSubject, combineLatest, ReplaySubject, Subject} from 'rxjs';
import {filter, takeUntil} from 'rxjs/operators';

import {ColorService} from '../../services/color_service';

import {jumpToTime} from './jump_to_time';
import {SelectableTable} from './selectable_table';

/**
 * A place holder for a table showing thread antagonists.
 */
@Component({
  selector: 'antagonist-table',
  styleUrls: ['./thread_table.css'],
  templateUrl: 'antagonist_table.ng.html',
})
export class AntagonistTable extends SelectableTable implements OnInit,
                                                                OnDestroy {
  @Input() jumpToTimeNs!: ReplaySubject<number>;
  private readonly unsub$ = new Subject<void>();

  constructor(
      public colorService: ColorService, protected cdr: ChangeDetectorRef) {
    super(colorService, cdr);
    this.displayedColumns = this.displayedColumns.concat(
        ['pid', 'command', 'startTimeNs', 'endTimeNs', 'duration']);
  }

  ngOnInit() {
    super.ngOnInit();

    if (!this.jumpToTimeNs) {
      throw new Error('Missing Observable for jump to time');
    }

    this.jumpToTimeNs.pipe(takeUntil(this.unsub$)).subscribe((timeNs) => {
      jumpToTime(this.dataSource, timeNs);
    });
  }

  ngOnDestroy() {
    this.unsub$.next();
    this.unsub$.complete();
  }
}
