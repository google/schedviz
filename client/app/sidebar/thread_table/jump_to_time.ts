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
import {MatSort, Sort} from '@angular/material/sort';
import {MatTableDataSource} from '@angular/material/table';

import {Interval} from '../../models/interval';

/**
 * Jumps a given table of intervals to the first page which contains an interval
 * starting at or after a given time. If such an interval does not exist, the
 * table will be moved to the last page.
 *
 * @param table is a table of intervals to be jumped
 * @param timeNs is the time to jump to
 */
export function jumpToTime(
    table: MatTableDataSource<Interval>, timeNs: number) {
  const paginator = table.paginator;
  if (!paginator) {
    console.warn('Unable to jump to time, table not paginated.');
    return;
  }

  if (!table.sort) {
    console.warn('Unable to jump to time, table not sortable.');
    return;
  }

  // In order to jump to a time the timestamps must be in ascending
  // order
  ensureSort(table.sort, {active: 'startTimeNs', direction: 'asc'});

  const targetIntervalIndex =
      table.data.findIndex((interval) => interval.startTimeNs >= timeNs);

  if (targetIntervalIndex < 0) {
    // If a time was requested greater than any existing timestamp go to the
    // end
    paginator.lastPage();
    return;
  }

  const targetPageIndex = Math.floor(targetIntervalIndex / paginator.pageSize);

  paginator.pageIndex = targetPageIndex;

  // Due to a bug (https://github.com/angular/components/issues/12620, to be
  // fixed by https://github.com/angular/components/pull/12586) the page
  // change event isn't fired automatically, so we have to do it manually
  paginator.page.next({
    previousPageIndex: paginator.pageIndex,
    pageIndex: targetPageIndex,
    pageSize: paginator.pageSize,
    length: paginator.length
  });
}

function ensureSort(sortContainer: MatSort, sort: Sort) {
  if (!sortContainer.sortables.has(sort.active)) {
    console.warn('Unknown sort key', sort.active);
    return;
  }

  // due to a bug (https://github.com/angular/components/issues/10242), we
  // need to manually invoke the sort
  if (sortContainer.active !== sort.active ||
      sortContainer.direction !== sort.direction) {
    sortContainer.sort(
        {id: sort.active, start: sortContainer.start, disableClear: true});
  }
}
