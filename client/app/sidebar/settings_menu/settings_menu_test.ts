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
import {CdkDragDrop} from '@angular/cdk/drag-drop';
import {ComponentFixture, fakeAsync, TestBed, tick} from '@angular/core/testing';
import {FormsModule} from '@angular/forms';
import {MatFormFieldModule} from '@angular/material/form-field';
import {MatIconModule} from '@angular/material/icon';
import {MatInputModule} from '@angular/material/input';
import {BrowserDynamicTestingModule, platformBrowserDynamicTesting} from '@angular/platform-browser-dynamic/testing';
import {BehaviorSubject} from 'rxjs';

import {CollectionParameters, Layer} from '../../models';
import {Viewport} from '../../util';

import {SettingsMenu} from './settings_menu';
import {SettingsMenuModule} from './settings_menu_module';

try {
  TestBed.initTestEnvironment(
      BrowserDynamicTestingModule, platformBrowserDynamicTesting());
} catch {
  // Ignore exceptions when calling it multiple times.
}

const RED_HEX = '#ff0000';
const GREEN_HEX = '#00ff00';

// Delay time which will guarantee flush of time input
const TIME_INPUT_DEBOUNCE_MS = 1000;

function setupSettingsMenu(component: SettingsMenu) {
  component.parameters = new BehaviorSubject<CollectionParameters|undefined>(
      new CollectionParameters('collection_params', [], 0, 2e9));
  component.viewport = new BehaviorSubject<Viewport>(new Viewport());
  component.layers = new BehaviorSubject<Array<BehaviorSubject<Layer>>>([]);
  component.showMigrations = new BehaviorSubject<boolean>(true);
  component.showSleeping = new BehaviorSubject<boolean>(true);
  component.maxIntervalCount = new BehaviorSubject<number>(0);
}

function mockLayers(): Array<BehaviorSubject<Layer>> {
  const layers: Array<BehaviorSubject<Layer>> = [];
  const layerCount = 10;
  for (let i = 0; i < layerCount; i++) {
    // Use arbitrary colors spread evenly across the color space
    const maxHexColor = 0xFFFFFF;
    const colorVal = Math.floor(i * (maxHexColor - layerCount) / layerCount);
    const color = `#${colorVal.toString(16)}`;
    const layer = new Layer(`Mock Layer ${i}`, '', undefined, color);
    layers.push(new BehaviorSubject(layer));
  }

  return layers;
}

async function createSettingsMenuWithMockData(
    layers: Array<BehaviorSubject<Layer>>):
    Promise<ComponentFixture<SettingsMenu>> {
  const fixture = TestBed.createComponent(SettingsMenu);
  const component = fixture.componentInstance;
  setupSettingsMenu(component);
  component.layers.next(layers);
  await fixture.whenStable();
  fixture.detectChanges();

  return fixture;
}

function getLayerColorInput(
    root: Element, layerIndex: number): HTMLInputElement {
  const layerList = root.querySelector('.layer-list');
  expect(layerList).toBeTruthy();

  const colorInputs = layerList!.querySelectorAll('.color-input');
  expect(colorInputs.length).toBeGreaterThan(layerIndex);

  const colorInput = colorInputs[layerIndex];
  expect(colorInput instanceof HTMLInputElement).toBe(true);

  return colorInput as HTMLInputElement;
}

function isLayerVisible(root: Element, layerIndex: number): boolean {
  const layerList = root.querySelector('.layer-list');
  expect(layerList).toBeTruthy();

  const layerContainers = layerList!.querySelectorAll('.layer-box');
  expect(layerContainers.length).toBeGreaterThan(layerIndex);

  const layerContainer = layerContainers[layerIndex];
  return !layerContainer.classList.contains('hidden');
}

describe('SettingsMenu', () => {
  beforeEach(async () => {
    document.body.style.width = '500px';
    document.body.style.height = '500px';
    await TestBed
        .configureTestingModule({
          imports: [
            FormsModule, MatFormFieldModule, MatInputModule, MatIconModule,
            SettingsMenuModule
          ],
        })
        .compileComponents();
  });

  it('should create', async () => {
    const layers = mockLayers();
    const fixture = await createSettingsMenuWithMockData(layers);
    expect(fixture.componentInstance).toBeTruthy();
  });

  it('should respond to layer change', async () => {
    // Set up menu with initial layer data
    const expectedInitialColor = RED_HEX;
    const expectedInitialVisibility = true;
    const initialLayer = new Layer(
        'Layer to Change', '', undefined, expectedInitialColor, undefined,
        expectedInitialVisibility);
    const layerSubject = new BehaviorSubject<Layer>(initialLayer);
    const layers = mockLayers();
    const layerIndex = 4;
    layers.splice(layerIndex, 0, layerSubject);
    const fixture = await createSettingsMenuWithMockData(layers);

    // Verify initial layer data
    const actualInitialColor =
        getLayerColorInput(fixture.nativeElement, layerIndex).value;
    expect(actualInitialColor).toEqual(expectedInitialColor);
    const actualInitialVisibility =
        isLayerVisible(fixture.nativeElement, layerIndex);
    expect(actualInitialVisibility).toBe(expectedInitialVisibility);

    // Modify layer
    const expectedNewVisibility = false;
    const expectedNewColor = GREEN_HEX;
    const newLayer = new Layer(
        initialLayer.name, '', undefined, expectedNewColor, undefined,
        expectedNewVisibility);
    layerSubject.next(newLayer);
    await fixture.whenStable();
    fixture.detectChanges();

    // Verify layer was modified
    const actualNewColor =
        getLayerColorInput(fixture.nativeElement, layerIndex).value;
    expect(actualNewColor).toEqual(expectedNewColor);
    const actualNewVisibility =
        isLayerVisible(fixture.nativeElement, layerIndex);
    expect(actualNewVisibility).toBe(expectedNewVisibility);
  });

  it('should allow drag drop of layers', async () => {
    // Set up menu
    const layers = mockLayers();
    const fixture = await createSettingsMenuWithMockData(layers);

    // Simulate drag-drop of layer into a new index
    const initialIndex = 5;
    const newIndex = 2;
    const initialLayer = layers[initialIndex];
    const dropEvent = {previousIndex: initialIndex, currentIndex: newIndex} as
        CdkDragDrop<string[]>;
    fixture.componentInstance.drop(dropEvent);
    fixture.detectChanges();

    // Verify that the layer was moved
    const layerSpy = jasmine.createSpy('layerSpy');
    fixture.componentInstance.layers.subscribe(layerSpy);
    expect(layerSpy.calls.mostRecent().args[0][newIndex]).toBe(initialLayer);
  });

  it('should propagate layer changes', async () => {
    // Set up menu with initial layer data
    const initialColor = RED_HEX;
    const initialVisibility = true;
    const initialLayer = new Layer(
        'Layer to Change', '', undefined, initialColor, undefined,
        initialVisibility);
    const layerSubject = new BehaviorSubject<Layer>(initialLayer);
    const layers = mockLayers();
    const layerIndex = 2;
    layers.splice(layerIndex, 0, layerSubject);
    const fixture = await createSettingsMenuWithMockData(layers);
    const layerSubjectSpy = jasmine.createSpy('layerSubjectSpy');
    layerSubject.subscribe(layerSubjectSpy);
    expect(layerSubjectSpy).toHaveBeenCalledTimes(1);

    // Change the layer color
    const colorInput = getLayerColorInput(fixture.nativeElement, layerIndex);
    const expectedNewColor = GREEN_HEX;
    colorInput.value = expectedNewColor;
    colorInput.dispatchEvent(new Event('input'));
    await fixture.whenStable();
    fixture.detectChanges();

    // Verify that the layer was changed
    expect(layerSubjectSpy).toHaveBeenCalledTimes(2);
    const newLayer = layerSubjectSpy.calls.mostRecent().args[0] as Layer;
    expect(newLayer.color).toEqual(expectedNewColor);
  });

  it('should update viewport on valid time input change', fakeAsync(() => {
       const fixture = TestBed.createComponent(SettingsMenu);
       const component = fixture.componentInstance;
       setupSettingsMenu(component);
       fixture.detectChanges();
       const inputs = fixture.nativeElement.querySelectorAll('.viewport-input');
       inputs[0].value = '50 ms';
       inputs[0].dispatchEvent(new Event('input'));
       tick(TIME_INPUT_DEBOUNCE_MS);
       expect(component.viewport.value.left).toBe(0.025);
       expect(component.viewport.value.right).toBe(1.0);
     }));

  it('should not update viewport on invalid time input change',
     fakeAsync(() => {
       const fixture = TestBed.createComponent(SettingsMenu);
       const component = fixture.componentInstance;
       setupSettingsMenu(component);
       fixture.detectChanges();
       const inputs = fixture.nativeElement.querySelectorAll('.viewport-input');
       // Don't update if invalid
       inputs[0].value = '50 ms';
       inputs[0].dispatchEvent(new Event('input'));
       inputs[1].value = '5 ms';
       inputs[1].dispatchEvent(new Event('input'));
       tick(TIME_INPUT_DEBOUNCE_MS);
       expect(component.viewport.value.left).toBe(0.025);
       expect(component.viewport.value.right).toBe(1.0);
       // Update when valid
       inputs[1].value = '55 ms';
       inputs[1].dispatchEvent(new Event('input'));
       tick(TIME_INPUT_DEBOUNCE_MS);
       expect(component.viewport.value.left).toBe(0.025);
       expect(component.viewport.value.right).toBe(0.0275);
     }));

  it('should clip out of range time inputs', fakeAsync(() => {
       const fixture = TestBed.createComponent(SettingsMenu);
       const component = fixture.componentInstance;
       setupSettingsMenu(component);
       fixture.detectChanges();
       const inputs = fixture.nativeElement.querySelectorAll('.viewport-input');
       // Correct bad inputs
       inputs[0].value = '50 ms';
       inputs[0].dispatchEvent(new Event('input'));
       inputs[1].value = '3000 ms';
       inputs[1].dispatchEvent(new Event('input'));
       tick(TIME_INPUT_DEBOUNCE_MS);
       expect(component.viewport.value.left).toBe(0.025);
       expect(component.viewport.value.right).toBe(1.0);
     }));
});
