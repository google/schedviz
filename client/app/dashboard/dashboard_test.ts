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
import {Checkpoint, CpuIdleWaitLayer, CpuRunningLayer, CpuWaitQueueLayer, Layer} from '../models';

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
    cpuIdleWaitLayer = new CpuIdleWaitLayer();
    cpuFilter = new BehaviorSubject<string>('');
    filter = new BehaviorSubject<string>('');
    layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
    viewport = new BehaviorSubject<Viewport>(new Viewport());
    tab = new BehaviorSubject<number>(0);
    tabs = dashboard.tabs;
    threadSort = new BehaviorSubject<Sort>({active: '', direction: ''});
    showMigrations = new BehaviorSubject<boolean>(false);
    showSleeping = new BehaviorSubject<boolean>(false);
    expandedThread = new BehaviorSubject<string|undefined>(undefined);
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
        '#collection=abc&share=eJzFVU2P2jAQ%252FSvIvbRSoPkEkuuqqJVa7bZLtYeKgxMPiUWII3sCbFf8946TNMBWQr205JBYz29m3huNJi9MSFOX%252FPm%252BRqkqw5IXltXNQpYImiUsYg4TgFyWC56hIihyHVZLMTBiYmRqu%252BXVCSMIecoSz2GmUPsvMte8T7%252FmpYEOfiwBalnlA4iFBi4%252BS4OPSuNCQiko2Z5LRLkF9ppwr4WtRkV2EvY1IVZ9CWv6kkhUdfvVMi%252BwpaUKUW3bY1ZwjU9SYPFwYMnUjXroI1i2xTzXd48Oo9aAJt0%252FXljFSUTClq2IxAv8aewlm06TsH3iyJfP9YlDmBQ2tuOubKNK20P2JoZY%252BKElVNSyHfkn2so6MTItKQXqBvrbWpUcYehSqqzvb1zIhoICcig0338QOZhTmETJS%252FkTRB92dP7Q7%252Ftu7CebvdIb0O%252BjxLtuoaWfW1ivZ24Y3NLCfDo%252FN%252BBfNWDJ5%252FJdl2eZd0v5s3g2nw%252F6%252FTBxrxpo6ecOeBrO0uwmDu4evo8%252BDVUvVNPVIPlyXOxzRW0v7V%252FJfaI1MvraQAN%252FJ1jn6ds4cEZ%252B6NPLC9%252F9x0av7P40m2GfCthR9S0%252F%252FO75nWoqtLvYdTvq66XZS%252BtGqGNfEvvlSeFwqGl3g1jSLUX6wSTPYEwlJ2pnxo2pjT2IdLKTGitAezUulMGJN42jYDqLYvuXGNJ0UztMuR9FYRQn3ZSz4%252FEXB7Uklw%253D%253D';
    dashboard.initFromUrl.call(mockDashboard, hash);
    expect(mockDashboard.collectionName).toBe('abc');
    expect(mockDashboard.cpuFilter.value).toBe('5');
    expect(mockDashboard.expandedThread.value).toBe('Thread:255459:worker');
    expect(mockDashboard.threadSort.value).toEqual({
      active: 'waittime',
      direction: 'desc',
    });
    expect(mockDashboard.tab.value).toBe(1);
    expect(mockDashboard.showMigrations.value).toBe(false);
    expect(mockDashboard.showSleeping.value).toBe(false);
    expect(mockDashboard.maxIntervalCount.value).toBe(5000);
    expect(mockDashboard.filter.value).toBe('9');
    expect(mockDashboard.layers.value.length).toBe(4);
    const viewport = new Viewport({
      bottom: 1,
      left: 0,
      right: 1,
      top: 0,
      chartWidthPx: 605,
      chartHeightPx: 1020,
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
