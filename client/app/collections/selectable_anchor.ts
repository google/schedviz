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
import {Component, Input, OnInit} from '@angular/core';

/**
 * An anchor component which supports text selection without following the link.
 */
@Component({
  selector: 'selectable-anchor',
  template: `
    <a class="selectable-text"
      draggable="false"
      [href]="href"
      (click)="onClick()">
      <ng-content></ng-content>
    </a>
  `,
  styles: [`
    .selectable-text {
      text-decoration: inherit;
      color: inherit;
      user-select: text;
      pointer-events: auto;
    }
  `]
})
export class SelectableAnchor implements OnInit {
  @Input() href!: string;

  ngOnInit() {
    if (!this.href) {
      throw new Error('Must provide href to SelectableAnchor');
    }
  }

  onClick() {
    // Stop the event if there is text selected
    return !this.isTextSelected();
  }

  isTextSelected() {
    const selection = window.getSelection();
    if (!selection) {
      return false;
    }

    return selection.toString().length > 0;
  }
}
