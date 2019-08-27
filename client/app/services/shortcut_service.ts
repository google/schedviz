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
import {Injectable, OnDestroy} from '@angular/core';

/**
 * A keyboard shortcut.
 */
export interface Shortcut {
  readonly description: string;
  readonly friendlyKeyText: string;
  readonly keyPress: KeyPress;
}

/**
 * A single keypress occurrence, including any modifier keys.
 */
export interface KeyPress {
  readonly key: string;
  readonly metaKey?: boolean;
  readonly ctrlKey?: boolean;
  readonly shiftKey?: boolean;
  readonly altKey?: boolean;
}

/**
 * A collection of IDs corresponding to all available shortcuts.
 */
export enum ShortcutId {
  SHOW_SHORTCUTS,
  COPY_TOOLTIP,
  RESET_VIEWPORT,
  CLEAR_CPU_FILTER
}

/**
 * A handler to respond to the entry of a keyboard shortcut.
 */
export type ShortcutHandler = (event: KeyboardEvent) => void;

/**
 * A callback to deregister a shortcut handler.
 */
export type DeregistrationCallback = () => void;

/**
 * A service which handles the registration and event dispatch of keyboard
 * shortcuts.
 */
@Injectable({providedIn: 'root'})
export class ShortcutService implements OnDestroy {
  private readonly activeShortcuts: Map<ShortcutId, ShortcutHandler>;

  private readonly shortcutDictionary: Record<ShortcutId, Shortcut> = {
    [ShortcutId.SHOW_SHORTCUTS]: {
      description: 'Show this shortcut dialog',
      friendlyKeyText: '?',
      keyPress: {key: '?', shiftKey: true},
    },
    [ShortcutId.COPY_TOOLTIP]: {
      description: 'Copy the current tooltip text to the clipboard',
      friendlyKeyText: 'Shift + c',
      keyPress: {key: 'C', shiftKey: true},
    },
    [ShortcutId.RESET_VIEWPORT]: {
      description: 'Reset zoom level in heatmap viewport',
      friendlyKeyText: 'Shift + a',
      keyPress: {key: 'A', shiftKey: true},
    },
    [ShortcutId.CLEAR_CPU_FILTER]: {
      description: 'Clear the CPU filter',
      friendlyKeyText: 'Shift + x',
      keyPress: {key: 'X', shiftKey: true},
    },
  };

  constructor() {
    this.activeShortcuts = new Map<ShortcutId, ShortcutHandler>();
    window.addEventListener('keydown', this.onKeyDown);
  }

  private readonly onKeyDown = (event: KeyboardEvent) => {
    this.handleKeyDown(event);
  };

  private handleKeyDown(event: KeyboardEvent) {
    if (shouldIgnore(event)) {
      return;
    }

    for (const [shortcutId, action] of this.activeShortcuts) {
      const shortcut = this.shortcutDictionary[shortcutId];
      if (isMatchingKeyPress(event, shortcut.keyPress)) {
        action(event);
        return;
      }
    }
  }

  ngOnDestroy() {
    window.removeEventListener('keydown', this.onKeyDown);
  }

  register(id: ShortcutId, action: ShortcutHandler): DeregistrationCallback {
    this.activeShortcuts.set(id, action);
    return () => this.activeShortcuts.delete(id);
  }

  getShortcuts(): Array<Shortcut&{isEnabled: boolean}> {
    const ids = Object.keys(this.shortcutDictionary) as unknown as ShortcutId[];
    return ids.map((id) => ({
                     ...this.shortcutDictionary[id],
                     isEnabled: this.activeShortcuts.has(Number(id))
                   }));
  }
}

function isMatchingKeyPress(event: KeyboardEvent, keyPress: KeyPress) {
  return event.key === keyPress.key &&
      event.ctrlKey === Boolean(keyPress.ctrlKey) &&
      event.altKey === Boolean(keyPress.altKey) &&
      event.shiftKey === Boolean(keyPress.shiftKey) &&
      event.metaKey === Boolean(keyPress.metaKey);
}

function shouldIgnore(event: KeyboardEvent) {
  const target = event.target;
  if (!(target instanceof Element)) {
    return false;
  }

  // Ignore any shortcuts that occur within text fields
  if (target.getAttribute('contenteditable') === 'true' ||
      target.tagName === 'TEXTAREA' || target.tagName === 'INPUT') {
    return true;
  }

  return false;
}
