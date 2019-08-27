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
import {MatButtonModule} from '@angular/material/button';
import {MatDialogModule} from '@angular/material/dialog';
import {MatDividerModule} from '@angular/material/divider';
import {MatIconModule} from '@angular/material/icon';
import {MatTableModule} from '@angular/material/table';
import {MatToolbarModule} from '@angular/material/toolbar';

// Prevent unused import error
import {production} from '../environments/environment';

import {AppRoot} from './app_root';
import {AppRoutingModule} from './app_routing_module';
import {DialogShortcuts} from './dialog_shortcuts';
import {HttpCollectionDataService, LocalCollectionDataService} from './services/collection_data_service';
import {HttpMetricsService, LocalMetricsService} from './services/metrics_service';
import {HttpRenderDataService, LocalRenderDataService} from './services/render_data_service';

const collectionDataService =
    production ? HttpCollectionDataService : LocalCollectionDataService;
const renderDataService =
    production ? HttpRenderDataService : LocalRenderDataService;
const metricsService = production ? HttpMetricsService : LocalMetricsService;

@NgModule({
  declarations: [AppRoot, DialogShortcuts],
  exports: [AppRoot],
  imports: [
    AppRoutingModule,
    FormsModule,
    MatButtonModule,
    MatDialogModule,
    MatDividerModule,
    MatIconModule,
    MatTableModule,
    MatToolbarModule,
  ],
  bootstrap: [AppRoot],
  providers: [
    {provide: 'RenderDataService', useClass: renderDataService},
    {provide: 'MetricsService', useClass: metricsService},
    {provide: 'CollectionDataService', useClass: collectionDataService},
  ],
  entryComponents: [DialogShortcuts]
})
export class AppRootModule {
}
