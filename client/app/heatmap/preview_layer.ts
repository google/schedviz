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

import {Interval} from '../models';
import {Viewport} from '../util';

// TODO(sainsley): Explore overlaying thread intervals ontop of CPU intervals
// for mouseover preview as per b/120971777
/**
 * The preview layer highlights relevant CPU rows in the heatmap on threadlist
 * row mouseover.
 */
@Component({
  selector: '[previewLayer]',
  template: `<svg:g #previewLayer></svg:g>`,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PreviewLayer implements OnInit {
  @ViewChild('previewLayer', {static: true}) previewLayer!: ElementRef;
  @Input() preview!: BehaviorSubject<Interval|undefined>;
  @Input() viewport!: BehaviorSubject<Viewport>;
  @Input() sortedFilteredCpus!: number[];
  chartHeightPx = 500;
  chartWidthPx = 500;

  ngOnInit() {
    // Check required inputs
    if (!this.viewport) {
      throw new Error('Missing Observable for viewport');
    }
    if (!this.preview) {
      throw new Error('Missing Observable for preview');
    }
    if (!this.sortedFilteredCpus) {
      throw new Error('Missing required CPU list');
    }
    // Draw preview on change
    // TODO(sainsley): Possibly throttle updates from thread list.
    this.preview.subscribe((preview) => {
      this.drawPreview(preview);
    });
    // Rescale internal render dimensions on viewport change
    this.viewport.subscribe((viewport) => {
      this.chartHeightPx = viewport.scaleY * viewport.chartHeightPx;
      this.chartWidthPx = viewport.scaleX * viewport.chartWidthPx;
    });
  }

  /**
   * Clears current preview and redraws on preview change.
   * TODO(sainsley): Investigate a performance improvement for thread list
   * mouseover by either debouncing calls to preview.next() or redesigning
   * this feature such that we do not do a full redraw on hover.
   */
  drawPreview(preview?: Interval) {
    const view = d3.select(this.previewLayer.nativeElement);
    view.selectAll('line').remove();
    if (!preview) {
      return;
    }
    const leaves = preview.leaves;
    const cpuGroup = view.selectAll('.cpuPre').data(leaves);
    // Draw line for each CPU
    cpuGroup.enter()
        .append('line')
        .classed('cpu-preview', true)
        .attr('x1', 0)
        .attr('x2', this.chartWidthPx)
        .attr('y1', d => this.getPreviewPos(d) * this.chartHeightPx)
        .attr('y2', d => this.getPreviewPos(d) * this.chartHeightPx)
        .style(
            'stroke-width',
            d => this.chartHeightPx * d.rowHeight(this.sortedFilteredCpus))
        .style('stroke-opacity', 0.5)
        .style('stroke', '#ffca28');
    cpuGroup.exit().remove();
    const durationLines = leaves.filter(leaf => this.renderDuration(leaf));
    // Draw cross-hair for each duration
    const durationGroup = view.selectAll('.durationPre').data(durationLines);
    if (this.renderDuration(preview)) {
      durationGroup.enter()
          .append('line')
          .attr('x1', d => this.chartWidthPx * d.x())
          .attr('x2', d => this.chartWidthPx * d.x())
          .attr('y1', 0)
          .attr('y2', this.chartHeightPx)
          .style('stroke-width', 2)
          .style('stroke', '#ffca28');
      durationGroup.exit().remove();
    }
  }

  /**
   * @return the relative position for the given interval's CPU preview
   */
  getPreviewPos(preview: Interval) {
    const visibleRows = this.sortedFilteredCpus;
    if (!visibleRows.length) {
      return 0;
    }
    const rowY = visibleRows.indexOf(preview.cpu) / visibleRows.length;
    const rowHeight = visibleRows.length ? 1.0 / visibleRows.length : 0;
    return rowY + rowHeight / 2;
  }

  /**
   * @return true if the given interval's duration should be rendered
   */
  renderDuration(preview: Interval) {
    return preview.width() < 1;
  }
}
