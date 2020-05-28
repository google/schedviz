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
import {HttpErrorResponse} from '@angular/common/http';
import {ComponentFixture, fakeAsync, TestBed, tick} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatFormFieldModule} from '@angular/material/form-field';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {MatSnackBar} from '@angular/material/snack-bar';
import {Sort} from '@angular/material/sort';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import {BehaviorSubject, throwError} from 'rxjs';

import {CollectionParameters, Interval, Layer} from '../models';
import {LocalMetricsService, MetricsService} from '../services/metrics_service';
import {LocalRenderDataService, RenderDataService} from '../services/render_data_service';
import {SystemTopology, Viewport} from '../util';

import {Sidebar} from './sidebar';
import {SidebarModule} from './sidebar_module';
import {mockThreads} from './thread_table/table_helpers_test';

try {
  TestBed.initTestEnvironment(
      BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
  // Ignore exceptions when calling it multiple times.
}

// Delay time which will guarantee flush of viewport update
const VIEWPORT_UPDATE_DEBOUNCE_MS = 1000;

function setupSidebar(component: Sidebar) {
  component.parameters = new BehaviorSubject<CollectionParameters|undefined>(
      new CollectionParameters('collection_params', [], 0, 100));
  component.expandedThread = new BehaviorSubject<string|undefined>(undefined);
  component.systemTopology = new SystemTopology([]);
  component.preview = new BehaviorSubject<Interval|undefined>(undefined);
  component.layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
  component.viewport = new BehaviorSubject<Viewport>(new Viewport());
  component.viewport.value.updateSize(50, 50);
  component.tab = new BehaviorSubject<number>(0);
  component.threadSort = new BehaviorSubject<Sort>({active: '', direction: ''});
  component.filter = new BehaviorSubject<string>('');
  component.showMigrations = new BehaviorSubject<boolean>(true);
  component.showSleeping = new BehaviorSubject<boolean>(true);
  component.maxIntervalCount = new BehaviorSubject<number>(0);
  component.cpuFilter = new BehaviorSubject<string>('');
}

function createSidebarWithMockData(): ComponentFixture<Sidebar> {
  const fixture = TestBed.createComponent(Sidebar);
  const component = fixture.componentInstance;
  setupSidebar(component);
  fixture.detectChanges();

  return fixture;
}

function mockMetricServiceHttpError(functionToMock: keyof MetricsService):
    jasmine.Spy {
  // Set up failing request
  const metricsService = TestBed.get('MetricsService') as MetricsService;
  return spyOn(metricsService, functionToMock)
      .and.returnValue(
          throwError(new HttpErrorResponse({error: 'lorem ipsum'})));
}

function mockRenderDataServiceHttpError(
    functionToMock: keyof RenderDataService): jasmine.Spy {
  // Set up failing request

  // Not deprecated until Angular 9.0.0, which isn't GA yet.
  // tslint:disable:deprecation
  const renderDataService =
      TestBed.get('RenderDataService') as RenderDataService;
  // tslint:enable:deprecation
  return spyOn(renderDataService, functionToMock)
      .and.returnValue(
          throwError(new HttpErrorResponse({error: 'lorem ipsum'})));
}

describe('Sidebar', () => {
  beforeEach(async () => {
    document.body.style.width = '500px';
    document.body.style.height = '500px';
    await TestBed
        .configureTestingModule({
          imports: [
            FormsModule, MatFormFieldModule, MatInputModule, MatIconModule,
            SidebarModule
          ],
          providers: [
            {provide: 'MetricsService', useClass: LocalMetricsService},
            {provide: 'RenderDataService', useClass: LocalRenderDataService},
          ],
        })
        .compileComponents();
  });

  it('should create', () => {
    const fixture = createSidebarWithMockData();
    expect(fixture.componentInstance).toBeTruthy();
  });

  it('should update threads on viewport change', fakeAsync(() => {
       const fixture = createSidebarWithMockData();
       const component = fixture.componentInstance;

       const metricsService = TestBed.get('MetricsService') as MetricsService;
       const expectedThreads = mockThreads().slice(0, 10);
       spyOn(metricsService, 'getThreadSummaries')
           .and.returnValue(new BehaviorSubject(expectedThreads));

       const threadSpy = jasmine.createSpy('threadSpy');
       component.threads.subscribe(threadSpy);
       expect(threadSpy).toHaveBeenCalledTimes(1);

       // Trigger viewport update. Calls should be debounced
       const updatedViewport = new Viewport(component.viewport.value);
       updatedViewport.updateSize(50, 50);
       component.viewport.next(updatedViewport);
       component.viewport.next(updatedViewport);
       component.viewport.next(updatedViewport);
       component.viewport.next(updatedViewport);
       expect(threadSpy).toHaveBeenCalledTimes(1);

       tick(VIEWPORT_UPDATE_DEBOUNCE_MS);

       // Verify thread data was properly updated
       expect(threadSpy).toHaveBeenCalledTimes(2);
       expect(threadSpy).toHaveBeenCalledWith(
           jasmine.arrayContaining(expectedThreads));
     }));

  it('should update expanded thread', async () => {
    const fixture = createSidebarWithMockData();
    const component = fixture.componentInstance;
    await fixture.whenStable();

    const expandedFtraceIntervalsSpy =
        jasmine.createSpy('expandedFtraceIntervalsSpy');
    component.expandedFtraceIntervals.subscribe(expandedFtraceIntervalsSpy);

    const expandedThreadIntervalsSpy =
        jasmine.createSpy('expandedThreadIntervalsSpy');
    component.expandedThreadIntervals.subscribe(expandedThreadIntervalsSpy);

    const expandedThreadAntagonistsSpy =
        jasmine.createSpy('expandedThreadAntagonistsSpy');
    component.expandedThreadAntagonists.subscribe(expandedThreadAntagonistsSpy);

    // Simulate thread expansion
    const threadToExpand = component.threads.value[3];
    component.expandedThread.next(threadToExpand.label);

    expect(expandedFtraceIntervalsSpy).toHaveBeenCalled();
    expect(expandedFtraceIntervalsSpy)
        .toHaveBeenCalledWith(jasmine.arrayContaining(threadToExpand.events));

    expect(expandedThreadIntervalsSpy).toHaveBeenCalled();
    expect(expandedThreadIntervalsSpy)
        .toHaveBeenCalledWith(
            jasmine.arrayContaining(threadToExpand.intervals));

    expect(expandedThreadAntagonistsSpy).toHaveBeenCalled();
    expect(expandedThreadAntagonistsSpy)
        .toHaveBeenCalledWith(
            jasmine.arrayContaining(threadToExpand.antagonists));
  });

  it('should surface error message upon failure of thread summary request',
     fakeAsync(() => {
       const fixture = createSidebarWithMockData();
       const component = fixture.componentInstance;

       mockMetricServiceHttpError('getThreadSummaries');
       const snackBar = TestBed.get(MatSnackBar) as MatSnackBar;
       const snackBarSpy = spyOn(snackBar, 'openFromComponent');

       // Failed request should occur during viewport update
       const updatedViewport = new Viewport(component.viewport.value);
       updatedViewport.updateSize(0, 50);
       component.viewport.next(updatedViewport);
       tick(VIEWPORT_UPDATE_DEBOUNCE_MS);

       expect(snackBarSpy).toHaveBeenCalledTimes(1);
       const componentParameters = component.parameters.value;
       expect(componentParameters).toBeTruthy();
       const actualError = snackBarSpy.calls.mostRecent().args[1].data.summary;
       expect(actualError)
           .toContain(`Failed to get thread summaries for ${
               componentParameters!.name}`);
     }));

  it('should surface error message upon failure of thread event request',
     async () => {
       const fixture = createSidebarWithMockData();
       const component = fixture.componentInstance;
       await fixture.whenStable();

       mockMetricServiceHttpError('getPerThreadEvents');
       const snackBar = TestBed.get(MatSnackBar) as MatSnackBar;
       const snackBarSpy = spyOn(snackBar, 'openFromComponent');

       // Failed request should occur during thread expansion
       const threadToExpand = component.threads.value[0];
       component.expandedThread.next(threadToExpand.label);

       expect(snackBarSpy).toHaveBeenCalledTimes(1);
       const actualError = snackBarSpy.calls.mostRecent().args[1].data.summary;
       expect(actualError)
           .toContain(
               `Failed to get thread events for PID: ${threadToExpand.pid}`);
     });

  it('should surface error message upon failure of thread intervals request',
     async () => {
       const fixture = createSidebarWithMockData();
       const component = fixture.componentInstance;
       await fixture.whenStable();

       mockRenderDataServiceHttpError('getPidIntervals');

       // Not deprecated until Angular 9.0.0, which isn't GA yet.
       // tslint:disable-next-line:deprecation
       const snackBar = TestBed.get(MatSnackBar) as MatSnackBar;
       const snackBarSpy = spyOn(snackBar, 'openFromComponent');

       // Failed request should occur during thread expansion
       const threadToExpand = component.threads.value[2];
       component.expandedThread.next(threadToExpand.label);

       expect(snackBarSpy).toHaveBeenCalledTimes(1);
       const actualError = snackBarSpy.calls.mostRecent().args[1].data.summary;
       expect(actualError)
           .toContain(
               `Failed to get thread intervals for PID: ${threadToExpand.pid}`);
     });

  it('should surface error message upon failure of thread antagonists request',
     async () => {
       const fixture = createSidebarWithMockData();
       const component = fixture.componentInstance;
       await fixture.whenStable();

       mockMetricServiceHttpError('getThreadAntagonists');
       const snackBar = TestBed.get(MatSnackBar) as MatSnackBar;
       const snackBarSpy = spyOn(snackBar, 'openFromComponent');

       // Failed request should occur during thread expansion
       const threadToExpand = component.threads.value[2];
       component.expandedThread.next(threadToExpand.label);

       expect(snackBarSpy).toHaveBeenCalledTimes(1);
       const actualError = snackBarSpy.calls.mostRecent().args[1].data.summary;
       expect(actualError)
           .toContain(`Failed to get thread antagonists for PID: ${
               threadToExpand.pid}`);
     });

});
