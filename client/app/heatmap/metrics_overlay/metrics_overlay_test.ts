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
import {TestBed, waitForAsync} from '@angular/core/testing';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import {BehaviorSubject} from 'rxjs';

import {CollectionParameters} from '../../models';
import {LocalMetricsService} from '../../services/metrics_service';
import {LocalRenderDataService} from '../../services/render_data_service';
import {Viewport} from '../../util';

import {MetricsOverlay} from './metrics_overlay';
import {MetricsOverlayModule} from './metrics_overlay_module';

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

function setupMetricsOverlay(component: MetricsOverlay) {
  component.parameters =
      new BehaviorSubject<CollectionParameters|undefined>(mockParameters());
  component.visibleCpus = new BehaviorSubject<number[]>(CPUS);
  component.viewport = new BehaviorSubject<Viewport>(new Viewport());
}

function mockParameters(): CollectionParameters {
  const startTime = 1540768090000;
  const endTime = 1540768139000;
  return new CollectionParameters('foo', CPUS, startTime, endTime);
}

describe('MetricsOverlayLayer', () => {
  beforeEach(waitForAsync(() => {
    document.body.style.width = '500px';
    document.body.style.height = '500px';
    TestBed
        .configureTestingModule({
          imports: [MetricsOverlayModule],
          providers: [
            {provide: 'MetricsService', useClass: LocalMetricsService},
            {provide: 'RenderDataService', useClass: LocalRenderDataService},
          ]
        })
        .compileComponents();
  }));

  it('should create', () => {
    const fixture = TestBed.createComponent(MetricsOverlay);
    const component = fixture.componentInstance;
    setupMetricsOverlay(component);
    fixture.detectChanges();
    expect(component).toBeTruthy();
  });

  it('update on zoom', () => {
    const fixture = TestBed.createComponent(MetricsOverlay);
    const component = fixture.componentInstance;
    setupMetricsOverlay(component);
    component.fetchMetrics();
    fixture.detectChanges();
    const element = fixture.nativeElement;
    let metricLabels = Array.from(element.querySelectorAll('.metric-percent'));
    expect(metricLabels.length).toBeGreaterThan(0);
    const metricsBefore = metricLabels.map(node => (node as Element).innerHTML);
    const cpus = CPUS.slice(0, CPUS.length / 2);
    component.visibleCpus.next(cpus);
    component.fetchMetrics();
    fixture.detectChanges();
    metricLabels = Array.from(element.querySelectorAll('.metric-percent'));
    for (let i = 0; i < metricsBefore.length; i++) {
      const metricBefore = metricsBefore[i];
      expect((metricLabels[i] as Element).innerHTML).not.toEqual(metricBefore);
    }
  });
});
