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
import {Component} from '@angular/core';
import * as d3 from 'd3';

import {ComplexCpuLabel, ComplexSystemTopology, Viewport} from '../../util';

import {CpuAxisLayer} from './cpu_axis_layer';

const OFFSET_Y = 4;

/**
 * The topological cpuAxis component displays the current list of visible CPUs
 * sorted topologically by NUMA node, physical core, and hyperthread.
 */
@Component({
  selector: '[topoCpuAxis]',
  template: `
    <svg:g #cpuAxis>
      <rect
        [attr.x]="0"
        width="30"
        fill="#000"
      ></rect>
      <rect class="axisBase"
        y="-50"
        width="100"
        fill="#000"
      ></rect>
    </svg:g>
  `,
  styleUrls: ['cpu_axes.css'],
})
export class TopologicalCpuAxisLayer extends CpuAxisLayer {
  /**
   * @return a string label for the given axis tick index.
   */
  getLabelForIndex(
      index: number, cpus: ComplexCpuLabel[], numaBlocks: number[][],
      blockSize: number) {
    const cpuCount = cpus.length;
    if (index < 0 || index >= cpuCount) {
      return '';
    }
    const cpu = cpus[Math.floor(index)];
    const ht = cpu.getHyperThread();
    const numa = cpu.getNumaNode();
    const core = cpu.getCore();
    // Display hyperthread labels only for row labels
    let axisLabel = '';
    const showHt = Number.isInteger(index);
    let showCore = !showHt && index % 1 === 0.5;
    let showNuma = !showHt && !showCore;
    // If proper tick, show hyperthread.
    if (showHt) {
      axisLabel += `HT ${ht} `;
      showCore = numaBlocks[numa][core] === 1;
    }
    if (showCore) {
      axisLabel += `Core ${core} `;
      showNuma = numaBlocks[numa][blockSize] === 1;
    }
    if (showNuma) {
      axisLabel += `NUMA ${numa}`;
    }
    return axisLabel;
  }

  /**
   * Draws the CPU visual markers and text labels.
   */
  drawLabels(viewport: Viewport) {
    if (!(this.topology instanceof ComplexSystemTopology)) {
      return;
    }
    const view = d3.select(this.cpuAxis.nativeElement);
    view.selectAll('.cpuLabel').remove();

    // Label axis based on visible cpus.
    const cpus = this.visibleRows as ComplexCpuLabel[];
    // Count visible cpu count in each numa block, for label positioning.
    const cpuCount = this.topology.cpuCount;
    const numaCount = this.topology.getNumaNodeCount();
    const hyperthreadsPerCore = this.topology.getHyperthreadCountPerCore();
    const blockSize = this.topology.getBlockSize();
    const numaBlocks = new Array(numaCount).fill(0).map(
        () => new Array(blockSize + 1).fill(0));
    for (const cpu of cpus) {
      const numa = cpu.getNumaNode();
      const core = cpu.getCore();
      numaBlocks[numa][core]++;
      numaBlocks[numa][blockSize]++;
    }

    // Add ticks based on block sizes
    const tickValues = new Array(cpus.length).fill(0).map((d, i) => i);
    let i = 0;
    let currentNuma = -1;
    for (const cpu of cpus) {
      const numa = cpu.getNumaNode();
      const core = cpu.getCore();
      if (cpu.getHyperThread() === 0) {
        // Add core tick
        const htCount = numaBlocks[numa][core];
        if (htCount > 1) {
          tickValues.push(i + (htCount - 1) / 2);
        }
      }
      // Add numa tick
      if (currentNuma !== numa) {
        const coreCount = numaBlocks[numa][blockSize];
        if (coreCount > 1) {
          tickValues.push(i + (coreCount - 0.5) / 2);
        }
        currentNuma = numa;
      }
      i++;
    }

    const dataGroup = view.selectAll('.cpuGroup').data(tickValues);
    // Add labels
    dataGroup.enter()
        .append('text')
        .classed('cpuLabel', true)
        .text(tick => this.getLabelForIndex(tick, cpus, numaBlocks, blockSize))
        .on('click', (tick) => {
          const cpu = cpus[Math.floor(tick)];
          const numa = cpu.getNumaNode();
          const core = cpu.getCore();
          const isHt = Number.isInteger(tick);
          const isCore = !isHt && tick % 1 === 0.5;
          // Compute selected cores
          let filteredCpus = [];
          if (isHt) {
            filteredCpus = [cpus[tick]];
          } else if (isCore) {
            filteredCpus = cpus.filter(
                (cpu) => cpu.getNumaNode() === numa && cpu.getCore() === core);
          } else {
            filteredCpus = cpus.filter((cpu) => cpu.getNumaNode() === numa);
          }
          this.toggleCpuFilter(filteredCpus.map(cpu => cpu.cpuIndex));
        });
    dataGroup.exit().remove();
    this.scaleLabels(viewport);
  }

  /**
   * @return the given tick's scaled Y position with zoom/filtering applied, in
   * pixels.
   */
  scaledTickYPx(tick: number, viewport: Viewport) {
    const visibleRows = this.visibleRows;
    const rowHeight = this.scaledRowHeightPx(viewport);
    return tick * rowHeight + viewport.translateYPx + rowHeight / 4 + OFFSET_Y;
  }

  /**
   * @return rescales the axis labels on viewport change.
   */
  scaleLabels(viewport: Viewport) {
    const view = d3.select(this.cpuAxis.nativeElement);
    view.select('.axisBase').attr('height', viewport.chartHeightPx + 50);
    view.select('.axisBase').attr('x', viewport.chartWidthPx);
    view.selectAll('.cpuLabel')
        .attr(
            'x',
            (tick) => {
              let offset = 5;
              if ((tick as number) % 1 !== 0) {
                if ((tick as number) % 1 === 0.5) {
                  offset = 25;
                } else {
                  offset = 60;
                }
              }
              return offset + viewport.chartWidthPx;
            })
        .attr('y', tick => this.scaledTickYPx(tick as number, viewport));
  }
}
