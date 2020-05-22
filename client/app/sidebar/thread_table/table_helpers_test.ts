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

import {CollectionParameters, Interval, Thread, ThreadInterval} from '../../models';
import {ThreadState} from '../../models/render_data_services';
import {SelectableTable} from './selectable_table';

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
  const sortButtons = root.querySelectorAll('.mat-sort-header-container');
  expect(sortButtons.length).toEqual(expectedColumns.length);
  const paginator = table.paginator;
  expect(paginator).not.toBeNull();

  for (let i = 0; i < sortButtons.length; i++) {
    const sortButton = sortButtons[i] as HTMLButtonElement;
    const expectedSortId = expectedColumns[i];

    paginator!.lastPage();
    sortButton.click();
    expect(table.sort!.active).toBe(expectedSortId);
    expect(table.sort!.direction).toBe('asc');
    expect(paginator!.pageIndex).toBe(0);

    paginator!.lastPage();
    sortButton.click();
    expect(table.sort!.active).toBe(expectedSortId);
    expect(table.sort!.direction).toBe('desc');
    expect(paginator!.pageIndex).toBe(0);
  }
}

/**
 * Verifies that a preview action is generated when hovering over a row
 * in the given table.
 *
 * @param root is an element which contains the table
 * @param selectableTable is a table containing a row to preview
 */
export function verifyPreviewOnHover(
    root: Element, selectableTable: SelectableTable) {
  const previewSpy = jasmine.createSpy('previewSpy');
  selectableTable.preview.subscribe(previewSpy);

  // component should be initialized to have no active preview
  expect(previewSpy).toHaveBeenCalledTimes(1);
  expect(previewSpy).toHaveBeenCalledWith(undefined);

  // hover over the first row of the table to preview it
  const rowElement = root.querySelector('mat-row');
  expect(rowElement).not.toBeNull();

  rowElement!.dispatchEvent(new MouseEvent('mouseenter'));
  expect(previewSpy).toHaveBeenCalledTimes(2);
  expect(previewSpy).toHaveBeenCalledWith(selectableTable.data.value[0]);
}

/**
 * Verifies that layer toggle events are generated upon clicking the toggle
 * button for a row in the given table.
 *
 * @param root is an element which contains the table
 * @param selectableTable is a table containing a row to toggle
 */
export function verifyLayerToggle(
    root: Element, selectableTable: SelectableTable) {
  const toggleButton = getToggleButton(root);

  const layerSpy = jasmine.createSpy('layerSpy');
  selectableTable.layers.subscribe(layerSpy);

  // component should be initialized to have no active layers
  expect(layerSpy).toHaveBeenCalledTimes(1);
  expect(layerSpy).toHaveBeenCalledWith([]);
  const layerSubjects = layerSpy.calls.mostRecent().args[0];
  expect(layerSubjects.length).toEqual(0);

  // toggle the layer on
  toggleButton.click();
  expect(layerSpy).toHaveBeenCalledTimes(2);

  // the only active layer should now contain the first thread of the table
  const toggleOnLayerSubjects = layerSpy.calls.mostRecent().args[0];
  expect(toggleOnLayerSubjects.length).toEqual(1);
  const toggleOnLayerElementSpy = jasmine.createSpy('toggleOnLayerElementSpy');
  toggleOnLayerSubjects[0].subscribe(toggleOnLayerElementSpy);
  expect(toggleOnLayerElementSpy).toHaveBeenCalledTimes(1);
  expect(toggleOnLayerElementSpy.calls.mostRecent().args[0].intervals)
      .toEqual(jasmine.arrayContaining([selectableTable.data.value[0]]));

  // toggle the layer off
  toggleButton.click();
  expect(layerSpy).toHaveBeenCalledTimes(3);

  // there should be no active layers
  const toggleOffLayerSubjects = layerSpy.calls.mostRecent().args[0];
  expect(toggleOffLayerSubjects.length).toEqual(0);
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

/**
 * Generates mock thread intervals
 */
export function mockThreadIntervals(): ThreadInterval[] {
  const parameters = mockParameters();
  const intervalCount = 1000;
  const threadData: ThreadInterval[] = [];
  let startTime = 0;
  for (let i = 0; i < intervalCount; i++) {
    const cpuCount = parameters.size;
    const endTime = startTime + getRandomInt(1000);
    threadData.push(new ThreadInterval(
        parameters, getRandomInt(cpuCount), startTime, endTime,
        getRandomInt(100), getRandomCommand(), [{
          duration: getRandomInt(endTime - startTime),
          state: getRandomState(),
          includesSyntheticTransitions: false,
          droppedEventIDs: [],
          thread: {
            priority: getRandomInt(100),
            command: getRandomCommand(),
            pid: getRandomInt(100)
          }
        }]));
    startTime = endTime;
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

function getRandomState() {
  switch (getRandomInt(3)) {
    case 0:
      return ThreadState.UNKNOWN_STATE;
    case 1:
      return ThreadState.RUNNING_STATE;
    case 2:
      return ThreadState.WAITING_STATE;
    case 3:
      return ThreadState.SLEEPING_STATE;
    default:
      return ThreadState.UNKNOWN_STATE;
  }
}
