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
  /**
   * The CPUs to request intervals for.  If empty, all CPUs are selected.
   */
  cpus: number[];
  /**
   * Designates a minimum interval duration.  Adjacent intervals smaller than
   * this duration may be merged together, retaining waiting PID count data but
   * possibly losing running thread data; merged intervals are truncated as soon
   * as they meet or exceed the specified duration.  Intervals smaller than this
   * may still appear in the output, if they could not be merged with neighbors.
   * If 0, no merging is performed.
   */
  minIntervalDurationNs?: number;
  /**
   * The time span over which to request CPU intervals, specified in
   * nanoseconds.  If start_timestamp_ns is -1, the time span will
   * begin at the first valid collection timestamp.  If end_timestamp_ns is -1,
   * the time span will end at the last valid collection timestamp.
   */
  startTimestampNs?: number;
  endTimestampNs?: number;
}

/**
 * Thread describes a single thread's PID, command string, and priority.
 */
export declare interface Thread {
  pid: number;
  command: string;
  priority: number;
}

/**
 * ThreadResidency describes a duration of time a thread held a state on a CPU.
 */
export declare interface ThreadResidency {
  thread: Thread;
  /**
   * The duration of the residency in ns. If StartTimestamp is Unknown, reflects
   * a cumulative duration.
   */
  duration: number;
  state: ThreadState;
  droppedEventIDs: number[];
  /**
   * Set to true if this was constructed from at least one synthetic transitions
   * i.e. a transition that was not in the raw event set.
   */
  includesSyntheticTransitions: boolean;
}

/**
 * Interval contains information about a what was running and waiting on a
 * CPU during an interval
 */
export declare interface Interval {
  /**
   * The start time of this interval in nanoseconds from the beginning of the
   * collection.
   */
  startTimestamp: number;
  /**
   * How long the interval lasted for in nanoseconds.
   */
  duration: number;
  cpu: number;
  threadResidencies: ThreadResidency[];
  mergedIntervalCount: number;
}

/**
 * CPUIntervals is a tuple holding a CPU ID and its intervals
 */
export declare interface CpuIntervals {
  cpu: number;
  running: Interval[];
  waiting: Interval[];
}

/**
 * A response for a CPU intervals request.  If no matching collection
 * was found, cpu_intervals is empty.
 */
export declare interface CpuIntervalsResponse {
  collectionName: string;
  intervals: CpuIntervals[];
}

/**
 * A request for a view of ftrace events in the collection.
 */
export declare interface FtraceEventsRequest {
  /**
   * The name of the collection.
   */
  collectionName: string;
  /**
   * The CPUs to request intervals for.  If empty, all CPUs are selected.
   */
  cpus: number[];
  /**
   * The time span (in nanoseconds) over which to request ftrace events.
   */
  startTimestamp: number;
  endTimestamp: number;
  /**
   * The event type names to fetch.  If empty, no events are returned.
   */
  eventTypes: string[];
}

/**
 *  A response for a ftrace events request.
 */
export declare interface FtraceEventsResponse {
  /**
   * The name of the collection.
   */
  collectionName: string;
  /**
   * A map from CPU to lists of events that occurred on that CPU.
   */
  eventsByCpu: {[k: number]: FtraceEvent[]};
}

/**
 * FtraceEvent describes a single trace event.
 * A Collection stores its constituent events in a much more compact, but less
 * usable, format than this, so it is recommended to generate Events on demand
 * (via Collection::EventByIndex) rather than persisting more than a few Events.
 */
export declare interface FtraceEvent {
  /**
   * An index uniquely identifying this Event within its Collection.
   */
  index: number;
  /**
   * The name of the event's type.
   */
  name: string;
  /**
   * The CPU that logged the event.  Note that the CPU that logs an event may be
   * otherwise unrelated to the event.
   */
  cpu: number;
  /**
   * The event timestamp.
   */
  timestamp: number;
  /**
   * True if this Event fell outside of the known-valid range of a trace which
   * experienced buffer overruns.  Some kinds of traces are only valid for
   * unclipped events.
   */
  clipped: boolean;
  /**
   * A map of text properties, indexed by name.
   */
  textProperties: {[k: string]: string};
  /**
   * A map of numeric properties, indexed by name.
   */
  numberProperties: {[k: string]: number};
}

/**
 * A request for PID intervals for the specified collection and PIDs.
 */
export declare interface PidIntervalsRequest {
  /**
   * The name of the collection to look up intervals in
   */
  collectionName: string;
  /**
   * The PIDs to request intervals for
   */
  pids: number[];
  /**
   * The time span over which to request PID intervals, specified in
   * nanoseconds.  If start_timestamp_ns is -1, the time span will
   * begin at the first valid collection timestamp.  If end_timestamp_ns is -1,
   * the time span will end at the last valid collection timestamp.
   */
  startTimestampNs: number;
  endTimestampNs: number;
  /**
   * Designates a minimum interval duration.  Adjacent intervals on the same CPU
   * smaller than this duration may be merged together, losing state and
   * post-wakeup status; merged intervals are truncated as soon as they meet or
   * exceed the specified duration.  Intervals smaller than this may still
   * appear in the output, if they could not be merged with neighbors.  If 0, no
   * merging is performed.
   */
  minIntervalDurationNs: number;
}


/**
 * ThreadState is an enum describing the state of a thread
 */
export enum ThreadState {
  /**
   * Unknown thread state
   */
  UNKNOWN_STATE = -1,
  /**
   * Scheduled and switched-in on a CPU.
   */
  RUNNING_STATE = 0,
  /**
   * Not running but runnable.
   */
  WAITING_STATE = 1,
  /**
   * Neither running nor on the run queue.
   */
  SLEEPING_STATE = 2,
}

/**
 * Get the string value of a ThreadState
 */
export function threadStateToString(state: ThreadState) {
  switch (state) {
    case ThreadState.UNKNOWN_STATE:
      return 'Unknown';
    case ThreadState.RUNNING_STATE:
      return 'Running';
    case ThreadState.WAITING_STATE:
      return 'Waiting';
    case ThreadState.SLEEPING_STATE:
      return 'Sleeping';
    default:
      return 'Invalid State';
  }
}

/**
 * PIDIntervals is a tuple holding a PID and its intervals
 */
export declare interface PidIntervals {
  /**
   * The PID that these intervals correspond to
   */
  pid: number;
  /**
   * A list of PID intervals
   */
  intervals: Interval[];
}

/**
 * A response for a PID intervals request. If no matching collection was found,
 * pid_intervals is empty.
 */
export declare interface PidIntervalsResponse {
  /**
   * The name of the collection
   */
  collectionName: string;
  /**
   * A list of PID intervals
   */
  pidIntervals: PidIntervals[];
}
