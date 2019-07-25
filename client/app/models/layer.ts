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
import * as d3 from 'd3';

import {Interval} from './interval';

/**
 * Client-side collection interval representation, for rendering.
 */
export class Layer {
  // Flag used to indicate when a Layer is new and has no initial render data
  initialized = false;

  constructor(
      public name: string, public dataType: string, public ids: number[] = [],
      public color = '#ffffff', public intervals: Interval[] = [],
      public visible = true, public interpolate = ids.length > 1,
      public borderRadius = 30, public drawEdges = true,
      public parent ?: string) {}

  /**
   * @return the render color to use for the given layer datum.
   */
  getIntervalColor(interval: Interval) {
    if (this.interpolate) {
      const idx = this.ids.indexOf(interval.id);
      const brighter = d3.hsl(this.color).brighter(1);
      const darker = d3.hsl(this.color).darker(2);
      const color =
          d3.interpolateHsl(brighter, darker)(idx / (this.ids.length - 1));
      return color;
    }
    return this.color;
  }
}
