# Linux Scheduling Visualization

![Build and Test SchedViz](https://github.com/google/schedviz/workflows/Build%20and%20Test%20SchedViz/badge.svg)

This is not an officially supported Google product.

SchedViz is a tool for gathering and visualizing kernel scheduling traces on
Linux machines. It helps to:

*   quantify task starvation due to round-robin queueing,
*   identify primary antagonists stealing work from critical threads,
*   determine when core allocation choices yield unnecessary waiting,
*   evaluate different scheduling policies,
*   and much more.

To learn more about what SchedViz can do and what problems it can solve, read
our [announcement post](https://opensource.googleblog.com/2019/10/understanding-scheduling-behavior-with.html)
on the Google open source blog.

## Running SchedViz

To get started clone this repo:

```bash
git clone https://github.com/google/schedviz.git
```

SchedViz requires *yarn*. Closely follow the installation instructions
on the [yarn Website](https://www.yarnpkg.com). Make sure you have a recent
version of Node.js (>= 10.9.0) installed as well.

Building SchedViz also requires the GNU build tools and the unzip utility. On Debian, for
example, the dependencies can be installed by executing this command:

```bash
sudo apt-get update && sudo apt-get install build-essential unzip
```

To run SchedViz, run the following commands:

```bash
cd schedviz # The location where the repo was cloned
yarn install
```

Once yarn has finished run the following command in the root of the repo to
start the server:

```bash
yarn bazel run server -- -- -storage_path="Path to a folder to store traces in"
```

The server binary takes several options:

| Name           | Type   | Description                                                                                                                                                                           |
| -------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| storage_path   | String | Required.<br>The folder where trace data is/will be stored.<br>This should be an empty folder that is not used by anything else.                                                                                                                           |
| cache_size     | Int    | Optional.<br>The maximum number of collections to keep open in memory at once.                                                                                                        |
| port           | Int    | Optional.<br>The port to run the server on.<br>Defaults to 7402.                                                                                                                      |
| resources_root | String | Optional.<br>The folder where the static files (e.g. HTML and JavaScript) are stored.<br>Default is "client".<br>If using bazel to run the server, you shouldn't need to change this. |

To load SchedViz, go to http://localhost:7402/collections

## Manually collecting a scheduling trace

1.  Run the [trace.sh](util/trace.sh) as root on the machine that you
    want to collect a trace from:

    ```bash
    sudo ./trace.sh -out 'Path to directory to save trace in' \
                    -capture_seconds 'Number of seconds to record a trace' \
                    [-buffer_size 'Size of the trace buffer in KB'] \
                    [-copy_timeout 'Time to wait for copying to finish']
    ```

    Default number of seconds to wait for copy to finish is `5`.
    The copying of the raw traces from Ftrace to the output file won't finish
    automatically as the raw trace pipes aren't closed. This setting is a
    timeout for when the copying should stop. It should be at least as long as
    it takes to copy out all the trace files. If you see incomplete traces,
    try increasing the timeout.

    Default buffer size is `4096 KB`.

    The shell script collects the `sched_switch`, `sched_wakeup`,
    `sched_wakeup_new`, and `sched_migrate_task` tracepoints.

    > NOTE: There is also a binary version of the trace collector script, which
      can collect traces larger than the size of the buffer.
    >
    > To build it, run `bazel build util:trace` from the root of the repo.
    >
    > To run it, run `sudo bazel-bin/util/trace`. It takes the same arguments
      as the shell script, except -copy_timeout.

2.  Copy the generated tar.gz file off of the trace machine to a machine that
    can access the SchedViz UI.

3.  Upload the output tar.gz file to SchedViz by clicking the "Upload Trace"
    button on the SchedViz collections page. The tar.gz file will be located at
    `$OUTPUT_PATH/trace.tar.gz`.

## Collecting a scheduling trace on a GCE machine

Using [gcloud](https://cloud.google.com/sdk/gcloud/) you can easily collect a
trace on a remote GCE machine. We've written a
[helper script](util/gcloud_trace.sh) to collect a trace on a GCE machine
with a single command. This script is a wrapper around the manual
[trace script](util/trace.sh).

Usage:
```bash
./gcloud_trace.sh -instance 'GCE Instance Name' \
-trace_args 'Arguments to forward to trace script' \
[-project 'GCP Project Name'] \
[-zone 'GCP Project Zone'] \
[-script 'Path to trace script']
```

## Analyzing a trace

Take a look at our [features and usage walkthrough](doc/walkthrough.md).

## Keyboard Shortcuts

| Key                  | Description                                          |
| -------------------- | ---------------------------------------------------- |
| `?` (`Shift` + `/`)  | Show a shortcut cheatsheet dialog                    |
| `Shift` + `c`        | Copy the current tooltip text to the clipboard       |
| `Shift` + `a`        | Reset zoom level in heatmap viewport                 |
| `Shift` + `x`        | Clear the CPU filter                                 |

## Common sources of errors

### Errors collecting traces

* When the trace takes longer to copy than the timeout passed to the trace
  collection script (default timeout is five seconds), traces will not be
  fully copied into the output file. To fix this, increase the copy timeout
  parameter to a larger and sufficient value.

* If a CPU's trace buffer fills up before the timeout is reached, recording
  will be stopped for that CPU. Other CPUs may keep recording until their
  buffers fill up, but only the events occurring up to the last event in the
  first buffer to fill up will be used.

### Errors loading traces

SchedViz infers CPU and thread state information that isn't directly attested
in the trace, and fails aggressively when this inference does not succeed.
In particular, two factors may increase the likelihood of inference failures:
* PID reuse. Machines with small PID address spaces, or long traces,
  may experience PID reuse, where two separate threads are assigned the same
  PID. This can lead to inference failures, for example, threads last seen on
  one CPU could appear on another without an intervening migration.
* Event ordering. Scheduling trace interpretation relies on an accurate
  total ordering between the scheduling events; event timestamps are
  generated by their emitting CPU's rdtsc. If the cores of a machine do
  not have their rdtscs tightly aligned, events may appear to slip, which
  can lead to inference errors as above.

  Events can also have incorrect timestamps written if the kernel is
  interrupted while it is recording an event, and the interrupt tries to
  write another event. This will result in many events appearing having the
  same timestamp when they shouldn't. This type of error occurs when recording
  high-frequency events such as the workqueue family, and it very rarely occurs
  when recording only scheduling events.
