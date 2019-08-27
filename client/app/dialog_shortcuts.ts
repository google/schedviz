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
import {Component} from '@angular/core';
import {MatDialogRef} from '@angular/material/dialog';

import {Shortcut, ShortcutService} from './services/shortcut_service';

/**
 * Dialog containing a list of all available shortcuts.
 */
@Component({
  selector: 'dialog-shortcuts',
  template: `
    <h1 mat-dialog-title>
      Keyboard shortcuts
    </h1>
    <mat-divider></mat-divider>
    <div mat-dialog-content>
      <mat-table [dataSource]="shortcuts">
        <ng-container matColumnDef="description">
          <mat-cell *matCellDef="let element"> {{element.description}} </mat-cell>
        </ng-container>
        <ng-container matColumnDef="keys">
          <mat-cell *matCellDef="let element"
                    class="shortcut-key"> {{element.friendlyKeyText}} </mat-cell>
        </ng-container>
        <mat-row *matRowDef="let row; columns: displayedColumns"
                class="detail-row"></mat-row>
      </mat-table>
    </div>
    <div mat-dialog-actions
        align="end">
      <button type="button"
              mat-stroked-button
              mat-dialog-close>
        Close
      </button>
    </div>
  `,
  styles: [`
    .shortcut-key {
      justify-content: flex-end;
      font-family: monospace;
    }
  `]
})
export class DialogShortcuts {
  displayedColumns = ['description', 'keys'];
  shortcuts: Shortcut[];
  constructor(
      public dialogRef: MatDialogRef<DialogShortcuts>,
      shortcutService: ShortcutService) {
    this.shortcuts = shortcutService.getShortcuts();
  }
}
