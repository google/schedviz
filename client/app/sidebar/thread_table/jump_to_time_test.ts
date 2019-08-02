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
import {async, TestBed} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatFormFieldModule} from '@angular/material/form-field';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {MatPaginator} from '@angular/material/paginator';
import {MatTableModule} from '@angular/material/table';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import {BrowserAnimationsModule} from '@angular/platform-browser/animations';

import {Interval} from '../../models';

import {AntagonistTable} from './antagonist_table';
import {setupAntagonistTable} from './antagonist_table_test';
import {jumpToTime} from './jump_to_time';
import {mockThreads} from './table_helpers_test';
import {ThreadTableModule} from './thread_table_module';

try {
  TestBed.initTestEnvironment(
      BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
  // Ignore exceptions when calling it multiple times.
}

/**
 * Asserts that given table has been jumped to the proper page, such that there
 * is an element with a start time after the targeted jump time.
 */
function expectIntervalOnPage(
    inputIntervals: Interval[], tableData: Interval[], paginator: MatPaginator,
    timeNs: number) {
  const expectedInterval =
      inputIntervals
          .sort(
              (interval1, interval2) =>
                  interval1.startTimeNs - interval2.startTimeNs)
          .find((interval) => interval.startTimeNs >= timeNs);

  expect(expectedInterval).toBeDefined();

  const pageStartIndex = paginator.pageIndex * paginator.pageSize;
  const pageEndIndex = pageStartIndex + paginator.pageSize;
  const pageData = tableData.slice(pageStartIndex, pageEndIndex);

  expect(pageData).toContain(expectedInterval!);
}

describe('jumpToTime', () => {
  beforeEach(async(() => {
    document.body.style.width = '500px';
    document.body.style.height = '500px';
    TestBed
        .configureTestingModule({
          imports: [
            BrowserAnimationsModule, FormsModule, MatFormFieldModule,
            MatInputModule, MatTableModule, MatIconModule, ThreadTableModule
          ],
        })
        .compileComponents();
  }));

  it('should jump to time', () => {
    const fixture = TestBed.createComponent(AntagonistTable);
    const component = fixture.componentInstance;
    setupAntagonistTable(component);
    fixture.detectChanges();

    const threads = mockThreads();
    component.data.next(threads);
    const table = component.dataSource;

    // jump forward
    const firstJumpNs = 3000;
    jumpToTime(table, firstJumpNs);
    expectIntervalOnPage(
        threads, table.filteredData, table.paginator!, firstJumpNs);

    // jump to end
    const secondJumpMs = 1000000000;
    jumpToTime(table, secondJumpMs);
    expect(table.paginator!.hasNextPage()).toBeFalsy();

    // jump back to start
    const thirdJumpMs = 0;
    jumpToTime(table, thirdJumpMs);
    expectIntervalOnPage(
        threads, table.filteredData, table.paginator!, thirdJumpMs);
  });

  it('should enforce sorting', () => {
    const fixture = TestBed.createComponent(AntagonistTable);
    const component = fixture.componentInstance;
    setupAntagonistTable(component);
    fixture.detectChanges();

    const threads = mockThreads();
    component.data.next(threads);
    const table = component.dataSource;

    table.sort!.active = 'endTimeNs';
    table.sort!.direction = 'desc';

    jumpToTime(table, 1000);

    expect(table.sort!.active).toBe('startTimeNs');
    expect(table.sort!.direction).toBe('asc');
  });
});
