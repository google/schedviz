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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"


	log "github.com/golang/glog"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"

	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"
	"github.com/google/schedviz/server/storageservice"
	"github.com/google/schedviz/testhelpers/testhelpers"
	"github.com/google/schedviz/tracedata/trace"
)

var collectionName string

var url string

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


func setupTests(ss storageservice.StorageService) func() {
	var server *httptest.Server
	var err error

	storageService = ss
	setStorageService = func(ctx context.Context) error { return nil }

	startServer = func(r *mux.Router) {
		server = httptest.NewServer(r)
		url = server.URL
	}
	// Create a test collection.
	runFiles := testhelpers.GetRunFilesPath()
	testFilePath := path.Join(runFiles,
		"server",
		"testdata",
		"test.tar.gz")
	testFile, err := os.Open(testFilePath)
	if err != nil {
		log.Fatalf("failed to open test file: %s", err)
	}
	createReq := &models.CreateCollectionRequest{
		Creator:     defaultHTTPUser,
		Owners:      []string{"joe"},
		Tags:        []string{"test"},
		Description: "test",
	}
	collectionName, err = storageService.UploadFile(context.Background(), createReq, testFile)
	if err != nil {
		log.Fatalf("failed to save collection: %s", err)
	}

	*resourceRoot = path.Join(runFiles, "client")

	runServer(context.Background())

	// Cleanup function
	return func() {
		if server != nil {
			server.CloseClientConnections()
			server.Close()
		}
	}
}

func TestMain(m *testing.M) {
	ret := 0
	var ss storageservice.StorageService
	var err error

	log.Info("Testing FSStorage")
	// Set up a collections path.
	tmpDir, err := ioutil.TempDir("", "collections")
	if err != nil {
		log.Fatalf("failed to create temp dir: %s", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Fatal(err)
		}
	}()
	*storagePath = tmpDir
	ss, err = storageservice.CreateFSStorage(*storagePath, *cacheSize)
	if err != nil {
		log.Fatalf("Failed to start file storage service: %s", err)
	}
	cleanup := setupTests(ss)
	ret |= m.Run()
	cleanup()


	os.Exit(ret)
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
	// Don't bother comparing creation time.
	got.CreationTime = 0
	got.TargetMachine = ""
	want := &models.Metadata{
		CollectionUniqueName: collectionName,
		Creator:              defaultHTTPUser,
		Owners:               []string{"joe"},
		Tags:                 []string{"test"},
		Description:          "test",
		FtraceEvents: []string{
			"sched_migrate_task",
			"sched_switch",
			"sched_wakeup",
			"sched_wakeup_new",
		},
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
	endpoint := fmt.Sprintf("list_collection_metadata?request=%s", defaultHTTPUser)
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
	if len(got) != 1 {
		t.Fatal("Expected 1 returned metadata, got %d", len(got))
	}
	// Don't bother comparing creation time.
	got[0].CreationTime = 0
	got[0].TargetMachine = ""
	want := []models.Metadata{{
		CollectionUniqueName: collectionName,
		Creator:              defaultHTTPUser,
		Owners:               []string{"joe"},
		Tags:                 []string{"test"},
		Description:          "test",
		FtraceEvents: []string{
			"sched_migrate_task",
			"sched_switch",
			"sched_wakeup",
			"sched_wakeup_new",
		},
	}}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestListCollectionMetadata: Diff -want +got:\n%s", diff)
	}
}

func TestEditCollection(t *testing.T) {
	// Edit the new collection's metadata
	editRequestProto := encodeJSON(t, &models.EditCollectionRequest{
		CollectionName: collectionName,
		Description:    "abc",
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
	// Revert the edit
	editRequestProto = encodeJSON(t, &models.EditCollectionRequest{
		CollectionName: collectionName,
		Description:    "test",
		RemoveTags:     []string{"edited"},
		AddTags:        []string{"test"},
	})
	editEndpoint = fmt.Sprintf("edit_collection?request=%s", editRequestProto)
	_, err = http.Post(fullURL("edit_collection"), "application/json", strings.NewReader(editRequestProto))
	if err != nil {
		t.Fatalf("unexpected error fetching %s: %s", editEndpoint, err)
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
			Intervals: []*sched.Interval{
				{
					CPU:            0,
					StartTimestamp: 0,
					Duration:       21845,
					ThreadResidencies: []*sched.ThreadResidency{{
						Thread: &sched.Thread{
							PID:      17254,
							Command:  "trace.sh",
							Priority: 120,
						},
						Duration: 21845,
						State:    sched.RunningState,
					}},
					MergedIntervalCount: 1,
				},
				{
					StartTimestamp: 21845,
					Duration:       43860,
					ThreadResidencies: []*sched.ThreadResidency{{
						Thread: &sched.Thread{
							PID:      17254,
							Command:  "trace.sh",
							Priority: 120,
						},
						Duration: 43860,
						State:    sched.SleepingState,
					}},
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
		Pids:             []sched.PID{17254},
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
		Pids:             []sched.PID{17254},
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
			Events: []*trace.Event{
				{
					Index:     2,
					Name:      "sched_switch",
					Timestamp: 21845,
					TextProperties: map[string]string{
						"common_flags":         "\x01",
						"common_preempt_count": "",
						"next_comm":            "kauditd",
						"prev_comm":            "trace.sh",
					},
					NumberProperties: map[string]int64{
						"common_pid":  17254,
						"common_type": 0,
						"next_pid":    430,
						"next_prio":   120,
						"prev_pid":    17254,
						"prev_prio":   120,
						"prev_state":  4096,
					},
				},
				{
					Index:     5,
					Name:      "sched_switch",
					Timestamp: 65705,
					TextProperties: map[string]string{
						"common_flags":         "\x01",
						"common_preempt_count": "",
						"next_comm":            "trace.sh",
						"prev_comm":            "auditd",
					},
					NumberProperties: map[string]int64{
						"common_pid":  449,
						"common_type": 0,
						"next_pid":    17254,
						"next_prio":   120,
						"prev_pid":    449,
						"prev_prio":   116,
						"prev_state":  1,
					},
				},
				{
					Index:     7,
					Name:      "sched_switch",
					Timestamp: 71547,
					TextProperties: map[string]string{
						"common_flags":         "\x01",
						"common_preempt_count": "",
						"next_comm":            "kauditd",
						"prev_comm":            "trace.sh",
					},
					NumberProperties: map[string]int64{
						"common_pid":  17254,
						"common_type": 0,
						"next_pid":    430,
						"next_prio":   120,
						"prev_pid":    17254,
						"prev_prio":   120,
						"prev_state":  0,
					},
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
		Cpus:             []sched.CPUID{0},
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
		Metrics: []sched.Metrics{{
			WakeupCount:      270,
			UnknownTimeNs:    6269650,
			RunTimeNs:        5680247,
			WaitTimeNs:       2459979,
			SleepTimeNs:      1994740679,
			Pids:             []sched.PID{3},
			Commands:         []string{"ksoftirqd/0"},
			Priorities:       []sched.Priority{120},
			Cpus:             []sched.CPUID{0},
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
		EventsByCPU: map[sched.CPUID][]*trace.Event{
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
		Cpus:             []sched.CPUID{0},
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
			Cpus:             []sched.CPUID{0},
			StartTimestampNs: 0,
			EndTimestampNs:   2009150555,
		},
		UtilizationMetrics: sched.Utilization{
			WallTime:            0,
			PerCPUTime:          0,
			PerThreadTime:       0,
			UtilizationFraction: 1,
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestGetUtilizationMetrics: Diff -want +got:\n%s", diff)
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

