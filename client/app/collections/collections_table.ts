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
import {SelectionModel} from '@angular/cdk/collections';
import {formatDate} from '@angular/common';
import {HttpErrorResponse} from '@angular/common/http';
import {ChangeDetectionStrategy, ChangeDetectorRef, Component, ElementRef, Inject, Input, LOCALE_ID, OnDestroy, OnInit, ViewChild} from '@angular/core';
import {MAT_DIALOG_DATA, MatDialog, MatDialogRef} from '@angular/material/dialog';
import {MatPaginator} from '@angular/material/paginator';
import {MatSnackBar} from '@angular/material/snack-bar';
import {MatSort} from '@angular/material/sort';
import {MatTableDataSource} from '@angular/material/table';
import {Router} from '@angular/router';
import {csvFormat} from 'd3';
import {BehaviorSubject, forkJoin, Subject} from 'rxjs';
import {debounceTime, takeUntil} from 'rxjs/operators';

import {CollectionMetadata} from '../models';
import {CollectionsFilter} from '../models/collections_filter';
import {CollectionDataService} from '../services/collection_data_service';
import {createHttpErrorMessage, Reflect} from '../util/helpers';

/**
 * The CollectionsTable displays collection info in a sortable, searchable,
 * Angular 2 material table.
 */
@Component({
  selector: 'collections-table',
  styleUrls: ['collections_table.css'],
  templateUrl: 'collections_table.ng.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {'(window:resize)': 'onResize()'},
})
export class CollectionsTable implements OnInit, OnDestroy {
  @ViewChild(MatSort, {static: true}) matSort!: MatSort;
  columns: Array<keyof CollectionMetadata> = [
    'targetMachine',
    'creationTime',
    'tags',
    'description',
    'name',
  ];

  private unsub$ = new Subject<void>();

  displayedColumns = ['select', 'creator', ...this.columns];
  rowHeightPx = 37;
  @ViewChild('table', {static: true}) table!: ElementRef;
  @ViewChild(MatPaginator, {static: true}) paginator!: MatPaginator;

  // Safe to assert non-null because of check in ngOnInit()
  @Input() loading!: BehaviorSubject<boolean>;
  @Input() owner!: BehaviorSubject<string>;
  @Input() refresh!: Subject<void>;
  dataSource = new MatTableDataSource<CollectionMetadata>();

  @Input() filter!: CollectionsFilter;

  selection: SelectionModel<CollectionMetadata>;

  filterPredicate = (data: CollectionMetadata, filter: string):
      boolean => {
        if (filter === '') {
          return true;
        }
        const parsedFilter = new CollectionsFilter(JSON.parse(filter));
        return this.columns.reduce<boolean>((acc, col) => {
          let cell = data[col];
          if (cell instanceof Date) {
            cell = formatDate(cell, 'medium', this.locale);
          }
          const filterValue = Reflect.get(parsedFilter, col) as string;
          return acc && `${cell}`.toLowerCase().indexOf(filterValue) > -1;
        }, true);
      }

  constructor(
      @Inject('CollectionDataService') public collectionDataService:
          CollectionDataService, public router: Router,
      private readonly cdr: ChangeDetectorRef, public dialog: MatDialog,
      private readonly snackBar: MatSnackBar,
      @Inject(LOCALE_ID) private readonly locale: string) {
    this.selection = new SelectionModel<CollectionMetadata>(true, []);
  }

  ngOnInit() {
    if (!this.refresh || !this.owner || !this.loading || !this.filter) {
      throw new Error(
          'Must provide refresh, owner, loading, and filter inputs to CollectionsTable');
    }
    this.dataSource.paginator = this.paginator;
    this.dataSource.sort = this.matSort;
    this.dataSource.sort.sort(
        {id: 'creationTime', start: 'desc', disableClear: false});
    this.dataSource.filterPredicate = this.filterPredicate;

    this.filter.changes.pipe(debounceTime(500)).subscribe(() => {
      this.dataSource.filter = JSON.stringify(this.filter);
      // Reset selection when filter changes
      this.selection = new SelectionModel<CollectionMetadata>(true, []);
    });

    this.refresh.subscribe(() => {
      this.listCollectionMetadata(this.owner.value);
    });

    this.owner.pipe(debounceTime(500)).subscribe((owner) => {
      if (owner != null) {
        this.listCollectionMetadata(owner);
      }
    });
  }

  ngOnDestroy() {
    this.unsub$.next();
    this.unsub$.complete();
  }

  /**
   * Fetch Observable for collection parameters, given collection name.
   */
  listCollectionMetadata(owner: string) {
    this.loading.next(true);
    this.collectionDataService.listCollectionMetadata(owner).subscribe(
        metadata => {
          this.loading.next(false);
          this.dataSource.data = metadata;
          this.selection = new SelectionModel<CollectionMetadata>(true, []);
          this.onResize();
        },
        (err: HttpErrorResponse) => {
          this.loading.next(false);
          const errMsg =
              createHttpErrorMessage('Failed to get list of collections', err);
          this.snackBar.open(errMsg, 'Dismiss');
        });
  }

  onResize() {
    const clientHeight = this.table.nativeElement.clientHeight;
    if (!clientHeight) {
      return;
    }
    // Update pageSize
    // 57.563px for header, and 56px for footer
    const headerFooterSize = 57.563 + 56;
    const tableHeightPx = clientHeight - headerFooterSize;
    const pageSize = Math.floor(tableHeightPx / this.rowHeightPx) - 1;
    this.paginator._changePageSize(pageSize);
    this.cdr.detectChanges();
  }


  /**
   * Whether the number of selected elements matches the total number of rows.
   */
  areAllSelected() {
    const numSelected = this.selection.selected.length;
    const numFilteredRows = this.dataSource.filteredData.length;
    return numSelected === numFilteredRows;
  }

  /**
   * Selects all rows if they are not all selected; otherwise clear selection.
   */
  selectAllToggle() {
    this.areAllSelected() ?
        this.selection.clear() :
        this.dataSource.filteredData.forEach(row => this.selection.select(row));
  }

  deleteCollections() {
    if (!this.selection.selected.length) {
      return;
    }
    const dialogRef = this.dialog.open(
        DialogDeleteConfirm,
        {data: {collectionNames: this.selection.selected.map(s => s.name)}});
    dialogRef.afterClosed().subscribe((res: boolean) => {
      if (!res) {
        return;
      }
      this.loading.next(true);
      forkJoin(this.selection.selected.map(
                   s => this.collectionDataService.deleteCollection(s.name)))
          .pipe(takeUntil(this.unsub$))
          .subscribe(
              () => {
                this.refresh.next();
              },
              (err: HttpErrorResponse) => {
                this.loading.next(false);
                const errMsg =
                    createHttpErrorMessage('Failed to delete collection', err);
                this.snackBar.open(errMsg, 'Dismiss');
              });
    });
  }

  downloadCollectionsList() {
    type SerializedCollectionMetadata = {
      [K in Exclude<keyof CollectionMetadata, 'creationTime'>]?:
          CollectionMetadata[K];
    }&{creationTime: Date | string | undefined};
    // Serialize dates in the collection list.
    const data =
        this.dataSource.sortData(this.dataSource.filteredData, this.matSort)
            .map(row => {
              const creationTime = row.creationTime;
              let newRow: SerializedCollectionMetadata = row;
              if (creationTime != null) {
                newRow = {
                  ...row,
                  creationTime: formatDate(creationTime, 'medium', this.locale)
                };
              }
              return newRow;
            });

    const csvOutput = csvFormat(data);
    const dataURL =
        `data:text/csv;charset=utf-8,${encodeURIComponent(csvOutput)}`;

    const anchor = document.createElement('a');
    anchor.download = 'collections.csv';
    anchor.target = '_blank';
    anchor.href = dataURL;
    anchor.click();
  }

  getCollectionUrl(name: string): string {
    return this.router
        .createUrlTree(['/dashboard'], {fragment: `collection=${name}`})
        .toString();
  }
}

/**
 * The data that is passed to DialogDeleteConfirm
 */
export interface DeleteDialogData {
  collectionNames: string[];
}

/**
 * A delete confirmation dialog
 */
@Component({
  selector: 'dialog-delete-confirm',
  template: `
    <h1 mat-dialog-title>Are you sure you want to delete these collections?</h1>
    <div mat-dialog-content>
      <pre>{{data.collectionNames.join("\\n")}}</pre>
    </div>
    <div mat-dialog-actions align="end">
      <button mat-stroked-button [mat-dialog-close]="false">No</button>
      <button mat-stroked-button [mat-dialog-close]="true">Yes</button>
    </div>
  `,
})
export class DialogDeleteConfirm {
  constructor(
      public dialogRef: MatDialogRef<DialogDeleteConfirm>,
      @Inject(MAT_DIALOG_DATA) public data: DeleteDialogData) {}
}
