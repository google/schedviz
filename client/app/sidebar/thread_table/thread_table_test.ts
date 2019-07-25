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
import {async, ComponentFixture, TestBed} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatFormFieldModule} from '@angular/material/form-field';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {Sort} from '@angular/material/sort';
import {MatTableModule} from '@angular/material/table';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import {BrowserAnimationsModule} from '@angular/platform-browser/animations';
import {BehaviorSubject} from 'rxjs';

import {CollectionParameters, Interval, Layer, Thread, ThreadEvent, ThreadInterval} from '../../models';

import {ThreadTable} from './thread_table';
import {ThreadTableModule} from './thread_table_module';

try {
  TestBed.initTestEnvironment(
      BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
  // Ignore exceptions when calling it multiple times.
}

function setupThreadTable(component: ThreadTable) {
  component.data = new BehaviorSubject<Interval[]>([]);
  component.preview = new BehaviorSubject<Interval|undefined>(undefined);
  component.layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
  component.sort = new BehaviorSubject<Sort>({active: '', direction: ''});
  component.filter = new BehaviorSubject<string>('');
  component.expandedThread = new BehaviorSubject<Thread|undefined>(undefined);
  component.expandedThreadEvents = new BehaviorSubject<ThreadEvent[]>([]);
  component.expandedThreadAntagonists =
      new BehaviorSubject<ThreadInterval[]>([]);
  component.tab = new BehaviorSubject<number>(0);
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

function mockThreads(): Thread[] {
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

describe('ThreadTable', () => {
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
    const columns = component.displayedColumns;
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
    const toggle =
        element.querySelector('layer-toggle').querySelector('.toggle-root');
    const layersBefore = component.layers.value.length;
    toggle.click();
    fixture.detectChanges();
    const layersAfter = component.layers.value.length;
    expect(layersAfter).toEqual(layersBefore + 1);
    toggle.click();
    fixture.detectChanges();
    expect(layersAfter).toEqual(component.layers.value.length + 1);
  });
});
