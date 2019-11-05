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
import {fakeAsync, TestBed, tick} from '@angular/core/testing';
import {MatDialog} from '@angular/material/dialog';
import {MatSnackBar} from '@angular/material/snack-bar';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import {NoopAnimationsModule} from '@angular/platform-browser/animations';
import * as d3 from 'd3';
import {BehaviorSubject, of} from 'rxjs';

import {CollectionParameters, CpuIdleWaitLayer, CpuInterval, CpuIntervalCollection, CpuRunningLayer, CpuWaitQueueLayer, Interval, Layer, ThreadInterval} from '../models';
import * as services from '../models/render_data_services';
import {LocalMetricsService} from '../services/metrics_service';
import {LocalRenderDataService, RenderDataService} from '../services/render_data_service';
import {ShortcutId, ShortcutService} from '../services/shortcut_service';
import {triggerShortcut} from '../services/shortcut_service_test';
import {SystemTopology, Viewport} from '../util';
import * as clipboard from '../util/clipboard';

import {Heatmap} from './heatmap';
import {HeatmapModule} from './heatmap_module';
import {DialogChooseThreadLayer} from './intervals_layer';

const TICK_DURATION = 1000;

const START_TIME = 5e8;
const END_TIME = 2.5e+9;
const CPU_COUNT = 72;
const CPUS: number[] = [];
for (let i = 0; i < CPU_COUNT; i++) {
  CPUS.push(i);
}

try {
  TestBed.initTestEnvironment(
      BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
  // Ignore exceptions when calling it multiple times.
}

function setupHeatmap(component: Heatmap) {
  component.parameters =
      new BehaviorSubject<CollectionParameters|undefined>(mockParameters());
  component.systemTopology = mockTopology();
  component.preview = new BehaviorSubject<Interval|undefined>(undefined);
  component.layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([
    new BehaviorSubject(new CpuRunningLayer() as unknown as Layer),
    new BehaviorSubject(new CpuIdleWaitLayer() as unknown as Layer),
    new BehaviorSubject(new CpuWaitQueueLayer() as unknown as Layer),
  ]);
  component.viewport = new BehaviorSubject<Viewport>(new Viewport());
  component.cpuFilter = new BehaviorSubject<string>('');
  component.maxIntervalCount = new BehaviorSubject<number>(5000);
  component.showMigrations = new BehaviorSubject<boolean>(true);
  component.showSleeping = new BehaviorSubject<boolean>(true);
}

function addMockIntervals(component: Heatmap): CpuIntervalCollection[] {
  const params = mockParameters();
  const runningThreads = [
    {
      thread: {
        pid: 0,
        command: '<h3>hey!</h3>',
        priority: 120,
      },
      duration: (params.endTimeNs - params.startTimeNs) / 2,
      state: services.ThreadState.RUNNING_STATE,
      droppedEventIDs: [],
      includesSyntheticTransitions: false
    },
    {
      thread: {
        pid: 1,
        command: 'test2',
        priority: 100,
      },
      duration: (params.endTimeNs - params.startTimeNs) / 2,
      state: services.ThreadState.RUNNING_STATE,
      droppedEventIDs: [],
      includesSyntheticTransitions: false
    },
  ];
  const waitingThreads = [
    {
      thread: {
        pid: 2,
        command: '<script>expect("xss").toBeNull()</script>',
        priority: 120,
      },
      duration: (params.endTimeNs - params.startTimeNs),
      state: services.ThreadState.WAITING_STATE,
      droppedEventIDs: [],
      includesSyntheticTransitions: false
    },
    {
      thread: {
        pid: 3,
        command: 'test4',
        priority: 80,
      },
      duration: (params.endTimeNs - params.startTimeNs),
      state: services.ThreadState.WAITING_STATE,
      droppedEventIDs: [],
      includesSyntheticTransitions: false
    },
  ];

  const cpuIntervalCollection = [
    new CpuIntervalCollection(
        0,
        [
          new CpuInterval(
              params,
              0,
              params.startTimeNs,
              params.endTimeNs,
              runningThreads,
              waitingThreads,
              ),
        ],
        ),
  ];

  component.setCpuIntervals(cpuIntervalCollection);

  return cpuIntervalCollection;
}

function mockParameters(): CollectionParameters {
  return new CollectionParameters('foo', CPUS, START_TIME, END_TIME);
}

function mockTopology(): SystemTopology {
  return new SystemTopology(CPUS);
}

function createHoverEvent(xPos: number, yPos: number): MouseEvent {
  const event = document.createEvent('MouseEvent');
  event.initMouseEvent(
      'mouseover', true, true, window, 0, 0, 0, xPos, yPos, false, false,
    false, false, 0, null);
  return event;
}

function isTooltipBelow(tooltipRect: ClientRect | DOMRect, yPos: number) {
  // Since the tooltip is offset from the cursor, the Y position of the cursor
  // will always be between the top and bottom edges of the tooltip. So, to
  // determine if the tooltip is below the cursor, we will do a relative
  // comparison of the distance to the top and bottom edges.
  return Math.abs(tooltipRect.top - yPos) < Math.abs(tooltipRect.bottom - yPos);
}

function isTooltipOnRight(tooltipRect: ClientRect|DOMRect, xPos: number) {
  return tooltipRect.left > xPos;
}

describe('Heatmap', () => {
  beforeEach(async () => {
    document.body.style.width = '500px';
    document.body.style.height = '500px';
    await TestBed
        .configureTestingModule({
          imports: [
            HeatmapModule,
            NoopAnimationsModule,
          ],
          providers: [
            {provide: 'MetricsService', useClass: LocalMetricsService},
            {provide: 'RenderDataService', useClass: LocalRenderDataService},
            {provide: 'ShortcutService', useClass: ShortcutService},
          ],
        })
        .compileComponents();
  });

  it('should create', () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    fixture.detectChanges();
    expect(component).toBeTruthy();
  });

  it('should render CPU intervals', () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    fixture.detectChanges();
    const element = fixture.nativeElement;
    const layers = component.layers.value;
    expect(layers.length).toEqual(3);
    expect(layers[0].value.intervals.length).toBeGreaterThan(1);
    const renderedLayers = element.querySelectorAll('.layersGroup');
    expect(renderedLayers.length).toEqual(3);
    // Check layers render in reverse order, with expected interval count drawn
    let intervalCount = renderedLayers[0].querySelectorAll('rect').length;
    expect(intervalCount).toEqual(layers[2].value.intervals.length);
    intervalCount = renderedLayers[2].querySelectorAll('rect').length;
    expect(intervalCount).toEqual(layers[0].value.intervals.length);
    // TODO(sainsley): Check base intervals set
  });

  it('should open a dialog when clicking on a merged interval', async () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    component.ngOnInit();
    fixture.detectChanges();

    await fixture.whenStable();

    const mockIntervals = addMockIntervals(component);
    const runningThreads = mockIntervals[0].running[0].running;

    fixture.detectChanges();
    await fixture.whenStable();

    const dialog = TestBed.get(MatDialog) as MatDialog;
    const dialogSpy = spyOn(dialog, 'open').and.callThrough();

    const event = document.createEvent('SVGEvents');
    event.initEvent('click', true, true);
    (document.querySelector('.interval') as SVGElement).dispatchEvent(event);

    await fixture.whenStable();

    expect(dialogSpy).toHaveBeenCalledTimes(1);
    expect(dialogSpy).toHaveBeenCalledWith(
        DialogChooseThreadLayer, {data: runningThreads});
  });

  it('should not open a dialog when clicking on a non-merged interval',
     async () => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       component.ngOnInit();
       fixture.detectChanges();

       await fixture.whenStable();

       const mockIntervals = addMockIntervals(component);
       const runningThreads = mockIntervals[0].running[0].running;
       delete runningThreads[1];

       fixture.detectChanges();
       await fixture.whenStable();

       const dialog = TestBed.get(MatDialog) as MatDialog;
       const dialogSpy = spyOn(dialog, 'open').and.callThrough();

       const event = document.createEvent('SVGEvents');
       event.initEvent('click', true, true);
       (document.querySelector('.interval') as SVGElement).dispatchEvent(event);

       await fixture.whenStable();

       expect(dialogSpy).not.toHaveBeenCalled();
     });

  it('should create a tooltip on hover', async () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    component.ngOnInit();
    fixture.detectChanges();

    await fixture.whenStable();

    addMockIntervals(component);

    fixture.detectChanges();
    await fixture.whenStable();

    const event = document.createEvent('SVGEvents');
    event.initEvent('mouseover', true, true);
    (document.querySelector('.interval') as SVGElement).dispatchEvent(event);

    await fixture.whenStable();
    fixture.detectChanges();
    await fixture.whenStable();

    const tooltip = document.querySelector('.tooltip') as HTMLElement;
    expect(tooltip.innerText)
        .toBe(
            'Running: \n' +
            '  (50.0%) 0:<h3>hey!</h3> (P:120)\n' +
            '  (50.0%) 1:test2 (P:100)\n' +
            'CPU: 0\n' +
            'Start Time: 500 msec\n' +
            'End Time: 2500 msec\n' +
            'Duration: 2000 msec\n' +
            'Idle Time: (0.00%) 0.00 msec\n' +
            'Running Time: (100%) 2000 msec\n' +
            'Waiting Time:  (200%) 4000 msec\n' +
            'Waiting PID Count: 2\n' +
            'Waiting: \n' +
            '  (100%) 2:<script>expect("xss").toBeNull()</script> (P:120)\n' +
            '  (100%) 3:test4 (P:80)');
  });

  it('should place tooltip below and to the right by default', async () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    component.ngOnInit();

    // Expand the container element to allow room for tooltip to go in any
    // direction
    const rootElement = (fixture.nativeElement as HTMLElement);
    rootElement.style.height = '1000px';
    rootElement.style.width = '1000px';

    addMockIntervals(component);
    fixture.detectChanges();
    await fixture.whenStable();

    const interval = document.querySelector('.interval') as SVGElement;
    const tooltip = document.querySelector('.tooltip') as HTMLElement;

    // Hover in the middle of the screen, check that tooltip is below and to the
    // right
    const yPos = Math.floor(rootElement.getBoundingClientRect().bottom / 2);
    const xPos = Math.floor(rootElement.getBoundingClientRect().right / 2);
    interval.dispatchEvent(createHoverEvent(xPos, yPos));
    await fixture.whenStable();
    fixture.detectChanges();
    await fixture.whenStable();
    const tooltipRect = tooltip.getBoundingClientRect();
    expect(isTooltipBelow(tooltipRect, yPos)).toBe(true);
    expect(isTooltipOnRight(tooltipRect, xPos)).toBe(true);
  });

  it(`should place tooltip properly at the corners of the container`,
     async () => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       component.ngOnInit();

       const rootElement = (fixture.nativeElement as HTMLElement);
       rootElement.style.height = '1000px';
       rootElement.style.width = '1000px';

       addMockIntervals(component);
       fixture.detectChanges();
       await fixture.whenStable();

       const containerTop = rootElement.getBoundingClientRect().top;
       const containerBottom = rootElement.getBoundingClientRect().bottom;
       const containerLeft = rootElement.getBoundingClientRect().left;
       const containerRight = rootElement.getBoundingClientRect().right;

       const testCases = [
         {
           cornerLocation: 'top left',
           hoverCoordinate: {x: containerLeft, y: containerTop},
           expectedTooltipPlacement: {below: true, right: true}
         },
         {
           cornerLocation: 'top right',
           hoverCoordinate: {x: containerRight, y: containerTop},
           expectedTooltipPlacement: {below: true, right: false}
         },
         {
           cornerLocation: 'bottom right',
           hoverCoordinate: {x: containerRight, y: containerBottom},
           expectedTooltipPlacement: {below: false, right: false}
         },
         {
           cornerLocation: 'bottom left',
           hoverCoordinate: {x: containerLeft, y: containerBottom},
           expectedTooltipPlacement: {below: false, right: true}
         },
       ];

       // Simulate each hover and verify the resulting tooltip locations in turn
       for (const testCase of testCases) {
         const cursorX = testCase.hoverCoordinate.x;
         const cursorY = testCase.hoverCoordinate.y;
         const interval = document.querySelector('.interval') as SVGElement;
         interval.dispatchEvent(createHoverEvent(cursorX, cursorY));
         await fixture.whenStable();
         fixture.detectChanges();
         await fixture.whenStable();

         const tooltip = document.querySelector('.tooltip') as HTMLElement;
         const tooltipRect = tooltip.getBoundingClientRect();
         expect(isTooltipBelow(tooltipRect, cursorY))
             .toBe(
                 testCase.expectedTooltipPlacement.below,
                 `Incorrect vertical placement for corner location: ${
                     testCase.cornerLocation}`);
         expect(isTooltipOnRight(tooltipRect, cursorX))
             .toBe(
                 testCase.expectedTooltipPlacement.right,
                 `Incorrect horizontal placement for corner location: ${
                     testCase.cornerLocation}`);
       }
     });

  it('should fetch PID intervals on thread layer added', async () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    fixture.detectChanges();
    const element = fixture.nativeElement;
    const layers = component.layers.value;
    const green = 'rgb(0, 255, 0)';
    const newLayer = new BehaviorSubject<Layer>(
        new Layer('thread_foo', 'Thread', [1234], green));
    layers.push(newLayer);
    component.layers.next(layers);
    await fixture.whenStable();
    const fetchedIntervalCount = newLayer.value.intervals.length;
    expect(fetchedIntervalCount).toBe(1296);
    const renderedLayers = element.querySelectorAll('.layersGroup');
    expect(renderedLayers.length).toEqual(4);
    // Query interval count for newest (top-most) layer
    const drawnIntervals = renderedLayers[0].querySelectorAll('rect');
    expect(fetchedIntervalCount).toEqual(drawnIntervals.length);
    for (const interval of drawnIntervals) {
      expect(d3.select(interval).style('fill')).toEqual(green);
      expect(Number(d3.select(interval).attr('rx'))).toBeGreaterThan(0);
    }
  });

  it('should make only one request when multiple PID layers are added at once',
     async () => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();

       const renderDataService =
           TestBed.get('RenderDataService') as RenderDataService;
       const requestSpy =
           spyOn(renderDataService, 'getPidIntervals').and.callThrough();

       const layers = component.layers.value;
       layers.push(new BehaviorSubject<Layer>(
           new Layer('thread_foo', 'Thread', [1234], 'rgb(0, 255, 0)')));
       layers.push(new BehaviorSubject<Layer>(
           new Layer('thread_bar', 'Thread', [4321], 'rgb(255, 0, 0)')));
       component.layers.next(layers);

       await fixture.whenStable();

       expect(requestSpy).toHaveBeenCalledTimes(1);
     });

  it('should fetch sched events on event layer added', () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    fixture.detectChanges();
    const element = fixture.nativeElement;
    const layers = component.layers.value;
    const red = 'rgb(255, 0, 0)';
    const newLayer = new BehaviorSubject<Layer>(
        new Layer('event_foo', 'SchedEvent', [], red));
    layers.push(newLayer);
    component.layers.next(layers);
    const fetchedIntervalCount = newLayer.value.intervals.length;
    expect(fetchedIntervalCount).toBeGreaterThan(0);
    const renderedLayers = element.querySelectorAll('.layersGroup');
    expect(renderedLayers.length).toEqual(4);
    // Query interval count for newest (top-most) layer
    const drawnIntervals = renderedLayers[0].querySelectorAll('rect');
    expect(fetchedIntervalCount).toEqual(drawnIntervals.length);
    for (const interval of drawnIntervals) {
      expect(d3.select(interval).style('fill')).toEqual(red);
      expect(Number(d3.select(interval).attr('rx'))).toEqual(0);
    }
  });

  it('should reuse base intervals when fully zoomed out', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       const renderService = component.renderDataService;
       setupHeatmap(component);
       fixture.detectChanges();
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       const viewport = new Viewport();
       viewport.left = 0.25;
       viewport.right = 0.75;
       component.viewport.next(viewport);
       tick(TICK_DURATION);
       // Should fetch new intervals on zoom in
       expect(viewport.width).toBe(0.5);
       expect(renderService.getCpuIntervals).toHaveBeenCalled();
       spy.calls.reset();
       component.viewport.next(new Viewport());
       // Should fetch new intervals on zoom out to default
       tick(TICK_DURATION);
       expect(renderService.getCpuIntervals).not.toHaveBeenCalled();
     }));

  it('should reset viewport height on CPU filter change', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       setupHeatmap(component);
       fixture.detectChanges();
       tick(TICK_DURATION);
       spy.calls.reset();
       const viewport = new Viewport();
       // First, zoom in
       viewport.left = 0.25;
       viewport.right = 0.75;
       viewport.top = 0.25;
       viewport.bottom = 0.75;
       component.viewport.next(viewport);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       expect(viewport.width).toBe(0.5);
       // Then set a CPU filter
       component.cpuFilter.next('5-10');
       tick(TICK_DURATION);
       // Expect viewport to reset
       expect(spy.calls.count()).toBe(2);
       const newViewport = component.viewport.value;
       expect(newViewport.left).toBe(viewport.left);
       expect(newViewport.right).toBe(viewport.right);
       expect(newViewport.height).toBe(1.0);
     }));

  it('should update d3 zoom transform on viewport update', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       setupHeatmap(component);
       fixture.detectChanges();
       tick(TICK_DURATION);
       spy.calls.reset();
       const viewport = new Viewport();
       viewport.left = 0.25;
       viewport.right = 0.75;
       component.viewport.next(viewport);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       expect(viewport.width).toBe(0.5);
       expect(d3.select(component.zoomGroup.nativeElement).attr('transform'))
           .toBe('translate(-250, 0)');
     }));

  it('should fetch new intervals on max interval count change',
     fakeAsync(() => {
       const maxIntervalCount = 100;
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       setupHeatmap(component);
       fixture.detectChanges();
       tick(TICK_DURATION);
       spy.calls.reset();
       // Update max interval count -- expect new intervals to be fetched
       component.maxIntervalCount.next(maxIntervalCount);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       // TODO(sainsley): Check that base intervals update
       const intervals = component.cpuRunLayer.value.intervals;
       expect(maxIntervalCount).toBeGreaterThan(intervals.length);
     }));

  it('should update y zoom on shift-zoom', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       component.maybeUpdateViewport(transform, true);
       tick(TICK_DURATION);
       const viewport = component.viewport.value;
       expect(viewport.height).toBe(0.5);
       expect(viewport.width).toBe(0.5);
     }));

  it('should not update y on default zoom-out or zoom-in', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       fixture.detectChanges();
       tick(TICK_DURATION);
       spy.calls.reset();
       let transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       // Zoom in just x
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.width).toBe(0.5);
       expect(viewport.height).toBe(1.0);
       transform = d3.zoomIdentity.scale(1.25);
       // Zoom out in just x
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(2);
       expect(viewport.width).toBe(0.8);
       expect(viewport.height).toBe(1.0);
     }));

  it('should update y on default zoom-out when viewport width == 1',
     fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       fixture.detectChanges();
       tick(TICK_DURATION);
       spy.calls.reset();
       let transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       component.maybeUpdateViewport(transform, true);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       // Both should zoom in both axes on shift
       expect(viewport.width).toBe(0.5);
       expect(viewport.height).toBe(0.5);
       transform = d3.zoomIdentity.scale(1);
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(2);
       // Only x should zoom out w/o shift
       expect(viewport.width).toBe(1.0);
       expect(viewport.height).toBe(0.5);
       transform = d3.zoomIdentity.scale(0.5);
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       // Only y should zoom out with x maximized
       expect(spy.calls.count()).toBe(2);
       expect(viewport.width).toBe(1.0);
       expect(viewport.height).toBe(1.0);
     }));

  it('should pan in x when viewport width is less than 1 and zoom delta is 0',
     fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       tick(TICK_DURATION);
       spy.calls.reset();
       // First zoom
       let transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.height).toBe(1.0);
       expect(viewport.width).toBe(0.5);
       expect(viewport.left).toBe(0);
       // Trigger pan
       transform = d3.zoomIdentity.scale(2).translate(-200, 0);
       expect(transform.x).toBe(-400);
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(2);
       expect(viewport.height).toBe(1.0);
       expect(viewport.width).toBe(0.5);
       expect(viewport.left).toBe(0.4);
     }));

  it('should pan in y when viewport height is less than 1 and zoom delta is 0',
     fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       tick(TICK_DURATION);
       spy.calls.reset();
       // First zoom
       let transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       component.maybeUpdateViewport(transform, true);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.height).toBe(0.5);
       expect(viewport.width).toBe(0.5);
       expect(viewport.top).toBe(0);
       // Trigger pan
       transform = d3.zoomIdentity.scale(2).translate(0, -200);
       expect(transform.y).toBe(-400);
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(2);
       expect(viewport.height).toBe(0.5);
       expect(viewport.width).toBe(0.5);
       expect(viewport.top).toBe(0.4);
     }));

  it('should redraw on zoom-in', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       tick(TICK_DURATION);
       spy.calls.reset();
       const element = fixture.nativeElement;
       let renderedLayers = element.querySelectorAll('.layersGroup');
       const preZoomIntervalCount =
           renderedLayers[2].querySelectorAll('rect').length;
       // Zoom
       const viewport = new Viewport();
       viewport.left = 0.25;
       viewport.right = 0.75;
       component.viewport.next(viewport);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       renderedLayers = element.querySelectorAll('.layersGroup');
       const zoomedIntervalCount =
           renderedLayers[2].querySelectorAll('rect').length;
       expect(zoomedIntervalCount).not.toEqual(preZoomIntervalCount);
     }));

  it('should not zoom out when viewport width and height are 1',
     fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       tick(TICK_DURATION);
       spy.calls.reset();
       // Attempt zoom out
       const transform = d3.zoomIdentity.scale(0.5);
       component.maybeUpdateViewport(transform, true);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(0);
       const viewport = component.viewport.value;
       expect(viewport.height).toBe(1.0);
       expect(viewport.top).toBe(0.0);
       expect(viewport.width).toBe(1.0);
       expect(viewport.left).toBe(0.0);
     }));

  it('should not zoom in x when viewport is at minimum width', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       tick(TICK_DURATION);
       spy.calls.reset();
       const maxZoomX = component.domainSize / 1E5;
       // Initial zoom in
       let transform = d3.zoomIdentity.scale(maxZoomX);
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.width).toBe(0.00005);
       // Attempt zoom past min
       transform = d3.zoomIdentity.scale(2 * maxZoomX);
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       expect(viewport.width).toBe(0.00005);
     }));

  it('should not zoom in y when viewport is at minimum height',
     fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       tick(TICK_DURATION);
       spy.calls.reset();
       const maxZoomY = CPU_COUNT;
       // Initial zoom in
       let transform = d3.zoomIdentity.scale(maxZoomY);
       component.maybeUpdateViewport(transform, true);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.height).toBe(1 / maxZoomY);
       expect(viewport.width).toBe(1 / maxZoomY);
       // Attempt zoom past min
       transform = d3.zoomIdentity.scale(2 * maxZoomY);
       component.maybeUpdateViewport(transform, true);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(2);
       expect(viewport.height).toBe(1 / maxZoomY);
       expect(viewport.width).toBe(0.5 / maxZoomY);
     }));

  it('should shown loading bar while intervals are pending', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       const element = fixture.nativeElement;
       const renderService = component.renderDataService;
       setupHeatmap(component);
       fixture.detectChanges();
       spyOn(renderService, 'getCpuIntervals').and.callFake(() => {
         expect(component.loading).toBe(true);
         component.cdr.detectChanges();
         expect(element.querySelectorAll('.heatmap-loading').length).toBe(1);
         return of([]);
       });
       const viewport = new Viewport();
       viewport.left = 0.25;
       viewport.right = 0.75;
       viewport.top = 0.25;
       viewport.bottom = 0.75;
       component.viewport.next(viewport);
       tick(TICK_DURATION);
       expect(element.querySelectorAll('.heatmap-loading').length).toBe(0);
     }));

  it('should not move the center line on zoom', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       tick(TICK_DURATION);
       spy.calls.reset();
       const viewport = component.viewport.value;
       const transform =
           d3.zoomIdentity.scale(2).translate(-viewport.chartWidthPx / 4, 0);
       component.maybeUpdateViewport(transform, false);
       tick(TICK_DURATION);
       expect(spy.calls.count()).toBe(1);
       expect(viewport.width).toBe(0.5);
       expect(viewport.left).toBe(0.25);
     }));

  it('should arrange waiting pid queues such that no two overlap', () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    fixture.detectChanges();
    const startTime = START_TIME;
    const duration = END_TIME - START_TIME;
    const layer1 = new Layer('thread_foo', 'Thread', [1234], '#ff0000');
    const layer2 = new Layer('thread_bar', 'Thread', [3456], '#00ff00');
    const layer3 = new Layer('thread_foo_bar', 'Thread', [4567], '#0000ff');
    const layers = [layer1, layer2, layer3];
    // Create three layers each with a few intervals w/ waiting
    // One full width, one that overlaps with 1/3 of intervals, and one that
    // overlaps with both
    const durations = [
      [{start: startTime, end: startTime + duration}],
      [
        {start: startTime, end: startTime + duration / 3},
        {start: startTime + 2 * duration / 3, end: startTime + duration}
      ],
      [{start: startTime + 3 * duration / 4, end: startTime + duration}]
    ];
    for (let i = 0; i < layers.length; i++) {
      for (let ii = 0; ii < durations[i].length; ii++) {
        const duration = durations[i][ii];
        const layer = layers[i];
        layer.intervals.push(new ThreadInterval(
            mockParameters(), 0, duration.start, duration.end, layer.ids[0],
            layer.name, [{
              thread: {
                pid: 2,
                command: 'waiter',
                priority: 120,
              },
              duration: duration.end - duration.start,
              state: services.ThreadState.WAITING_STATE,
              droppedEventIDs: [],
              includesSyntheticTransitions: false
            }]));
      }
    }
    component.arrangeWaitingPids(layers);
    const expectations = [
      [{offset: 0, count: 4}], [{offset: 1, count: 2}, {offset: 1, count: 3}],
      [{offset: 2, count: 3}]
    ];
    for (let i = 0; i < layers.length; i++) {
      for (let ii = 0; ii < expectations[i].length; ii++) {
        const expectation = expectations[i][ii];
        const interval = layers[i].intervals[ii] as ThreadInterval;
        expect(interval.queueOffset).toBe(expectation.offset);
        expect(interval.queueCount).toBe(expectation.count);
      }
    }
  });

  it('should register copy to clipboard handler', async () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    component.ngOnInit();
    addMockIntervals(component);

    await fixture.whenStable();
    fixture.detectChanges();

    // Trigger a tooltip
    const event = document.createEvent('SVGEvents');
    event.initEvent('mouseover', true, true);
    (document.querySelector('.interval') as SVGElement).dispatchEvent(event);

    await fixture.whenStable();

    const snackBar = TestBed.get(MatSnackBar) as MatSnackBar;
    const snackBarSpy = spyOn(snackBar, 'open');

    const shortcutId = ShortcutId.COPY_TOOLTIP;
    const copyToClipboardSpy =
        spyOn(clipboard, 'copyToClipboard').and.returnValue(true);
    const shortcutService = fixture.debugElement.injector.get(ShortcutService);
    const shortcut = shortcutService.getShortcuts()[shortcutId];
    expect(shortcut.isEnabled).toBe(true);
    triggerShortcut(shortcut);
    expect(copyToClipboardSpy).toHaveBeenCalledTimes(1);

    const tooltip = document.querySelector('.tooltip') as HTMLElement;
    expect(copyToClipboardSpy)
      .toHaveBeenCalledWith(tooltip.innerText);

    const snackBarMessage = snackBarSpy.calls.mostRecent().args[0];
    expect(snackBarMessage).toBe('Tooltip copied to clipboard');

    // Verify that shortcut is deregistered
    component.ngOnDestroy();
    const shortcutAfterDestroy = shortcutService.getShortcuts()[shortcutId];
    expect(shortcutAfterDestroy.isEnabled).toBe(false);
  });

  it('should handle copy to clipboard shortcut error', async () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    component.ngOnInit();
    addMockIntervals(component);

    await fixture.whenStable();
    fixture.detectChanges();

    const event = document.createEvent('SVGEvents');
    event.initEvent('mouseover', true, true);
    (document.querySelector('.interval') as SVGElement).dispatchEvent(event);

    await fixture.whenStable();

    const snackBar = TestBed.get(MatSnackBar) as MatSnackBar;
    const snackBarSpy = spyOn(snackBar, 'open');

    const copyToClipboardSpy =
        spyOn(clipboard, 'copyToClipboard').and.returnValue(false);
    const shortcutService = fixture.debugElement.injector.get(ShortcutService);
    const shortcut = shortcutService.getShortcuts()[ShortcutId.COPY_TOOLTIP];
    triggerShortcut(shortcut);
    expect(copyToClipboardSpy).toHaveBeenCalledTimes(1);

    const tooltip = document.querySelector('.tooltip') as HTMLElement;
    expect(copyToClipboardSpy).toHaveBeenCalledWith(tooltip.innerText);

    const snackBarMessage = snackBarSpy.calls.mostRecent().args[0];
    expect(snackBarMessage).toBe('Error copying tooltip to clipboard');
  });

  it('should register reset viewport shortcut', async () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    component.ngOnInit();
    addMockIntervals(component);

    fixture.detectChanges();
    await fixture.whenStable();

    const shortcutId = ShortcutId.RESET_VIEWPORT;
    const shortcutService = fixture.debugElement.injector.get(ShortcutService);
    const shortcut = shortcutService.getShortcuts()[shortcutId];

    const viewport = new Viewport();
    viewport.left = 0.25;
    viewport.right = 0.75;
    component.viewport.next(viewport);

    expect(shortcut.isEnabled).toBe(true);
    triggerShortcut(shortcut);

    const viewportAfter = component.viewport.value;
    expect(viewportAfter.left).toBe(0);
    expect(viewportAfter.right).toBe(1);

    // Verify that shortcut is deregistered
    component.ngOnDestroy();
    const shortcutAfterDestroy = shortcutService.getShortcuts()[shortcutId];
    expect(shortcutAfterDestroy.isEnabled).toBe(false);
  });

  it('should register clear CPU filter shortcut', async () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    component.ngOnInit();
    addMockIntervals(component);

    fixture.detectChanges();
    await fixture.whenStable();

    component.cpuFilter.next('1');

    const shortcutId = ShortcutId.CLEAR_CPU_FILTER;
    const shortcutService = fixture.debugElement.injector.get(ShortcutService);
    const shortcut = shortcutService.getShortcuts()[shortcutId];
    triggerShortcut(shortcut);

    expect(component.cpuFilter.value).toBe('');

    // Verify that shortcut is deregistered
    component.ngOnDestroy();
    const shortcutAfterDestroy = shortcutService.getShortcuts()[shortcutId];
    expect(shortcutAfterDestroy.isEnabled).toBe(false);
  });
});
