#!/bin/bash

PROG=$0
USAGE=$"Usage: ${PROG} -out 'Path to directory to save trace in' \
[-trace_lines 'Number of trace lines to record.  Trace duration is only loosely
correlated with recorded lines, and will vary based on system topology and
workload.  Default 1000000'] \
[-script 'Path to trace script']\n
Must run as root."

if [[ "$EUID" -ne 0 ]]
  then echo "Run ${0} as root."; exit 1
fi

# Parse arguments
while [[ "$#" -gt 0 ]]; do case $1 in
  -o|-out) OUT="$2"; shift;;
  -l|-trace_lines) TRACELINES="$2"; shift;;
  -s|-script) LOCAL_PATH_TO_SCRIPT="$2"; shift;;
  -h|-help) echo "$USAGE"; exit 0;;
  *) echo "Unknown parameter passed: $1"; echo "$USAGE"; exit 1;;
esac; shift; done

if [[ -z "${OUT}"  ]]; then
  echo "Missing required argument -out"
  echo
  echo "$USAGE"
  exit 1
fi

if [[ -z "${TRACELINES}" ]]; then
  TRACELINES=1000000
fi

if [[ -z "${LOCAL_PATH_TO_SCRIPT}" ]]; then
  LOCAL_PATH_TO_SCRIPT=`dirname "${PROG}"`/sched.bt
fi

TMP="${OUT}/tmp"

kill_trap() {
  exit_status=$?
  rm -rf "${TMP}"
  exit "${exit_status}"
}

trap kill_trap SIGINT SIGTERM EXIT

# Make the output directories if needed
[[ ! -d "${OUT}" ]] && mkdir "${OUT}"
mkdir "${TMP}"
mkdir "${TMP}/topology"

# Save the metadata file
echo "trace_type: EBPF" > "${TMP}/metadata.textproto"

# Save the topology
find /sys/devices/system/node -regex "/sys/devices/system/node/node[0-9]+/cpu[0-9]+" -print0 | xargs -0 -I{} cp -RL -f --parents {}/topology .
mv sys/devices/system/node/* "${TMP}/topology"
rm -rf sys/

echo "Starting tracing..."
bpftrace ${LOCAL_PATH_TO_SCRIPT} |
  head -${TRACELINES} |
  awk -F: '{val="0x" $2; print strtonum(val),$0 ;}' /dev/stdin |
  sort -n |
  sed 's/^[^ ]* //' > "${TMP}/ebpf_trace"

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

echo "Tracing done."
