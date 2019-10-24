#!/usr/bin/env bash
#
# Copyright 2019 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS-IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
#


PROG=$0
USAGE="Usage: ${PROG} -out 'Path to directory to save trace in' \
-capture_seconds 'Number of seconds to record a trace' \
[-buffer_size 'Size of the trace buffer in KB. Default 4096'] \
[-copy_timeout 'Time to wait for copying to finish. Default 5.']"

declare -a events=("sched:sched_switch" "sched:sched_wakeup" "sched:sched_wakeup_new" "sched:sched_migrate_task")

if [[ "$(whoami)" != "root" ]]; then
  echo "The trace collector must be run as root in order to access TraceFS"
  exit 1
fi

# Parse arguments
while [[ "$#" -gt 0 ]]; do case $1 in
  -o|-out) OUT="$2"; shift;;
  -cs|-capture_seconds) capture_seconds="$2"; shift;;
  -bs|-buffer_size) BUFSIZEK="$2"; shift;;
  -ct|-copy_timeout) COPYTIME="$2"; shift;;
  -h|-help) echo "$USAGE"; exit 0;;
  *) echo "Unknown parameter passed: $1"; echo ${USAGE}; exit 1;;
esac; shift; done

if [[ -z "${COPYTIME}" ]]; then
  COPYTIME=5
fi
if [[ -z "${BUFSIZEK}" ]]; then
  BUFSIZEK=4096
fi

if [[ "$((${capture_seconds} + 0))" != "${capture_seconds}" ]]; then
  echo "capture_seconds must be a number"
  exit 1
fi

if [[ -z "${OUT}"  ]]; then
  echo "out is required"
  exit 1
fi

TMP="${OUT}/tmp"

readonly date="$(date +%Y-%m-%d--%H:%M)"

pids=()

kill_spawned() {
  for pid in "${pids[@]}"
  do
    kill -9 "${pid}" 2>/dev/null
  done
}

kill_trap() {
  exit_status=$?
  kill_spawned
  rm -rf "${TMP}"
  exit "${exit_status}"
}

trap kill_trap SIGINT SIGTERM EXIT

trace_set() {
  local on=/sys/kernel/debug/tracing/tracing_on
  [[ -f "${on}" ]] && echo "$1" >"${on}"
}

trace_set 0
echo "Trace date ${date}: capture for ${capture_seconds} seconds, send output to ${OUT}"

echo nop > /sys/kernel/debug/tracing/current_tracer
echo ${BUFSIZEK} > /sys/kernel/debug/tracing/buffer_size_kb
echo > /sys/kernel/debug/tracing/set_event
for event in "${events[@]}"
do
  echo ${event} >> /sys/kernel/debug/tracing/set_event
done

trace_set 1
echo "Trace capture started at $(date)"
echo "Waiting ${capture_seconds} seconds"
sleep "${capture_seconds}"
trace_set 0

# Make the output directories if needed, and clear it out.
[[ ! -d "${OUT}" ]] && mkdir "${OUT}"
mkdir "${TMP}"
mkdir "${TMP}/traces"
mkdir "${TMP}/topology"

# Save the metadata file
echo "trace_type: FTRACE" > "${TMP}/metadata.textproto"

# Save the event formats
for event in "${events[@]}"
do
  event_format_path=`echo ${event} | sed -r 's/:/\//'`
  mkdir -p "${TMP}/formats/${event_format_path}"
  cat "/sys/kernel/debug/tracing/events/${event_format_path}/format" >> "${TMP}/formats/${event_format_path}/format"
done

# Save the topology
find /sys/devices/system/node -regex "/sys/devices/system/node/node[0-9]+/cpu[0-9]+" -print0 | xargs -0 -I{} cp -RL -f --parents {}/topology .
mv sys/devices/system/node/* "${TMP}/topology"
rm -rf sys/

cat "/sys/kernel/debug/tracing/events/header_page" > "${TMP}/formats/header_page"

for cf in /sys/kernel/debug/tracing/per_cpu/cpu*
do
  cpuname="${cf##*/}"
  echo "Copying ${cf}/trace_pipe_raw to ${TMP}/traces/${cpuname}"
  touch "${TMP}/traces/${cpuname}"
  cat "${cf}/trace_pipe_raw" > "${TMP}/traces/${cpuname}" &
  pids+=($!)
  disown
done

# Sleep to give enough time for cats to copy
echo "Waiting ${COPYTIME} seconds for copies to complete"
sleep "${COPYTIME}"

kill_spawned

echo "Creating tar file"
rm -f "${OUT}/trace.tar.gz"
chmod -R a+rwX "${TMP}"
pushd ${OUT} > /dev/null
ABS_OUT=`pwd`
popd > /dev/null
pushd "${TMP}" > /dev/null
tar -zcf "${ABS_OUT}/trace.tar.gz" *
popd > /dev/null
rm -rf "${TMP}"
chmod a+rw "${OUT}/trace.tar.gz"

echo "Trace capture finished at $(date)"
