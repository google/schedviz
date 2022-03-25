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
import {ChangeDetectionStrategy, Component, ElementRef, Inject, OnDestroy, OnInit, ViewChild} from '@angular/core';
import {UntypedFormControl, FormGroupDirective, NgForm} from '@angular/forms';
import {ErrorStateMatcher} from '@angular/material/core';
import {MatSnackBar} from '@angular/material/snack-bar';
import {Router} from '@angular/router';
import {BehaviorSubject, Subject} from 'rxjs';

import {CollectionDuration} from '../models/collection';
import {CollectionsFilter} from '../models/collections_filter';
import {CollectionDataService} from '../services/collection_data_service';
import {showErrorSnackBar} from '../util';
import {parseHashFragment, serializeHashFragment} from '../util/hash_compressor';
import {COLLECTIONS_FILTER_KEYS, OWNER_KEY} from '../util/hash_keys';


const COLLECTION_URL = '/dashboard#collection=';

/**
 * Collections is a wrapper component for the collections page.
 */
@Component({
  selector: 'collections',
  templateUrl: './collections.ng.html',
  styleUrls: ['./collections.css'],
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class Collections implements OnInit, OnDestroy {
  private readonly global = window;
  loading = new BehaviorSubject<boolean>(true);
  owner = new BehaviorSubject<string>('');
  filter = new CollectionsFilter();
  refreshTable$ = new Subject<void>();
  lookupFormModel = {
    traceID: '',
  };
  matcher = new DisabledErrorStateMatcher();

  @ViewChild('fileInput', {static: false}) fileInput!: ElementRef;

  constructor(
      @Inject('CollectionDataService') public collectionDataService:
          CollectionDataService, public router: Router,
      private readonly snackBar: MatSnackBar) {}

  ngOnInit() {
    this.initFromURL(window.location.hash);
    this.owner.subscribe(
        owner =>
            // Store 'me' in the URL if no owner to make it look better
        this.updateFilterURL('owner', owner || 'me'));

    this.filter.changes.subscribe(change => {
      if (!change) {
        return;
      }
      const {prop, newVal} = change;
      this.updateFilterURL(prop, newVal);
    });

  }

  /**
   * updateFilterURL stores a key value pair in the URL.
   *
   * @param prop The key to store
   * @param newVal The value to store
   */
  updateFilterURL(prop: string, newVal: string) {
    const hash = parseHashFragment(this.global.location.hash);
    if (newVal) {
      hash[prop] = newVal;
    } else {
      delete hash[prop];
    }
    const serializedHash = serializeHashFragment(hash);
    const newURL = `${this.global.location.pathname}${serializedHash}`;
    this.global.history.replaceState(null, '', newURL);
  }

  /**
   * initFromURL sets the filter and owner to the values stored in the URL,
   * if any.
   * @param hash A URL hash fragment
   */
  initFromURL(hash: string) {
    const hashMap = parseHashFragment(hash);

    // For every filter key (i.e. not owner).
    for (const key of COLLECTIONS_FILTER_KEYS) {
      this.filter[key] = hashMap[key];
    }

    let owner = hashMap[OWNER_KEY];
    if (owner != null) {
      if (owner === 'me') {
        owner = '';
      }
      this.owner.next(owner);
    }
  }

  ngOnDestroy() {
    this.filter.changes.complete();
    this.refreshTable$.complete();
    this.loading.complete();
    this.owner.complete();
  }


  onLookupFormSubmit() {
    const {traceID} = this.lookupFormModel;
    if (!traceID) {
      return;
    }
    this.router.navigateByUrl(COLLECTION_URL + traceID);
  }

  uploadFile() {
    const files = this.fileInput.nativeElement.files;
    if (!files || !files.length) {
      return;
    }
    const file: File = files[0];
    // Reset file picker to allow for uploading more than one file
    this.fileInput.nativeElement.value = '';
    this.loading.next(true);
    this.collectionDataService
        .upload(file, this.owner.value || 'local_user', ['recorded'])
        .subscribe(
            (traceID: string) => {
              this.loading.next(false);
              this.router.navigateByUrl(COLLECTION_URL + traceID);
            },
            (err: HttpErrorResponse) => {
              this.loading.next(false);
              showErrorSnackBar(this.snackBar,
                  `Failed to upload trace file ${file.name}`, err);
            });
  }
}

class DisabledErrorStateMatcher implements ErrorStateMatcher {
  isErrorState(control: UntypedFormControl|null, form: FormGroupDirective|NgForm|null):
      boolean {
    return false;
  }
}
