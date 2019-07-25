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
import {BehaviorSubject} from 'rxjs';

import {CollectionParameters, Interval, Layer} from '../models';
import {LocalMetricsService} from '../services/metrics_service';
import {LocalRenderDataService} from '../services/render_data_service';
import {SystemTopology, Viewport} from '../util';

import {Heatmap} from './heatmap';
import {HeatmapModule} from './heatmap_module';

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
  const startTime = 1540768090000;
  const endTime = 1540768139000;
  return new CollectionParameters('foo', CPUS, startTime, endTime);
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

  it('should render intervals', () => {
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
  });

  // TODO(sainsley): Add test to confirm redraw on layer property change and
  // interval refetch on zoom.
});
