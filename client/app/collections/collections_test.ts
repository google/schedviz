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
import {APP_BASE_HREF} from '@angular/common';
import {async, ComponentFixture, TestBed} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatButtonModule} from '@angular/material/button';
import {MatCheckboxModule} from '@angular/material/checkbox';
import {MatDialogModule} from '@angular/material/dialog';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {MatPaginatorModule} from '@angular/material/paginator';
import {MatProgressSpinnerModule} from '@angular/material/progress-spinner';
import {MatSelectModule} from '@angular/material/select';
import {MatSortModule} from '@angular/material/sort';
import {MatTableModule} from '@angular/material/table';
import {BrowserModule} from '@angular/platform-browser';
import {RouterTestingModule} from '@angular/router/testing';
import {BehaviorSubject,} from 'rxjs';

import {routes} from '../app_routing_module';
import {DashboardModule} from '../dashboard/dashboard_module';
import {CollectionsFilter, CollectionsFilterJSON} from '../models/collections_filter';
import {LocalCollectionDataService} from '../services/collection_data_service';

import {Collections} from './collections';
import {CollectionsTable} from './collections_table';
import {CollectionsToolbar} from './collections_toolbar';

describe('Collections', () => {
  let collectionsTable: CollectionsTable;
  let collectionsTableFixture: ComponentFixture<CollectionsTable>;
  let collections: Collections;
  let collectionsFixture: ComponentFixture<Collections>;

  class MockCollections {
    constructor() {}
    owner = new BehaviorSubject<string>('');
    filter = new CollectionsFilter();
    global = {
      'history': jasmine.createSpyObj('history', ['replaceState']),
      'location': {'hash': '', 'pathname': '/collections'}
    };
  }

  beforeEach(() => {
    TestBed.configureTestingModule({
      imports: [
        BrowserModule,
        DashboardModule,
        RouterTestingModule.withRoutes(routes),
        FormsModule,
        MatDialogModule,
        MatCheckboxModule,
        MatSortModule,
        MatIconModule,
        MatButtonModule,
        MatPaginatorModule,
        MatTableModule,
        MatInputModule,
        MatSelectModule,
        MatProgressSpinnerModule,
      ],
      providers: [
        {
          provide: 'CollectionDataService',
          useClass: LocalCollectionDataService
        },
        {provide: APP_BASE_HREF, useValue: '/'},
      ],
      declarations: [CollectionsTable, Collections, CollectionsToolbar],
    });
  });

  beforeEach(async(() => {
    TestBed.compileComponents().then(() => {
      collectionsTableFixture = TestBed.createComponent(CollectionsTable);
      collectionsTable = collectionsTableFixture.debugElement.componentInstance;
      collectionsFixture = TestBed.createComponent(Collections);
      collections = collectionsFixture.debugElement.componentInstance;
    });
  }));

  it('should create the collections table', () => {
    expect(collectionsTable).toBeTruthy();
  });

  it('should parse and load data from the hash fragment in the URL', () => {
    const mock = new MockCollections();

    const hash = '#creationTime=mar%202&name=9f8&owner=me';
    collections.initFromURL.call(mock, hash);
    expect(mock.owner.value).toBe('');
    expect(mock.filter.toJSON()).toEqual({
      'creationTime': 'mar 2',
      'name': '9f8',
    } as CollectionsFilterJSON);
  });

  it('should update filter URL', () => {
    const mock = new MockCollections();

    collections.updateFilterURL.call(mock, 'creationTime', 'mar 2');

    expect(mock.global.history.replaceState)
        .toHaveBeenCalledWith(null, '', '/collections#creationTime=mar%202');
  });
});
