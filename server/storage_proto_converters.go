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
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/schedviz/server/models"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"

)

func convertMetadataProtoToStruct(oldMetadata *eventpb.Metadata) models.Metadata {
	if oldMetadata == nil {
		return models.Metadata{}
	}
	creationTime := oldMetadata.CreationTime.Seconds*int64(time.Second) + int64(oldMetadata.CreationTime.Nanos)

	return models.Metadata{
		CreationTime:         creationTime,
		FtraceEvents:         append([]string{}, oldMetadata.FtraceEvents...),
		TargetMachine:        oldMetadata.TargetMachine,
		Description:          oldMetadata.Description,
		CollectionUniqueName: oldMetadata.CollectionUniqueName,
		Owners:               append([]string{}, oldMetadata.Owners...),
		Tags:                 append([]string{}, oldMetadata.Tags...),
		Creator:              oldMetadata.Creator,
	}
}

func convertMetadataStructToProto(oldMetadata *models.Metadata) (*eventpb.Metadata, error) {
	if oldMetadata == nil {
		return nil, nil
	}
	creationTime, err := ptypes.TimestampProto(time.Unix(0, oldMetadata.CreationTime))
	if err != nil {
		return nil, err
	}
	return &eventpb.Metadata{
		CreationTime:         creationTime,
		FtraceEvents:         oldMetadata.FtraceEvents,
		TargetMachine:        oldMetadata.TargetMachine,
		Description:          oldMetadata.Description,
		CollectionUniqueName: oldMetadata.CollectionUniqueName,
		Owners:               oldMetadata.Owners,
		Tags:                 oldMetadata.Tags,
		Creator:              oldMetadata.Creator,
	}, nil
}

func convertTopologyStructToProto(oldTopology *models.SystemTopology) *eventpb.SystemTopology {
	if oldTopology == nil {
		return nil
	}
	logicalCores := []*eventpb.SystemTopology_LogicalCore{}
	for _, lc := range oldTopology.LogicalCores {
		logicalCores = append(logicalCores, &eventpb.SystemTopology_LogicalCore{
			CoreId:     lc.CoreID,
			CpuId:      lc.CPUID,
			DieId:      lc.DieID,
			NumaNodeId: lc.NumaNodeID,
			SocketId:   lc.SocketID,
			ThreadId:   lc.ThreadID,
		})
	}

	return &eventpb.SystemTopology{
		CpuIdentifier: oldTopology.CPUIdentifier,
		CpuVendor:     oldTopology.CPUVendor,
		CpuModel:      oldTopology.CPUModel,
		CpuStepping:   oldTopology.CPUStepping,
		CpuFamily:     oldTopology.CPUFamily,
		LogicalCore:   logicalCores,
	}
}

func convertTopologyProtoToStruct(oldTopology *eventpb.SystemTopology) models.SystemTopology {
	if oldTopology == nil {
		return models.SystemTopology{}
	}
	logicalCores := []models.LogicalCore{}
	for _, lc := range oldTopology.LogicalCore {
		logicalCores = append(logicalCores, models.LogicalCore{
			CoreID:     lc.CoreId,
			CPUID:      lc.CpuId,
			DieID:      lc.DieId,
			NumaNodeID: lc.NumaNodeId,
			SocketID:   lc.SocketId,
			ThreadID:   lc.ThreadId,
		})
	}

	return models.SystemTopology{
		CPUIdentifier: oldTopology.CpuIdentifier,
		CPUVendor:     oldTopology.CpuVendor,
		CPUModel:      oldTopology.CpuModel,
		CPUStepping:   oldTopology.CpuStepping,
		CPUFamily:     oldTopology.CpuFamily,
		LogicalCores:  logicalCores,
	}
}

