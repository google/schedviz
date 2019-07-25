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
import {HttpClientModule} from '@angular/common/http';
import {NgModule, NO_ERRORS_SCHEMA} from '@angular/core';
import {MatButtonModule} from '@angular/material/button';
import {MatDialogModule} from '@angular/material/dialog';
import {MatIconModule} from '@angular/material/icon';
import {MatProgressBarModule} from '@angular/material/progress-bar';
import {MatSnackBarModule} from '@angular/material/snack-bar';
import {MatTooltipModule} from '@angular/material/tooltip';
import {BrowserModule} from '@angular/platform-browser';

import {UtilModule} from '../util';

import {CpuAxesModule} from './cpu_axes';
import {DialogMetricsHelp, Heatmap, IntervalsLayer, MetricsOverlay, PreviewLayer, TimelineZoomBrush, XAxisLayer} from './index';


@NgModule({
  declarations: [
    Heatmap,
    PreviewLayer,
    XAxisLayer,
    IntervalsLayer,
    TimelineZoomBrush,
    MetricsOverlay,
    DialogMetricsHelp,
  ],
  imports: [
    BrowserModule,
    CpuAxesModule,
    HttpClientModule,
    MatButtonModule,
    MatDialogModule,
    MatIconModule,
    MatProgressBarModule,
    MatSnackBarModule,
    MatTooltipModule,
    UtilModule,
  ],
  exports: [
    Heatmap,
  ],
  schemas: [NO_ERRORS_SCHEMA],
  entryComponents: [DialogMetricsHelp]
})
export class HeatmapModule {
}
