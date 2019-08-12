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
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import * as d3 from 'd3';
import {BehaviorSubject, Observable} from 'rxjs';
import {map} from 'rxjs/operators';

import {CollectionParameters, CpuIntervalCollection} from '../models';
import {ThreadResidency, ThreadState} from '../models/render_data_services';
import {LocalMetricsService} from '../services/metrics_service';
import {LocalRenderDataService} from '../services/render_data_service';
import {Viewport} from '../util';

import {HeatmapModule} from './heatmap_module';
import {TimelineZoomBrush} from './timeline_zoom_brush';

const CPU_COUNT = 72;
const CPUS: number[] = [];
for (let i = 0; i < CPU_COUNT; i++) {
  CPUS.push(i);
}
const renderService = new LocalRenderDataService();

try {
  TestBed.initTestEnvironment(
      BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
  // Ignore exceptions when calling it multiple times.
}

function setupZoomBrush(component: TimelineZoomBrush):
    Observable<CpuIntervalCollection[]> {
  const parameters = mockParameters();
  const viewport = new Viewport();
  component.parameters =
      new BehaviorSubject<CollectionParameters|undefined>(parameters);
  component.viewport = new BehaviorSubject<Viewport>(viewport);

  const duration = parameters.endTimeNs - parameters.startTimeNs;

  // For testing, increase wait count over time.
  function getWaiting(i: number): ThreadResidency[] {
    return new Array(i).fill(null).map(
        () => ({
          thread: {command: 'bar', pid: i, priority: 99},
          state: ThreadState.WAITING_STATE,
          duration,
          droppedEventIDs: [],
          includesSyntheticTransitions: false,
        }));
  }

  return renderService.getCpuIntervals(parameters, viewport, 0, CPUS)
      .pipe(
          map((collections: CpuIntervalCollection[]) =>
                  collections.map(collection => {
                    collection.running.forEach((interval, i) => {
                      interval.waiting = getWaiting(i);
                    });
                    collection.waiting.forEach((interval, i) => {
                      interval.waiting = getWaiting(i);
                    });
                    return collection;
                  })),
      );
}

function mockParameters(): CollectionParameters {
  const startTime = 1540768090000;
  const endTime = 1540768139000;
  return new CollectionParameters('foo', CPUS, startTime, endTime);
}

// Test preview change yields correct rendering
describe('TimelineZoomBrush', () => {
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

  it('should create', (done) => {
    const fixture = TestBed.createComponent(TimelineZoomBrush);
    const component = fixture.componentInstance;
    setupZoomBrush(component).subscribe(intervals => {
      component.intervals = intervals;
      fixture.detectChanges();
      expect(component).toBeTruthy();
      expect(component.intervals.length).toBeGreaterThan(0);
      const element = fixture.nativeElement;
      const heatmapCells = element.querySelectorAll('.brushCell');
      expect(heatmapCells.length).toEqual(100);
      let prevCell;
      for (const cell of heatmapCells) {
        expect(d3.select(cell).attr('width')).toBe('5');
        // Mock intervals have more waiting over time, expect markers to become
        // lighter from left to right
        if (prevCell) {
          const brightness = d3.hsl(d3.select(cell).attr('fill')).l;
          const prevBrightness = d3.hsl(d3.select(prevCell).attr('fill')).l;
          expect(brightness).toBeGreaterThan(prevBrightness);
        }
        prevCell = cell;
      }
      done();
    });
  });

  it('should update on zoom', (done) => {
    const fixture = TestBed.createComponent(TimelineZoomBrush);
    const component = fixture.componentInstance;
    setupZoomBrush(component).subscribe(intervals => {
      fixture.detectChanges();
      expect(component).toBeTruthy();
      const element = d3.select(fixture.nativeElement);
      const selectionWidthFull =
          Number(element.select('.selection').attr('width'));
      const viewport = new Viewport();
      viewport.left = 0.25;
      viewport.right = 0.75;
      component.viewport.next(viewport);
      fixture.detectChanges();
      const selectionWidthZoomed =
          Number(element.select('.selection').attr('width'));
      expect(selectionWidthFull).toEqual(2 * selectionWidthZoomed);
      done();
    });
  });
});
