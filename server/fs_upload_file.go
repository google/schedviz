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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
	"github.com/golang/protobuf/proto"
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

const (
	headerSuffix             = "/header_page"
	formatSuffix             = "/format"
	coreIDSuffix             = "/core_id"
	physicalPackageIDSuffix  = "/physical_package_id"
	threadSiblingsListSuffix = "/thread_siblings_list"
)

// UploadFile creates a new collection from the uploaded file and saves it to disk
func (fs *FsStorage) UploadFile(ctx context.Context, req *models.CreateCollectionRequest, file io.Reader) (string, error) {
	eventSet, topology, err := readTar(file)
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
	metadataProto, err := convertMetadataStructToProto(metadata)
	if err != nil {
		return err
	}
	sort.Slice(eventSet.Event, func(i, j int) bool {
		return eventSet.Event[i].TimestampNs < eventSet.Event[j].TimestampNs
	})

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

/*
readTar reads a tar containing a folder with the following directory structure:

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
func readTar(fileReader io.Reader) (*eventpb.EventSet, *models.SystemTopology, error) {
	// Unzip the tar.gz file.
	gzipReader, err := gzip.NewReader(fileReader)
	if err != nil {
		return nil, nil, err
	}
	// Copy the unzipped contents to a temp file. We have to do this because seeking by absolute
	// bytes does not work on the gzip reader, and we need this to parse the trace files.
	tmpFile, err := ioutil.TempFile("", "collection_tar")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			log.Errorf("failed to remove temp file: %s", err)
		}
	}()
	_, err = io.Copy(tmpFile, gzipReader)
	if err != nil {
		return nil, nil, err
	}
	// Close and reopen the temp file so that the reader points to the beginning again.
	tmpFileName := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return nil, nil, err
	}
	tmpFile, err = os.Open(tmpFileName)
	if err != nil {
		return nil, nil, err
	}

	tarReader := tar.NewReader(tmpFile)

	haveReadTrace := false
	var headerContent string
	var formatFiles = []string{}
	tb := newTopologyBuilder()

	var traceParser *traceparser.TraceParser
	var eventSetBuilder *traceparser.EventSetBuilder
	addTraceEvent := func(traceEvent *traceparser.TraceEvent) (bool, error) {
		if err := eventSetBuilder.AddTraceEvent(traceEvent); err != nil {
			return false, fmt.Errorf("error in AddTraceEvent: %s", err)
		}
		return true, nil
	}

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break // End of archive
			}
			return nil, nil, err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			continue
		case tar.TypeReg:
			name := header.Name
			if strings.HasSuffix(name, headerSuffix) || strings.HasSuffix(name, formatSuffix) {
				if haveReadTrace {
					return nil, nil, errors.New("tried to read an additional format file after reading a trace file")
				}
				format, err := readString(tarReader)
				if err != nil {
					return nil, nil, err
				}
				if strings.HasSuffix(name, formatSuffix) {
					formatFiles = append(formatFiles, format)
				} else {
					if headerContent != "" {
						return nil, nil, errors.New("multiple header_page formats found")
					}
					headerContent = format
				}
			} else if matches := cpuRe.FindStringSubmatch(name); matches != nil {
				cpu, err := strconv.ParseInt(matches[1], 10, 64)
				if err != nil {
					return nil, nil, err
				}
				haveReadTrace = true
				if traceParser == nil {
					if headerContent == "" {
						return nil, nil, errors.New("header format not found")
					}
					if len(formatFiles) == 0 {
						return nil, nil, errors.New("no format files found. Must have at last one format file")
					}
					tp, err := traceparser.New(headerContent, formatFiles)
					if err != nil {
						return nil, nil, fmt.Errorf("failed to parse formats: %s", err)
					}
					traceParser = &tp
				}
				if eventSetBuilder == nil {
					eventSetBuilder = traceparser.NewEventSetBuilder(traceParser)
				}
				bufferedTarReader := bufio.NewReader(tarReader)
				if err := traceParser.ParseTrace(bufferedTarReader, cpu, addTraceEvent); err != nil {
					return nil, nil, err
				}
			} else if topoRe.MatchString(name) {
				cpuID, numaID, err := extractCPUAndNUMAFromPath(name)
				if err != nil {
					return nil, nil, err
				}
				if err := tb.RecordCPUTopology(tarReader, name, cpuID, numaID); err != nil {
					return nil, nil, err
				}
			} else {
				log.Infof("unknown file %s in archive, ignoring", name)
			}
		}
	}
	finalTopo := tb.FullTopology()
	return eventSetBuilder.EventSet, finalTopo, nil
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

	switch {
	case strings.HasSuffix(name, coreIDSuffix):
		coreID, err := readInt32(r)
		if err != nil {
			return err
		}
		lc.CoreID = coreID
	case strings.HasSuffix(name, physicalPackageIDSuffix):
		ppID, err := readInt32(r)
		if err != nil {
			return err
		}
		lc.SocketID = ppID
	case strings.HasSuffix(name, threadSiblingsListSuffix):
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
