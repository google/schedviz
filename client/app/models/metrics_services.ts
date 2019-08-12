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
import {FtraceEvent, Thread} from './render_data_services';

/**
 * A request for thread summary information across a specified timespan for a
 * specified collection, filtered to the requested CPU set.  If
 * start_timestamp_ns is -1, the first timestamp in the collection is used
 * instead.  If end_timestamp_ns is -1, the last timestamp in the collection is
 * used instead.  If the provided CPU set is empty, all CPUs are filtered in.
 */
export declare interface ThreadSummariesRequest {
  collectionName: string;
  startTimestampNs: number;
  endTimestampNs: number;
  cpus: number[];
}


/**
 * Metrics holds a set of aggregated metrics for some or all of the sched trace.
 */
export declare interface Metrics {
  // The number of migrations observed in the aggregated trace.  If CPU
  // filtering was used generating this Metric, only migrations inbound to a
  // filtered-in CPU are aggregated.
  migrationCount: number;
  // The number of wakeups observed in the aggregated trace.
  wakeupCount: number;
  // Aggregated thread-state times over the aggregated trace.
  unknownTimeNs: number;
  runTimeNs: number;
  waitTimeNs: number;
  sleepTimeNs: number;
  // Unique PIDs, COMMs, priorities, and CPUs observed in the aggregated trace.
  // Note that these fields are not correlated; if portions of trace containing
  // execution from several different PIDs are aggregated together in a metric,
  // all of their PIDs, commands, and priorities will be present here, and the
  // Metrics can reveal which PIDs were present, but it will not be possible to
  // tell from the Metrics which commands go with which PIDs, and so forth.
  // TODO(sabarabc) Create maps from PID -> ([]command, []priority),
  //  command -> ([]PID, []priority), and priority -> ([]PID, []command)
  //  so that we can tell which of these are correlated.
  pids: number[];
  commands: string[];
  priorities: number[];
  cpus: number[];
  // The time range over which these metrics were aggregated.
  startTimestampNs: number;
  endTimestampNs: number;
}

/**
 * ThreadSummariesResponse contains the response to a ThreadSummariesRequest.
 */
export declare interface ThreadSummariesResponse {
  collectionName: string;
  metrics: Metrics[];
}

/**
 * A request for antagonist information for a specified set of threads, across a
 * specified timestamp for a specified collection.  If start_timestamp_ns is -1,
 * the first timestamp in the collection is used instead.  If end_timestamp_ns
 * is -1, the last timestamp in the collection is used instead.
 */
export declare interface AntagonistsRequest {
  // The collection name.
  collectionName: string;
  pids: number[];
  startTimestampNs: number;
  endTimestampNs: number;
}


/**
 * An antagonism is an interval during which a single thread running on a
 * single CPU antagonized the waiting victim.
 */
declare interface Antagonism {
  runningThread: Thread;
  cpu: number;
  startTimestamp: number;
  endTimestamp: number;
}

declare interface Antagonists {
  victims: Thread[];
  antagonisms: Antagonism[];
  // The time range over which these antagonists were gathered.
  startTimestamp: number;
  endTimestamp: number;
}

/**
 * A response for an antagonist request.
 */
export declare interface AntagonistsResponse {
  collectionName: string;
  // All matching stalls sorted in order of decreasing duration - longest first.
  antagonists: Antagonists[];
}

/**
 * A request for all events on the specified threads across a specified
 * timestamp for a specified collection.  If start_timestamp_ns is -1, the first
 * timestamp in the collection is used instead.  If end_timestamp is -1, the
 * last timestamp in the collection is used instead.
 */
export declare interface PerThreadEventSeriesRequest {
  // The collection name.
  collectionName: string;
  pids: number[];
  startTimestampNs: number;
  endTimestampNs: number;
}

/**
 * PerThreadEventSeries is a tuple containing a PID and its events.
 */
export declare interface PerThreadEventSeries {
  pid: number;
  events: FtraceEvent[];
}

/**
 * A response for a per-thread event sequence request.  The Events are unique
 * and are provided in increasing temporal order.
 */
export declare interface PerThreadEventSeriesResponse {
  // The PCC collection name.
  collectionName: string;
  eventSeries: PerThreadEventSeries[];
}

/**
 * A request for the amount of time, in the specified collection over the
 * specified interval and CPU set, that some of the CPUs were idle while others
 * were overloaded.
 */
export declare interface UtilizationMetricsRequest {
  collectionName: string;
  cpus: number[];
  startTimestampNs: number;
  endTimestampNs: number;
}

/**
 * UtilizationMetrics contains various stats relating to the utilization of
 * CPUs.
 */
export declare interface UtilizationMetrics {
  // The wall time during which at least one CPU was idle while at least one
  // other CPU was overloaded.
  wallTime: number;
  // The aggregated time that a single CPU was idle while another CPU was
  // overloaded.  For example, if two CPUs were idle for 1s, and two other CPUs
  // overloaded during that same 1s, that's 1s of wall time but 2s of per-CPU
  // time.
  perCpuTime: number;
  // The aggregated time that a single CPU was idle while another thread waited
  // on some other, overloaded CPU.
  // For example, if two CPUs were overloaded for one second, one with one
  // waiting thread and the other with two waiting threads, and four other CPUs
  // were idle for that same second, the Wall Time for that interval would be
  // one second (At least one CPU was idle while another was overloaded for the
  // entire second); the Per-CPU Time would be two seconds (two CPUs were
  // overloaded while at least two others were idle); and the Per-Thread Time
  // would be three seconds (three threads were waiting while at least three
  // CPUs were idle.) If, however, only two CPUs were idle during that second,
  // Per-CPU Time would remain the same while Per-Thread Time would only be two
  // seconds, because while three threads were waiting over that second, only
  // two CPUs were idle.
  perThreadTime: number;
  // The CPU utilization over the requested interval and set of CPUs.  CPU
  // utilization is the proportion (in the range [0,1]) of total CPU-time spent
  // not idle.  For example, a UtilizationMetricsRequest for .5s over 4 CPUs
  // would return a CPU utilization of .5 if two of those CPUs lay idle for .5s
  // each; .75 if two of those CPUs lay idle for .25s each, or one was idle for
  // .5s; and so forth.
  cpuUtilizationFraction: number;
}

/**
 * A response for an idle-while-overloaded request.
 */
export declare interface UtilizationMetricsResponse {
  request: UtilizationMetricsRequest;
  utilizationMetrics: UtilizationMetrics;
}

