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
import {MatTableDataSource} from '@angular/material/table';

import {CollectionParameters, Interval, Thread} from '../../models';

/**
 * Verifies that a given root element contains the expected columns,
 * which when clicked correctly toggle sorting on the given table.
 *
 * @param root is an element which is expected to contain the given table
 * @param table is a sortable table of interval data
 * @param expectedColumns is a list of expected sortable columns, in the
 *     expected order of appearance
 */
export function verifySorting(
    root: Element, table: MatTableDataSource<Interval>,
    expectedColumns: string[]) {
  expect(table.sort).not.toBeNull();
  const sortButtons = root.querySelectorAll('.mat-sort-header-button');
  expect(sortButtons.length).toEqual(expectedColumns.length);

  for (let i = 0; i < sortButtons.length; i++) {
    const sortButton = sortButtons[i] as HTMLButtonElement;
    const expectedSortId = expectedColumns[i];

    sortButton.click();
    expect(table.sort!.active).toBe(expectedSortId);
    expect(table.sort!.direction).toBe('asc');

    sortButton.click();
    expect(table.sort!.active).toBe(expectedSortId);
    expect(table.sort!.direction).toBe('desc');
  }
}

/**
 * Locates a layer toggle button under the given root element.
 *
 * @param root is an element which contains a layer toggle button.
 */
export function getToggleButton(root: Element): HTMLDivElement {
  const layerToggle = root.querySelector('layer-toggle');
  expect(layerToggle).not.toBeNull();

  const toggleRoot = layerToggle!.querySelector('.toggle-root');
  expect(toggleRoot).not.toBeNull();
  expect(toggleRoot instanceof HTMLDivElement).toBe(true);

  return toggleRoot as HTMLDivElement;
}

/**
 * Generates mock threads
 */
export function mockThreads(): Thread[] {
  const parameters = mockParameters();
  const threadCount = 500;
  const threadData: Thread[] = [];
  for (let i = 0; i < threadCount; i++) {
    const cpus = [];
    const cpuCount = parameters.size;
    for (let ii = 0; ii < cpuCount; ii++) {
      cpus.push(getRandomInt(cpuCount));
    }
    threadData.push(new Thread(
        parameters, getRandomInt(10000), cpus, getRandomCommand(),
        getRandomInt(100), getRandomInt(100), getRandomFloat(500000),
        getRandomFloat(500000), getRandomFloat(500000),
        getRandomFloat(500000)));
  }
  return threadData;
}

function mockParameters(): CollectionParameters {
  const startTime = 1540768090000;
  const endTime = 1540768139000;
  const cpuCount = 72;
  const cpus = [];
  for (let i = 0; i < cpuCount; i++) {
    cpus.push(i);
  }
  return new CollectionParameters('foo', cpus, startTime, endTime);
}

function getRandomInt(max: number) {
  return Math.floor(Math.random() * Math.floor(max));
}

function getRandomFloat(max: number) {
  return Math.random() * max;
}

function getRandomCommand() {
  let text = '';
  const possible = 'abcdefghijklmnopqrstuvwxyz0123456789';
  const commandLen = 5 + getRandomInt(8);
  for (let i = 0; i < commandLen; i++) {
    text += possible.charAt(Math.floor(Math.random() * possible.length));
  }
  return text;
}
