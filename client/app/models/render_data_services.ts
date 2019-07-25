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
/**
 * A request for CPU intervals for the specified collection.
 */
export declare interface CpuIntervalsRequest {
  collectionName: string;
  // The CPUs to request intervals for.  If empty, all CPUs are selected.
  cpus: number[];
  // Designates a minimum interval duration.  Adjacent intervals smaller than
  // this duration may be merged together, retaining waiting PID count data but
  // possibly losing running thread data; merged intervals are truncated as soon
  // as they meet or exceed the specified duration.  Intervals smaller than this
  // may still appear in the output, if they could not be merged with neighbors.
  // If 0, no merging is performed.
  minIntervalDurationNs?: number;
  // The time span over which to request CPU intervals, specified in
  // nanoseconds.  If start_timestamp_ns is -1, the time span will
  // begin at the first valid collection timestamp.  If end_timestamp_ns is -1,
  // the time span will end at the last valid collection timestamp.
  startTimestampNs?: number;
  endTimestampNs?: number;
}

/**
 * CPUInterval contains information about a what was running and waiting on a
 * CPU during an interval
 */
export declare interface CpuInterval {
  cpu: number;
  // If running_pid is not populated, the running thread is unknown or not
  // provided, and the running_command and running_priority are not meaningful.
  runningPid: number;
  runningCommand: string;
  runningPriority: number;
  // The set of PIDs that waited during this interval.  If intervals were split
  // on a change in waiting PIDs, this is the set of PIDs that waited throughout
  // the entire interval; otherwise it's the set of PIDs that waited for any
  // time during this interval.
  waitingPids: number[];
  // The average number of known waiting PIDs.
  waitingPidCount: number;
  startTimestampNs: number;
  endTimestampNs: number;
  // How many CpuIntervals were merged to form this one.
  mergedIntervalCount: number;
  // The amount of time, over this interval, that the CPU was idle.  A CPU is
  // considered idle when it is running PID 0, the swapper.  If this interval is
  // not merged, idle_ns will either be its entire duration (if the running PID
  // was 0) or 0 (if the running PID was not 0).
  idleNs: number;
  // TODO(ilhamster) Consider whether we should also include switch-out state
  // (sleeping or round-robin) of the running PID here.  Note that this can be
  // gathered from PID intervals, though.
}

/**
 * CPUIntervals is a tuple holding a CPU ID and its intervals
 */
export declare interface CpuIntervals {
  cpu: number;
  intervals: CpuInterval[];
}

/**
 * A response for a CPU intervals request.  If no matching collection
 * was found, cpu_intervals is empty.
 */
export declare interface CpuIntervalsResponse {
  collectionName: string;
  cpuIntervals: CpuIntervals[];
}

/**
 * A request for a view of ftrace events in the collection.
 */
export declare interface FtraceEventsRequest {
  // The name of the collection.
  collectionName: string;
  // The CPUs to request intervals for.  If empty, all CPUs are selected.
  cpus: number[];
  // The time span (in nanoseconds) over which to request ftrace events.
  startTimestamp: number;
  endTimestamp: number;
  // The event type names to fetch.  If empty, no events are returned.
  eventTypes: string[];
}

/**
 *  A response for a ftrace events request.
 */
export declare interface FtraceEventsResponse {
  // The name of the collection.
  collectionName: string;
  // A map from CPU to lists of events that occurred on that CPU.
  eventsByCpu: {[k: number]: FtraceEvent[]};
}

/**
 * FtraceEvent describes a single trace event.
 * A Collection stores its constituent events in a much more compact, but less
 * usable, format than this, so it is recommended to generate Events on demand
 * (via Collection::EventByIndex) rather than persisting more than a few Events.
 */
export declare interface FtraceEvent {
  // An index uniquely identifying this Event within its Collection.
  index: number;
  // The name of the event's type.
  name: string;
  // The CPU that logged the event.  Note that the CPU that logs an event may be
  // otherwise unrelated to the event.
  cpu: number;
  // The event timestamp.
  timestamp: number;
  // True if this Event fell outside of the known-valid range of a trace which
  // experienced buffer overruns.  Some kinds of traces are only valid for
  // unclipped events.
  clipped: boolean;
  // A map of text properties, indexed by name.
  textProperties: {[k: string]: string};
  // A map of numeric properties, indexed by name.
  numberProperties: {[k: string]: number};
}

/**
 * A request for PID intervals for the specified collection and PIDs.
 */
export declare interface PidIntervalsRequest {
  // The name of the collection to look up intervals in
  collectionName: string;
  // The PIDs to request intervals for
  pids: number[];
  // The time span over which to request PID intervals, specified in
  // nanoseconds.  If start_timestamp_ns is -1, the time span will
  // begin at the first valid collection timestamp.  If end_timestamp_ns is -1,
  // the time span will end at the last valid collection timestamp.
  startTimestampNs: number;
  endTimestampNs: number;
  // Designates a minimum interval duration.  Adjacent intervals on the same CPU
  // smaller than this duration may be merged together, losing state and
  // post-wakeup status; merged intervals are truncated as soon as they meet or
  // exceed the specified duration.  Intervals smaller than this may still
  // appear in the output, if they could not be merged with neighbors.  If 0, no
  // merging is performed.
  minIntervalDurationNs: number;
}


/**
 * ThreadState is an enum describing the state of a thread
 */
export enum ThreadState {
  // Unknown thread state
  UNKNOWN_STATE = 0,
  // Scheduled and switched-in on a CPU.
  RUNNING_STATE = 1,
  // Not running but runnable.
  WAITING_STATE = 2,
  // Neither running nor on the run queue.
  SLEEPING_STATE = 3,
}

/**
 * PidInterval describes a maximal interval over a PID's lifetime during which
 * its command, priority, state, and CPU remain unchanged.
 */
export declare interface PidInterval {
  pid: number;
  command: string;
  priority: number;
  // If this PidInterval is the result of merging several intervals, state will
  // be set to UNKNOWN.  This can be distinguished from actually unknown state
  // by checking merged_interval_count; if it is == 1, the thread's state is
  // actually unknown over the interval; if it is > 1, the thread had several
  // states over the merged interval.
  state: ThreadState;
  // If state is WAITING, post_wakeup determines if the thread started waiting
  // as the result of a wakeup (true) or as a result of round-robin descheduling
  // (false).  post_wakeup is always false for merged intervals.
  postWakeup: boolean;
  cpu: number;
  startTimestampNs: number;
  endTimestampNs: number;
  // How many PidIntervals were merged to form this one.
  mergedIntervalCount: number;
}

/**
 * PIDIntervals is a tuple holding a PID and its intervals
 */
export declare interface PidIntervals {
  // The PID that these intervals correspond to
  pid: number;
  // A list of PID intervals
  intervals: PidInterval[];
}

/**
 * A response for a PID intervals request. If no matching collection was found,
 * pid_intervals is empty.
 */
export declare interface PidIntervalsResponse {
  // The name of the collection
  collectionName: string;
  // A list of PID intervals
  pidIntervals: PidIntervals[];
}
