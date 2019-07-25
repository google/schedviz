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
import {Injectable} from '@angular/core';
import * as d3 from 'd3';
import {BehaviorSubject} from 'rxjs';

import {Layer} from '../models';

const GOOGLE_COLORS = [
  '#4285f4', '#db4437', '#fbc02d', '#0f9d58', '#ab47bc', '#00acc1', '#ff7043',
  '#9e9d24', '#5c6bc0', '#f06292', '#00796b', '#c2185b', '#ff9800', '#8bc34a'
];

const MATERIAL_COLORS = [
  // Red-Pink
  '#f44336',
  // Pink
  '#e91e63',
  // Purple
  '#9c27b0',
  // Indigo
  '#673ab7',
  // Grey blue
  '#3f51b5',
  // Blue
  '#2196f3',
  // Cyan
  '#03a9f4',
  // Turquoise
  '#00bcd4',
  // Teal
  '#009688',
  // Green
  '#4caf50',
  // Olive
  '#8bc34a',
  // Yellow
  '#cddc39',
  // Golden
  '#ffeb3b',
  // Orange
  '#ffc107',
  // Red-Orange
  '#ff9800',
  // Red
  '#ff5722',
];


/**
 * A service that provides root colors for new layers, attempting to maximize
 * the distance between colors in the heatmap.
 */
@Injectable({providedIn: 'root'})
export class ColorService {
  private lastColorIdx = -1;


  usedColors: {[name: string]: number|string} = {};


  /**
   * @param name The key of the layer to get/generate a color for
   * @return The next available render color, given the current set
   * of layers. If the layer has already been assigned a color, the existing
   * color will be returned.
   */
  getColorFor(
      layer: Layer|string,
      ): string {
    const name = layer instanceof Layer ? layer.name : layer;
    const existingColor = this.usedColors[name];
    if (existingColor) {
      return (typeof existingColor === 'string') ?
          existingColor :
          MATERIAL_COLORS[existingColor];
    }
    const nextColorIdx = (this.lastColorIdx + 1) % MATERIAL_COLORS.length;
    this.usedColors[name] = nextColorIdx;
    const nextColor = MATERIAL_COLORS[nextColorIdx];
    this.lastColorIdx = nextColorIdx;
    return nextColor;
  }


  /**
   * Update the saved color for a layer
   * @param name The key of a layer to set the color for
   * @param color The color to save
   */
  setColorFor(name: string, color: string) {
    this.usedColors[name] = color;
  }

  /**
   * Remove the saved color for a layer
   * @param name The key of the layer to remove the color for
   */
  removeColorFor(name: string) {
    delete this.usedColors[name];
  }
}
