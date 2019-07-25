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
import {BehaviorSubject, merge} from 'rxjs';
import {switchMap} from 'rxjs/operators';

import {CpuRunningLayer, CpuWaitQueueLayer, Layer} from '../../models';
import {ColorService} from '../../services/color_service';

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
  @Input() maxIntervalCount!: BehaviorSubject<number>;
  @Input() showMigrations!: BehaviorSubject<boolean>;
  @Input() showSleeping!: BehaviorSubject<boolean>;
  @Input() layers!: BehaviorSubject<Array<BehaviorSubject<Layer>>>;

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
  }

  // TODO(sainsley): The only fixed layers are the CPU layers. Use instanceof
  // Checks as a layer that happens to be empty will end up here as well.
  get fixedLayers() {
    return this.layers.value.filter(
        layer => layer.value instanceof CpuRunningLayer ||
            layer.value instanceof CpuWaitQueueLayer);
  }

  get adjustableLayers() {
    return this.layers.value.filter(
        layer => !(layer.value instanceof CpuRunningLayer) &&
            !(layer.value instanceof CpuWaitQueueLayer));
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
}
