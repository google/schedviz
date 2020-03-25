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
import {HttpClient, HttpErrorResponse} from '@angular/common/http';
import {ChangeDetectionStrategy, Component, Inject, OnInit} from '@angular/core';
import {MAT_DIALOG_DATA, MatDialog, MatDialogRef} from '@angular/material/dialog';
import {MatSnackBar} from '@angular/material/snack-bar';
import {Router} from '@angular/router';
import {BehaviorSubject, of} from 'rxjs';
import {catchError, map, switchMap, take} from 'rxjs/operators';

import {CollectionMetadata} from '../models';
import {CollectionDataService} from '../services/collection_data_service';
import {parseHashFragment, recordHttpErrorMessage} from '../util';
import {copyToClipboard} from '../util/clipboard';
import {COLLECTION_NAME_KEY} from '../util/hash_keys';



/**
 * A toolbar for the dashboard
 */
@Component({
  selector: 'dashboard-toolbar',
  styleUrls: ['dashboard_toolbar.css'],
  templateUrl: './dashboard_toolbar.ng.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DashboardToolbar implements OnInit {
  private readonly global = window;

  collectionName?: string;
  collectionMetadata =
      new BehaviorSubject<CollectionMetadata|undefined>(undefined);
  constructor(
      @Inject('CollectionDataService') public collectionDataService:
          CollectionDataService,
      public router: Router,
      private readonly http: HttpClient,
      private readonly snackBar: MatSnackBar,
      public dialog: MatDialog,
  ) {}

  ngOnInit() {
    this.initFromURL(this.global.location.hash);
  }

  /**
   * initFromURL gets the collection name from the URL and uses it to fetch
   * the collection metadata.
   * @param hash A URL hash fragment
   */
  initFromURL(hash: string) {
    const hashMap = parseHashFragment(decodeURIComponent(hash));
    if (hashMap[COLLECTION_NAME_KEY]) {
      this.collectionName = hashMap[COLLECTION_NAME_KEY];
      this.collectionDataService.getCollectionMetadata(this.collectionName)
          .subscribe(
              metadata => {
                this.collectionMetadata.next(metadata);
              },
              (err: HttpErrorResponse) => {
                const errMsg = recordHttpErrorMessage(
                    `Failed to get collection metadata for ${
                        this.collectionName}`,
                    err);
                this.snackBar.open(errMsg, 'Dismiss');
              });
    }
  }

  /**
   * Used by the list icon to go to the collections page
   */
  goToCollections() {
    this.router.navigate(['/collections']);
  }


  /**
   * Used by the edit collection button. Shows a dialog with a form to edit
   * the current collection's metadata.
   */
  editCollectionInfo() {
    if (!this.collectionName || !this.collectionMetadata.value) {
      return;
    }
    const metadata = this.collectionMetadata.value;
    const collectionName = this.collectionName;
    const oldTags = metadata.tags;

    // Open Dialog
    const dialogRef = this.dialog.open(DialogEditCollection, {
      data: {
        description: metadata.description,
        tags: metadata.tags.join(','),
        owners: metadata.owners.join(','),
      },
      minWidth: 370,
    });
    // Wait for dialog to be closed
    dialogRef.afterClosed()
        .pipe(
            // Send editCollection request
            switchMap((res: EditCollectionDialogData|null) => {
              if (!res) {
                throw new Error('canceled');
              }
              const {description, tags, owners} = res;
              const newTags =
                  tags.split(',').map(o => o.trim()).filter(o => o.length);
              const ownersArr =
                  owners.split(',').map(o => o.trim()).filter(o => o.length);

              const removedTags =
                  oldTags.filter(tag => newTags.indexOf(tag) === -1);
              const addedTags =
                  newTags.filter(tag => oldTags.indexOf(tag) === -1);

              return this.collectionDataService.editCollection(
                  collectionName,
                  removedTags,
                  addedTags,
                  description,
                  ownersArr,
              );
            }),
            // Get the updated collection back
            switchMap(() => {
              return this.collectionDataService.getCollectionMetadata(
                  collectionName);
            }),
            )
        .subscribe(
            metadata => {
              // Save the updated collection in the component state.
              this.collectionMetadata.next(metadata);
            },
            (err: HttpErrorResponse) => {
              if (err.message && err.message !== 'canceled') {
                const errMsg = recordHttpErrorMessage(
                    `Failed to update collection metadata for ${
                        collectionName}`,
                    err);
                this.snackBar.open(errMsg, 'Dismiss');
              }
            });
  }

  /**
   * Opens a link to file a new bug for Schedviz
   */
  createBug() {
    this.global.open(
        'https://github.com/google/schedviz/issues/new', '_blank');
  }
}

/**
 * The data that is passed to DialogEditCollection
 */
export interface EditCollectionDialogData {
  description: string;
  tags: string;
  owners: string;
}


/**
 * A dialog to edit the collection metadata
 */
@Component({
  selector: 'dialog-edit-collection',
  template: `
    <h1 mat-dialog-title>Tell us about it.</h1>
    <form #collectionForm="ngForm" (ngSubmit)="onCollectionFormSubmit()">
      <div mat-dialog-content>
          <mat-form-field>
            <input matInput
                   [(ngModel)]="data.description"
                   placeholder="Description"
                   name="description">
          </mat-form-field>
          <mat-form-field>
            <input matInput
                   [(ngModel)]="data.tags"
                   placeholder="Comma-separated tags"
                   name="tags">
          </mat-form-field>
          <mat-form-field>
            <input matInput
                   [(ngModel)]="data.owners"
                   placeholder="Comma-separated LDAPs to share with"
                   name="owners">
          </mat-form-field>
      </div>
      <div mat-dialog-actions align="end">
        <button type="button" mat-stroked-button [mat-dialog-close]="null">
          Cancel
        </button>
        <button type="submit" mat-stroked-button>Submit</button>
      </div>
    </form>
  `,
  styles: [`
    mat-form-field.mat-form-field {
      display: block;
    }
  `]
})
export class DialogEditCollection {
  constructor(
      public dialogRef: MatDialogRef<DialogEditCollection>,
      @Inject(MAT_DIALOG_DATA) public data: EditCollectionDialogData) {}

  onCollectionFormSubmit() {
    this.dialogRef.close(this.data);
  }
}
