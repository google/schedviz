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
import {CpuInterval} from './cpu_intervals';
import {Layer} from './layer';

// White: Used for intervals with maximum number of waiting threads
const INTERVAL_MAX_INTENSITY = 255;
// Darkest gray: used for intervals with 0 waiting threads
const INTERVAL_BASE_INTENSITY = 100;
// Range for scaling
const INTERVAL_INTENSITY_RANGE =
    INTERVAL_MAX_INTENSITY - INTERVAL_BASE_INTENSITY;

/**
 * The SchedViz 'base' layer that shows aggregate thread states at the CPU level
 */
export class CpuRunningLayer extends Layer {
  private intervalsInternal: CpuInterval[] = [];

  constructor() {
    super('CPU Intervals', 'CPU');
  }

  set intervals(cpuIntervals: CpuInterval[]) {
    if (!cpuIntervals.length) {
      this.intervalsInternal = cpuIntervals;
      return;
    }
    // Compute the min and max waiting pid counts, set interval intensity
    // based on reweighted wait metric on a logarithmic scale
    const minWaitMetric = cpuIntervals.reduce((min, interval) => {
      const waitingPidCount = interval.waiting.length;
      return waitingPidCount !== 0 && waitingPidCount < min ? waitingPidCount :
                                                              min;
    }, Infinity);
    const maxWaitMetric = cpuIntervals.reduce(
        (max, interval) => Math.max(max, interval.waiting.length),
        cpuIntervals[0].waiting.length);
    for (const interval of cpuIntervals) {
      if (minWaitMetric !== maxWaitMetric) {
        // Use a log scaling for the waiting pid count relative to the
        // maximum, with a log base of minRatio = minWaitMetric / maxWaitMetric.
        // At max count, Math.log(waitingPidCount / max) = Math.log(1.0) = 0,
        // approaching 1.0 as the interval's waiting pid count approaches min.
        // Interval's intensity is offset from white based on the interval's
        // relative (to max) waiting pid count.
        const waitRatio = interval.waiting.length / maxWaitMetric;
        const logOffset = waitRatio ?
            Math.log(waitRatio) / Math.log(minWaitMetric / maxWaitMetric) :
            1;
        const intensityDelta = INTERVAL_INTENSITY_RANGE * logOffset;
        interval.renderWeight =
            Math.ceil(INTERVAL_MAX_INTENSITY - intensityDelta);
      } else {
        // If there is no range of wait times, assign all intervals a default
        // color
        interval.renderWeight = INTERVAL_BASE_INTENSITY;
      }
    }
    this.intervalsInternal = cpuIntervals;
  }

  get intervals() {
    return this.intervalsInternal;
  }

  /**
   * Grayscale interval intensity indicates waiting interval count, so users can
   * easily identify 'hot spots'.
   */
  getIntervalColor(interval: CpuInterval) {
    const intensity = interval.renderWeight;
    return `rgb(${intensity},${intensity},${intensity})`;
  }
}

/**
 * Secondary SchedViz 'base' layer: Renders CPU Intervals that are waiting while
 * their CPU is idle.
 */
export class CpuIdleWaitLayer extends Layer {
  constructor() {
    super('Waiting While Idle CPU Intervals', 'CPU', [], '#f93e3e', [], false);
  }
}

/**
 * Secondary SchedViz 'base' layer: Renders height of CPU wait queues over time.
 */
export class CpuWaitQueueLayer extends Layer {
  constructor() {
    super('CPU Wait Queues', 'CPU', [], 'rgb(93, 242, 214)');
  }
}
