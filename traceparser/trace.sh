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
#!/bin/bash

readonly capture_seconds="$1"
readonly OUT="$2"
COPYTIME=${3:-5}
BUFSIZEK=${4:-4096}

declare -a events=("sched:sched_switch" "sched:sched_wakeup" "sched:sched_wakeup_new" "sched:sched_migrate_task")

if [[ "$(whoami)" != "root" ]]; then
  echo "The trace collector must be run as root in order to access TraceFS"
  exit 1
fi

if [[ "$(($1 + 0))" != "$1" || -z "$OUT" ]]; then
  echo "Usage: $0 <seconds to capture> <path to output files> <copy timeout (sec)>? <buffer size>?"
  exit 1
fi
readonly date="$(date +%Y-%m-%d--%H:%M)"


trace_set() {
  local on=/sys/kernel/debug/tracing/tracing_on
  [[ -f "${on}" ]] && echo "$1" >"${on}"
}

trace_set 0
echo "I am pid $$"
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
rm -rf ${OUT}/*
TMP="${OUT}/tmp"
mkdir "${TMP}"
mkdir "${TMP}/traces"
mkdir "${TMP}/topology"

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

pids=()
for cf in /sys/kernel/debug/tracing/per_cpu/cpu*
do
  cpuname="${cf##*/}"
  echo "Copying ${TMP}/traces/${cpuname}"
  touch "${TMP}/traces/${cpuname}"
  # TODO(tracked) Use a different utility than cat to copy the trace.
  cat "${cf}/trace_pipe_raw" > "${TMP}/traces/${cpuname}" &
  pids+=($!)
  disown
done

# Sleep to give enough time for cats to copy
echo "Waiting ${COPYTIME} seconds for copies to complete"
sleep "${COPYTIME}"

for pid in "${pids[@]}"
do
  kill -9 "${pid}"
done

echo "Creating tar file"
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
