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
import {async, ComponentFixture, TestBed} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatButtonModule} from '@angular/material/button';
import {MatDialogModule} from '@angular/material/dialog';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {MatProgressSpinnerModule} from '@angular/material/progress-spinner';
import {MatSidenavModule} from '@angular/material/sidenav';
import {MatSnackBar, MatSnackBarModule} from '@angular/material/snack-bar';
import {Sort} from '@angular/material/sort';
import {MatTooltipModule} from '@angular/material/tooltip';
import {BrowserModule} from '@angular/platform-browser';
import {BehaviorSubject, throwError} from 'rxjs';

import {HeatmapModule} from '../heatmap/heatmap_module';
import {Checkpoint, CpuRunningLayer, CpuWaitQueueLayer, Layer} from '../models';

import {LocalCollectionDataService} from '../services/collection_data_service';
import {ColorService} from '../services/color_service';
import {LocalMetricsService} from '../services/metrics_service';
import {LocalRenderDataService} from '../services/render_data_service';
import {SidebarModule} from '../sidebar/sidebar_module';
import {Viewport} from '../util';

import {Dashboard} from './dashboard';
import {DashboardToolbar} from './dashboard_toolbar';

describe('Dashboard', () => {
  let dashboard: Dashboard;
  let mockDashboard: MockDashboard;
  let fixture: ComponentFixture<Dashboard>;

  // Use a mock class so that async requests are not made
  class MockDashboard {
    collectionName?: string;
    cpuRunLayer = new CpuRunningLayer();
    cpuWaitQueueLayer = new CpuWaitQueueLayer();
    cpuFilter = new BehaviorSubject<string>('');
    filter = new BehaviorSubject<string>('');
    layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
    viewport = new BehaviorSubject<Viewport>(new Viewport());
    tab = new BehaviorSubject<number>(0);
    tabs = dashboard.tabs;
    sort = new BehaviorSubject<Sort>({active: '', direction: ''});
    showMigrations = new BehaviorSubject<boolean>(false);
    showSleeping = new BehaviorSubject<boolean>(false);
    maxIntervalCount = new BehaviorSubject<number>(5000);
    getCollectionParameters(...args: unknown[]) {}
    checkpoint = {displayOptions: {}} as Checkpoint;
    colorService = new ColorService();
    hashState = dashboard.hashState;

    global = {
      'history': jasmine.createSpyObj('history', ['replaceState']),
      'location': {'hash': '', 'pathname': '/dashboard'}
    };

  }

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [
        BrowserModule,
        FormsModule,
        MatButtonModule,
        MatDialogModule,
        MatIconModule,
        MatInputModule,
        MatSnackBarModule,
        MatTooltipModule,
        HeatmapModule,
        MatProgressSpinnerModule,
        MatSidenavModule,
        SidebarModule,
      ],
      providers: [
        {provide: 'MetricsService', useClass: LocalMetricsService},
        {provide: 'RenderDataService', useClass: LocalRenderDataService},
        {
          provide: 'CollectionDataService',
          useClass: LocalCollectionDataService
        },
      ],
      declarations: [Dashboard, DashboardToolbar],
    });
  });

  beforeEach(async(() => {
    TestBed.compileComponents().then(() => {
      fixture = TestBed.createComponent(Dashboard);
      dashboard = fixture.debugElement.componentInstance;
      mockDashboard = new MockDashboard();
    });
  }));

  it('should create the dashboard', () => {
    expect(dashboard).toBeTruthy();
  });

  it('should parse and load data from the hash fragment in the URL', () => {
    const hash =
        '#collection=abc&share=eJzFlEGPmzAQhf9K5F5aCalg2AC%252BVo1aqdVuu1vtocrB4AmM4mBkhqXbKP%252B9NolY0kpRL204AHp%252BY3%252FvCbFnCrtWy%252BfbltA0HRN7Vrb9CjWBZYKxgCkgiXolSzJOuQkD1qKaG0qz28nmTCJZMBEHrKvN8BkrK0%252Bbb6Tu4Cjfa4AWm2oSqbYg1Sfs6N5YWiFo5TYbJBLhDtjvhlur%252FGlRwJ4QhtYpnl3Dxj0dI5l2fFqsahpthSEyu%252FG1rKWlR1RU3%252F1gIs34SfoA3u21ZRoeAuZ6Aeuwv%252B9ZIx2DYA8jg4hivswjsT0iKd%252BSJPnw3L54nIbKzx69a9%252BT9g2yVznkiife0LjGnlx8Z1v7IB0W2m1BtofTamu0JJhKKoyP%252FVUq7N1Q7AIqK4f3qoLuZQwJpcafoE5jh%252BAPfs7DnIvtYOwW7NsbEV2OMNrnETabNEzia0bIltk8AL8YwJvn%252BGEoyzK6Jn6ap1k28fNEhBcDjPZ5AlkkaVFeJcG7u2%252BLj9OpZ9RuaUI%252B%252F1z8dYH2hPavcB%252FdX2TxpYce%252Fg7YVsXrPA4WPOHuFiVv%252FmPR68PhF6qM2gI%253D';
    dashboard.initFromUrl.call(mockDashboard, hash);
    expect(mockDashboard.collectionName).toBe('abc');
    expect(mockDashboard.cpuFilter.value).toBe('');
    expect(mockDashboard.sort.value).toEqual({
      active: 'waittime',
      direction: 'desc',
    });
    expect(mockDashboard.tab.value).toBe(3);
    expect(mockDashboard.showMigrations.value).toBe(false);
    expect(mockDashboard.showSleeping.value).toBe(false);
    expect(mockDashboard.maxIntervalCount.value).toBe(5000);
    expect(mockDashboard.filter.value).toBe('');
    expect(mockDashboard.layers.value.length).toBe(4);
    const viewport = new Viewport({
      bottom: 1,
      left: 0,
      right: 1,
      top: 0,
      chartWidthPx: 782,
      chartHeightPx: 670,
    } as Viewport);
    expect(mockDashboard.viewport.value).toEqual(viewport);
  });

  it('should update share URL', () => {
    dashboard.updateShareURL.call(mockDashboard, 'cpuFilter')('abc');

    expect(mockDashboard.global.history.replaceState)
        .toHaveBeenCalledWith(
            null, '',
            '/dashboard#share=eJyrVkrJLC7ISaz0LyjJzM8rVrKqVkouKHXLzClJLVKyUkpMSlaqrQUAFyIN5w%253D%253D');
  });


  it('should display snack bar on error', async () => {
    const serviceSpy =
        spyOn(dashboard.collectionDataService, 'getCollectionParameters')
            .and.returnValue(throwError(new HttpErrorResponse({error: 'err'})));

    const snackBar = TestBed.get(MatSnackBar) as MatSnackBar;
    const snackSpy = spyOn(snackBar, 'open');

    await fixture.whenStable();

    dashboard.getCollectionParameters('abc');
    expect(serviceSpy).toHaveBeenCalled();

    expect(snackSpy).toHaveBeenCalledWith(
        'Failed to get parameters for abc\n' +
            'Reason:\n' +
            ' err',
        'Dismiss');
  });

  // TODO(sainsley): Test headmap / sidebar presence / size?
});
