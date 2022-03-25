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
import {CdkDragDrop, moveItemInArray} from '@angular/cdk/drag-drop';
import {ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnInit} from '@angular/core';
import {UntypedFormControl} from '@angular/forms';
import {ErrorStateMatcher} from '@angular/material/core';
import {BehaviorSubject, merge} from 'rxjs';
import {switchMap} from 'rxjs/operators';


import {CollectionParameters, CpuIdleWaitLayer, CpuRunningLayer, CpuWaitQueueLayer, Layer} from '../../models';
import {ColorService} from '../../services/color_service';
import {Viewport} from '../../util';
import {getHumanReadableDurationFromNs, timeInputToNs} from '../../util/duration';

/**
 * Matcher which enters an error state immediately, ignoring empty fields.
 */
class ImmediateErrorStateMatcher implements ErrorStateMatcher {
  isErrorState(control: UntypedFormControl|null): boolean {
    if (!control) {
      return false;
    }

    return control.invalid && control.value !== '';
  }
}

/**
 * A display settings menu. Contains all components the control rendering
 * parameters, including render layers, which can be reordered via drag-drop.
 */
@Component({
  selector: 'settings-menu',
  styleUrls: ['./settings_menu.css'],
  templateUrl: 'settings_menu.ng.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SettingsMenu implements OnInit {
  @Input() parameters!: BehaviorSubject<CollectionParameters|undefined>;
  @Input() maxIntervalCount!: BehaviorSubject<number>;
  @Input() showMigrations!: BehaviorSubject<boolean>;
  @Input() showSleeping!: BehaviorSubject<boolean>;
  @Input() layers!: BehaviorSubject<Array<BehaviorSubject<Layer>>>;
  @Input() viewport!: BehaviorSubject<Viewport>;
  jumpToTimeMatcher = new ImmediateErrorStateMatcher();
  viewportStartTimeInput = new BehaviorSubject<string>('');
  viewportEndTimeInput = new BehaviorSubject<string>('');

  constructor(
      private readonly cdr: ChangeDetectorRef,
      private readonly colorService: ColorService,
  ) {}

  ngOnInit() {
    // Check required inputs
    if (!this.layers) {
      throw new Error('Missing Observable for layers');
    }
    if (!this.showMigrations) {
      throw new Error('Missing Observable for migrations render flag');
    }
    if (!this.showSleeping) {
      throw new Error('Missing Observable for sleep state render flag');
    }
    if (!this.maxIntervalCount) {
      throw new Error('Missing Observable for max interval count');
    }
    // Listen for changes on all indices in the layer list so we can
    // force-refresh rows. Note: We use change detection here on the
    // assumption that the layers list is relatively small.
    // TODO(sainsley): Investigate removing force change detection here.
    this.layers.pipe(switchMap(layers => merge(...layers)))
        .subscribe(() => this.cdr.detectChanges());
    this.viewportStartTimeInput.pipe(timeInputToNs).subscribe((startTimeNs) => {
      this.updateViewportInX(startTimeNs);
    });
    this.viewportEndTimeInput.pipe(timeInputToNs).subscribe((endTimeNs) => {
      this.updateViewportInX(endTimeNs, /** updateRight */ true);
    });
  }

  // TODO(sainsley): The only fixed layers are the CPU layers. Use instanceof
  // Checks as a layer that happens to be empty will end up here as well.
  get fixedLayers() {
    return this.layers.value.filter(
        layer => layer.value instanceof CpuRunningLayer ||
            layer.value instanceof CpuWaitQueueLayer ||
            layer.value instanceof CpuIdleWaitLayer);
  }

  get adjustableLayers() {
    return this.layers.value.filter(
        layer => !(layer.value instanceof CpuRunningLayer) &&
            !(layer.value instanceof CpuWaitQueueLayer) &&
            !(layer.value instanceof CpuIdleWaitLayer));
  }

  shouldShowColorPicker(layer: BehaviorSubject<Layer>) {
    return !(layer.value instanceof CpuRunningLayer) &&
        !(layer.value instanceof CpuWaitQueueLayer);
  }

  drop(event: CdkDragDrop<string[]>) {
    const layers = this.layers.value;
    moveItemInArray(layers, event.previousIndex, event.currentIndex);
    this.layers.next(layers);
  }

  removeLayer(layer: BehaviorSubject<Layer>) {
    const layers = this.layers.value;
    layers.splice(layers.indexOf(layer), 1);
    layer.complete();
    this.colorService.removeColorFor(layer.value.name);
    this.layers.next(layers);
  }

  hideLayer(layerSub: BehaviorSubject<Layer>) {
    const layer = layerSub.value;
    layer.visible = !layer.visible;
    layerSub.next(layer);
  }

  updateLayerColor(layerSub: BehaviorSubject<Layer>, color: string) {
    const layer = layerSub.value;
    layer.color = color;
    this.colorService.setColorFor(layer.name, color);
    // Update the colors in other components.
    this.cdr.detectChanges();
    layerSub.next(layer);
  }

  /**
   * Formats viewport left in best fit units of time.
   */
  getViewportStartTime(viewport: Viewport) {
    return this.getViewportTime(viewport);
  }

  /**
   * Formats viewport right in best fit units of time.
   */
  getViewportEndTime(viewport: Viewport) {
    return this.getViewportTime(viewport, true /** returnEndTime */);
  }

  /**
   * Formats viewport parametric value in best fit units of time.
   */
  private getViewportTime(viewport: Viewport, returnEndTime = false) {
    const parameters = this.parameters.value;
    if (!parameters) {
      return 0;
    }
    const viewportTime = returnEndTime ? viewport.right : viewport.left;
    const timeNs = Math.round(
        viewportTime * parameters.domainSizeNs + parameters.startTimeNs);
    return getHumanReadableDurationFromNs(timeNs);
  }

  private updateViewportInX(viewportTimeNs: number, updateRight = false) {
    const parameters = this.parameters.value;
    if (!parameters || !parameters.domainSizeNs) {
      return;
    }
    const viewport = this.viewport.value;
    const startNs = parameters.startTimeNs;
    const domainSize = parameters.domainSizeNs;
    const viewportX = (viewportTimeNs - startNs) / domainSize;
    const viewportRight =
        updateRight ? Math.min(1.0, viewportX) : viewport.right;
    const viewportLeft = updateRight ? viewport.left : Math.max(0, viewportX);
    if (viewportLeft < viewportRight) {
      viewport.left = viewportLeft;
      viewport.right = viewportRight;
      this.viewport.next(viewport);
    }
  }
}
