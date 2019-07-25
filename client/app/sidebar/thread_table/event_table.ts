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
import {ChangeDetectorRef, Component} from '@angular/core';

import {ColorService} from '../../services/color_service';
import * as Duration from '../../util/duration';

import {SelectableTable} from './selectable_table';

/**
 * The EventTable displays thread events in an Angular 2 material table.
 */
@Component({
  selector: 'event-table',
  styleUrls: ['thread_table.css'],
  templateUrl: 'event_table.ng.html',
})
export class EventTable extends SelectableTable {
  constructor(
      public colorService: ColorService, protected cdr: ChangeDetectorRef) {
    super(colorService, cdr);
    this.displayedColumns = ['startTimeNs', 'description'];
  }

  /**
   * Helper method to format durations as trimmed, human readable strings.
   * @param durationNs the duration value in nanoseconds
   */
  formatTime(durationNs: number) {
    return Duration.getHumanReadableDurationFromNs(durationNs, 'ms');
  }
}
