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
package storageservice

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/google/schedviz/analysis/sched"
	"github.com/google/schedviz/server/models"
	"github.com/google/schedviz/tracedata/trace"
)

var colRequest = &models.CreateCollectionRequest{
	Creator:      "bob",
	Owners:       []string{"joe"},
	Tags:         []string{"test"},
	Description:  "test",
	CreationTime: 1,
}

var (
	ctx = context.Background()
)

// TODO(tracked) Consider using schedtestcommon.TestTrace1(t) here
// as a lighter-weight alternative.
func getTestTarFile(t *testing.T, name string) io.Reader {
	t.Helper()
	// Bazel stores test location in these environment variables
	runFiles := path.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"))
	file, err := os.Open(path.Join(runFiles, "server", "testdata", name))
	if err != nil {
		t.Fatalf("error fetching test tar: %s", err)
	}
	return file
}

var fh = func(t *testing.T) io.Reader {
	t.Helper()
	return getTestTarFile(t, "test.tar.gz")
}

func createCollectionDir() (string, error) {
	tmpDir, err := ioutil.TempDir("", "collections")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %s", err)
	}
	return tmpDir, err
}

func cleanup(t *testing.T, tmpDir string) {
	t.Helper()
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Fatal(err)
	}
}

func createFSStorage(t *testing.T, path string, count int) StorageService {
	ss, err := CreateFSStorage(path, count)
	if err != nil {
		t.Fatalf("Failed to create storage service: %s", err)
	}
	return ss
}

func TestFsStorage_UploadFile(t *testing.T) {
	tmpDir, err := createCollectionDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, tmpDir)
	fsStorage := createFSStorage(t, tmpDir, 1)

	readers := []io.Reader{fh(t), getTestTarFile(t, "test_no_metadata.tar.gz")}

	for _, reader := range readers {
		collectionName, err := fsStorage.UploadFile(ctx, colRequest, reader)
		if err != nil {
			t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
		}

		cachedValue, err := fsStorage.GetCollection(ctx, collectionName)
		if err != nil {
			t.Fatalf("unexpected error thrown by FsStorage::GetCollection: %s", err)
		}

		rawEvents, err := cachedValue.Collection.GetRawEvents()
		if err != nil {
			t.Fatalf("unexpected error thrown while checking number of raw events: %s", err)
		}
		if len(rawEvents) != 28922 {
			t.Errorf("wrong number of events in event set. got: %d, want: %d", len(rawEvents), 28922)
		}
		gotStart, gotEnd := cachedValue.Collection.Interval()
		if gotStart != 0 {
			t.Errorf("wrong start time of collection. got: %d, want: %d", gotStart, 0)
		}
		if gotEnd != 2009150555 {
			t.Errorf("wrong end time of collection. got: %d, want: %d", gotEnd, 2009150555)
		}

		wantLogicalCores := []models.LogicalCore{{
			SocketID:   0,
			DieID:      0,
			ThreadID:   0,
			NumaNodeID: 0,
			CPUID:      0,
			CoreID:     0,
		}}

		if diff := cmp.Diff(cachedValue.SystemTopology.LogicalCores, wantLogicalCores); diff != "" {
			t.Errorf("wrong system topology returned; Diff -want +got %v", diff)
		}
	}
}

func TestFsStorage_DeleteCollection(t *testing.T) {
	collectionName := "coll_to_delete"
	tmpDir, err := createCollectionDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, tmpDir)

	tmpFile := path.Join(tmpDir, collectionName+".binproto")
	if err := ioutil.WriteFile(tmpFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create temp file: %s", err)
	}

	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Fatalf("temp file was not created: %s", err)
	}

	fsStorage := createFSStorage(t, tmpDir, 1)
	if err := fsStorage.DeleteCollection(ctx, "", collectionName); err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::DeleteCollection: %s", err)
	}

	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Fatalf("temp file was not deleted: %s", err)
	}
}

func TestFsStorage_GetCollection(t *testing.T) {
	tmpDir, err := createCollectionDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, tmpDir)
	fsStorage, ok := createFSStorage(t, tmpDir, 1).(*FsStorage)
	if !ok {
		t.Fatalf("CreateFSStorage returned wrong type")
	}

	origAddToCache := addToCache
	cacheAdds := 0
	addToCache = func(sb *storageBase, path string, collection *CachedCollection) {
		cacheAdds++
		origAddToCache(sb, path, collection)
	}
	defer func() { addToCache = origAddToCache }()

	// Adds an entry to the cache.
	firstCollectionName, err := fsStorage.UploadFile(ctx, colRequest, fh(t))
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
	}
	_, err = fsStorage.GetCollection(ctx, firstCollectionName)
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::GetCollection: %s", err)
	}
	if cacheAdds != 1 {
		t.Errorf("Expected 1 cache adds after first GetCollection, got %d", cacheAdds)
	}
	// Adds an entry to the cache, evicting the old one.
	secondCollectionName, err := fsStorage.UploadFile(ctx, colRequest, fh(t))
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
	}
	// Should hit cache
	_, err = fsStorage.GetCollection(ctx, secondCollectionName)
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::GetCollection: %s", err)
	}
	if cacheAdds != 2 {
		t.Errorf("Expected 2 cache adds after second GetCollection, got %d", cacheAdds)
	}
	// Adds an entry to the cache, evicting the old one.
	_, err = fsStorage.GetCollection(ctx, firstCollectionName)
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::GetCollection: %s", err)
	}
	if cacheAdds != 3 {
		t.Errorf("Expected 3 cache adds after third GetCollection, got %d", cacheAdds)
	}
	// Should hit cache
	_, err = fsStorage.GetCollection(ctx, firstCollectionName)
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::GetCollection: %s", err)
	}
	if cacheAdds != 3 {
		t.Errorf("Expected 3 cache adds after fourth GetCollection, got %d", cacheAdds)
	}
}

func TestFsStorage_GetCollectionMetadata(t *testing.T) {
	tmpDir, err := createCollectionDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, tmpDir)
	fsStorage := createFSStorage(t, tmpDir, 1)
	collectionName, err := fsStorage.UploadFile(ctx, colRequest, fh(t))
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
	}
	got, err := fsStorage.GetCollectionMetadata(ctx, collectionName)
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::GetCollectionMetadata: %s", err)
	}

	want := models.Metadata{
		CollectionUniqueName: collectionName,
		Creator:              "bob",
		Owners:               []string{"joe"},
		Tags:                 []string{"test"},
		Description:          "test",
		CreationTime:         1,
		FtraceEvents: []string{
			"sched_migrate_task",
			"sched_switch",
			"sched_wakeup",
			"sched_wakeup_new",
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestFsStorage_GetCollectionMetadata: Diff -want +got:\n%s", diff)
	}
}

func TestFsStorage_EditCollection(t *testing.T) {
	tmpDir, err := createCollectionDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, tmpDir)
	fsStorage := createFSStorage(t, tmpDir, 1)

	collectionName, err := fsStorage.UploadFile(ctx, colRequest, fh(t))
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
	}

	req := &models.EditCollectionRequest{
		CollectionName: collectionName,
		Description:    "abc",
		AddOwners:      []string{"john"},
		AddTags:        []string{"edited"},
		RemoveTags:     []string{"test"},
	}

	if err := fsStorage.EditCollection(ctx, "", req); err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::EditCollection: %s", err)
	}

	want := models.Metadata{
		CollectionUniqueName: collectionName,
		Creator:              "bob",
		Owners:               []string{"joe", "john"},
		Tags:                 []string{"edited"},
		Description:          "abc",
		CreationTime:         1,
		FtraceEvents: []string{
			"sched_migrate_task",
			"sched_switch",
			"sched_wakeup",
			"sched_wakeup_new",
		},
	}

	got, err := fsStorage.GetCollectionMetadata(ctx, collectionName)
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::GetCollectionMetadata: %s", err)
	}
	sort.Strings(got.Owners)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestFsStorage_EditCollection: Diff -want +got:\n%s", diff)
	}
}

func TestFsStorage_ListCollectionMetadata(t *testing.T) {
	tmpDir, err := createCollectionDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, tmpDir)
	fsStorage := createFSStorage(t, tmpDir, 1)

	firstCollectionName, err := fsStorage.UploadFile(ctx, colRequest, fh(t))
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
	}
	secondCollectionName, err := fsStorage.UploadFile(ctx, colRequest, fh(t))
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
	}

	files, err := ioutil.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("eror reading temp directory: %s", err)
	}
	if len(files) != 2 {
		t.Errorf("wrong number of files written. want %d, got %d", 2, len(files))
	}

	gotFiles := make(map[string]struct{})
	for _, file := range files {
		gotFiles[file.Name()] = struct{}{}
	}
	wantFiles := map[string]struct{}{
		firstCollectionName + ".binproto":  {},
		secondCollectionName + ".binproto": {},
	}

	if diff := cmp.Diff(wantFiles, gotFiles); diff != "" {
		t.Fatalf("TestFsStorage_ListCollectionMetadata: Diff -want +got:\n%s", diff)
	}
}

func TestFsStorage_GetCollectionParameters(t *testing.T) {
	tmpDir, err := createCollectionDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, tmpDir)
	fsStorage := createFSStorage(t, tmpDir, 1)

	collectionName, err := fsStorage.UploadFile(ctx, colRequest, fh(t))
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
	}

	got, err := fsStorage.GetCollectionParameters(ctx, collectionName)
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::GetCollectionParameters: %s", err)
	}

	want := models.CollectionParametersResponse{
		CollectionName:   collectionName,
		CPUs:             []int64{0},
		StartTimestampNs: 0,
		EndTimestampNs:   2009150555,
		FtraceEvents:     []string{"sched_migrate_task", "sched_switch", "sched_wakeup", "sched_wakeup_new"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestFsStorage_GetCollectionParameters: Diff -want +got:\n%s", diff)
	}
}

func TestFsStorage_GetFtraceEvents(t *testing.T) {
	tmpDir, err := createCollectionDir()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(t, tmpDir)
	fsStorage := createFSStorage(t, tmpDir, 1)

	collectionName, err := fsStorage.UploadFile(ctx, colRequest, fh(t))
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::UploadFile: %s", err)
	}

	req := &models.FtraceEventsRequest{
		CollectionName: collectionName,
		Cpus:           []int64{0},
		EventTypes:     []string{"sched_switch"},
		StartTimestamp: 0,
		EndTimestamp:   22000,
	}

	got, err := fsStorage.GetFtraceEvents(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error thrown by FsStorage::GetFtraceEvents: %s", err)
	}

	want := models.FtraceEventsResponse{
		CollectionName: collectionName,
		EventsByCPU: map[sched.CPUID][]*trace.Event{
			0: {{
				Index:     2,
				Name:      "sched_switch",
				CPU:       0,
				Timestamp: 21845,
				Clipped:   false,
				TextProperties: map[string]string{
					"prev_comm": "trace.sh",
					"next_comm": "kauditd",
				},
				NumberProperties: map[string]int64{
					"common_type":          0,
					"common_flags":         1,
					"common_pid":           17254,
					"common_preempt_count": 0,
					"prev_pid":             17254,
					"prev_prio":            120,
					"prev_state":           4096,
					"next_pid":             430,
					"next_prio":            120,
				},
			}},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("TestFsStorage_GetFtraceEvents: Diff -want +got:\n%s", diff)
	}
}

func TestConvertIntRangeToList(t *testing.T) {
	tests := []struct {
		in  string
		out []int64
		err string
	}{
		{
			in:  "0-4,7,9,11-12",
			out: []int64{0, 1, 2, 3, 4, 7, 9, 11, 12},
		},
		{
			in:  "0,1,2,3,4,5,6,7,8,9",
			out: []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			in:  "9,8,7,6,5,4,3,2,1,0",
			out: []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			in:  "abc,123",
			err: "strconv.ParseInt: parsing \"abc\": invalid syntax",
		},
		{
			in:  "1-2-3,4",
			err: "malformed range string. Ranges must be of the form int-int, or just a int. Got: 1-2-3",
		},
		{
			in:  "4-3,123",
			err: "malformed range string. End of range must be after start. Got 4-3",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConvertIntRangeToList Case: %d", i), func(t *testing.T) {
			got, err := convertIntRangeToList(test.in)
			if test.err != "" && err == nil {
				t.Errorf("Expected %q error, but no error was thrown", test.err)
			} else if err != nil && err.Error() != test.err {
				t.Errorf("Expected %q error, but got %q error instead", test.err, err)
			} else if diff := cmp.Diff(got, test.out); diff != "" {
				t.Errorf("convertIntRangeToList(%q): Diff -want +got: \n%s", test.in, diff)
			}
		})
	}
}
