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
import {ChangeDetectionStrategy, ChangeDetectorRef, Component, ElementRef, Input, OnDestroy, OnInit, ViewChild} from '@angular/core';
import {MatPaginator} from '@angular/material/paginator';
import {MatSort, Sort} from '@angular/material/sort';
import {MatTableDataSource} from '@angular/material/table';
import {BehaviorSubject, Subject} from 'rxjs';
import {takeUntil} from 'rxjs/operators';

import {Interval, Layer} from '../../models';
import {ColorService} from '../../services/color_service';
import * as Duration from '../../util/duration';
import {findIndex, throttle} from '../../util/helpers';

/**
 * Base class for table of SchedViz data for which dedicated layers can be
 * toggled on/off.
 * TODO(tracked) Make templated and remove casts of data member
 */
@Component({
  selector: 'selectable-table',
  template: `
    <div>
      <mat-table [dataSource]="dataSource"
                 matSort>
      </mat-table>
      <mat-paginator [pageSize]="pageSize" showFirstLastButtons>
      </mat-paginator>
    </div>
      `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SelectableTable implements OnInit, OnDestroy {
  @ViewChild(MatSort, {static: true}) matSort!: MatSort;
  @ViewChild(MatPaginator, {static: true}) paginator!: MatPaginator;
  @ViewChild('table', {static: true}) table!: ElementRef;

  @Input() data!: BehaviorSubject<Interval[]>;
  @Input() sort!: BehaviorSubject<Sort>;
  @Input() preview!: BehaviorSubject<Interval|undefined>;
  @Input() layers!: BehaviorSubject<Array<BehaviorSubject<Layer>>>;
  @Input() tab!: BehaviorSubject<number>;
  @Input() dataPending = false;

  selection = new SelectionModel<string>(true, []);
  displayedColumns = ['selected'];
  dataSource: MatTableDataSource<Interval>;
  layersArray: Layer[] = [];
  pageSize = 10;

  protected readonly unsub$ = new Subject<void>();

  protected readonly outputErrorThrottled = throttle((message) => {
    console.error(message);
  }, 500);

  constructor(
      public colorService: ColorService, protected cdr: ChangeDetectorRef) {
    this.dataSource = new MatTableDataSource<Interval>();
  }

  ngOnInit() {
    // Check required inputs
    if (!this.data) {
      throw new Error('Missing Observable for table data');
    }
    if (!this.layers) {
      throw new Error('Missing Observable for layers');
    }
    if (!this.preview) {
      throw new Error('Missing Observable for preview');
    }
    if (!this.sort) {
      throw new Error('Missing Observable for table sort');
    }
    this.dataSource.sort = this.matSort;
    if (this.paginator) {
      this.dataSource.paginator = this.paginator;
      this.sort.pipe(takeUntil(this.unsub$)).subscribe(() => {
        this.paginator.firstPage();
      });
    }
    // Subscribe to data changes
    this.data.subscribe((data: Interval[]) => {
      this.dataSource.data = data;
      const selected = this.selection.selected;
      for (const row of data) {
        const selectedIdx =
            findIndex(selected, selection => row.label.startsWith(selection));
        row.selected = selectedIdx >= 0;
        const selectionName = selected[selectedIdx];
      }
    });
    // Update layer toggle button on layers change
    this.layers.subscribe((layers: Array<BehaviorSubject<Layer>>) => {
      // Listen for layer deselections
      const selected = this.selection.selected;
      if (layers.length < selected.length) {
        for (const row of selected) {
          const layerIdx = findIndex(layers, layer => layer.value.name === row);
          if (layerIdx < 0) {
            this.selection.deselect(row);
          }
        }
      } else if (layers.length > selected.length) {
        for (const layer of layers) {
          this.selection.select(layer.value.name);
        }
      }
      // TODO(sainsley): Remove by adding listener on layer toggle element
      this.cdr.detectChanges();
    });
    if (this.tab) {
      // If this tab is not active, onResize will compute a useless value.
      // Subscribe to tab changes to recompute onResize when this tab is
      // visible.
      this.tab.subscribe(() => {
        this.onResize();
      });
    }
  }

  ngOnDestroy(): void {
    this.unsub$.next();
    this.unsub$.complete();
  }

  onResize() {}

  /**
   * Toggles selection (layer creation) for the given row.
   */
  toggleSelection(row: Interval) {
    this.toggleLayer(row, row.dataType, [row.id]);
  }

  /**
   * Toggles selection (layer creation) for the given row and list of data ids
   * for the new layer.
   */
  toggleLayer(row: Interval|string, dataType: string, ids: number[] = []) {
    const rowLabel = row instanceof Interval ? row.label : row;
    const renderData = row instanceof Interval ? [row] : [];
    if (row instanceof Interval) {
      row.selected = !row.selected;
    }
    this.selection.toggle(rowLabel);
    const layers = this.layers.value;
    if (this.selection.isSelected(rowLabel)) {
      const color = this.colorService.getColorFor(rowLabel);
      const newLayer = new Layer(rowLabel, dataType, ids, color, renderData);
      layers.unshift(new BehaviorSubject(newLayer));
    } else {
      const layerIdx =
          findIndex(layers, (layer) => layer.value.name === rowLabel);
      if (layerIdx >= 0) {
        layers.splice(layerIdx, 1);
      }
      this.colorService.removeColorFor(rowLabel);
    }
    this.layers.next(layers);
  }

  /**
   * Helper method to format durations as trimmed, human readable strings.
   * @param durationNs the duration value in nanoseconds
   */
  formatTime(durationNs: number) {
    const formattedTime =
        Duration.getHumanReadableDurationFromNs(durationNs, undefined, 1);
    return formattedTime;
  }

  /**
   * Thread row mouseover callback.
   * @param Thread the thread currently under the user mouse, for preview
   */
  previewRow(row?: Interval) {
    this.preview.next(row);
  }

  /**
   * Returns a boolean indicating whether text is currently selected
   */
  isTextSelected() {
    const selection = window.getSelection();
    if (!selection) {
      return false;
    }

    return selection.toString().length > 0;
  }
}
