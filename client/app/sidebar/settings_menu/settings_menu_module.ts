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

import {DragDropModule} from '@angular/cdk/drag-drop';
import {CommonModule} from '@angular/common';
import {NgModule} from '@angular/core';
import {FormsModule} from '@angular/forms';
import {MatButtonModule} from '@angular/material/button';
import {MAT_HAMMER_OPTIONS} from '@angular/material/core';
import {MatExpansionModule} from '@angular/material/expansion';
import {MatFormFieldModule} from '@angular/material/form-field';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {MatSlideToggleModule} from '@angular/material/slide-toggle';
import {MatSliderModule} from '@angular/material/slider';
import {MatTooltipModule} from '@angular/material/tooltip';
import {BrowserModule} from '@angular/platform-browser';
import {BrowserAnimationsModule} from '@angular/platform-browser/animations';

import {UtilModule} from '../../util';

import {SettingsMenu} from './settings_menu';

@NgModule({
  declarations: [
    SettingsMenu,
  ],
  exports: [
    SettingsMenu,
  ],
  imports: [
    BrowserAnimationsModule,
    BrowserModule,
    CommonModule,
    DragDropModule,
    MatButtonModule,
    MatIconModule,
    MatInputModule,
    MatExpansionModule,
    MatFormFieldModule,
    MatSliderModule,
    MatSlideToggleModule,
    MatTooltipModule,
    FormsModule,
    UtilModule,
  ],
  providers: [
    {
      provide: MAT_HAMMER_OPTIONS,
      // Allow for selection of items underneath tooltips
      useValue: {cssProps: {userSelect: true}},
    },
  ],
})
export class SettingsMenuModule {
}
