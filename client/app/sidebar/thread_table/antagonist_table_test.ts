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
import {ComponentFixture, TestBed, waitForAsync} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatFormFieldModule} from '@angular/material/form-field';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {Sort} from '@angular/material/sort';
import {MatTableModule} from '@angular/material/table';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import {BrowserAnimationsModule} from '@angular/platform-browser/animations';
import {BehaviorSubject, ReplaySubject} from 'rxjs';

import {Interval, Layer} from '../../models';

import {AntagonistTable} from './antagonist_table';
import * as jumpToTime from './jump_to_time';
import {mockThreads, verifyLayerToggle, verifyPreviewOnHover, verifySorting} from './table_helpers_test';
import {ThreadTableModule} from './thread_table_module';

try {
  TestBed.initTestEnvironment(
      BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
  // Ignore exceptions when calling it multiple times.
}

/**
 * Initializes the properties for a given AntagonistTable.
 *
 * @param component is the table component to initialize.
 */
export function setupAntagonistTable(component: AntagonistTable) {
  component.data = new BehaviorSubject<Interval[]>([]);
  component.preview = new BehaviorSubject<Interval|undefined>(undefined);
  component.layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
  component.sort = new BehaviorSubject<Sort>({active: '', direction: ''});
  component.tab = new BehaviorSubject<number>(0);
  component.jumpToTimeNs = new ReplaySubject<number>();
}

function createTableWithMockData(): ComponentFixture<AntagonistTable> {
  const fixture = TestBed.createComponent(AntagonistTable);
  const component = fixture.componentInstance;
  setupAntagonistTable(component);
  component.data.next(mockThreads());
  fixture.detectChanges();

  return fixture;
}

describe('AntagonistTable', () => {
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
    const fixture = createTableWithMockData();
    expect(fixture.componentInstance).toBeTruthy();
  });

  it('should jump to time', () => {
    const fixture = createTableWithMockData();
    const component = fixture.componentInstance;

    const jumpSpy = spyOn(jumpToTime, 'jumpToTime');

    const firstJumpNs = 3000;
    component.jumpToTimeNs.next(firstJumpNs);
    expect(jumpSpy).toHaveBeenCalledWith(component.dataSource, firstJumpNs);
    expect(jumpSpy).toHaveBeenCalledTimes(1);

    const secondJumpNs = 5000;
    component.jumpToTimeNs.next(secondJumpNs);
    expect(jumpSpy).toHaveBeenCalledWith(component.dataSource, secondJumpNs);
    expect(jumpSpy).toHaveBeenCalledTimes(2);
  });

  it('should create layer on toggle click', () => {
    const fixture = createTableWithMockData();
    verifyLayerToggle(fixture.nativeElement, fixture.componentInstance);
  });

  it('should update preview on row hover', () => {
    const fixture = createTableWithMockData();
    verifyPreviewOnHover(fixture.nativeElement, fixture.componentInstance);
  });

  it('should allow sorting', () => {
    const fixture = createTableWithMockData();
    const expectedColumns =
        ['selected', 'pid', 'command', 'startTimeNs', 'endTimeNs', 'duration'];
    verifySorting(
        fixture.nativeElement, fixture.componentInstance.dataSource,
        expectedColumns);
  });
});
