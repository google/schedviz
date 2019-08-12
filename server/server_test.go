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
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"

	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"
	"github.com/google/schedviz/testhelpers/testhelpers"

)

const collectionName = "f0d3d241-b27f-4ad0-b5cf-17bb7482674b_1_bob"

var url string
var tmpDir string

func fullURL(endpoint string) string {
	return fmt.Sprintf("%s/%s", url, endpoint)
}

func checkStatusCode(res *http.Response, code int) error {
	if gotCode := res.StatusCode; gotCode != code {
		return fmt.Errorf("unexpected status code. want: %d, got %d", code, gotCode)
	}
	return nil
}

func checkResponseContentType(res *http.Response, contentType string) error {
	gotContentType := res.Header.Get("Content-Type")
	if gotContentType != contentType {
		return fmt.Errorf("unexpected content type. want: %s, got: %s", contentType, gotContentType)
	}
	return nil
}

func readResponseBodyIntoString(res *http.Response) (string, error) {
	if err := checkResponseContentType(res, "text/plain"); err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("error reading body: %s", err)
	}
	if err := res.Body.Close(); err != nil {
		return "", fmt.Errorf("error closing response body: %s", err)
	}
	return string(body), nil
}

func readResponseBodyIntoStruct(res *http.Response, s interface{}) error {
	if err := checkResponseContentType(res, "application/json"); err != nil {
		return err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading body: %s", err)
	}
	if err := res.Body.Close(); err != nil {
		return fmt.Errorf("error closing response body: %s", err)
	}
	if err := json.Unmarshal(body, s); err != nil {
		return fmt.Errorf("failed to unmarshal response JSON: %s", err)
	}
	return nil
}

func encodeJSON(t *testing.T, s interface{}) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %s", err)
	}
	return string(b)
}

func TestMain(m *testing.M) {
	var server *httptest.Server
	defer func() {
		if server != nil {
			server.Close()
		}
	}()


	startServer = func(r *mux.Router) {
		server = httptest.NewServer(r)
		url = server.URL
	}
	var err error
	tmpDir, err = ioutil.TempDir("", "collections")
	if err != nil {
		log.Fatalf("failed to create temp dir: %s", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Fatal(err)
		}
	}()

	// Copy runfile to temp directory so we can write there
	runFiles := testhelpers.GetRunFilesPath()
	testFilePath := path.Join(runFiles,
		"server",
		"testdata",
		"collections",
		fmt.Sprintf("%s.binproto", collectionName))
	testFile, err := os.Open(testFilePath)
	if err != nil {
		log.Fatalf("failed to open test file: %s", err)
	}
	destFile, err := os.Create(fmt.Sprintf("%s/%s.binproto", tmpDir, collectionName))
	if err != nil {
		log.Fatalf("failed to copy test file: %s", err)
	}
	_, err = io.Copy(destFile, testFile)
	if err != nil {
		log.Fatalf("failed to copy test file: %s", err)
	}

	*resourceRoot = path.Join(runFiles, "client")

	storagePath = &tmpDir
	runServer()

	os.Exit(m.Run())
}

func TestHTML(t *testing.T) {
	endpoints := []string{"/", "/sched.html", "/jsadlkgaksg"}
	for _, endpoint := range endpoints {
		res, err := http.Get(fullURL(endpoint))
		if err != nil {
			t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
		}
		if err := checkStatusCode(res, http.StatusOK); err != nil {
			t.Fatal(err)
		}
		if err := checkResponseContentType(res, "text/html"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestJS(t *testing.T) {
	endpoint := "/bundle.js"
	res, err := http.Get(fullURL(endpoint))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	if err := checkResponseContentType(res, "application/javascript"); err != nil {
		t.Fatal(err)
	}
}

func TestCSS(t *testing.T) {
	endpoint := "/material-theme"
	res, err := http.Get(fullURL(endpoint))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	if err := checkResponseContentType(res, "text/css"); err != nil {
		t.Fatal(err)
	}
}

func TestGetCollectionMetadata(t *testing.T) {
	endpoint := fmt.Sprintf("get_collection_metadata?request=%s", collectionName)
	res, err := http.Get(fullURL(endpoint))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.Metadata{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	want := &models.Metadata{
		CollectionUniqueName: collectionName,
		Creator:              "bob",
		Owners:               []string{"joe"},
		Tags:                 []string{"test"},
		Description:          "test",
		CreationTime:         1,
		FtraceEvents:         []string{},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetCollectionMetadata: Diff -want +got:\n%s", diff)
	}
}

func TestGetCollectionParameters(t *testing.T) {
	endpoint := fmt.Sprintf("get_collection_parameters?request=%s", collectionName)
	res, err := http.Post(fullURL(endpoint), "text/plain", strings.NewReader(collectionName))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.CollectionParametersResponse{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	want := &models.CollectionParametersResponse{
		CollectionName:   collectionName,
		CPUs:             []int64{0},
		StartTimestampNs: 0,
		EndTimestampNs:   2009150555,
		FtraceEvents:     []string{"sched_migrate_task", "sched_switch", "sched_wakeup", "sched_wakeup_new"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetCollectionParameters: Diff -want +got:\n%s", diff)
	}
}

func TestListCollectionMetadata(t *testing.T) {
	// Parameter is ignored with fs backend.
	endpoint := fmt.Sprintf("list_collection_metadata?request=%s", "")
	res, err := http.Post(fullURL(endpoint), "application/json", strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	var got = []models.Metadata{}
	if err := readResponseBodyIntoStruct(res, &got); err != nil {
		t.Fatal(err)
	}

	want := []models.Metadata{{
		CollectionUniqueName: collectionName,
		Creator:              "bob",
		Owners:               []string{"joe"},
		Tags:                 []string{"test"},
		Description:          "test",
		CreationTime:         1,
		FtraceEvents:         []string{},
	}}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestListCollectionMetadata: Diff -want +got:\n%s", diff)
	}
}

func TestUploadEditDeleteCollection(t *testing.T) {
	// Create a new collection
	createRequestProto := encodeJSON(t, &models.CreateCollectionRequest{
		Creator:      "bob",
		Owners:       []string{"joe"},
		Tags:         []string{"test"},
		Description:  "test",
		CreationTime: 0,
	})
	uploadEndpoint := "upload"
	runFiles := testhelpers.GetRunFilesPath()
	file, err := os.Open(path.Join(runFiles, "server", "testdata", "test.tar.gz"))
	if err != nil {
		t.Fatalf("error reading tar for UploadFile: %s", err)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("request", createRequestProto); err != nil {
		t.Fatalf("error writing request to multipart form for UploadFile: %s", err)
	}
	fileWriter, err := writer.CreateFormFile("file", "test.tar.gz")
	if err != nil {
		t.Fatalf("error creating multipart form file writer for UploadFile: %s", err)
	}
	_, err = io.Copy(fileWriter, file)
	if err != nil {
		t.Fatalf("error writing file to multipart form for UploadFile: %s", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("error closing multipart form file writer for UploadFile: %s", err)
	}
	uploadRes, err := http.Post(fullURL(uploadEndpoint), writer.FormDataContentType(), body)
	if err != nil {
		t.Fatalf("unexpected error posting %s: %s", uploadEndpoint, err)
	}
	if err := checkStatusCode(uploadRes, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	newCollectionName, err := readResponseBodyIntoString(uploadRes)
	if err != nil {
		t.Fatal(err)
	}
	if newCollectionName == "" {
		t.Fatal("no collection name was returned")
	}
	newCollectionFile := fmt.Sprintf("%s/%s.binproto", tmpDir, newCollectionName)
	if _, err := os.Stat(newCollectionFile); os.IsNotExist(err) {
		t.Fatalf("new collection file was not created: %s", err)
	}
	// Edit the new collection's metadata
	editRequestProto := encodeJSON(t, &models.EditCollectionRequest{
		CollectionName: newCollectionName,
		Description:    "abc",
		AddOwners:      []string{"john"},
		AddTags:        []string{"edited"},
		RemoveTags:     []string{"test"},
	})
	editEndpoint := fmt.Sprintf("edit_collection?request=%s", editRequestProto)
	editRes, err := http.Post(fullURL("edit_collection"), "application/json", strings.NewReader(editRequestProto))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", editEndpoint, err)
	}
	if err := checkStatusCode(editRes, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	// Delete the new collection
	deleteEndpoint := fmt.Sprintf("delete_collection?request=%s", newCollectionName)
	deleteRes, err := http.Get(fullURL(deleteEndpoint))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", deleteEndpoint, err)
	}
	if err := checkStatusCode(deleteRes, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newCollectionFile); !os.IsNotExist(err) {
		t.Fatalf("new collection file was not deleted: %s", err)
	}
}

func TestGetCPUIntervals(t *testing.T) {
	requestJSON := encodeJSON(t, &models.CPUIntervalsRequest{
		CollectionName:        collectionName,
		CPUs:                  []int64{0},
		MinIntervalDurationNs: 0,
		StartTimestampNs:      0,
		EndTimestampNs:        10000,
	})
	endpoint := fmt.Sprintf("get_cpu_intervals?request=%s", requestJSON)
	res, err := http.Post(fullURL(endpoint), "application/json", strings.NewReader(requestJSON))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.CPUIntervalsResponse{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	want := &models.CPUIntervalsResponse{
		CollectionName: collectionName,
		Intervals: []models.CPUIntervals{{
			CPU: 0,
			Running: []*sched.Interval{
				{
					Duration: 21845,
					ThreadResidencies: []*sched.ThreadResidency{
						{
							Thread: &sched.Thread{
								PID:      17254,
								Command:  "trace.sh",
								Priority: 120,
							},
							Duration: 21845,
							State:    sched.RunningState,
						},
						{
							Thread: &sched.Thread{
								PID:      17287,
								Command:  "trace.sh",
								Priority: 120,
							},
							Duration: 21845,
							State:    sched.WaitingState,
						},
					},
					MergedIntervalCount: 1,
				},
			},
			Waiting: []*sched.Interval{
				{
					Duration: 21845,
					ThreadResidencies: []*sched.ThreadResidency{
						{
							Thread: &sched.Thread{
								PID:      17254,
								Command:  "trace.sh",
								Priority: 120,
							},
							Duration: 21845,
							State:    sched.RunningState,
						},
						{
							Thread: &sched.Thread{
								PID:      17287,
								Priority: 120,
								Command:  "trace.sh",
							},
							Duration: 21845,
							State:    sched.WaitingState,
						},
					},
					MergedIntervalCount: 1,
				},
			},
		}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetCPUIntervals: Diff -want +got:\n%s", diff)
	}
}

func TestGetPIDIntervals(t *testing.T) {
	requestJSON := encodeJSON(t, &models.PidIntervalsRequest{
		CollectionName:        collectionName,
		Pids:                  []int64{17254},
		MinIntervalDurationNs: 0,
		StartTimestampNs:      0,
		EndTimestampNs:        25000,
	})
	endpoint := fmt.Sprintf("get_pid_intervals?request=%s", requestJSON)
	res, err := http.Post(fullURL(endpoint), "application/json", strings.NewReader(requestJSON))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.PIDntervalsResponse{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	want := &models.PIDntervalsResponse{
		CollectionName: collectionName,
		PIDIntervals: []models.PIDIntervals{{
			PID: 17254,
			Intervals: []models.PIDInterval{
				{
					Pid:                 17254,
					Command:             "trace.sh",
					Priority:            120,
					State:               sched.RunningState,
					PostWakeup:          false,
					CPU:                 0,
					StartTimestampNs:    0,
					EndTimestampNs:      21845,
					MergedIntervalCount: 1,
				},
				{
					Pid:                 17254,
					Command:             "trace.sh",
					Priority:            120,
					State:               sched.SleepingState,
					StartTimestampNs:    21845,
					EndTimestampNs:      65705,
					MergedIntervalCount: 1,
				}},
		}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetPIDIntervals: Diff -want +got:\n%s", diff)
	}
}

func TestGetAntagonists(t *testing.T) {
	requestJSON := encodeJSON(t, &models.AntagonistsRequest{
		CollectionName:   collectionName,
		Pids:             []int64{17254},
		StartTimestampNs: 71540,
		EndTimestampNs:   73790,
	})
	endpoint := fmt.Sprintf("get_antagonists?request=%s", requestJSON)
	res, err := http.Post(fullURL(endpoint), "application/json", strings.NewReader(requestJSON))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.AntagonistsResponse{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	want := &models.AntagonistsResponse{
		CollectionName: collectionName,
		Antagonists: []sched.Antagonists{{
			Victims: []sched.Thread{{
				PID:      17254,
				Command:  "trace.sh",
				Priority: 120,
			}},
			Antagonisms: []sched.Antagonism{{
				RunningThread: sched.Thread{
					PID:      430,
					Command:  "kauditd",
					Priority: 120,
				},
				StartTimestamp: 71547,
				EndTimestamp:   73788,
			},
				{
					RunningThread: sched.Thread{
						PID:      449,
						Command:  "auditd",
						Priority: 116,
					},
					StartTimestamp: 73788,
					EndTimestamp:   73790,
				}},
			StartTimestamp: 71540,
			EndTimestamp:   73790,
		}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetAntagonists: Diff -want +got:\n%s", diff)
	}
}

func TestGetPerThreadEventSeries(t *testing.T) {
	requestJSON := encodeJSON(t, &models.PerThreadEventSeriesRequest{
		CollectionName:   collectionName,
		Pids:             []int64{17254},
		StartTimestampNs: 0,
		EndTimestampNs:   72000,
	})
	endpoint := fmt.Sprintf("get_per_thread_event_series?request=%s", requestJSON)
	res, err := http.Post(fullURL(endpoint), "application/json", strings.NewReader(requestJSON))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.PerThreadEventSeriesResponse{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	want := &models.PerThreadEventSeriesResponse{
		CollectionName: collectionName,
		EventSeries: []models.PerThreadEventSeries{{
			Pid: 17254,
			Events: []models.Event{
				{
					UniqueID:      2,
					EventType:     4,
					TimestampNs:   21845,
					Pid:           430,
					Command:       "kauditd",
					Priority:      120,
					PrevPid:       17254,
					PrevCommand:   "trace.sh",
					PrevPriority:  120,
					PrevTaskState: 4096,
				},
				{
					UniqueID:      5,
					EventType:     4,
					TimestampNs:   65705,
					Pid:           17254,
					Command:       "trace.sh",
					Priority:      120,
					PrevPid:       449,
					PrevCommand:   "auditd",
					PrevPriority:  116,
					PrevTaskState: 1,
				},
				{
					UniqueID:     7,
					EventType:    4,
					TimestampNs:  71547,
					Pid:          430,
					Command:      "kauditd",
					Priority:     120,
					PrevPid:      17254,
					PrevCommand:  "trace.sh",
					PrevPriority: 120,
				},
			}},
		}}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetPerThreadEventSeries: Diff -want +got:\n%s", diff)
	}
}

func TestGetThreadSummaries(t *testing.T) {
	requestJSON := encodeJSON(t, &models.ThreadSummariesRequest{
		CollectionName:   collectionName,
		Cpus:             []int64{0},
		StartTimestampNs: 0,
		EndTimestampNs:   2009150555,
	})
	endpoint := fmt.Sprintf("get_thread_summaries?request=%s", requestJSON)
	res, err := http.Post(fullURL(endpoint), "application/json", strings.NewReader(requestJSON))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.ThreadSummariesResponse{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	// Only compare the first element of got - there are hundreds of them, and its not practical
	// to list all of them
	got.Metrics = got.Metrics[:1]

	want := &models.ThreadSummariesResponse{
		CollectionName: collectionName,
		Metrics: []models.Metrics{{
			WakeupCount:      270,
			UnknownTimeNs:    6269650,
			RunTimeNs:        5680247,
			WaitTimeNs:       2459979,
			SleepTimeNs:      1994740679,
			Pids:             []int64{3},
			Commands:         []string{"ksoftirqd/0"},
			Priorities:       []int64{120},
			Cpus:             []int64{0},
			StartTimestampNs: 0,
			EndTimestampNs:   2009150555,
		}},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetThreadSummaries: Diff -want +got:\n%s", diff)
	}
}

func TestGetFtraceEvents(t *testing.T) {
	requestJSON := encodeJSON(t, &models.FtraceEventsRequest{
		CollectionName: collectionName,
		Cpus:           []int64{0},
		EventTypes:     []string{"sched_switch"},
		StartTimestamp: 0,
		EndTimestamp:   25000,
	})
	endpoint := fmt.Sprintf("get_ftrace_events?request=%s", requestJSON)
	res, err := http.Post(fullURL(endpoint), "application/json", strings.NewReader(requestJSON))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.FtraceEventsResponse{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	want := &models.FtraceEventsResponse{
		CollectionName: collectionName,
		EventsByCPU: map[int64][]models.FtraceEvent{
			0: {{
				Index:     2,
				Name:      "sched_switch",
				CPU:       0,
				Timestamp: 21845,
				Clipped:   false,
				TextProperties: map[string]string{
					"common_flags":         "\x01",
					"common_preempt_count": "",
					"prev_comm":            "trace.sh",
					"next_comm":            "kauditd",
				},
				NumberProperties: map[string]int64{
					"common_type": 0,
					"common_pid":  17254,
					"prev_pid":    17254,
					"prev_prio":   120,
					"prev_state":  4096,
					"next_pid":    430,
					"next_prio":   120,
				},
			}},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetFtraceEvents: Diff -want +got:\n%s", diff)
	}
}

func TestGetUtilizationMetrics(t *testing.T) {
	requestJSON := encodeJSON(t, &models.UtilizationMetricsRequest{
		CollectionName:   collectionName,
		Cpus:             []int64{0},
		StartTimestampNs: 0,
		EndTimestampNs:   2009150555,
	})
	endpoint := fmt.Sprintf("get_utilization_metrics?request=%s", requestJSON)
	res, err := http.Post(fullURL(endpoint), "application/json", strings.NewReader(requestJSON))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := models.UtilizationMetricsResponse{}
	if err := readResponseBodyIntoStruct(res, &got); err != nil {
		t.Fatal(err)
	}

	want := models.UtilizationMetricsResponse{
		Request: models.UtilizationMetricsRequest{
			CollectionName:   collectionName,
			Cpus:             []int64{0},
			StartTimestampNs: 0,
			EndTimestampNs:   2009150555,
		},
		UtilizationMetrics: models.UtilizationMetrics{
			WallTime:               0,
			PerCPUTime:             0,
			PerThreadTime:          0,
			CPUUtilizationFraction: 1,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetPerThreadEventSeries: Diff -want +got:\n%s", diff)
	}
}

func TestGetSystemTopology(t *testing.T) {
	endpoint := fmt.Sprintf("get_system_topology?request=%s", collectionName)
	res, err := http.Get(fullURL(endpoint))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", endpoint, err)
	}
	if err := checkStatusCode(res, http.StatusOK); err != nil {
		t.Fatal(err)
	}
	got := &models.SystemTopologyResponse{}
	if err := readResponseBodyIntoStruct(res, got); err != nil {
		t.Fatal(err)
	}

	want := &models.SystemTopologyResponse{
		CollectionName: collectionName,
		SystemTopology: models.SystemTopology{
			LogicalCores: []models.LogicalCore{{
				SocketID:   0,
				DieID:      0,
				ThreadID:   0,
				NumaNodeID: 0,
				CPUID:      0,
				CoreID:     0,
			}},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetSystemTopology: Diff -want +got:\n%s", diff)
	}
}
