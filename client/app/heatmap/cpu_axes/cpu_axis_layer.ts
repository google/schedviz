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
import {Component, ElementRef, Input, OnInit, ViewChild, ViewEncapsulation} from '@angular/core';
import * as d3 from 'd3';
import {BehaviorSubject} from 'rxjs';

import {CpuLabel, SystemTopology, Viewport} from '../../util';

// TODO(sainsley): Scale based on topology size
const FONT_SIZE = 8;

/**
 * The cpuAxis component displays the current list of visible CPUs and a visual
 * indicator of the current vertical zoom level and the current list of filtered
 * CPUs.
 */
@Component({
  selector: '[cpuAxis]',
  template: `
    <svg:g #cpuAxis>
      <rect class="yAxisBase"
        x="-50"
        y="-50"
        width="50"
        fill="#000"
      ></rect>
      <rect
        class="viewportMarker"
        x="-50"
        width="8"
        fill="#ffca28"
        opacity=0.5
      ></rect>
    </svg:g>
  `,
  styleUrls: ['cpu_axes.css'],
  encapsulation: ViewEncapsulation.None,
})
export class CpuAxisLayer implements OnInit {
  @ViewChild('cpuAxis', {static: true}) cpuAxis!: ElementRef;
  @Input() topology!: SystemTopology;
  @Input() cpuFilter!: BehaviorSubject<string>;
  @Input() viewport!: BehaviorSubject<Viewport>;

  ngOnInit() {
    // Check required inputs
    if (!this.topology) {
      throw new Error('Missing required SystemTopology');
    }
    if (!this.viewport) {
      throw new Error('Missing Observable for viewport');
    }
    if (!this.cpuFilter) {
      throw new Error('Missing Observable for CPU filter');
    }
    // Listen for changes to CPU filter from input box
    this.cpuFilter.subscribe((filter) => {
      this.drawLabels(this.viewport.value);
    });

    // Rescale labels on viewport size change
    this.viewport.subscribe((viewport) => {
      this.scaleLabels(viewport);
    });
    // Initial draw
    this.drawLabels(this.viewport.value);
  }

  /**
   * Draws the CPU visual markers and text labels.
   */
  drawLabels(viewport: Viewport) {
    const view = d3.select(this.cpuAxis.nativeElement);
    view.selectAll('.cpuLabel').remove();
    view.selectAll('.cpuMarker').remove();
    const dataGroup = view.selectAll('.cpuGroup').data(this.visibleRows);
    // Add markers
    dataGroup.enter()
        .append('rect')
        .classed('cpuMarker', true)
        .attr('x', -50)
        .attr('width', 8)
        .style('fill', '#ffca28')
        .style('opacity', 0.5);
    dataGroup.exit().remove();
    // Add labels
    dataGroup.enter()
        .append('text')
        .classed('cpuLabel', true)
        .text(cpu => cpu.label)
        .attr('x', -35)
        .on('click', cpu => this.toggleCpuFilter(`${cpu.cpuIndex}`));
    dataGroup.exit().remove();
    this.scaleLabels(viewport);
  }

  /**
   * Queries existing labels and updates positions based on viewport size.
   */
  scaleLabels(viewport: Viewport) {
    const view = d3.select(this.cpuAxis.nativeElement);
    view.select('.yAxisBase').attr('height', viewport.chartHeightPx + 50);
    view.select('.viewportMarker')
        .attr('y', viewport.chartHeightPx * viewport.top)
        .attr('height', viewport.chartHeightPx * viewport.height);
    view.selectAll('.cpuMarker')
        .attr('y', cpu => this.unscaledCpuYPx(cpu as CpuLabel, viewport))
        .attr('height', this.unscaledRowHeightPx(viewport));
    view.selectAll('.cpuLabel')
        .attr('y', cpu => this.scaledCpuYPx(cpu as CpuLabel, viewport));
  }


  /**
   * @return the row height, in pixels, with *no* zoom/filtering
   * applied.
   */
  unscaledRowHeightPx(viewport: Viewport) {
    const rowHeight = this.topology.cpuCount ? 1.0 / this.topology.cpuCount : 0;
    return rowHeight * viewport.chartHeightPx;
  }


  /**
   * @return the row height, in pixels, with zoom/filtering applied.
   */
  scaledRowHeightPx(viewport: Viewport) {
    const visibleRows = this.visibleRows;
    const rowHeight = visibleRows.length ? 1.0 / visibleRows.length : 0;
    return rowHeight * viewport.scaleY * viewport.chartHeightPx;
  }

  /**
   * @return the given CPU's topological position with *no*
   * zoom/filtering applied, in pixels.
   */
  unscaledCpuYPx(cpuLabel: CpuLabel, viewport: Viewport) {
    const cpuId = cpuLabel.cpuIndex;
    const rowHeight = this.unscaledRowHeightPx(viewport);
    const rowY = this.topology.getRow(cpuId);
    return rowY * rowHeight;
  }

  /**
   * @return the given CPU's topological position with zoom/filtering
   * applied, in pixels.
   */
  scaledCpuYPx(cpuLabel: CpuLabel, viewport: Viewport) {
    const visibleRows = this.visibleRows;
    const rowHeight = this.scaledRowHeightPx(viewport);
    const rowY = visibleRows.indexOf(cpuLabel);
    return rowY * rowHeight + viewport.translateYPx + rowHeight / 4 +
        FONT_SIZE / 2;
  }

  /**
   * Filters down to the given CPU on click.
   */
  toggleCpuFilter(newFilter: string) {
    const cpuFilter = this.cpuFilter.value === newFilter ? '' : newFilter;
    this.cpuFilter.next(cpuFilter);
  }

  /**
   * @return the filtered in set of CPU labels, in current sorted
   *     order.
   */
  get visibleRows() {
    return this.topology.getSortedFilteredCpuLabels(this.cpuFilter.value);
  }
}
