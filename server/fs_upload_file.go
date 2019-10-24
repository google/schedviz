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
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/google/uuid"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"

	"github.com/google/schedviz/server/models"
	"github.com/google/schedviz/traceparser/traceparser"
)

var (
	timeNow = time.Now // stubbed for testing
	cpuRe   = regexp.MustCompile(`cpu(\d+)$`)
	topoRe  = regexp.MustCompile(`node(?P<NUMA>\d+)/cpu(?P<CPU>\d+)/topology/\w+$`)
)

const (
	// TODO(tracked) make this a parameter. Dies-per-socket is not exposed by sysfs, which does
	//  not distinguish between sockets and dies, so for machines that have more than one die per
	//  socket, we will have an inaccurate die and socket count without adjusting this constant.
	diesPerSocket = 1
)

// UploadFile creates a new collection from the uploaded file and saves it to disk
func (fs *FsStorage) UploadFile(ctx context.Context, req *models.CreateCollectionRequest, file io.Reader) (string, error) {
	eventSet, topology, err := readTar(file, fs.failOnUnknownEventFormat)
	if err != nil {
		return "", err
	}

	metadata := makeMetadata(req)

	if err := fs.saveCollection(ctx, metadata, eventSet, topology); err != nil {
		return "", err
	}

	return metadata.CollectionUniqueName, nil
}

// generateUniqueName returns a new unique name suitable for collections.
// It is not required that all unique names be generated via this method:
// unique names may be any string value, but must be unique.
func generateUniqueName(creator string, timeStamp int64) string {
	uid := uuid.New()
	// The format of generated unique names is
	// <UUID>_<timestamp>_<creator-role-tag>.
	return fmt.Sprintf("%s_%x_%s", uid, timeStamp, creator)
}

func makeMetadata(req *models.CreateCollectionRequest) *models.Metadata {
	var creationTime int64
	if req.CreationTime != 0 {
		creationTime = req.CreationTime
	} else {
		creationTime = timeNow().UnixNano()
	}
	collectionUniqueName := generateUniqueName(req.Creator, creationTime)

	metadata := &models.Metadata{
		CollectionUniqueName: collectionUniqueName,
		Creator:              req.Creator,
		Owners:               req.Owners,
		Tags:                 req.Tags,
		Description:          req.Description,
		CreationTime:         creationTime,
	}

	return metadata
}

func (fs *FsStorage) saveCollection(ctx context.Context, metadata *models.Metadata, eventSet *eventpb.EventSet, topology *models.SystemTopology) error {
	sort.Slice(eventSet.Event, func(i, j int) bool {
		return eventSet.Event[i].TimestampNs < eventSet.Event[j].TimestampNs
	})

	ftraceEvents, err := fs.extractEventNames(eventSet)
	if err != nil {
		return err
	}
	metadata.FtraceEvents = ftraceEvents

	metadataProto, err := convertMetadataStructToProto(metadata)
	if err != nil {
		return err
	}

	outProto := &eventpb.Collection{
		Metadata: metadataProto,
		EventSet: eventSet,
		Topology: convertTopologyStructToProto(topology),
	}

	protoBytes, err := proto.Marshal(outProto)
	if err != nil {
		return err
	}

	fullPath := fs.getCollectionPath(metadata.CollectionUniqueName)
	if err := ioutil.WriteFile(fullPath, protoBytes, 0644); err != nil {
		return err
	}

	_, err = fs.GetCollection(ctx, metadata.CollectionUniqueName)
	return err
}

// readTar reads a tar containing a trace of some type.
// New tars will contain a metadata.textproto file at the root which describes
// the type of the trace recorded which implies what the other files are.
// Old tars created before the metadata was added will not contain the
// metadata.textproto file; tars lacking the file will be treated as containing
// FTrace traces.
func readTar(inputTar io.Reader, failOnUnknownEventFormat bool) (*eventpb.EventSet, *models.SystemTopology, error) {
	tmpDir, err := ioutil.TempDir("", "temptar")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory: %s", err)
	}
	defer func() {
		// Clean up temp directory after parsing or on error.
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Errorf("failed to clean up temp directory while parsing tar: %s", err)
		}
	}()

	if err := untar(inputTar, tmpDir); err != nil {
		return nil, nil, err
	}

	config, err := readMetadataFile(tmpDir)
	if err != nil {
		return nil, nil, err
	}

	switch config.TraceType {
	case eventpb.ArchiveMetadataConfig_FTRACE:
		return parseFTraceTar(tmpDir, failOnUnknownEventFormat)
	default:
		return nil, nil, status.Errorf(codes.Internal, "unknown trace type %s", config.TraceType)
	}
}

// untar unpacks a gzip compressed tar to the destination directory.
func untar(inputTar io.Reader, destination string) (err error) {
	addedFiles := []string{}
	defer func() {
		if err != nil {
			// If an error occurred, clean any partially written files.
			for _, file := range addedFiles {
				_ = os.Remove(file)
			}
		}
	}()

	gzipReader, err := gzip.NewReader(inputTar)
	if err != nil {
		err = fmt.Errorf("tar must be gzipped. Error: %s", err)
		return
	}

	tarReader := tar.NewReader(gzipReader)

	for {
		var header *tar.Header
		header, err = tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break // End of archive
			}
			return
		}

		outputPath := filepath.Join(destination, header.Name)
		addedFiles = append(addedFiles, outputPath)
		switch header.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(outputPath, 0755); err != nil {
				return
			}
		case tar.TypeReg:
			var tmpFile *os.File
			tmpFile, err = os.OpenFile(outputPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return
			}
			if _, err = io.Copy(tmpFile, tarReader); err != nil {
				return
			}
			if err = tmpFile.Close(); err != nil {
				return
			}
		}
	}
	err = nil
	addedFiles = []string{}
	return
}

// readMetadataFile reads and parses the metadata file located at
// dir/metadata.textproto. Available configuration options in the metadata file
// are defined in eventpb.ArchiveMetadataConfig
func readMetadataFile(dir string) (*eventpb.ArchiveMetadataConfig, error) {
	metadataPath := path.Join(dir, "metadata.textproto")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		log.Warning("Trace file lacks metadata; assuming it's an FTrace collection.")
		return &eventpb.ArchiveMetadataConfig{
			TraceType: eventpb.ArchiveMetadataConfig_FTRACE,
		}, nil
	}
	bytes, err := ioutil.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}
	config := string(bytes)

	configProto := &eventpb.ArchiveMetadataConfig{}
	if err := proto.UnmarshalText(config, configProto); err != nil {
		return nil, fmt.Errorf("error parsing metadata file: %s", err)
	}

	return configProto, nil
}

/*
parseFTraceTar parses a tar that has an FTrace trace inside of it.
The format of the tar is:

metadata.textproto
formats
  - header_page
  - event category (e.g. sched)
    - event name
      - format
    - second event name
      - format
    - second event category
      - event name
        - format
topology
  - node0
    - cpu0
      - topology
        - core_id
        - core_siblings
        - core_siblings_list
        - physical_package_id
        - thread_siblings
        - thread_siblings_list
    - cpu1
			...
    ...
  - node1
    ...
traces
  - cpu0
  - cpu1
    ...
  - cpuN
*/
func parseFTraceTar(dir string, failOnUnknownEventFormat bool) (*eventpb.EventSet, *models.SystemTopology, error) {
	// Read formats
	headerFormat, eventFormats, err := readFormats(path.Join(dir, "formats"))
	if err != nil {
		return nil, nil, fmt.Errorf("error reading formats: %s", err)
	}

	// Read topology
	topology, err := readTopology(path.Join(dir, "topology"))
	if err != nil {
		log.Warningf("error reading topology. Using empty topology. error: %s", err)
		topology = &models.SystemTopology{
			LogicalCores: []models.LogicalCore{},
		}
	}

	traceParser, err := traceparser.New(headerFormat, eventFormats)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse formats: %s", err)
	}
	traceParser.SetFailOnUnknownEventFormat(failOnUnknownEventFormat)

	eventSetBuilder := traceparser.NewEventSetBuilder(&traceParser)

	addTraceEvent := func(traceEvent *traceparser.TraceEvent) (bool, error) {
		if err := eventSetBuilder.AddTraceEvent(traceEvent); err != nil {
			return false, fmt.Errorf("error in AddTraceEvent: %s", err)
		}
		return true, nil
	}

	if err := readFTraceTraces(path.Join(dir, "traces"), &traceParser, addTraceEvent); err != nil {
		return nil, nil, fmt.Errorf("failed to read Ftrace trace files: %s", err)
	}

	return eventSetBuilder.EventSet, topology, nil
}

// readFormats reads the formats directory of an FTrace tar and returns the
// formats as strings.
func readFormats(formatDir string) (string, []string, error) {
	var headerFormat string
	eventFormats := []string{}

	err := filepath.Walk(formatDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Do nothing
			return nil
		}
		bytes, err := ioutil.ReadFile(path)
		if err != nil || len(bytes) == 0 {
			return fmt.Errorf("error reading header format: %s", err)
		}

		switch info.Name() {
		case "header_page":
			headerFormat = string(bytes)
		case "format":
			eventFormats = append(eventFormats, string(bytes))
		default:
			return fmt.Errorf("unknown file in formats directory: %s", path)
		}

		return nil
	})
	if err != nil {
		return "", nil, err
	}

	if len(eventFormats) == 0 {
		return "", nil, errors.New("no format files found. Must have at last one format file")
	}

	return headerFormat, eventFormats, nil
}

// readTopology reads the topology directory of an FTrace tar and returns the
// topology in its fully parsed format.
func readTopology(topoDir string) (*models.SystemTopology, error) {
	tb := newTopologyBuilder()

	err := filepath.Walk(topoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Do nothing
			return nil
		}
		cpuID, numaID, err := extractCPUAndNUMAFromPath(path)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening %s for reading: %s", path, err)
		}
		if err := tb.RecordCPUTopology(file, info.Name(), cpuID, numaID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return tb.FullTopology(), nil
}

// readFTraceTraces reads the trace files contained in an FTrace tar and
// parses them with the provided TraceParser.
func readFTraceTraces(traceDir string, traceParser *traceparser.TraceParser, callback traceparser.AddEventCallback) error {
	err := filepath.Walk(traceDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() != path.Base(traceDir) {
				return fmt.Errorf("expected trace directory to be flat, but found a subdirectory: %s", filePath)
			}
			return nil
		}
		if matches := cpuRe.FindStringSubmatch(info.Name()); matches != nil {
			cpu, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				return fmt.Errorf("error extracting CPU number from filename (filePath: %s): %s", filePath, err)
			}
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("error opening %s for reading: %s", filePath, err)
			}
			reader := bufio.NewReader(file)
			if err := traceParser.ParseTrace(reader, cpu, callback); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unknown file in trace directory: %s", filePath)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func readString(r io.Reader) (string, error) {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func readInt32(r io.Reader) (int32, error) {
	str, err := readString(r)
	if err != nil {
		return 0, err
	}
	num, err := strconv.ParseInt(strings.TrimSpace(str), 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(num), nil
}

func extractCPUAndNUMAFromPath(path string) (int64, int64, error) {
	match := topoRe.FindStringSubmatch(path)
	matches := map[string]string{}
	for i, name := range match {
		matches[topoRe.SubexpNames()[i]] = name
	}

	cpuStr, ok := matches["CPU"]
	if !ok {
		return 0, 0, errors.New("CPU not found in path")
	}
	numaStr, ok := matches["NUMA"]
	if !ok {
		return 0, 0, errors.New("NUMA not found in path")
	}

	cpu, err := strconv.ParseInt(cpuStr, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to convert CPU to int: %s", err)
	}
	numa, err := strconv.ParseInt(numaStr, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to convert NUMA to int: %s", err)
	}
	return cpu, numa, nil
}

type topologyBuilder struct {
	partialTopology map[int64]*models.LogicalCore
}

// newTopologyBuilder creates a new topology builder
func newTopologyBuilder() *topologyBuilder {
	return &topologyBuilder{
		partialTopology: map[int64]*models.LogicalCore{},
	}
}

// RecordCPUTopology saves a single CPU's topology to the topology builder
func (tb *topologyBuilder) RecordCPUTopology(r io.Reader, name string, cpuID, numaID int64) error {
	lc, ok := tb.partialTopology[cpuID]
	if !ok {
		lc = &models.LogicalCore{
			CPUID:      uint64(cpuID),
			CoreID:     models.UnknownLogicalID,
			ThreadID:   models.UnknownLogicalID,
			NumaNodeID: int32(numaID),
			SocketID:   models.UnknownLogicalID,
		}
	}

	switch name {
	case "core_id":
		coreID, err := readInt32(r)
		if err != nil {
			return err
		}
		lc.CoreID = coreID
	case "physical_package_id":
		ppID, err := readInt32(r)
		if err != nil {
			return err
		}
		lc.SocketID = ppID
	case "thread_siblings_list":
		// ThreadID is the zero indexed ID of the hyperthread of within the current core.
		// The thread siblings list is a list of cpus that are hyperthreads of each other.
		// e.g. if CPUs 0 and 3 are hyperthreads of each other, then the thread siblings list is:
		// 0,3, and CPU 0's ThreadID is 0, and CPU 3's ThreadID is 1.
		strList, err := readString(r)
		if err != nil {
			return err
		}
		threadSiblings, err := convertIntRangeToList(strList)
		if err != nil {
			return err
		}
		idx := sort.Search(len(threadSiblings), func(i int) bool {
			return threadSiblings[i] == cpuID
		})
		lc.ThreadID = int32(idx)
	}

	tb.partialTopology[cpuID] = lc
	return nil
}

// FullTopology returns the topology of all CPUs.
func (tb *topologyBuilder) FullTopology() *models.SystemTopology {
	ret := &models.SystemTopology{
		LogicalCores: []models.LogicalCore{},
		// TODO get CPU family info
	}

	for _, lc := range tb.partialTopology {
		if lc.SocketID == models.UnknownLogicalID && lc.NumaNodeID > models.UnknownLogicalID {
			// If no physical package ID was reported, set the socket ID to the NUMA ID.
			// Note that on some machines, NUMA ID may not be the same as the socket ID.
			lc.SocketID = lc.NumaNodeID
			// Renumber socketID and dieID based off of the number of dies-per-socket.
			// Many CPUs have only one die per socket, but some have more.
			lc.DieID = lc.SocketID % diesPerSocket
			lc.SocketID /= diesPerSocket
		}
		ret.LogicalCores = append(ret.LogicalCores, *lc)
	}


	return ret
}

// Converts comma-separated integer range strings to a list.
// For example the string "0-4,7,9,11-12" would be returned as [0, 1, 2, 3, 4, 7, 9, 11, 12].
func convertIntRangeToList(rangeStr string) ([]int64, error) {
	intList := []int64{}
	ranges := strings.Split(strings.TrimSpace(rangeStr), ",")
	for _, r := range ranges {
		subRange := strings.Split(r, "-")
		// Convert to ints
		intRange := []int64{}
		for _, sr := range subRange {
			i, err := strconv.ParseInt(sr, 10, 64)
			if err != nil {
				return nil, err
			}
			intRange = append(intRange, i)
		}
		if len(intRange) == 1 {
			intList = append(intList, intRange[0])
		} else if len(intRange) == 2 {
			start := intRange[0]
			end := intRange[1]
			if end <= start {
				return nil, fmt.Errorf("malformed range string. End of range must be after start. Got %s", r)
			}
			for i := start; i <= end; i++ {
				intList = append(intList, i)
			}
		} else {
			return nil, fmt.Errorf("malformed range string. Ranges must be of the form int-int, or just a int. Got: %s", r)
		}
	}
	sort.Slice(intList, func(i, j int) bool {
		return intList[i] < intList[j]
	})
	return intList, nil
}
