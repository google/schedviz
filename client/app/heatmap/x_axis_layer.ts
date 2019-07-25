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
import {Component, ElementRef, Input, OnInit, ViewChild} from '@angular/core';
import * as d3 from 'd3';
import {BehaviorSubject} from 'rxjs';

import {CollectionParameters} from '../models';
import {Viewport} from '../util';
import * as Duration from '../util/duration';

/**
 * The xAxis component displays adaptable time ticks for the heatmap, based on
 * the current viewport.
 */
@Component({
  selector: '[xAxis]',
  template: `
    <svg:g #xAxis>
      <rect class="axisBase"
        x="-100"
        y="0"
        height="100"
        fill="#000"
      ></rect>
    </svg:g>
  `,
})
export class XAxisLayer implements OnInit {
  constructor() {}
  @ViewChild('xAxis', {static: true}) xAxis!: ElementRef;
  @Input() parameters!: BehaviorSubject<CollectionParameters|undefined>;
  @Input() viewport = new BehaviorSubject<Viewport>(new Viewport());

  ngOnInit() {
    // Check required inputs
    if (!this.parameters) {
      throw new Error('Missing required CollectionParameters');
    }
    if (!this.viewport) {
      throw new Error('Missing Observable for viewport');
    }
    // Redraw axis on viewport change.
    this.viewport.subscribe((viewport) => {
      this.drawAxis(viewport);
    });
  }

  /**
   * Draws the x-axis ticks based on the current domain size.
   */
  drawAxis(viewport: Viewport) {
    const parameters = this.parameters.value;
    if (!parameters) {
      return;
    }
    const domainSize = parameters.endTimeNs - parameters.startTimeNs;
    const xAxisDomain =
        [viewport.left * domainSize, viewport.right * domainSize];
    const zoomedDomainSize = xAxisDomain[1] - xAxisDomain[0];
    const unit = Duration.getMinDurationUnits(zoomedDomainSize);
    const chartWidthPx = viewport.chartWidthPx;
    const chartHeightPx = viewport.chartHeightPx;
    // Draw x-axis according to domain size.
    const x = d3.scaleLinear().domain(xAxisDomain).range([0, chartWidthPx]);
    const xAxisFunc = d3.axisBottom(x).tickFormat((d) => {
      return Duration.getHumanReadableDurationFromNs(
          d.valueOf(), unit.units[0]);
    });
    // Style ticks.
    const xAxis = d3.select(this.xAxis.nativeElement);
    xAxis.select('.axisBase').attr('width', chartWidthPx + 200);
    xAxis.attr('transform', `translate(0, ${chartHeightPx})`);
    xAxis.call(xAxisFunc);
    xAxis.selectAll('text')
        .style('text-anchor', 'end')
        .attr('dx', '-.8em')
        .attr('dy', '.15em')
        .attr('transform', 'rotate(-45)')
        .attr('color', '#eee');
    xAxis.selectAll('.tick line')
        .attr('y1', -1 * chartHeightPx)
        .attr('stroke', '#eee');
    xAxis.selectAll('.domain').attr('stroke', '#eee');
  }
}
