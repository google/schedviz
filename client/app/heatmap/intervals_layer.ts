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
import {AfterViewInit, ChangeDetectionStrategy, ChangeDetectorRef, Component, ElementRef, Inject, Input, OnInit, SecurityContext, ViewChild} from '@angular/core';
import {MatDialog} from '@angular/material/dialog';
import {MAT_DIALOG_DATA, MatDialogRef} from '@angular/material/dialog';
import {DomSanitizer} from '@angular/platform-browser';
import * as d3 from 'd3';
import {BehaviorSubject} from 'rxjs';

import {CpuInterval, CpuRunningLayer, CpuWaitQueueLayer, Interval, Layer, ThreadInterval} from '../models';
import {ThreadResidency, ThreadState} from '../models/render_data_services';
import {ColorService} from '../services/color_service';
import {Viewport} from '../util';

/**
 * Heatmap renderer that draws all of the intervals for a given layer.
 */
@Component({
  selector: '[intervalsLayer]',
  template: `
    <svg:g #layerRoot>
      <svg:g #intervalsRoot *ngIf="layer.value.visible"></svg:g>
    </svg:g>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class IntervalsLayer implements AfterViewInit, OnInit {
  @ViewChild('layerRoot', {static: true}) layerRoot!: ElementRef;
  @ViewChild('intervalsRoot', {static: false}) intervalsRoot!: ElementRef;
  @Input() tooltip!: Element;
  // TODO(sainsley): Look into removing nested BehaviorSubjects by tracking
  // *which* layer changed (on layers change)
  @Input() layer!: BehaviorSubject<Layer>;
  @Input() layers!: BehaviorSubject<Array<BehaviorSubject<Layer>>>;
  @Input() viewport!: BehaviorSubject<Viewport>;
  @Input() cpus!: number[];
  @Input() generateEdges!: BehaviorSubject<boolean>;
  @Input() showSleeping!: BehaviorSubject<boolean>;

  hoverInterval?: Interval;
  renderedColor?: string;
  // Local tracking of layer dimensions of last render, for incremental scaling
  renderedWidthPx = 500;
  renderedHeightPx = 500;

  constructor(
      private readonly colorService: ColorService,
      private readonly cdr: ChangeDetectorRef, public dialog: MatDialog,
      private readonly sanitizer: DomSanitizer) {}

  ngOnInit() {
    // Check required inputs
    if (!this.layer) {
      throw new Error('Missing Observable for layer');
    }
    if (!this.layers) {
      throw new Error('Missing Observable for layer set');
    }
    if (!this.viewport) {
      throw new Error('Missing Observable for viewport');
    }
    if (!this.tooltip) {
      throw new Error('Missing tooltip view child');
    }
    if (!this.generateEdges) {
      throw new Error('Missing Observable for edges flag');
    }
    if (!this.showSleeping) {
      throw new Error('Missing Observable for sleeping intervals flag');
    }
    // Rescale transform on viewport change
    this.viewport.subscribe((viewport) => {
      this.rescaleTransform(viewport);
    });
    // Recolor on layer color change. Redraw on intervals, visibility change
    this.layer.subscribe((layer) => {
      if (this.renderedColor && layer.color !== this.renderedColor) {
        this.recolor(layer);
      } else {
        // Change detection just toggles layer visibility here
        this.cdr.detectChanges();
        this.redraw(layer, this.viewport.value);
      }
    });
    this.generateEdges.subscribe(
        () => this.redraw(this.layer.value, this.viewport.value));
    this.showSleeping.subscribe(
        () => this.redraw(this.layer.value, this.viewport.value));
  }

  ngAfterViewInit() {
    const view = d3.select(this.layerRoot.nativeElement);
    view.on('mouseover', this.showTooltip.bind(this))
        .on('mousemove', this.updateTooltip.bind(this))
        .on('mouseout', this.hideTooltip.bind(this))
        .on('click', this.toggleLayer.bind(this));
  }

  /**
   * Recolors (without redrawing) intervals and edges on layer color change.
   */
  recolor(layer: Layer) {
    if (!this.intervalsRoot) {
      return;
    }
    this.renderedColor = layer.color;
    const view = d3.select(this.intervalsRoot.nativeElement);
    view.selectAll('.interval')
        .style('fill', d => layer.getIntervalColor(d as Interval))
        .style('stroke', d => layer.getIntervalColor(d as Interval));
    view.selectAll('.edge').style('stroke', d => {
      const edge = d as Interval[];
      return layer.getIntervalColor(edge[0]);
    });
  }

  /**
   * Clear and redraw intervals on update.
   */
  redraw(layer: Layer, viewport: Viewport) {
    if (!this.intervalsRoot) {
      return;
    }
    const intervals = layer.intervals;
    // Store most recently rendered dimensions on redraw
    this.renderedWidthPx = this.scaledWidthPx(viewport);
    this.renderedHeightPx = this.scaledHeightPx(viewport);
    // Clear any existing children
    const view = d3.select(this.intervalsRoot.nativeElement);
    view.selectAll('*').remove();
    // TODO(sainsley): Add efficient support for rendering interval "leaves"
    // once we support hierarchical intervals (not currently the case)
    const data = intervals;
    this.renderIntervalGroup(data, layer, viewport);
    // Auto-generate edges on demand.
    if (this.generateEdges.value) {
      const edges: Interval[][] = [];
      const intervalsById = new Map<number, Interval[]>();
      for (const interval of intervals) {
        const id = interval.id;
        const idIntervals = intervalsById.get(id);
        if (idIntervals) {
          idIntervals.push(interval);
        } else {
          intervalsById.set(id, [interval]);
        }
      }
      for (const id of intervalsById.keys()) {
        let prevInterval: Interval|undefined;
        for (const interval of intervalsById.get(id) as Interval[]) {
          // Add edge for each CPU switch
          if (prevInterval && prevInterval.cpu !== interval.cpu) {
            edges.push([prevInterval, interval]);
          }
          prevInterval = interval;
        }
      }
      this.renderEdges(edges, layer, viewport);
    }
    if (layer.drawEdges) {
      for (const interval of intervals) {
        if (interval.edges.length) {
          this.renderEdges(interval.edges, layer, viewport);
        }
      }
    }
    this.rescaleTransform(viewport);
  }

  /**
   * Renders a set of intervals as 'pills', rectangles with rounded ends (to
   * easily distinguish adjecent intervals).
   */
  renderIntervalGroup(intervals: Interval[], layer: Layer, viewport: Viewport) {
    if (!intervals || !intervals.length) {
      return;
    }
    let data = intervals.filter(interval => interval.shouldRender);
    // Hide sleeping intervals on request
    if (!this.showSleeping.value) {
      data = data.filter(
          interval => !(interval instanceof ThreadInterval) ||
              interval.state !== ThreadState.SLEEPING_STATE);
    }
    // Increase stroke weight with zoom
    const scaledWidthPx = this.scaledWidthPx(viewport);
    const scaledHeightPx = this.scaledHeightPx(viewport);
    const strokeWeight = 0.1 / viewport.height;
    const aspectRatio = viewport.width / viewport.height;
    const cpus = this.cpus;
    const view = d3.select(this.intervalsRoot.nativeElement);
    const dataGroup = view.selectAll('.intervalGroup').data(data);
    // Draws intervals scaled to zoomed chart dimensions
    dataGroup.enter()
        .append('rect')
        .classed('interval', true)
        .attr('rx', d => scaledWidthPx * d.rx(cpus) * aspectRatio)
        .attr('ry', d => scaledHeightPx * d.ry(cpus))
        .attr('x', d => scaledWidthPx * d.x())
        .attr('y', d => scaledHeightPx * d.y(cpus))
        .attr('height', d => scaledHeightPx * d.height(cpus))
        .attr(
            'width',
            d => scaledWidthPx *
                (d.width() ? d.width() :
                             d.minWidth ? aspectRatio * d.height(cpus) / 2 :
                                          2.0 / scaledWidthPx))
        .style('fill', d => layer.getIntervalColor(d))
        .style('fill-opacity', d => d.opacity)
        .style('stroke', d => layer.getIntervalColor(d))
        .style('stroke-width', strokeWeight)
        .style('cursor', 'pointer');
    dataGroup.exit().remove();
  }

  /**
   * Draws edges between flattened interval set, if applicable.
   */
  renderEdges(edges: Interval[][], layer: Layer, viewport: Viewport) {
    const scaledWidthPx = this.scaledWidthPx(viewport);
    const scaledHeightPx = this.scaledHeightPx(viewport);
    const strokeWeight = 0.5 / viewport.height;
    const cpus = this.cpus;
    const view = d3.select(this.intervalsRoot.nativeElement);
    const edgeGroup = view.selectAll('.edgeGroup').data(edges);
    const edge = edgeGroup.enter()
                     .append('g')
                     .classed('edge', true)
                     .style('stroke-width', strokeWeight)
                     .style('stroke', d => layer.getIntervalColor(d[0]))
                     .style('fill', 'none');
    edge.append('line')
        .attr('x1', d => scaledWidthPx * (d[0].x() + d[0].width()))
        .attr('x2', d => scaledWidthPx * d[1].x())
        .attr(
            'y1', d => scaledHeightPx * (d[0].y(cpus) + d[0].height(cpus) / 2))
        .attr(
            'y2', d => scaledHeightPx * (d[1].y(cpus) + d[1].height(cpus) / 2));
    edgeGroup.exit().remove();
  }

  /**
   * Creates a new Layer for the given thread, or returns (and makes visible)
   * the layer for the given thread, if one already exists.
   * @param threadResidency The thread to get or create a layer for.
   */
  getOrCreateLayer(threadResidency: ThreadResidency) {
    const name = getThreadName(threadResidency);
    const {pid} = threadResidency.thread;
    if (pid < 0) {
      return;
    }

    const layers = this.layers.value;
    const pidLayer = layers.find(layer => layer.value.name === name);
    if (!pidLayer) {
      const color = this.colorService.getColorFor(name);
      const newLayer = new Layer(name, 'Thread', [pid], color);
      layers.unshift(new BehaviorSubject(newLayer));
      this.layers.next(layers);
    } else {
      pidLayer.value.visible = true;
      pidLayer.next(pidLayer.value);
    }
  }

  /**
   * Show a layer for a running thread when an interval is clicked.
   * If there is more than one running thread in the interval, ask the user
   * which one(s) they want to make layer(s) for.
   * @param clickInterval The interval that was clicked.
   */
  showLayer(clickInterval: CpuInterval) {
    const currentLayers = new Map<string, Layer>(
        this.layers.value.map(ls => [ls.value.name, ls.value]));
    // Get a list of layers that don't already exist or are invisible.
    const runningThreads = clickInterval.running.filter(tr => {
      const existingLayer = currentLayers.get(getThreadName(tr));
      return !existingLayer || !existingLayer.visible;
    });
    if (runningThreads.length > 1) {
      // Show dialog to ask the user which thread to make a layer for
      const dialogRef =
          this.dialog.open(DialogChooseThreadLayer, {data: runningThreads});
      dialogRef.afterClosed().subscribe((trs: ThreadResidency[]|undefined) => {
        if (!trs) {
          return;
        }
        trs.forEach(tr => {
          this.getOrCreateLayer(tr);
        });
      });
    } else if (runningThreads.length === 1) {
      this.getOrCreateLayer(runningThreads[0]);
    }
  }

  /**
   * Toggle layer visibility on interval click.
   */
  toggleLayer() {
    const layer = this.layer.value;
    // On CpuInterval click, create a new Thread layer for running PID (or
    // toggle layer visibility, if one already exists)
    if (layer instanceof CpuRunningLayer ||
        layer instanceof CpuWaitQueueLayer) {
      const event = d3.event;
      const data = d3.select(event.target).data();
      // TODO(sainsley): Traverse up from target until data is found.
      if (data.length && data[0] instanceof CpuInterval) {
        this.showLayer(data[0] as CpuInterval);
      }
    } else {
      // Else, simply toggle layer visibility
      layer.visible = false;
      this.layer.next(layer);
    }
  }

  /**
   * Shows a tooltip for the currently mousedover heatmap data
   */
  showTooltip() {
    const tooltip = d3.select(this.tooltip);
    tooltip.transition().duration(200).style('opacity', .9);
    this.updateTooltip();
  }

  /**
   * Hides tooltip on mouseout
   */
  hideTooltip() {
    const tooltip = d3.select(this.tooltip);
    tooltip.transition().duration(200).style('opacity', 0);
  }

  /**
   * Updates tooltip pos and reference interval on mousemove
   */
  updateTooltip() {
    const layer = this.layer.value;
    const event = d3.event;
    const data = d3.select(event.target).data();
    // TODO(sainsley): Traverse up from target until data is found.
    if (!data.length || !(data[0] instanceof Interval)) {
      this.hideTooltip();
      return;
    }
    this.hoverInterval = data[0] as Interval;
    const offsetX = 10;
    const offsetY = 28;
    this.tooltip.innerHTML = this.tooltipHtml;
    const tooltip = d3.select(this.tooltip);
    tooltip.style('border-color', layer.getIntervalColor(this.hoverInterval));
    tooltip.style('left', `${(event.layerX as number) + offsetX}px`)
        .style('top', `${(event.layerY as number) - offsetY}px`);
  }

  /**
   * Rescale transform on viewport size change.
   */
  rescaleTransform(viewport: Viewport) {
    if (!this.intervalsRoot) {
      return;
    }
    const view = d3.select(this.intervalsRoot.nativeElement);
    const localScaleX = this.scaledWidthPx(viewport) / this.renderedWidthPx;
    const localScaleY = this.scaledHeightPx(viewport) / this.renderedHeightPx;
    view.attr('transform', `scale(${localScaleX}, ${localScaleY})`);
  }

  scaledWidthPx(viewport: Viewport) {
    return viewport.chartWidthPx / viewport.width;
  }

  scaledHeightPx(viewport: Viewport) {
    return viewport.chartHeightPx / viewport.height;
  }

  /**
   * @return the inner HTML for the global tooltip given the current
   * mouseover interval
   */
  get tooltipHtml() {
    if (!this.hoverInterval) {
      return '';
    }
    const hoverInterval = this.hoverInterval;
    // Add a new line for each valid tooltip property
    const tooltipHtml =
        this.tooltipKeys
            .map((key) => {
              const prop = hoverInterval.tooltipProps[key];
              return prop ? `<div><b>${key}: </b><span>${prop}</span></div>` :
                            '';
            })
            .join('');
    const safeHtml = this.sanitizer.sanitize(SecurityContext.HTML, tooltipHtml);
    return safeHtml ? safeHtml : '';
  }

  /**
   * @return keys to render in tooltip
   */
  get tooltipKeys() {
    if (!this.hoverInterval) {
      return [];
    }
    return Object.keys(this.hoverInterval.tooltipProps);
  }
}

/**
 * A dialog to choose which thread layer to use to make a new layer.
 */
@Component({
  selector: 'dialog-choose-thread-layer',
  template: `
  <h1 mat-dialog-title>
    Which running threads do you want to show layers for?
  </h1>
  <form #form="ngForm" (ngSubmit)="onFormSubmit()">
    <div mat-dialog-content>
      <mat-form-field>
        <mat-label>Choose threads</mat-label>
        <mat-select [(ngModel)]="selectedThreads"
                    multiple
                    name="selectedThread"
                    required>
          <mat-select-trigger>
            {{getLabel()}}
          </mat-select-trigger>
          <mat-option *ngFor="let thread of this.data" [value]="thread">
            {{getThreadName(thread)}}
          </mat-option>
        </mat-select>
      </mat-form-field>
    </div>
    <div mat-dialog-actions align="end">
      <button type="button" mat-stroked-button [mat-dialog-close]="null">
        Cancel
      </button>
      <button type="submit" mat-stroked-button>Submit</button>
    </div>
  </form>
  `,
  styles: [`
    mat-form-field.mat-form-field {
      display: block;
    }
    [mat-dialog-content] {
      margin-bottom: 225px;
    }
    [mat-dialog-actions] button {
      margin-left: 5px;
    }
  `]
})
export class DialogChooseThreadLayer {
  selectedThreads: ThreadResidency[] = [];
  // Make visible in the template
  getThreadName = getThreadName;

  constructor(
      public dialogRef: MatDialogRef<DialogChooseThreadLayer>,
      @Inject(MAT_DIALOG_DATA) public data: ThreadResidency[]) {}

  onFormSubmit() {
    this.dialogRef.close(this.selectedThreads);
  }

  getLabel() {
    return this.selectedThreads.map(t => getThreadName(t)).join(', ');
  }
}

function getThreadName(threadResidency?: ThreadResidency): string {
  if (!threadResidency) {
    return '';
  }
  return `Thread:${threadResidency.thread.pid}:${
      threadResidency.thread.command}`;
}
