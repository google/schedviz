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
import * as Duration from '../util/duration';

import {CollectionParameters} from './collection';

/**
 * Client-side collection interval representation, for rendering.
 */
export abstract class Interval {
  // TODO(sainsley): Add interval merging
  renderWeight = 1.0;
  mergedIntervalCount = 1;
  parent?: Interval;
  children: Interval[] = [];
  edges: Interval[][] = [];
  selected = false;
  duration: number;
  shouldRender = true;

  constructor(
      public parameters: CollectionParameters, public id: number,
      public cpu = -1, public startTimeNs = parameters.startTimeNs,
      public endTimeNs = startTimeNs, public opacity = 0.9,
      public minWidth = false) {
    this.duration = this.endTimeNs - this.startTimeNs;
  }

  /**
   * Subclass-specific key-value set for tooltip rendering
   */
  abstract get tooltipProps(): {[key: string]: string};

  /**
   * Subclass-specific key indicating the type of trace data represented
   */
  abstract get dataType(): string;

  get label() {
    return `${this.dataType}:${this.id}`;
  }

  /**
   * Flattened set of all leaves in this Interval's hierarchy
   */
  get leaves() {
    let leaves: Interval[] = this.childIntervals.length ? [] : [this];
    for (const child of this.childIntervals) {
      const childLeaves = child.leaves;
      leaves = leaves.concat(...childLeaves);
    }
    return leaves;
  }

  get parentInterval() {
    return this.parent;
  }

  get childIntervals() {
    return this.children;
  }

  addEdge(child1: Interval, child2: Interval) {
    this.edges.push([child1, child2]);
  }

  addChild(child: Interval) {
    child.parent = this;
    this.children.push(child);
    // -1 indicates multiple cpus
    this.cpu = child.cpu === this.cpu ? this.cpu : -1;
  }

  /**
   * @return weighted x pos for rendering
   */
  x() {
    return this.timeToPos(this.startTimeNs);
  }

  /**
   * @return y position for rendering
   */
  y(visibleRows: number[]) {
    if (!visibleRows.length) {
      return 0;
    }
    const rowY = visibleRows.indexOf(this.cpu) / visibleRows.length;
    const rowHeight = this.rowHeight(visibleRows);
    const heightOffset = rowHeight / 2 - this.height(visibleRows);
    return rowY + heightOffset / 2;
  }

  /**
   * @return width for rendering
   */
  width() {
    const width = this.timeToPos(1.0 * this.endTimeNs) - this.x();
    return Math.max(width, 0);
  }

  /**
   * @return height for rendering
   */
  height(visibleRows: number[]) {
    return this.rowHeight(visibleRows) / 2;
  }

  /**
   * @return end-cap x radius, based on zoom aspect ratio
   */
  rx(visibleRows: number[]) {
    return this.height(visibleRows) / 2;
  }

  /**
   * @return end-cap y radius
   */
  ry(visibleRows: number[]) {
    return this.height(visibleRows) / 2;
  }

  /**
   * @return whether or not this interval is visible
   */
  visible(visibleRows: number[]) {
    return this.shouldRender && visibleRows.includes(this.cpu);
  }

  /**
   * @return row height, based on number of intervals in collection (currently
   *  hardcoded in APP_CONFIG)
   */
  rowHeight(visibleRows: number[]) {
    return visibleRows.length ? 1.0 / visibleRows.length : 0;
  }

  /**
   * Converts a timestamp to a pixel value for the current chart width.
   * @param timestamp the timestamp value
   * @return pixel value in x
   */
  timeToPos(timestamp: number) {
    // Get parametric value for time.
    const end = this.parameters.endTimeNs;
    const start = this.parameters.startTimeNs;
    return (timestamp - start) / (end - start);
  }

  formatTime(timestamp: number) {
    return Duration.getHumanReadableDurationFromNs(timestamp);
  }
}
