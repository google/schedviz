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
import {BehaviorSubject} from 'rxjs';

import {CollectionParameters, Interval, Layer} from '../../models';
import {LocalMetricsService} from '../../services/metrics_service';
import {LocalRenderDataService} from '../../services/render_data_service';
import {SystemTopology, Viewport} from '../../util';
import {Heatmap} from '../heatmap';
import {HeatmapModule} from '../heatmap_module';

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
  const startTime = 5E8;
  const endTime = 2.5e+9;
  return new CollectionParameters('foo', CPUS, startTime, endTime);
}

function mockTopology(): SystemTopology {
  return new SystemTopology(CPUS);
}

describe('CpuAxisLayer', () => {
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
    expect(component.cpuAxisGroup).toBeTruthy();
  });

  it('should draw cpu labels', () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    fixture.detectChanges();
    const element = fixture.nativeElement;
    const cpuLabels = element.querySelectorAll('.cpuLabel');
    expect(cpuLabels.length).toEqual(CPU_COUNT);
    expect(d3.select(cpuLabels[0]).attr('y')).toBeCloseTo(5.736);
    // No zoom: viewport bar should be full height
    const axisBase = d3.select(element.querySelector('.yAxisBase'));
    const viewportMarker = d3.select(element.querySelector('.viewportMarker'));
    expect(Number(axisBase.attr('height')))
        .toEqual(Number(viewportMarker.attr('height')) + 50);
  });

  it('should adjust label positions on zoom', () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    const viewport = new Viewport();
    viewport.top = 0.25;
    viewport.bottom = 0.75;
    component.viewport.next(viewport);
    fixture.detectChanges();
    const element = fixture.nativeElement;
    const cpuLabels = element.querySelectorAll('.cpuLabel');
    expect(cpuLabels.length).toEqual(CPU_COUNT);
    expect(d3.select(cpuLabels[0]).attr('y')).toBeCloseTo(-242.528);
    // Zoom in: viewport bar should be smaller than its container
    const axisBase = d3.select(element.querySelector('.yAxisBase'));
    const viewportMarker = d3.select(element.querySelector('.viewportMarker'));
    expect(Number(axisBase.attr('height')))
        .toEqual(2 * Number(viewportMarker.attr('height')) + 50);
  });

  it('should hide markers on filter', () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    component.cpuFilter.next('1-10');
    fixture.detectChanges();
    const element = fixture.nativeElement;
    const cpuLabels = element.querySelectorAll('.cpuLabel');
    expect(cpuLabels.length).toEqual(10);
    const cpuMarkers = element.querySelectorAll('.cpuMarker');
    expect(cpuMarkers.length).toEqual(10);
  });

  it('should filter on CPU label click', () => {
    const fixture = TestBed.createComponent(Heatmap);
    const component = fixture.componentInstance;
    setupHeatmap(component);
    fixture.detectChanges();
    const element = fixture.nativeElement;
    const cpuLabel = element.querySelector('.cpuLabel');
    d3.select(cpuLabel).dispatch('click');
    fixture.detectChanges();
    expect(component.cpuFilter.value).toBe('0');
     d3.select(cpuLabel).dispatch('click');
    fixture.detectChanges();
    expect(component.cpuFilter.value).toBe('');
  });
});
