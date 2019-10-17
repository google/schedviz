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
/**
 * A parametric viewport describing the current chart view.
 */
export class Viewport {
  left = 0.0;
  top = 0.0;
  right = 1.0;
  bottom = 1.0;
  chartWidthPx = 500;
  chartHeightPx = 500;

  constructor(viewport?: Viewport) {
    if (viewport) {
      Object.assign(this, viewport);
    }
  }

  /**
   * @return the zoom scale in X
   */
  get scaleX() {
    return 1.0 / this.width;
  }

  /**
   * @return the zoom scale in Y
   */
  get scaleY() {
    return 1.0 / this.height;
  }

  /**
   * @return the X offset, in pixels
   */
  get translateXPx() {
    return -1 * this.scaleX * this.left * this.chartWidthPx;
  }

  /**
   * @return the Y offset, in pixels
   */
  get translateYPx() {
    return -1 * this.scaleY * this.top * this.chartHeightPx;
  }

  get width() {
    return this.right - this.left;
  }

  get height() {
    return this.bottom - this.top;
  }

  /**
   * Get the viewport left boundary in nanoseconds
   */
  getStartTime(startTimeNs: number, endTimeNs: number) {
    const startTime = startTimeNs;
    const domainDuration = endTimeNs - startTime;
    const xMin = Math.floor(this.left * domainDuration);
    return xMin + startTime;
  }

  /**
   * Get the viewport right boundary in nanoseconds
   */
  getEndTime(startTimeNs: number, endTimeNs: number) {
    const startTime = startTimeNs;
    const domainDuration = endTimeNs - startTime;
    const xMax = Math.floor(this.right * domainDuration);
    return xMax + startTime;
  }

  resetY() {
    this.top = 0;
    this.bottom = 1;
  }

  /**
   * TEMPORARY: Updates the viewports internal svg dimension references
   * TODO(sainsley): Refactor out
   * @param svgWidth
   * @param svgHeight
   */
  updateSize(svgWidth: number, svgHeight: number) {
    this.chartWidthPx = svgWidth;
    this.chartHeightPx = svgHeight;
    this.updateZoom(1, 1, 0, 0);
  }

  /**
   * Updates the viewport parametric size and offsets on zoom change
   * @param deltaKX the change (ratio) in X zoom scale
   * @param deltaKY the change (ratio) in Y zoom scale
   * @param deltaXPx the change (difference) in X position, in pixels
   * @param deltaYPx the change (difference) in Y position, in pixels
   */
  updateZoom(
      deltaKX: number, deltaKY: number, deltaXPx: number, deltaYPx: number) {
    // Update X
    let left = this.left;
    let width = this.width;
    width = Math.min(1.0, width / deltaKX);
    if (this.chartWidthPx) {
      const offsetX = deltaXPx * width / this.chartWidthPx;
      left = Math.min(1 - width, Math.max(0, left - offsetX));
    }
    // Update Y
    let top = this.top;
    let height = this.height;
    height = Math.min(1.0, height / deltaKY);
    if (this.chartHeightPx) {
      const offsetY = deltaYPx * height / this.chartHeightPx;
      top = Math.min(1 - height, Math.max(0, top - offsetY));
    }
    // Store values
    this.top = top;
    this.right = left + width;
    this.left = left;
    this.bottom = top + height;
  }
}
