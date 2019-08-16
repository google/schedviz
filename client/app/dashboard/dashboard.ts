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
import {ChangeDetectionStrategy, ChangeDetectorRef, Component, ElementRef, Inject, OnDestroy, OnInit, ViewChild} from '@angular/core';
import {MatSnackBar} from '@angular/material/snack-bar';
import {Sort, SortDirection} from '@angular/material/sort';
import {BehaviorSubject, merge} from 'rxjs';
import {switchMap} from 'rxjs/operators';

import {Checkpoint, CheckpointValue, CollectionParameters, CpuRunningLayer, CpuWaitQueueLayer, Interval, Layer} from '../models';
import {CollectionDataService} from '../services/collection_data_service';
import {ColorService} from '../services/color_service';
import {compress, createHttpErrorMessage, decompress, parseHashFragment, serializeHashFragment, SystemTopology, Viewport} from '../util';
import {COLLECTION_NAME_KEY, SHARE_KEY} from '../util/hash_keys';

/**
 * The SchedViz dashboard.
 */
@Component({
  selector: 'dashboard',
  templateUrl: './dashboard.ng.html',
  styleUrls: ['./dashboard.css'],
  // Default change detection disabled here and elsewhere. Diff checks are
  // triggered manually in the interest of performance.
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Dashboard implements OnInit, OnDestroy {
  private readonly global = window;
  @ViewChild('filterInput', {static: false}) filterInput!: ElementRef;

  loading = new BehaviorSubject<boolean>(true);
  // TODO(sainsley): Move the following properties into a global state.
  cpuFilter = new BehaviorSubject<string>('');
  preview = new BehaviorSubject<Interval|undefined>(undefined);
  layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
  viewport = new BehaviorSubject<Viewport>(new Viewport());
  tab = new BehaviorSubject<number>(0);
  threadSort = new BehaviorSubject<Sort>({active: '', direction: ''});
  filter = new BehaviorSubject<string>('');
  showMigrations = new BehaviorSubject<boolean>(false);
  showSleeping = new BehaviorSubject<boolean>(true);
  maxIntervalCount = new BehaviorSubject<number>(5000);
  expandedThread = new BehaviorSubject<string|undefined>(undefined);
  // End Global State

  cpuRunLayer = new CpuRunningLayer();
  cpuWaitQueueLayer = new CpuWaitQueueLayer();
  collectionName?: string;
  collectionParameters =
      new BehaviorSubject<CollectionParameters|undefined>(undefined);
  systemTopology?: SystemTopology;
  intervals: Interval[] = [];
  title = 'SchedViz';
  tabs = [
    'threads',
    'tasks',
    'events',
    'layers',
  ];

  // Force cast initial value to checkpoint because the real values will be
  // populated by the subscriptions to its component's BehaviorSubjects.
  private checkpoint = {displayOptions: {}} as Checkpoint;

  constructor(
      @Inject('CollectionDataService') public collectionDataService:
          CollectionDataService, public colorService: ColorService,
      private readonly cdr: ChangeDetectorRef,
      private readonly snackBar: MatSnackBar) {}


  hashState() {
    const shareValue = compress(this.checkpoint);
    const hash = parseHashFragment(this.global.location.hash);
    hash[SHARE_KEY] = shareValue;
    const serializedHash = serializeHashFragment(hash);
    const newURL = `${this.global.location.pathname}${serializedHash}`;
    this.global.history.replaceState(null, '', newURL);
  }

  updateShareURL<T extends CheckpointValue>(propName: string) {
    return (prop: T) => {
      // This cast is an external index signature for displayOptions
      // tsickle does not generate the proper Closure types for interfaces
      // with index signatures and statically defined fields, so we can't
      // include the index signature on the type itself.
      const dO = this.checkpoint.displayOptions as unknown as
          {[k: string]: CheckpointValue};
      dO[propName] = prop;
      this.hashState();
    };
  }

  ngOnInit() {
    this.initFromUrl(this.global.location.hash);
    this.cpuFilter.subscribe(this.updateShareURL('cpuFilter'));
    this.filter.subscribe(filter => {
      if (!isNaN(Number(filter.trim()))) {
        // pidFilter will always be a number because PIDs are numbers.
        this.checkpoint.displayOptions.pidFilter = filter;
        this.checkpoint.displayOptions.commandFilter = '';
      } else {
        this.checkpoint.displayOptions.commandFilter = filter;
        this.checkpoint.displayOptions.pidFilter = '';
      }
      this.hashState();
    });
    this.tab.subscribe(this.updateShareURL('tab'));
    this.showMigrations.subscribe(this.updateShareURL('showMigrations'));
    this.showSleeping.subscribe(this.updateShareURL('showSleeping'));
    this.expandedThread.subscribe(this.updateShareURL('expandedThread'));
    this.maxIntervalCount.subscribe(this.updateShareURL('maxIntervalCount'));
    this.threadSort.subscribe(sort => {
      this.checkpoint.displayOptions.threadListSortField = sort.active;
      // -1 is unused in schedviz 1, use it to mean undefined here.
      this.checkpoint.displayOptions.threadListSortOrder =
          Dashboard.serializeSortDirection(sort.direction);
      this.hashState();
    });
    this.viewport.subscribe(this.updateShareURL('viewport'));
    this.layers.pipe(switchMap(layers => merge(...layers)))
        .subscribe(
            () => this.updateShareURL('layers')(this.layers.value.map(
                layer => new Layer(
                    layer.value.name, layer.value.dataType, layer.value.ids,
                    layer.value.color, [], layer.value.visible))));
    const layers = this.layers.value;
    layers.push(new BehaviorSubject(this.cpuRunLayer as unknown as Layer));
    layers.push(new BehaviorSubject(this.cpuWaitQueueLayer));
    this.layers.next(layers);
  }

  ngOnDestroy() {
    // Close all Subjects to prevent leaks.
    this.cpuFilter.complete();
    this.preview.complete();
    for (const layer of this.layers.value) {
      layer.complete();
    }
    this.layers.complete();
    this.viewport.complete();
    this.tab.complete();
    this.threadSort.complete();
    this.filter.complete();
    this.showMigrations.complete();
    this.showSleeping.complete();
    this.expandedThread.complete();
  }

  initFromUrl(hash: string) {
    const hashMap = parseHashFragment(decodeURIComponent(hash));
    if (hashMap[COLLECTION_NAME_KEY]) {
      this.collectionName = hashMap[COLLECTION_NAME_KEY];
      if (this.collectionName) {
        this.getCollectionParameters(this.collectionName, hashMap);
      }
    }
    if (hashMap[SHARE_KEY]) {
      const compressed = hashMap[SHARE_KEY];
      if (compressed) {
        const share = decompress<Checkpoint>(compressed);
        if (share) {
          this.checkpoint = share;
          const dO = share.displayOptions;
          this.cpuFilter.next((dO.cpuFilter || '').trim().toLowerCase());
          this.tab.next(Math.max(Number(dO.tab), 0));
          this.viewport.next(new Viewport(dO.viewport));
          this.filter.next(dO.commandFilter || dO.pidFilter);
          this.showMigrations.next(dO.showMigrations);
          this.showSleeping.next(dO.showSleeping);
          this.expandedThread.next(dO.expandedThread);
          this.maxIntervalCount = new BehaviorSubject<number>(
              dO.maxIntervalCount != null ? dO.maxIntervalCount : 5000);
          this.threadSort.next({
            active: dO.threadListSortField,
            direction:
                Dashboard.deserializeSortDirection(dO.threadListSortOrder),
          });
          if (dO.layers) {
            const layerSubjects: Array<BehaviorSubject<Layer>> = [];
            for (const layer of dO.layers) {
              // Check saved visibility for CPU layers
              if (layer.name === this.cpuRunLayer.name) {
                this.cpuRunLayer.visible = layer.visible;
              } else if (layer.name === this.cpuWaitQueueLayer.name) {
                this.cpuWaitQueueLayer.visible = layer.visible;
              } else {
                // Create new layer
                layerSubjects.push(new BehaviorSubject(new Layer(
                    layer.name, layer.dataType, layer.ids, layer.color, [],
                    layer.visible)));
                this.colorService.setColorFor(layer.name, layer.color);
              }
            }
            this.layers.next(layerSubjects);
          }
        }
      }
    }
  }


  /**
   * Fetch Observable for collection parameters, given collection name.
   */
  getCollectionParameters(
      collectionName: string, urlFilterHash?: {[k: string]: string}) {
    this.loading.next(true);
    this.collectionDataService.getCollectionParameters(collectionName)
        .subscribe(
            (parameters) => {
              this.loading.next(false);
              this.collectionParameters.next(parameters);
              this.getSystemTopology(parameters);
            },
            (err: HttpErrorResponse) => {
              this.loading.next(false);
              const errMsg = createHttpErrorMessage(
                  `Failed to get parameters for ${collectionName}`, err);
              this.snackBar.open(errMsg, 'Dismiss');
            });
  }

  /**
   * Fetches system topology on parameters ready.
   */
  getSystemTopology(collectionParameters: CollectionParameters) {
    this.loading.next(true);
    this.collectionDataService.getSystemTopology(collectionParameters)
        .subscribe(
            topology => {
              this.loading.next(false);
              this.systemTopology = topology;
              this.cdr.detectChanges();
            },
            () => {
              this.loading.next(false);
              this.systemTopology =
                  new SystemTopology(collectionParameters.cpus);
              this.cdr.detectChanges();
            });
  }

  static serializeSortDirection(direction: SortDirection): number {
    return direction === 'desc' ? 1 : (direction === 'asc' ? 0 : -1);
  }

  static deserializeSortDirection(direction: number): SortDirection {
    return direction === 1 ? 'desc' : (direction === 0 ? 'asc' : '');
  }
}
