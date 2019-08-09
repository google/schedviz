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
import {async, fakeAsync, TestBed, tick} from '@angular/core/testing';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import * as d3 from 'd3';
import {BehaviorSubject, of} from 'rxjs';

import {CollectionParameters, Interval, Layer, WaitingThreadInterval} from '../models';
import {LocalMetricsService} from '../services/metrics_service';
import {LocalRenderDataService} from '../services/render_data_service';
import {SystemTopology, Viewport} from '../util';

import {Heatmap} from './heatmap';
import {HeatmapModule} from './heatmap_module';

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
  component.layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
  component.viewport = new BehaviorSubject<Viewport>(new Viewport());
  component.cpuFilter = new BehaviorSubject<string>('');
  component.maxIntervalCount = new BehaviorSubject<number>(5000);
  component.showMigrations = new BehaviorSubject<boolean>(true);
  component.showSleeping = new BehaviorSubject<boolean>(true);
}

function mockParameters(): CollectionParameters {
  return new CollectionParameters('foo', CPUS, START_TIME, END_TIME);
}

function mockTopology(): SystemTopology {
  return new SystemTopology(CPUS);
}

describe('Heatmap', () => {
  beforeEach(async(() => {
    document.body.style.width = '500px';
    document.body.style.height = '500px';
    TestBed
        .configureTestingModule({
          imports: [HeatmapModule],
          providers: [
            {provide: 'MetricsService', useClass: LocalMetricsService},
            {provide: 'RenderDataService', useClass: LocalRenderDataService},
          ]
        })
        .compileComponents();
  }));

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
    expect(layers.length).toEqual(2);
    expect(layers[0].value.intervals.length).toBeGreaterThan(1);
    const renderedLayers = element.querySelectorAll('.layersGroup');
    expect(renderedLayers.length).toEqual(2);
    // Check layers render in reverse order, with expected interval count drawn
    let intervalCount = renderedLayers[0].querySelectorAll('rect').length;
    expect(intervalCount).toEqual(layers[1].value.intervals.length);
    intervalCount = renderedLayers[1].querySelectorAll('rect').length;
    expect(intervalCount).toEqual(layers[0].value.intervals.length);
    // TODO(sainsley): Check base intervals set
  });

  it('should fetch PID intervals on thread layer added', () => {
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
    const fetchedIntervalCount = newLayer.value.intervals.length;
    expect(fetchedIntervalCount).toBe(1728);
    const renderedLayers = element.querySelectorAll('.layersGroup');
    expect(renderedLayers.length).toEqual(3);
    // Query interval count for newest (top-most) layer
    const drawnIntervals = renderedLayers[0].querySelectorAll('rect');
    expect(fetchedIntervalCount).toEqual(drawnIntervals.length);
    for (const interval of drawnIntervals) {
      expect(d3.select(interval).style('fill')).toEqual(green);
      expect(Number(d3.select(interval).attr('rx'))).toBeGreaterThan(0);
    }
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
    expect(renderedLayers.length).toEqual(3);
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
       tick(500);
       // Should fetch new intervals on zoom in
       expect(viewport.width).toBe(0.5);
       expect(renderService.getCpuIntervals).toHaveBeenCalled();
       spy.calls.reset();
       component.viewport.next(new Viewport());
       // Should fetch new intervals on zoom out to default
       tick(500);
       expect(renderService.getCpuIntervals).not.toHaveBeenCalled();
     }));

  it('should reset viewport height on CPU filter change', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       setupHeatmap(component);
       fixture.detectChanges();
       tick(500);
       spy.calls.reset();
       const viewport = new Viewport();
       // First, zoom in
       viewport.left = 0.25;
       viewport.right = 0.75;
       viewport.top = 0.25;
       viewport.bottom = 0.75;
       component.viewport.next(viewport);
       tick(500);
       expect(spy.calls.count()).toBe(1);
       expect(viewport.width).toBe(0.5);
       // Then set a CPU filter
       component.cpuFilter.next('5-10');
       tick(500);
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
       tick(500);
       spy.calls.reset();
       const viewport = new Viewport();
       viewport.left = 0.25;
       viewport.right = 0.75;
       component.viewport.next(viewport);
       tick(500);
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
       tick(500);
       spy.calls.reset();
       // Update max interval count -- expect new intervals to be fetched
       component.maxIntervalCount.next(maxIntervalCount);
       tick(500);
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
       tick(500);
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
       tick(500);
       spy.calls.reset();
       let transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       // Zoom in just x
       component.maybeUpdateViewport(transform, false);
       tick(500);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.width).toBe(0.5);
       expect(viewport.height).toBe(1.0);
       transform = d3.zoomIdentity.scale(1.25);
       // Zoom out in just x
       component.maybeUpdateViewport(transform, false);
       tick(500);
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
       tick(500);
       spy.calls.reset();
       let transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       component.maybeUpdateViewport(transform, true);
       tick(500);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       // Both should zoom in both axes on shift
       expect(viewport.width).toBe(0.5);
       expect(viewport.height).toBe(0.5);
       transform = d3.zoomIdentity.scale(1);
       component.maybeUpdateViewport(transform, false);
       tick(500);
       expect(spy.calls.count()).toBe(2);
       // Only x should zoom out w/o shift
       expect(viewport.width).toBe(1.0);
       expect(viewport.height).toBe(0.5);
       transform = d3.zoomIdentity.scale(0.5);
       component.maybeUpdateViewport(transform, false);
       tick(500);
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
       tick(500);
       spy.calls.reset();
       // First zoom
       let transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       component.maybeUpdateViewport(transform, false);
       tick(500);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.height).toBe(1.0);
       expect(viewport.width).toBe(0.5);
       expect(viewport.left).toBe(0);
       // Trigger pan
       transform = d3.zoomIdentity.scale(2).translate(-200, 0);
       expect(transform.x).toBe(-400);
       component.maybeUpdateViewport(transform, false);
       tick(500);
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
       tick(500);
       spy.calls.reset();
       // First zoom
       let transform = d3.zoomIdentity.scale(2);
       expect(transform.k).toBe(2);
       component.maybeUpdateViewport(transform, true);
       tick(500);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.height).toBe(0.5);
       expect(viewport.width).toBe(0.5);
       expect(viewport.top).toBe(0);
       // Trigger pan
       transform = d3.zoomIdentity.scale(2).translate(0, -200);
       expect(transform.y).toBe(-400);
       component.maybeUpdateViewport(transform, false);
       tick(500);
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
       tick(500);
       spy.calls.reset();
       const element = fixture.nativeElement;
       let renderedLayers = element.querySelectorAll('.layersGroup');
       const preZoomIntervalCount =
           renderedLayers[1].querySelectorAll('rect').length;
       // Zoom
       const viewport = new Viewport();
       viewport.left = 0.25;
       viewport.right = 0.75;
       component.viewport.next(viewport);
       tick(500);
       expect(spy.calls.count()).toBe(1);
       renderedLayers = element.querySelectorAll('.layersGroup');
       const zoomedIntervalCount =
           renderedLayers[1].querySelectorAll('rect').length;
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
       tick(500);
       spy.calls.reset();
       // Attempt zoom out
       const transform = d3.zoomIdentity.scale(0.5);
       component.maybeUpdateViewport(transform, true);
       tick(500);
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
       tick(500);
       spy.calls.reset();
       const maxZoomX = component.domainSize / 1E5;
       // Initial zoom in
       let transform = d3.zoomIdentity.scale(maxZoomX);
       component.maybeUpdateViewport(transform, false);
       tick(500);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.width).toBe(0.00005);
       // Attempt zoom past min
       transform = d3.zoomIdentity.scale(2 * maxZoomX);
       component.maybeUpdateViewport(transform, false);
       tick(500);
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
       tick(500);
       spy.calls.reset();
       const maxZoomY = CPU_COUNT;
       // Initial zoom in
       let transform = d3.zoomIdentity.scale(maxZoomY);
       component.maybeUpdateViewport(transform, true);
       tick(500);
       expect(spy.calls.count()).toBe(1);
       const viewport = component.viewport.value;
       expect(viewport.height).toBe(1 / maxZoomY);
       expect(viewport.width).toBe(1 / maxZoomY);
       // Attempt zoom past min
       transform = d3.zoomIdentity.scale(2 * maxZoomY);
       component.maybeUpdateViewport(transform, true);
       tick(500);
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
       tick(500);
       expect(element.querySelectorAll('.heatmap-loading').length).toBe(0);
     }));

  it('should not move the center line on zoom', fakeAsync(() => {
       const fixture = TestBed.createComponent(Heatmap);
       const component = fixture.componentInstance;
       setupHeatmap(component);
       fixture.detectChanges();
       const renderService = component.renderDataService;
       const spy = spyOn(renderService, 'getCpuIntervals').and.callThrough();
       tick(500);
       spy.calls.reset();
       const viewport = component.viewport.value;
       const transform =
           d3.zoomIdentity.scale(2).translate(-viewport.chartWidthPx / 4, 0);
       component.maybeUpdateViewport(transform, false);
       tick(500);
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
        layer.intervals.push(new WaitingThreadInterval(
            mockParameters(), 0, duration.start, duration.end, layer.ids[0],
            layer.name));
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
        const interval = layers[i].intervals[ii] as WaitingThreadInterval;
        expect(interval.queueOffset).toBe(expectation.offset);
        expect(interval.queueCount).toBe(expectation.count);
      }
    }
  });
});
