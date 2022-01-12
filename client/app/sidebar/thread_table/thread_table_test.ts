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
import {fakeAsync, TestBed, tick, waitForAsync} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatFormFieldModule} from '@angular/material/form-field';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {Sort} from '@angular/material/sort';
import {MatTableModule} from '@angular/material/table';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import {BrowserAnimationsModule} from '@angular/platform-browser/animations';
import {BehaviorSubject} from 'rxjs';

import {FtraceInterval, Interval, Layer, Thread, ThreadInterval} from '../../models';

import {getToggleButton, mockThreads, verifySorting} from './table_helpers_test';
import {ThreadTable} from './thread_table';
import {ThreadTableModule} from './thread_table_module';

try {
  TestBed.initTestEnvironment(
      BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
  // Ignore exceptions when calling it multiple times.
}

// Delay time which will guarantee flush of jump input
const JUMP_INPUT_DEBOUNCE_MS = 1000;

function setupThreadTable(component: ThreadTable) {
  component.data = new BehaviorSubject<Interval[]>([]);
  component.preview = new BehaviorSubject<Interval|undefined>(undefined);
  component.layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
  component.sort = new BehaviorSubject<Sort>({active: '', direction: ''});
  component.filter = new BehaviorSubject<string>('');
  component.expandedThread = new BehaviorSubject<string|undefined>(undefined);
  component.expandedFtraceIntervals = new BehaviorSubject<FtraceInterval[]>([]);
  component.expandedThreadAntagonists =
      new BehaviorSubject<ThreadInterval[]>([]);
  component.expandedThreadIntervals = new BehaviorSubject<ThreadInterval[]>([]);
  component.tab = new BehaviorSubject<number>(0);
}

describe('ThreadTable', () => {
  beforeEach(waitForAsync(() => {
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

  it('should create', () => {
    const fixture = TestBed.createComponent(ThreadTable);
    const component = fixture.componentInstance;
    setupThreadTable(component);
    component.data.next(mockThreads());
    fixture.detectChanges();
    expect(component).toBeTruthy();
  });

  it('should render rows', () => {
    const fixture = TestBed.createComponent(ThreadTable);
    const component = fixture.componentInstance;
    setupThreadTable(component);
    const element = fixture.nativeElement;
    element.style.height = '500px';
    component.onResize();
    fixture.detectChanges();
    component.data.next(mockThreads());
    fixture.detectChanges();
    const rowsDom = fixture.nativeElement.querySelectorAll('.thread-row');
    expect(rowsDom.length).toEqual(component.paginator.pageSize);
  });

  it('should update page size', () => {
    const fixture = TestBed.createComponent(ThreadTable);
    const component = fixture.componentInstance;
    setupThreadTable(component);
    component.data.next(mockThreads());
    fixture.detectChanges();
    const element = fixture.nativeElement;
    element.style.height = '500px';
    component.onResize();
    fixture.detectChanges();
    const pageCountBefore = component.paginator.pageSize;
    element.style.height = '1000px';
    component.onResize();
    fixture.detectChanges();
    const pageCountAfter = component.paginator.pageSize;
    expect(pageCountBefore).toBeLessThan(pageCountAfter);
  });

  it('should filter', () => {
    const fixture = TestBed.createComponent(ThreadTable);
    const component = fixture.componentInstance;
    setupThreadTable(component);
    component.data.next(mockThreads());
    fixture.detectChanges();
    const element = fixture.nativeElement;
    element.style.height = '5000px';
    component.onResize();
    fixture.detectChanges();
    expect(component.dataSource.data.length)
        .toBeGreaterThanOrEqual(component.pageSize);
    const rowCount = component.dataSource.filteredData.length;
    component.filter.next('26');
    fixture.detectChanges();
    const filteredRowCount = component.dataSource.filteredData.length;
    expect(filteredRowCount).toBeLessThan(rowCount);
  });

  it('should create layer on toggle click', () => {
    const fixture = TestBed.createComponent(ThreadTable);
    const component = fixture.componentInstance;
    setupThreadTable(component);
    component.data.next(mockThreads());
    fixture.detectChanges();
    const element = fixture.nativeElement;
    element.style.height = '500px';
    component.onResize();
    fixture.detectChanges();
    const toggle = getToggleButton(element);
    const layersBefore = component.layers.value.length;
    toggle.click();
    fixture.detectChanges();
    const layersAfter = component.layers.value.length;
    expect(layersAfter).toEqual(layersBefore + 1);
    toggle.click();
    fixture.detectChanges();
    expect(layersAfter).toEqual(component.layers.value.length + 1);
  });

  it('should debounce jump', fakeAsync(() => {
       const fixture = TestBed.createComponent(ThreadTable);
       const component = fixture.componentInstance;
       setupThreadTable(component);
       component.data.next(mockThreads());
       fixture.detectChanges();

       const jumpSpy = jasmine.createSpy('jumpSpy');
       component.jumpToTimeNs.subscribe(jumpSpy);

       component.jumpToTimeInput.next(`100 ns`);
       component.jumpToTimeInput.next(`200 ns`);
       component.jumpToTimeInput.next(`300 ns`);
       component.jumpToTimeInput.next(`400 ns`);

       tick(JUMP_INPUT_DEBOUNCE_MS);
       expect(jumpSpy).toHaveBeenCalledTimes(1);
       expect(jumpSpy).toHaveBeenCalledWith(400);

       component.jumpToTimeInput.next(`500 ns`);
       component.jumpToTimeInput.next(`600 ns`);
       tick(JUMP_INPUT_DEBOUNCE_MS);

       expect(jumpSpy).toHaveBeenCalledTimes(2);
       expect(jumpSpy).toHaveBeenCalledWith(600);
     }));

  it('should not propagate invalid input', fakeAsync(() => {
       const fixture = TestBed.createComponent(ThreadTable);
       const component = fixture.componentInstance;
       setupThreadTable(component);
       component.data.next(mockThreads());
       fixture.detectChanges();

       const jumpSpy = jasmine.createSpy('jumpSpy');
       component.jumpToTimeNs.subscribe(jumpSpy);

       component.jumpToTimeInput.next('10 ms');
       tick(JUMP_INPUT_DEBOUNCE_MS);
       expect(jumpSpy).toHaveBeenCalledTimes(1);

       component.jumpToTimeInput.next('invalid input');
       tick(JUMP_INPUT_DEBOUNCE_MS);
       expect(jumpSpy).toHaveBeenCalledTimes(1);
  }));

  it('should allow sorting', () => {
    const fixture = TestBed.createComponent(ThreadTable);
    const component = fixture.componentInstance;
    setupThreadTable(component);
    component.data.next(mockThreads());
    fixture.detectChanges();

    const expectedColumns = [
      'selected', 'pid', 'command', 'wakeups', 'migrations', 'waittime',
      'runtime', 'sleeptime', 'unknowntime'
    ];
    verifySorting(fixture.nativeElement, component.dataSource, expectedColumns);
  });
});
