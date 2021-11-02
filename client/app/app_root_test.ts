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
import {TestBed, waitForAsync} from '@angular/core/testing';
import {MatDialog} from '@angular/material/dialog';
import {Router} from '@angular/router';
import {RouterTestingModule} from '@angular/router/testing';

import {AppRoot} from './app_root';
import {AppRootModule} from './app_root_module';
import {routes} from './app_routing_module';
import {ShortcutId, ShortcutService} from './services/shortcut_service';
import {triggerShortcut} from './services/shortcut_service_test';
import {serializeHashFragment} from './util';

describe('AppRoot', () => {
  beforeEach(waitForAsync(() => {
    TestBed
        .configureTestingModule({
          imports: [AppRootModule, RouterTestingModule.withRoutes(routes)],
          providers: [
            {provide: 'ShortcutService', useClass: ShortcutService},
          ],
        })
        .compileComponents();
  }));

  it('should create component', () => {
    const fixture = TestBed.createComponent(AppRoot);
    const component = fixture.componentInstance;
    fixture.detectChanges();
    expect(component).toBeTruthy();
  });

  it('should register show shortcut handler', () => {
    const fixture = TestBed.createComponent(AppRoot);
    fixture.detectChanges();
    const dialog = TestBed.get(MatDialog) as MatDialog;
    const shortcutService = fixture.debugElement.injector.get(ShortcutService);

    const dialogOpenSpy = spyOn(dialog, 'open');
    const shortcut = shortcutService.getShortcuts()[ShortcutId.SHOW_SHORTCUTS];

    expect(shortcut.isEnabled).toBe(true);
    triggerShortcut(shortcut);
    expect(dialogOpenSpy).toHaveBeenCalledTimes(1);
  });

});
