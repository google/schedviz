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
USAGE="Usage: ${PROG} -instance 'GCE Instance Name' \
-trace_args 'Arguments to forward to trace script' \
[-project 'GCP Project Name'] \
[-zone 'GCP Project Zone'] \
[-script 'Path to trace script']"

# Parse arguments
while [[ "$#" -gt 0 ]]; do case $1 in
  -i|-instance) GCE_INSTANCE="$2"; shift;;
  -p|-project) GCP_PROJECT_NAME="$2"; shift;;
  -z|-zone) GCP_ZONE="$2"; shift;;
  -s|-script) LOCAL_PATH_TO_SCRIPT="$2"; shift;;
  -ta|-trace_args) TRACE_ARGS="$2"; shift;;
  -h|-help) echo "$USAGE"; exit 0;;
  *) echo "Unknown parameter passed: $1"; echo ${USAGE}; exit 1;;
esac; shift; done

if [[ -z "${GCE_INSTANCE}" ]]; then
  echo "GCE Instance Name is required."
  echo ${USAGE}
  exit 1
fi

if [[ -z "${TRACE_ARGS}" ]]; then
  echo "Trace Arguments are required."
  echo ${USAGE}
  exit 1
fi

if [[ -z "${LOCAL_PATH_TO_SCRIPT}" ]]; then
  LOCAL_PATH_TO_SCRIPT=`dirname "${PROG}"`/trace.sh
fi

if [[ ! -z "${GCP_PROJECT_NAME}" ]]; then
  OLD_PROJECT=$(gcloud config get-value project 2>/dev/null)
  gcloud config set project ${GCP_PROJECT_NAME}
fi

reset_project () {
  if [[ ! -z "${OLD_PROJECT+x}" ]]; then
    # If empty string, unset the config setting
    if [[ -z "${OLD_PROJECT}" ]]; then
      gcloud config unset project 2>/dev/null
    else
      gcloud config set project ${OLD_PROJECT} 2>/dev/null
    fi
  fi
}

kill_trap() {
  exit_status=$?
  reset_project
  exit "${exit_status}"
}

trap kill_trap SIGINT SIGTERM EXIT

if [[ ! -z "${GCP_ZONE}" ]]; then
  GCP_ZONE_CMD="--zone ${GCP_ZONE}"
fi

# Copy the script to the remote machine
echo "Uploading trace script to ${GCE_INSTANCE}"
gcloud compute scp ${GCP_ZONE_CMD} ${LOCAL_PATH_TO_SCRIPT} ${GCE_INSTANCE}:/tmp/trace.sh
# Make the script executable
gcloud compute ssh ${GCP_ZONE_CMD} ${GCE_INSTANCE} --command="chmod +x /tmp/trace.sh"
# Collect a trace on the remote machine
echo "Collecting trace on ${GCE_INSTANCE}"
gcloud compute ssh ${GCP_ZONE_CMD} ${GCE_INSTANCE} --command="sudo /tmp/trace.sh ${TRACE_ARGS} -out /tmp/trace/"
# Download recorded trace to the current directory
echo "Downloading recorded trace from ${GCE_INSTANCE}"
gcloud compute scp ${GCP_ZONE_CMD} ${GCE_INSTANCE}:/tmp/trace/trace.tar.gz trace.tar.gz
# Cleanup
echo "Cleaning up trace files on ${GCE_INSTANCE}"
gcloud compute ssh ${GCP_ZONE_CMD} ${GCE_INSTANCE} --command="sudo rm -rf /tmp/trace.sh /tmp/trace/"

reset_project
