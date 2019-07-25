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
  pid: number;
  command: string;
  cpu: number;
  startTimestampNs: number;
  endTimestampNs: number;
}

declare interface Antagonists {
  victimPid: number;
  // If there is more than one victim command, the victim PID was reused over
  // the requested interval. The times such reuse occurred can be determined by
  // iterating through per-PID events for the requested duration and looking for
  // where event.command (or prev_command) changes, but it is likely sufficient
  // to flag replies with multiple victim_commands as unreliable.
  // Thread reuse over the scale of a sched collection is very uncommon, except
  // for a few special OS threads (with PID 0).
  victimCommand: string;
  antagonisms: Antagonism[];
  // The time range over which these antagonists were gathered.
  startTimestampNs: number;
  endTimestampNs: number;
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
 * EventType is an enum containing different types of events
 */
export enum EventType {
  UNKNOWN = 0,
  // Regular sched: events.
  MIGRATE_TASK = 1,
  PROCESS_WAIT = 2,
  WAIT_TASK = 3,
  SWITCH = 4,
  WAKEUP = 5,
  WAKEUP_NEW = 6,
  STAT_RUNTIME = 7,
}

/**
 * ThreadState is an enum describing the state of a thread
 */
export enum ThreadState {
  UNKNOWN_STATE = 0,
  // Scheduled and switched-in on a CPU.
  RUNNING_STATE = 1,
  // Not running but runnable.
  WAITING_STATE = 2,
  // Neither running nor on the run queue.
  SLEEPING_STATE = 3,
}

/**
 * Event is a struct containing information from sched ftrace events
 */
export declare interface Event {
  // Fields used by all Events:

  // A unique ID for this event, stable within a sched collection.  Can be used
  // to associate Events gathered by different requests to the same collection:
  // they are the same if they have the same unique_id.  E.g., we  could
  // associate an event on a timeline with an event from a stall if their
  // unique_ids match.
  uniqueID: number;
  eventType: EventType;
  // The timestamp, in nanoseconds, at which the event occurs.  Events are
  // instantaneous (or at least modeled as such).
  timestampNs: number;
  // The primary PID of this event.  For MIGRATE_TASK, PROCESS_WAIT, WAIT_TASK,
  // WAKEUP, and WAKEUP_NEW, it is the affected PID.  For SWITCH, it is the PID
  // active after the switch, 'next_pid'.
  pid: number;
  // The command associated with pid, if any.  Note that this field is likely
  // truncated by the target OS.
  command: string;
  // The primary CPU of the event.  For MIGRATE_TASK, it is the CPU on which the
  // task will be active after the migration, 'dest_cpu'.  For WAKEUP or
  // WAKEUP_NEW, it is the CPU on which the wakeup will occur, 'target_cpu'.
  // The remaining events: PROCESS_WAIT, WAIT_TASK, and SWITCH, do not
  // explicitly record what CPU the event is occurring on; however, this CPU can
  // frequently be inferred from other events on the affected process:
  //  * The 'target_cpu' of WAKEUP or WAKEUP_NEW events, or the 'dest_cpu' of
  //    MIGRATE events, fixes the CPU of the affected process for subsequent
  //    events.
  //  * The 'orig_cpu' of MIGRATE_TASK events fixes the CPU of the affected
  //    process for previous events, if it is not known by other sources.
  //  * If the CPU for one process in a SWITCH is known, but the CPU for the
  //    other process is not known, the first process' CPU fixes the CPU of the
  //    other process for previous events.
  // If the CPU cannot be inferred, an empty Int64Value is returned.
  cpu: number;
  // The priority of the primary PID of this event.  May be inferred; unknown if
  // empty.
  priority: number;
  // The CPU that reported the event.
  reportingCpu: number;
  // The state of the thread referenced by pid just after this event completed.
  state: ThreadState;

  // Fields only used for SWITCH events:

  // The PID active prior to the switch
  prevPid: number;
  // The command associated with prev_pid, if any.  Note that this field is
  // likely truncated by the target OS.
  // BEGIn-INTERNAL
  // 'prev_comm' from ktrace.
  // END-INTERNAL
  prevCommand: string;
  // The priority of prev_pid.  Unused for other events.
  prevPriority: number;
  // The state of prev_pid immediately after the SWITCH.  These values are from
  // the kernel's task state bitmap, for example at
  // https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/include/linux/sched.h?h=v4.3.5#n197rm
  // We assume that, at least, state == 0 iff the task is in the scheduler run
  // queue.
  prevTaskState: number;
  // The state of the prev_pid.
  prevState: ThreadState;

  // Fields only used for MIGRATE_TASK events:

  // The CPU from which the process is migrating
  prevCpu: number;
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
  events: Event[];
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

