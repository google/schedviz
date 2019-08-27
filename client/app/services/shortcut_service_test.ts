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
import {Shortcut, ShortcutId, ShortcutService} from './shortcut_service';

function triggerKeyboardEvent(args: KeyboardEventInit, target: string) {
  const event = new KeyboardEvent('keydown', args);
  const element = window.document.createElement(target);

  // Workaround for target being a readonly property
  Object.defineProperty(event, 'target', {
    get() {
      return element;
    }
  });

  window.dispatchEvent(event);
}

/**
 * Simulates a keypress event for the given shortcut.
 *
 * @param shortcut is the keyboard shortcut to trigger
 */
export function triggerShortcut(shortcut: Shortcut) {
  const keyPress = shortcut.keyPress;
  triggerKeyboardEvent(keyPress, 'div');
}

describe('ShortcutService', () => {
  it('should call shortcut handler', () => {
    const shortcutService = new ShortcutService();
    const shortcuts = shortcutService.getShortcuts();
    const shortcutIds = Object.keys(ShortcutId)
                            .map(shortcutId => Number(shortcutId))
                            .filter(shortcutId => !isNaN(shortcutId));

    // Trigger every shortcut
    for (const shortcutId of shortcutIds) {
      const shortcutHandler = jasmine.createSpy('shortcutHandler');
      shortcutService.register(shortcutId, shortcutHandler);

      const shortcutFriendlyName = ShortcutId[shortcutId];
      expect(shortcutHandler)
          .withContext(shortcutFriendlyName)
          .toHaveBeenCalledTimes(0);
      triggerShortcut(shortcuts[shortcutId]);
      expect(shortcutHandler)
          .withContext(shortcutFriendlyName)
          .toHaveBeenCalledTimes(1);
    }
  });

  it('should not fire shortcut on input elements', () => {
    const shortcutService = new ShortcutService();
    const shortcuts = shortcutService.getShortcuts();
    const shortcutId = ShortcutId.CLEAR_CPU_FILTER;
    const shortcutHandler = jasmine.createSpy('shortcutHandler');
    shortcutService.register(shortcutId, shortcutHandler);

    const keyPress = shortcuts[shortcutId].keyPress;
    triggerKeyboardEvent(keyPress, 'input');
    expect(shortcutHandler).toHaveBeenCalledTimes(0);
  });

  it('should allow deregistration', () => {
    const shortcutService = new ShortcutService();
    const shortcuts = shortcutService.getShortcuts();
    const shortcutId = ShortcutId.CLEAR_CPU_FILTER;
    const shortcutHandler = jasmine.createSpy('shortcutHandler');
    const deregister = shortcutService.register(shortcutId, shortcutHandler);
    const shortcut = shortcutService.getShortcuts()[shortcutId];
    expect(shortcut.isEnabled).toBe(true);

    const keyPress = shortcuts[shortcutId].keyPress;
    triggerKeyboardEvent(keyPress, 'div');
    expect(shortcutHandler).toHaveBeenCalledTimes(1);
    deregister();
    triggerKeyboardEvent(keyPress, 'div');
    expect(shortcutHandler).toHaveBeenCalledTimes(1);

    const shortcutAfterDereg = shortcutService.getShortcuts()[shortcutId];
    expect(shortcutAfterDereg.isEnabled).toBe(false);
  });
});
