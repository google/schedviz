#!/bin/bash

PROG=$0
USAGE=$"Usage: ${PROG} -out 'Path to trace output file' \
[-trace_lines 'Number of trace lines to record.  Trace duration is only loosely
correlated with recorded lines, and will vary based on system topology and
workload.  Default 1000000']\n
Must run as root."

# Parse arguments
while [[ "$#" -gt 0 ]]; do case $1 in
  -o|-out) OUTFILE="$2"; shift;;
  -l|-trace_lines) TRACELINES="$2"; shift;;
  -h|-help) echo "$USAGE"; exit 0;;
  *) echo "Unknown parameter passed: $1"; echo "$USAGE"; exit 1;;
esac; shift; done

if [[ -z "${OUTFILE}"  ]]; then
  echo "Missing required argument -out"
  echo
  echo "$USAGE"
  exit 1
fi

if [ "$EUID" -ne 0 ]
  then echo "Run ${0} as root."; exit 1
fi


if [[ -z "${TRACELINES}" ]]; then
  TRACELINES=1000000
fi

echo "Starting tracing..."
bpftrace ./sched.bt |
  head -${TRACELINES} |
  awk -F: '{val="0x" $2; print strtonum(val),$0 ;}' /dev/stdin |
  sort -n |
  sed 's/^[^ ]* //' > '${OUTFILE}'
echo "Tracing done."
