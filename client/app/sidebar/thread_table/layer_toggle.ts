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
import {Component, EventEmitter, Input, Output} from '@angular/core';

import {Interval} from '../../models';

/**
 * A custom toggle for user-created layers. Layer toggles appear in Thread,
 * Event, Task, etc. rows in their respective tables, and create (or remove)
 * dedicated render layers for their associated data on click.
 */
@Component({
  selector: 'layer-toggle',
  template: `
    <div class="toggle-root"
        [class.closed]="!toggledOn"
        [class.opened]="toggledOn"
        (click)="onToggle()"
        [style.color]="toggledOn ? (color || DEFAULT_OPEN_COLOR) : DEFAULT_CLOSE_COLOR">
        <mat-icon class="vertical">add_circle_outline</mat-icon>
        <mat-icon class="horizontal"
                  matTooltip="{{toggledOn ? 'Remove Layer' : 'Create Layer'}}">remove_circle</mat-icon>
    </div>
  `,
  styleUrls: ['layer_toggle.css'],
})
export class LayerToggle {
  readonly DEFAULT_OPEN_COLOR = '#ffca28';
  readonly DEFAULT_CLOSE_COLOR = '#718792';

  @Input() toggledOn = false;
  @Input() color?: string;

  /** Thread, event, etc. currently selected via mouseover */
  previewDatum?: Interval;

  onToggle() {
    this.toggledOn = !this.toggledOn;
    this.toggledOnChange.emit(this.toggledOn);
  }

  @Output() toggledOnChange = new EventEmitter();
}
