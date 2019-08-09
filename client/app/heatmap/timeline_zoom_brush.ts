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
import {ChangeDetectionStrategy, Component, ElementRef, Input, OnInit, ViewChild} from '@angular/core';
import * as d3 from 'd3';
import {BehaviorSubject} from 'rxjs';

import {CollectionParameters, CpuInterval} from '../models';
import {Viewport} from '../util';
import * as Duration from '../util/duration';

const BIN_COUNT = 100;
const BRUSH_HEIGHT = 40;
const AXIS_MARGIN_Y = 40;

/**
 * The timeline zoom brush component displays a static, aggregate view of the
 * heatmap, with an overlay the control the viewport (in X).
 */
@Component({
  selector: '[zoomBrush]',
  template: `
    <svg:g #brushRoot class="context" opacity="0.8">
      <g #xAxis transform="translate(0, 20)"></g>
      <g #brush>
        <rect *ngFor="let bucket of get1DHeatmap(intervals); index as i"
            class="brushCell"
            [attr.fill]="bucket"
            [attr.x]="bucketSizePx * i"
            [attr.width]="bucketSizePx"
            fill-opacity="0.7"
            height="20"
            y="0">
        </rect>
      </g>
    </svg:g>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TimelineZoomBrush implements OnInit {
  @ViewChild('brushRoot', {static: true}) brushRoot!: ElementRef;
  @ViewChild('brush', {static: true}) brush!: ElementRef;
  @ViewChild('xAxis', {static: true}) xAxis!: ElementRef;
  @Input() parameters!: BehaviorSubject<CollectionParameters|undefined>;
  @Input() intervals: CpuInterval[] = [];
  @Input() viewport = new BehaviorSubject<Viewport>(new Viewport());
  left = 0;
  right = 1;
  chartHeightPx = 500;
  chartWidthPx = 500;

  ngOnInit() {
    // Check required inputs
    if (!this.parameters) {
      throw new Error('Missing required CollectionParameters');
    }
    if (!this.viewport) {
      throw new Error('Missing Observable for viewport');
    }
    // Initial render for zoom brush
    this.drawBrush(this.viewport.value);
    // On viewport change, redraw zoombrush
    this.viewport.subscribe((viewport) => {
      // Ignore changes coming from within the brush itself
      this.drawBrush(viewport);
    });
  }

  /**
   * Redraws the zoom brush on inputs change.
   */
  drawBrush(viewport: Viewport) {
    const parameters = this.parameters.value;
    if (!parameters) {
      return;
    }
    this.left = viewport.left;
    this.right = viewport.right;
    this.chartHeightPx = viewport.chartHeightPx;
    this.chartWidthPx = viewport.chartWidthPx;
    const brushRoot = d3.select(this.brushRoot.nativeElement);
    brushRoot.attr(
        'transform', `translate(0, ${this.chartHeightPx + AXIS_MARGIN_Y})`);
    const xAxisDomain = [parameters.startTimeNs, parameters.endTimeNs];
    const zoomedDomainSize = xAxisDomain[1] - xAxisDomain[0];
    const unit = Duration.getMinDurationUnits(zoomedDomainSize);
    const height = BRUSH_HEIGHT;
    // Draw x-axis according to domain size.
    const x =
        d3.scaleLinear().domain(xAxisDomain).range([0, this.chartWidthPx]);
    const xAxisFunc = d3.axisBottom(x).tickFormat((d) => {
      return Duration.getHumanReadableDurationFromNs(
          d.valueOf(), unit.units[0]);
    });
    const xAxis = d3.select(this.xAxis.nativeElement);
    xAxis.call(xAxisFunc);
    xAxis.selectAll('text').attr('color', '#eee');
    xAxis.selectAll('.tick line').attr('stroke', '#eee');
    xAxis.selectAll('.domain').attr('stroke', '#eee');
    const brushed = () => {
      if (d3.event.sourceEvent && d3.event.sourceEvent.type === 'zoom') {
        return;  // ignore zoom events, as those are handled by the heatmap.
      }
      const s = d3.event.selection || x.range();
      this.left = s[0] / this.chartWidthPx;
      this.right = s[1] / this.chartWidthPx;
      if (viewport.left !== this.left || viewport.right !== this.right) {
        viewport.left = this.left;
        viewport.right = this.right;
        this.viewport.next(viewport);
      }
    };
    const brush = d3.brushX()
                      .extent([[0, 0], [this.chartWidthPx, height]])
                      .on('brush end', brushed);
    const brushElement = d3.select(this.brush.nativeElement);
    brushElement.attr('class', 'brush').call(brush);
    brushElement.call(
        brush.move,
        [this.left * this.chartWidthPx, this.right * this.chartWidthPx]);
    brushElement.select('.selection')
        .attr('fill', '#ffc107')
        .attr('stroke', '#ffc107');
  }

  /**
   * Collapses the heatmap interval set into a fixed-bucket 1D heatmap.
   * @return the heatmap colors to render.
   */
  get1DHeatmap(intervals: CpuInterval[]) {
    const collectionStart = this.collectionStart;
    const bucketSize = this.bucketSizeNs;
    const buckets: number[] = [];
    for (let i = 0; i < BIN_COUNT; i++) {
      buckets[i] = intervals.reduce((acc, interval) => {
        const bucketStart = bucketSize * i + collectionStart;
        const bucketEnd = bucketStart + bucketSize;
        if (interval.startTimeNs > bucketEnd ||
            interval.endTimeNs < bucketStart) {
          return acc;
        }
        const left =
            (Math.max(bucketStart, interval.startTimeNs) - bucketStart) /
            bucketSize;
        const right =
            (bucketEnd - Math.min(bucketEnd, interval.endTimeNs)) / bucketSize;
        return acc + (1 - left - right) * interval.waitingPidCount;
      }, 0);
    }
    const max = buckets.reduce((a, b) => a > b ? a : b);
    return buckets.map((bucket) => {
      const intensity = Math.round(255 * bucket / max);
      return `rgb(${intensity},${intensity},${intensity})`;
    });
  }

  get collectionStart() {
    const parameters = this.parameters.value;
    if (!parameters) {
      return 0;
    }
    return parameters.startTimeNs;
  }

  get domainSize() {
    const parameters = this.parameters.value;
    if (!parameters) {
      return 0;
    }
    return parameters.endTimeNs - parameters.startTimeNs;
  }

  get bucketSizePx() {
    return this.bucketSizeNs / this.domainSize * this.chartWidthPx;
  }

  get bucketSizeNs() {
    return this.domainSize / BIN_COUNT;
  }
}
