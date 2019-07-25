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
import {HttpErrorResponse} from '@angular/common/http';
import {AfterViewInit, ChangeDetectionStrategy, ChangeDetectorRef, Component, ElementRef, Inject, Input, OnDestroy, OnInit, ViewChild} from '@angular/core';
import {MatSnackBar} from '@angular/material/snack-bar';
import * as d3 from 'd3';
import {BehaviorSubject, from, of, Subject, Subscription} from 'rxjs';
import {debounceTime, mergeMap, pairwise} from 'rxjs/operators';

import {CollectionParameters, CpuInterval, CpuRunningLayer, CpuWaitQueueLayer, Interval, Layer, WaitingCpuInterval, WaitingThreadInterval} from '../models';
import {RenderDataService} from '../services/render_data_service';
import {createHttpErrorMessage, SystemTopology, Viewport} from '../util';
import {nearlyEquals} from '../util/helpers';

const HEATMAP_MARGIN_X = 150;
const HEATMAP_MARGIN_Y = 100;
const HEATMAP_PADDING_X = 100;
const HEATMAP_PADDING_Y = 20;

// Minimum permitted interval size, in pixels
const MIN_INTERVAL_SIZE_PX = 2;
const MIN_WIDTH_NS = 100000;

/**
 * An interactive heatmap that displays CPU state intervals, thread lifecycle
 * intervals, thread events, etc.
 */
@Component({
  selector: 'heatmap',
  templateUrl: './heatmap.ng.html',
  styleUrls: ['heatmap.css'],
  host: {'(window:resize)': 'onResize()'},
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Heatmap implements AfterViewInit, OnInit, OnDestroy {
  // View Children
  @ViewChild('svg', {static: true}) svg!: ElementRef;
  @ViewChild('tooltip', {static: true}) tooltip!: ElementRef;
  @ViewChild('clipRectAxes', {static: true}) clipRectAxes!: ElementRef;
  @ViewChild('xAxisGroup', {static: true}) xAxisGroup!: ElementRef;
  @ViewChild('previewGroup', {static: true}) previewGroup!: ElementRef;
  @ViewChild('zoomGroup', {static: true}) zoomGroup!: ElementRef;

  // Required inputs
  @Input() parameters!: BehaviorSubject<CollectionParameters|undefined>;
  @Input() systemTopology!: SystemTopology;
  @Input() preview!: BehaviorSubject<Interval|undefined>;
  @Input() layers!: BehaviorSubject<Array<BehaviorSubject<Layer>>>;
  @Input() viewport!: BehaviorSubject<Viewport>;
  @Input() cpuFilter!: BehaviorSubject<string>;
  @Input() maxIntervalCount!: BehaviorSubject<number>;
  @Input() showMigrations!: BehaviorSubject<boolean>;
  @Input() showSleeping!: BehaviorSubject<boolean>;

  visibleCpus = new BehaviorSubject<number[]>([]);
  baseIntervals: CpuInterval[] = [];

  cpuRunLayer!: BehaviorSubject<CpuRunningLayer>;
  cpuWaitQueueLayer!: BehaviorSubject<CpuWaitQueueLayer>;
  // Subscriptions listen for data from the SchedViz backend
  cpuIntervalSubscription?: Subscription;
  pidIntervalSubscription?: Subscription;
  schedEventSubscription?: Subscription;
  pendingLayerCount = 0;

  // Zoom logic
  /** Debounced observer notifies for interval fetch on zoom changes. */
  zoomChanged = new Subject<Viewport>();
  /** Last seen zoom transform for computing viewport delta on zoom change. */
  lastTransform: d3.ZoomTransform = d3.zoomIdentity;
  /** TODO(sainsley): Hook up controls in thread list in follow-up CL */

  constructor(
      @Inject('RenderDataService') public renderDataService: RenderDataService,
      private readonly snackBar: MatSnackBar,
      private readonly cdr: ChangeDetectorRef) {}

  onResize() {
    this.updateViewportSize();
  }

  ngOnInit() {
    // Check required inputs
    if (!this.parameters) {
      throw new Error('Missing required CollectionParameters');
    }
    if (!this.systemTopology) {
      throw new Error('Missing required SystemTopology');
    }
    if (!this.preview) {
      throw new Error('Missing Observable for preview');
    }
    if (!this.layers) {
      throw new Error('Missing Observable for layers');
    }
    if (!this.viewport) {
      throw new Error('Missing Observable for viewport');
    }
    if (!this.cpuFilter) {
      throw new Error('Missing Observable for CPU filter');
    }
    if (!this.maxIntervalCount) {
      throw new Error('Missing Observable for maxIntervalCount');
    }
    if (!this.showMigrations) {
      throw new Error('Missing Observable for migrations flag');
    }
    if (!this.showSleeping) {
      throw new Error('Missing Observable for sleeping intervals flag');
    }
    // Find dedicated layers for running + waiting CPU intervals
    // Generate if missing
    const cpuRunLayer =
        this.layers.value.find(layer => layer.value instanceof CpuRunningLayer);
    if (cpuRunLayer) {
      this.cpuRunLayer =
          cpuRunLayer as unknown as BehaviorSubject<CpuRunningLayer>;
    } else {
      this.cpuRunLayer = new BehaviorSubject(new CpuRunningLayer());
      this.layers.value.push(
          this.cpuRunLayer as unknown as BehaviorSubject<Layer>);
    }
    const cpuWaitQueueLayer = this.layers.value.find(
        layer => layer.value instanceof CpuWaitQueueLayer);
    if (cpuWaitQueueLayer) {
      this.cpuWaitQueueLayer = cpuWaitQueueLayer;
    } else {
      this.cpuWaitQueueLayer = new BehaviorSubject(new CpuWaitQueueLayer());
      this.layers.value.push(this.cpuWaitQueueLayer);
    }
    this.layers.subscribe(layers => {
      // TODO(sainsley): Try to remove this call to change detection.
      // (Used to create the interval-layers)
      this.cdr.detectChanges();
      this.fetchMissingIntervals(layers, this.viewport.value);
    });
    // Immediately rescale transform on viewport change
    this.viewport.subscribe(viewport => this.rescaleTransforms(viewport));
    // Periodically refetch intervals from backend on viewport change
    this.viewport
        .pipe(debounceTime(300))  // wait at least 300ms between emits
        .subscribe(viewport => {
          this.pendingLayerCount = 0;
          // Refresh layer data on viewport change
          this.refreshLayers(viewport);
        });
    this.maxIntervalCount
        .pipe(debounceTime(300))  // wait at least 300ms between emits
        .subscribe(size => {
          this.pendingLayerCount = 0;
          // Refresh layer data on viewport change
          this.refreshLayers(this.viewport.value);
        });
    // Reset viewport height and force redraw on CPU filter change
    this.cpuFilter.pipe(pairwise()).subscribe(([oldFilter, newFilter]) => {
      if (oldFilter === newFilter) {
        return;
      }
      const viewport = this.viewport.value;
      viewport.resetY();
      this.viewport.next(viewport);
      // TODO(sainsley): Remove by adding appropriate listener in interval layer
      this.cdr.detectChanges();
    });
    // Fetch base intervals for zoom brush.
    if (this.parameters.value) {
      const viewport = new Viewport();
      const minDuration = this.getMinIntervalWidthNs(viewport);
      const params = this.parameters.value;
      this.renderDataService
          .getCpuIntervals(
              params, viewport, minDuration, this.systemTopology.cpus)
          .subscribe(
              intervals => {
                this.baseIntervals = intervals;
                if (this.hasBasicView) {
                  this.setCpuIntervals(this.baseIntervals);
                }
                this.cdr.detectChanges();
              },
              (err: HttpErrorResponse) => {
                const errMsg = createHttpErrorMessage(
                    `Failed to get CPU intervals for ${params.name}`, err);
                this.snackBar.open(errMsg, 'Dismiss');
              });
    }
  }

  ngAfterViewInit() {
    const svg = d3.select(this.svg.nativeElement);
    this.updateViewportSize();
    svg.call(d3.zoom().on('zoom', this.onZoom.bind(this)))
        .on('dblclick.zoom', null);
    this.onResize();
  }

  ngOnDestroy() {
    // Close all Subjects to prevent leaks.
    this.cpuRunLayer.complete();
    this.cpuWaitQueueLayer.complete();
    this.visibleCpus.complete();
    // TODO(sainsley): Use switchMap to avoid manually managing subscriptions
    if (this.cpuIntervalSubscription) {
      this.cpuIntervalSubscription.unsubscribe();
    }
    if (this.pidIntervalSubscription) {
      this.pidIntervalSubscription.unsubscribe();
    }
  }

  // VIEWPORT UPDATES

  /**
   * True if the heatmap's viewport are filters are unmodified.
   */
  get hasBasicView() {
    const viewport = this.viewport.value;
    const cpuFilter = this.cpuFilter.value;
    return viewport.width === 1.0 && viewport.height === 1.0 &&
        !cpuFilter.length;
  }

  /**
   * Updates viewport on d3 zoom.
   */
  onZoom() {
    const newTransform = d3.event.transform;
    this.maybeUpdateViewport(newTransform, d3.event.sourceEvent.shiftKey);
  }

  /**
   * Rescales clip paths and transforms on viewport change.
   */
  rescaleTransforms(viewport: Viewport) {
    const chartWidthPx = viewport.chartWidthPx;
    const chartHeightPx = viewport.chartHeightPx;
    const offsetXPx = -1 * viewport.translateXPx;
    const offsetYPx = -1 * viewport.translateYPx;
    // Rescale clip path
    d3.select(this.clipRectAxes.nativeElement)
        .attr('width', chartWidthPx + HEATMAP_MARGIN_X + HEATMAP_PADDING_X)
        .attr('height', chartHeightPx + HEATMAP_PADDING_Y);
    // Rescale zoom group
    d3.select(this.zoomGroup.nativeElement)
        .attr('transform', `translate(${- 1 * offsetXPx}, ${- 1 * offsetYPx})`);
  }

  /**
   * Updates internal viewport representation on d3 zoom transform change, if
   * within bounds.
   */
  maybeUpdateViewport(newTransform: d3.ZoomTransform, symmetricZoom: boolean) {
    const viewport = this.viewport.value;
    const deltaK = newTransform.k / this.lastTransform.k;
    const isPanning = nearlyEquals(deltaK, 1.0, 1e-10);
    // Clip zoom if viewport is maximized and zoom out attempted or viewport is
    // minimized and zoom in is attempted
    const minWidth = MIN_WIDTH_NS / this.domainSize;
    const minHeight = 1.0 / this.getVisibleCpus().length;
    const zoomXPermitted =
        !(viewport.width <= minWidth && deltaK > 1.0 ||
          viewport.width === 1.0 && deltaK < 1.0);
    const zoomYPermitted =
        !(viewport.height <= minHeight && deltaK > 1.0 ||
          viewport.height === 1.0 && deltaK < 1.0);
    const updateX = zoomXPermitted || isPanning;
    const updateY =
        (symmetricZoom || (viewport.width === 1.0 && deltaK < 1.0)) &&
            zoomYPermitted ||
        isPanning;
    const deltaXPx =
        updateX ? newTransform.x - this.lastTransform.x * deltaK : 0;
    const deltaYPx =
        updateY ? newTransform.y - this.lastTransform.y * deltaK : 0;
    const deltaKY = updateY ? deltaK : 1;
    const deltaKX = updateX ? deltaK : 1;
    this.updateViewportSize();
    viewport.updateZoom(deltaKX, deltaKY, deltaXPx, deltaYPx);
    this.lastTransform = newTransform;
    this.viewport.next(viewport);
  }

  /**
   * Window resize callback. Updates interval widths according to heatmap size.
   */
  updateViewportSize() {
    const viewport = this.viewport.value;
    const svgHeight = this.svg.nativeElement.clientHeight - HEATMAP_MARGIN_Y;
    const svgWidth = this.svg.nativeElement.clientWidth - HEATMAP_MARGIN_X;
    if (svgHeight > 0 && svgWidth > 0) {
      viewport.updateSize(svgWidth, svgHeight);
      this.viewport.next(viewport);
    }
  }

  /**
   * Refreshes layer data from SV backend on zoom debounce complete.
   */
  refreshLayers(viewport: Viewport) {
    const layers = this.layers.value;
    // Fetch CPU intervals on viewport change
    this.fetchCpuIntervals(viewport);
    const threadLayers =
        layers.filter(layer => layer.value.dataType === 'Thread');
    this.fetchPidIntervals(threadLayers, viewport);
    const eventLayers =
        layers.filter(layer => layer.value.dataType === 'SchedEvent');
    this.fetchSchedEvents(eventLayers, viewport);
    // Flag all other layers for redraw
    for (const layer of this.layers.value) {
      if (layer.value.dataType !== 'Thread' && layer.value.dataType !== 'CPU') {
        layer.next(layer.value);
      }
    }
    // Update loading indicator
    this.cdr.detectChanges();
  }

  /**
   * Fetches intervals for any layer that is flagged as dirty.
   */
  fetchMissingIntervals(
      layers: Array<BehaviorSubject<Layer>>, viewport: Viewport) {
    if (!this.cpuRunLayer.value.initialized) {
      // Initial CPU interval fetch
      this.fetchCpuIntervals(viewport);
    }
    const incompleteLayers = layers.filter(layer => !layer.value.initialized);
    this.fetchPidIntervals(
        incompleteLayers.filter(layer => layer.value.dataType === 'Thread'),
        viewport);
    this.fetchSchedEvents(
        incompleteLayers.filter(layer => layer.value.dataType === 'SchedEvent'),
        viewport);
    // Update loading indicator
    this.cdr.detectChanges();
  }

  /**
   * Async callback for refreshing CPU intervals on viewport change.
   */
  fetchCpuIntervals(viewport: Viewport) {
    if (!this.parameters.value) {
      return;
    }
    if (this.hasBasicView) {
      this.setCpuIntervals(this.baseIntervals);
      return;
    }
    if (this.cpuIntervalSubscription) {
      this.cpuIntervalSubscription.unsubscribe();
    }
    this.pendingLayerCount++;
    const minDuration = this.getMinIntervalWidthNs(viewport);
    const params = this.parameters.value;
    this.cpuIntervalSubscription =
        this.renderDataService
            .getCpuIntervals(
                params, viewport, minDuration, this.getVisibleCpus())
            .subscribe(
                intervals => {
                  this.setCpuIntervals(intervals);
                  this.pendingLayerCount--;
                  if (!this.loading) {
                    // Update loading indicator
                    this.cdr.detectChanges();
                  }
                },
                (err: HttpErrorResponse) => {
                  const errMsg = createHttpErrorMessage(
                      `Failed to get CPU intervals for ${params.name}`, err);
                  this.snackBar.open(errMsg, 'Dismiss');
                });
  }

  /**
   * Updates the interval set for the CPU run/wait layers
   */
  setCpuIntervals(intervals: CpuInterval[]) {
    const cpuRunLayer = this.cpuRunLayer.value;
    const cpuWaitLayer = this.cpuWaitQueueLayer.value;
    cpuRunLayer.initialized = true;
    cpuWaitLayer.initialized = true;
    // Store new set of running CPU intervals
    cpuRunLayer.intervals = intervals;
    // Filter out and store new set of waiting CPU intervals
    const waitIntervals =
        intervals.filter(interval => interval.waitingPidCount).map(interval => {
          return new WaitingCpuInterval(
              interval.parameters, interval.cpu, interval.startTimeNs,
              interval.endTimeNs, interval.command, interval.waitingPidCount,
              interval.runningPid, interval.waitingPidList);
        });
    cpuWaitLayer.intervals = waitIntervals;
    // Mark layer as ready for redraw.
    this.cpuRunLayer.next(cpuRunLayer);
    this.cpuWaitQueueLayer.next(cpuWaitLayer);
  }

  /**
   * Fetches PID intervals for visible thread layers on viewport change.
   */
  fetchPidIntervals(layers: Array<BehaviorSubject<Layer>>, viewport: Viewport) {
    if (this.pidIntervalSubscription) {
      this.pidIntervalSubscription.unsubscribe();
    }
    if (!this.parameters.value) {
      return;
    }
    const params = this.parameters.value;
    this.pendingLayerCount += layers.length;
    this.pidIntervalSubscription =
        from(layers)
            .pipe(mergeMap(layerSubj => {
              const layer = layerSubj.value;
              // Filter out PID 0, which can't have pid intervals
              layer.ids = layer.ids.filter(id => id !== 0);
              if (!layer.ids.length) {
                return of(null);
              }
              return this.renderDataService.getPidIntervals(
                  params, layer, viewport,
                  this.getMinIntervalWidthNs(viewport));
            }))
            .subscribe(
                layer => {
                  if (!layer || !(layer instanceof Layer)) {
                    return;
                  }
                  this.onLayerDataReady(layers, layer);
                  if (!this.loading) {
                    this.arrangeWaitingPids(layers.map(layer => layer.value));
                    // Force update of loading bar
                    // TODO(sainsley): Remove, if possible.
                    this.cdr.detectChanges();
                  }
                },
                (err: HttpErrorResponse) => {
                  const errMsg = createHttpErrorMessage(
                      `Failed to get PID intervals for ${params.name}`, err);
                  this.snackBar.open(errMsg, 'Dismiss');
                });
  }


  /**
   * Fetches events for visible sched event layers on viewport change.
   */
  fetchSchedEvents(layers: Array<BehaviorSubject<Layer>>, viewport: Viewport) {
    if (this.schedEventSubscription) {
      this.schedEventSubscription.unsubscribe();
    }
    if (!this.parameters.value) {
      return;
    }
    this.pendingLayerCount += layers.length;
    const params = this.parameters.value;
    this.schedEventSubscription =
        from(layers)
            .pipe(mergeMap(
                layer => this.renderDataService.getSchedEvents(
                    params, layer.value, viewport, this.getVisibleCpus())))
            .subscribe(
                layer => this.onLayerDataReady(layers, layer),
                (err: HttpErrorResponse) => {
                  const errMsg = createHttpErrorMessage(
                      `Failed to get get ftrace events for ${params.name}`,
                      err);
                  this.snackBar.open(errMsg, 'Dismiss');
                });
  }

  onLayerDataReady(layers: Array<BehaviorSubject<Layer>>, layer: Layer) {
    layer.initialized = true;
    // TODO(sainsley): Avoid looking up the original Subject.
    // Consider passing the Subject through to the render service,
    // and having the service call 'next'.
    const layerSub =
        layers.find(layerSub => layerSub.value.name === layer.name);
    if (layerSub) {
      layerSub.next(layer);
    }
    this.pendingLayerCount--;
  }

  /**
   * Assigns vertical positions to overlapping waiting PID intervals on
   * layers change. Organizes the current set of waiting PID intervals in view
   * by CPU (i.e. row) and sorts them by start time. Moving left to right, if
   * any waiting interval overlaps with a subsequent interval in the same CPU,
   * their heights and row positions are adjusted accordingly.
   */
  arrangeWaitingPids(threadLayers: Layer[]) {
    if (!this.parameters.value) {
      return;
    }
    let waitQueue: WaitingThreadInterval[] = [];
    for (const layer of threadLayers) {
      // Consolodate waiting intervals from all layers
      // (except the base CPU layers)
      const waiting =
          (layer.intervals)
              .filter(interval => interval instanceof WaitingThreadInterval) as
          WaitingThreadInterval[];
      waitQueue = waitQueue.concat(waiting);
    }
    // Organize waiting intervals by CPU to check for overlap
    const waitingIntervalsByCpu: WaitingThreadInterval[][] = [];
    for (let i = 0; i < this.parameters.value.size; i++) {
      waitingIntervalsByCpu.push([]);
    }
    for (const interval of waitQueue) {
      const cpuId = this.parameters.value.cpus.indexOf(interval.cpu);
      waitingIntervalsByCpu[cpuId].push(interval);
    }
    // Sort intervals within a given CPU by start time
    for (const cpuIntervals of waitingIntervalsByCpu) {
      cpuIntervals.sort((a, b) => a.startTimeNs - b.startTimeNs);
    }
    // For each waiting interval in a given CPU:
    for (const cpuIntervals of waitingIntervalsByCpu) {
      for (let i = 0; i < cpuIntervals.length; i++) {
        const intervalLeft = cpuIntervals[i];
        intervalLeft.queueOffset = 0;
        // Check if any subsequent intervals overlap
        for (let j = i + 1; j < cpuIntervals.length; j++) {
          const intervalRight = cpuIntervals[j];
          // Reach end of interval, stop checking for overlapping intervals
          if (intervalRight.startTimeNs >= intervalLeft.endTimeNs) {
            break;
          }
          intervalLeft.queueCount++;
          intervalRight.queueCount++;
          intervalRight.queueOffset++;
        }
      }
    }
  }

  get domainSize() {
    if (!this.parameters.value) {
      return 0;
    }
    return this.parameters.value.endTimeNs - this.parameters.value.startTimeNs;
  }

  /**
   * @return the minimum permitted interval width to use when fetching
   * intervals, based on the current zoom level, layer count, and visible cpu
   * count. Aims to maintain a constant upper bound to the number of visible
   * intervals.
   */
  getMinIntervalWidthNs(viewport: Viewport) {
    const maxIntervalCount = this.maxIntervalCount.value;
    const domainSize = viewport.width * this.domainSize;
    const filteredCpuCount =
        this.systemTopology.getSortedFilteredCpuLabels(this.cpuFilter.value)
            .length;
    const visibleCpuCount = Math.ceil(viewport.height * filteredCpuCount);
    const maxIntervalsPerLayer = maxIntervalCount / this.layers.value.length;
    const maxIntervalsPerRow = maxIntervalsPerLayer / visibleCpuCount;
    // Compute the minimum interval duration to maintain upper limit on
    // total interval count
    const minDurationForIntervalCount =
        Math.floor(domainSize / maxIntervalsPerRow);
    // Compute the minimum interval duration to render visible intervals
    const minDurationForVisibility =
        Math.floor(domainSize * MIN_INTERVAL_SIZE_PX / viewport.chartWidthPx);
    // Return this maximum duration given the two constraints
    return Math.max(minDurationForVisibility, minDurationForIntervalCount);
  }

  // ACCESSORS

  /** True if backend data pending */
  get loading() {
    // TODO(sainsley): Just listen for loading subject
    return this.pendingLayerCount !== 0;
  }

  // TODO(sainsley): Remove these in favor of subscribing to CPU filter
  // as needed in child layers.
  getVisibleCpus() {
    const visibleCpus = this.systemTopology.getVisibleCpuIds(
        this.viewport.value, this.cpuFilter.value);
    this.visibleCpus.next(visibleCpus);
    return visibleCpus;
  }

  get sortedFilteredCpus() {
    return this.systemTopology.getSortedFilteredCpuIds(this.cpuFilter.value);
  }
}
