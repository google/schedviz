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
import {HttpErrorResponse} from '@angular/common/http';
import {ComponentFixture, TestBed} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatButtonModule} from '@angular/material/button';
import {MatCheckboxModule} from '@angular/material/checkbox';
import {MatDialogModule} from '@angular/material/dialog';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {MatPaginatorModule} from '@angular/material/paginator';
import {MatProgressSpinnerModule} from '@angular/material/progress-spinner';
import {MatSelectModule} from '@angular/material/select';
import {MatSnackBar} from '@angular/material/snack-bar';
import {MatSortModule} from '@angular/material/sort';
import {MatTableModule} from '@angular/material/table';
import {By} from '@angular/platform-browser';
import {BrowserModule} from '@angular/platform-browser';
import {Router} from '@angular/router';
import {RouterTestingModule} from '@angular/router/testing';
import {BehaviorSubject,} from 'rxjs';
import {Subject, throwError} from 'rxjs';

import {routes} from '../app_routing_module';
import {DashboardModule} from '../dashboard/dashboard_module';
import {CollectionMetadata, CollectionsFilter, CollectionsFilterJSON} from '../models';
import {LocalCollectionDataService} from '../services';

import {Collections} from './collections';
import {CollectionsTable} from './collections_table';
import {CollectionsToolbar} from './collections_toolbar';
import {SelectableAnchor} from './selectable_anchor';

describe('Collections', () => {
  let collectionsTable: CollectionsTable;
  let collectionsTableFixture: ComponentFixture<CollectionsTable>;
  let collections: Collections;
  let collectionsFixture: ComponentFixture<Collections>;

  class MockCollections {
    owner = new BehaviorSubject<string>('');
    filter = new CollectionsFilter();
    global = {
      'history': jasmine.createSpyObj('history', ['replaceState']),
      'location': {'hash': '', 'pathname': '/collections'}
    };
  }

  function getMockData() {
    return [
      // The collections must have different collection dates to allow for
      // deterministic sorting
      new CollectionMetadata(
          'coll', 'joe', ['john'], ['abc'], 'def', new Date(2001, 1),
          ['switch'], 'mach'),
      new CollectionMetadata(
          'coll2', 'joe', ['john'], ['abc'], 'def', new Date(2000, 1),
          ['switch'], 'mach'),
    ];
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
      declarations:
          [CollectionsTable, Collections, CollectionsToolbar, SelectableAnchor],
    });
  });

  beforeEach(async () => {
    await TestBed.compileComponents();
    collectionsTableFixture = TestBed.createComponent(CollectionsTable);
    collectionsTable = collectionsTableFixture.debugElement.componentInstance;
    collectionsTable.refresh = new Subject<void>();
    collectionsTable.owner = new BehaviorSubject<string>('me');
    collectionsTable.loading = new BehaviorSubject<boolean>(false);
    collectionsTable.filter = new CollectionsFilter();
    collectionsTable.dataSource.data = getMockData();

    collectionsFixture = TestBed.createComponent(Collections);
    collections = collectionsFixture.debugElement.componentInstance;
  });

  it('should create the collections table', () => {
    expect(collectionsTable).toBeTruthy();
  });

  it('should open dashboard on collection name click', async () => {
    collectionsTableFixture.detectChanges();
    const row =
        collectionsTableFixture.debugElement.nativeElement.querySelector(
            '.mat-row-link');
    row.click();
    await collectionsTableFixture.whenStable();

    const router = TestBed.get(Router) as Router;
    expect(router.url).toBe('/dashboard#collection=coll');
  });


  it('should open collection on "look up by trace" submit', async () => {
    await collectionsFixture.whenStable();
    collectionsFixture.detectChanges();
    const de =
        collectionsFixture.debugElement.query(By.css('[name="lookupForm"]'));
    const traceIDField = de.query(By.css('[name=\'traceID\']')).nativeElement;
    traceIDField.focus();
    await collectionsFixture.whenStable();
    traceIDField.value = 'coll';
    traceIDField.dispatchEvent(new Event('input'));
    collectionsFixture.detectChanges();

    await collectionsFixture.whenStable();

    const btn = de.query(By.css('button')).nativeElement;
    expect(btn.disabled).toBe(false);
    btn.click();
    await collectionsFixture.whenStable();

    expect(collections.lookupFormModel).toEqual({traceID: 'coll'});

    const router = TestBed.get(Router) as Router;
    expect(router.url).toBe('/dashboard#collection=coll');
  });

  it('should display snack bar on error', async () => {
    const uploadSpy =
        spyOn(collections.collectionDataService, 'upload')
            .and.returnValue(throwError(new HttpErrorResponse({error: 'err'})));

    const snackBar = TestBed.get(MatSnackBar) as MatSnackBar;
    const snackSpy = spyOn(snackBar, 'open');
    const consoleSpy = spyOn(console, 'error');

    await collectionsFixture.whenStable();

    const mockFileInput = {nativeElement: {files: [new File([], 'file')]}};
    collections.fileInput = mockFileInput;

    collections.uploadFile();
    expect(uploadSpy).toHaveBeenCalled();

    expect(snackSpy).toHaveBeenCalledWith(
        'Failed to upload trace file file', 'Dismiss');
    expect(consoleSpy)
        .toHaveBeenCalledWith(
            'Failed to upload trace file file\n' +
            'Message:\n' +
            ' Http failure response for (unknown url): undefined undefined\n' +
            'Reason:\n' +
            ' err');
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

  it('should generate CSV file', () => {
    const spyObj = jasmine.createSpyObj('a', ['click']);
    spyOn(document, 'createElement').and.returnValue(spyObj);

    collectionsTable.downloadCollectionsList();

    expect(document.createElement).toHaveBeenCalled();
    expect(document.createElement).toHaveBeenCalledWith('a');

    const expectedDataURL =
        'data:text/csv;charset=utf-8,name%2Ccreator%2Cowners%2Ctags%2Cdescription%2CcreationTime%2CeventNames%2CtargetMachine%0Acoll%2Cjoe%2Cjohn%2Cabc%2Cdef%2C%22Feb%201%2C%202001%2C%2012%3A00%3A00%20AM%22%2Cswitch%2Cmach%0Acoll2%2Cjoe%2Cjohn%2Cabc%2Cdef%2C%22Feb%201%2C%202000%2C%2012%3A00%3A00%20AM%22%2Cswitch%2Cmach';
    expect(spyObj.href).toBe(expectedDataURL);
    expect(spyObj.download).toBe('collections.csv');
    expect(spyObj.click).toHaveBeenCalled();
  });
});
