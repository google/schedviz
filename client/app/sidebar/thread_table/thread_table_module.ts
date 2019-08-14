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
import {NgModule} from '@angular/core';
import {FormsModule} from '@angular/forms';
import {MatBadgeModule} from '@angular/material/badge';
import {MatButtonModule} from '@angular/material/button';
import {MatCardModule} from '@angular/material/card';
import {MatCheckboxModule} from '@angular/material/checkbox';
import {MAT_HAMMER_OPTIONS} from '@angular/material/core';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {MatPaginatorModule} from '@angular/material/paginator';
import {MatProgressSpinnerModule} from '@angular/material/progress-spinner';
import {MatSelectModule} from '@angular/material/select';
import {MatSlideToggleModule} from '@angular/material/slide-toggle';
import {MatSortModule} from '@angular/material/sort';
import {MatTableModule} from '@angular/material/table';
import {MatTooltipModule} from '@angular/material/tooltip';
import {BrowserModule} from '@angular/platform-browser';

import {AntagonistTable} from './antagonist_table';
import {EventTable} from './event_table';
import {DurationValidator} from './duration_validator';
import {LayerToggle} from './layer_toggle';
import {SchedEventsTable} from './sched_events_table';
import {SelectableTable} from './selectable_table';
import {ThreadTable} from './thread_table';

@NgModule({
  declarations: [
    SchedEventsTable, ThreadTable, EventTable, AntagonistTable, SelectableTable,
    LayerToggle, DurationValidator
  ],
  exports: [
    SchedEventsTable, ThreadTable, EventTable, AntagonistTable, SelectableTable,
    LayerToggle, DurationValidator
  ],
  imports: [
    BrowserModule,
    FormsModule,
    MatCheckboxModule,
    MatButtonModule,
    MatIconModule,
    MatSelectModule,
    MatSortModule,
    MatTableModule,
    MatInputModule,
    MatPaginatorModule,
    MatCardModule,
    MatProgressSpinnerModule,
    MatBadgeModule,
    MatSlideToggleModule,
    MatTooltipModule,
  ],
  providers: [
    {
      provide: MAT_HAMMER_OPTIONS,
      // Allow for selection of items underneath tooltips
      useValue: {cssProps: {userSelect: true}},
    },
  ],
})
export class ThreadTableModule {
}
