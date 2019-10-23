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
// Package schedbt provides a type to convert an eBPF sched trace gathered by
// sched.bt into an EventSet suitable for visualization in SchedViz.
package schedbt

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	log "github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	elpb "github.com/google/schedviz/analysis/event_loaders_go_proto"
	"github.com/google/schedviz/tracedata/eventsetbuilder"
	eventpb "github.com/google/schedviz/tracedata/schedviz_events_go_proto"
)

type prioAndComm struct {
	prio int64
	comm string
}

var unknownPrioAndComm = &prioAndComm{
	prio: 0,
	comm: "<unknown>",
}

// Parser assembles a string of sched.bt output rows into an EventSet.
// Formats.  All hex numbers lack leading 0x.
// ST:0:<0x start timestamp>
//    Start marker.  Trace timestamps are offset from this timestamp.
// P:<0x timestamp>:<0x pid>:<0x prio>:<command name>
//    Priority and command for pid.  Timestamp offset from start timestamp.
// S:<0x timestamp>:<0x cpu>:<0x prev pid>:<0x prev state>:<0x next pid>
//    Switch.  Timestamp offset from start timestamp.
// M:<0x timestamp>:<0x cpu>:<0x orig cpu>:<0x dest cpu>:<0x pid>
//    Migrate.  Timestamp offset from start timestamp.  cpu is the reporting
//    CPU.
// W:<0x timestamp>:<0x cpu>:<0x target cpu>:<0x pid>
//    Wakeup.  Timestamp offset from start timestamp.  cpu is the reporting
//    CPU.
type Parser struct {
	startTimestamp    int64
	hasStartTimestamp bool
	esb               *eventsetbuilder.Builder
	prioAndCommByPid  map[int64]*prioAndComm
}

const (
	swakeup  = "sched_wakeup"
	sswitch  = "sched_switch"
	smigrate = "sched_migrate_task"
)

func emptyEventSet() *eventsetbuilder.Builder {
	return eventsetbuilder.NewBuilder().
		WithEventDescriptor(
			swakeup,
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio"),
			eventsetbuilder.Number("target_cpu")).
		WithEventDescriptor(
			sswitch,
			eventsetbuilder.Number("prev_pid"),
			eventsetbuilder.Text("prev_comm"),
			eventsetbuilder.Number("prev_prio"),
			eventsetbuilder.Number("prev_state"),
			eventsetbuilder.Number("next_pid"),
			eventsetbuilder.Text("next_comm"),
			eventsetbuilder.Number("next_prio")).
		WithEventDescriptor(
			smigrate,
			eventsetbuilder.Number("pid"),
			eventsetbuilder.Text("comm"),
			eventsetbuilder.Number("prio"),
			eventsetbuilder.Number("orig_cpu"),
			eventsetbuilder.Number("dest_cpu")).
		WithDefaultEventLoadersType(elpb.LoadersType_FAULT_TOLERANT)
}

// NewParser returns a new schedbt.Parser ready to parse trace rows.
func NewParser() *Parser {
	return &Parser{
		esb:              emptyEventSet(),
		prioAndCommByPid: map[int64]*prioAndComm{},
	}
}

// EventSet returns an EventSet proto constructed from the parsed trace rows.
func (p *Parser) EventSet() (*eventpb.EventSet, error) {
	es, errs := p.esb.EventSet()
	if len(errs) > 0 {
		log.Errorf("Builder encountered errors:")
		for _, err := range errs {
			log.Errorf("  %s", err)
		}
		return nil, status.Errorf(codes.Internal, "errors encountered while building EventSet, see log for details")
	}
	return es, nil
}

func recombineParts(parts []string, fields int) ([]string, bool) {
	if len(parts) < fields {
		return nil, false
	}
	if len(parts) > fields {
		lastPart := strings.Join(parts[fields-1:], ":")
		parts = parts[:fields-1]
		parts = append(parts, lastPart)
	}
	return parts, true
}

var badRow = func(row string) error {
	return status.Errorf(codes.InvalidArgument, "failed to parse row `%s`", row)
}
var noStartTS = func() error {
	return status.Errorf(codes.InvalidArgument, "missing start timestamp")
}

func (p *Parser) parseStartTimestamp(row string, parts []string) error {
	// ST:0:<0x start timestamp>
	var err error
	if len(parts) != 2 {
		return badRow(row)
	}
	p.startTimestamp, err = strconv.ParseInt(parts[1], 16, 64)
	if err != nil {
		return badRow(row)
	}
	p.hasStartTimestamp = true
	return nil
}

func (p *Parser) parseSwitch(row string, parts []string) error {
	// S:<0x timestamp>:<0x cpu>:<0x prev pid>:<0x prev state>:<0x next pid>
	var (
		err       error
		cpu       int64
		ts        int64
		prevState int64
		prevPID   int64
		nextPID   int64
		prevPac   *prioAndComm
		nextPac   *prioAndComm
		ok        bool
	)
	if !p.hasStartTimestamp {
		return noStartTS()
	}
	if len(parts) < 5 {
		return badRow(row)
	}
	for idx, v := range []*int64{&ts, &cpu, &prevPID, &prevState, &nextPID} {
		*v, err = strconv.ParseInt(parts[idx], 16, 64)
		if err != nil {
			return badRow(row)
		}
	}
	if prevPac, ok = p.prioAndCommByPid[prevPID]; !ok {
		prevPac = unknownPrioAndComm
	}
	if nextPac, ok = p.prioAndCommByPid[nextPID]; !ok {
		nextPac = unknownPrioAndComm
	}
	p.esb.WithEvent(sswitch, cpu, ts+p.startTimestamp, false,
		prevPID, prevPac.comm, prevPac.prio, prevState,
		nextPID, nextPac.comm, nextPac.prio)
	return nil
}

func (p *Parser) parsePIDInfo(row string, parts []string) error {
	var (
		err  error
		ts   int64
		pid  int64
		prio int64
		comm string
		ok   bool
	)
	// P:<0x timestamp>:<0x pid>:<0x prio>:<command name>
	parts, ok = recombineParts(parts, 4)
	if !ok {
		return badRow(row)
	}
	for idx, v := range []*int64{&ts, &pid, &prio} {
		*v, err = strconv.ParseInt(parts[idx], 16, 64)
		if err != nil {
			return badRow(row)
		}
	}
	comm = parts[3]
	p.prioAndCommByPid[pid] = &prioAndComm{
		prio: prio,
		comm: comm,
	}
	return nil
}

func (p *Parser) parseMigrate(row string, parts []string) error {
	// M:<0x timestamp>:<0x cpu>:<0x orig cpu>:<0x dest cpu>:<0x pid>
	var (
		err     error
		ts      int64
		pid     int64
		cpu     int64
		origCPU int64
		destCPU int64
		pac     *prioAndComm
		ok      bool
	)
	if !p.hasStartTimestamp {
		return noStartTS()
	}
	if len(parts) != 5 {
		return badRow(row)
	}
	for idx, v := range []*int64{&ts, &cpu, &origCPU, &destCPU, &pid} {
		*v, err = strconv.ParseInt(parts[idx], 16, 64)
		if err != nil {
			return badRow(row)
		}
	}
	if pac, ok = p.prioAndCommByPid[pid]; !ok {
		pac = unknownPrioAndComm
	}
	p.esb.WithEvent(smigrate, cpu, ts+p.startTimestamp, false,
		pid, pac.comm, pac.prio,
		origCPU, destCPU)
	return nil
}

func (p *Parser) parseWakeup(row string, parts []string) error {
	// W:<0x timestamp>:<0x cpu>:<0x target cpu>:<0x pid>
	var (
		err     error
		ts      int64
		pid     int64
		cpu     int64
		destCPU int64
		pac     *prioAndComm
		ok      bool
	)
	if !p.hasStartTimestamp {
		return noStartTS()
	}
	if len(parts) != 4 {
		return badRow(row)
	}
	for idx, v := range []*int64{&ts, &cpu, &destCPU, &pid} {
		*v, err = strconv.ParseInt(parts[idx], 16, 64)
		if err != nil {
			return badRow(row)
		}
	}
	if pac, ok = p.prioAndCommByPid[pid]; !ok {
		pac = unknownPrioAndComm
	}
	p.esb.WithEvent(swakeup, cpu, ts+p.startTimestamp, false,
		pid, pac.comm, pac.prio, destCPU)
	return nil
}

// Parse parses a sched trace gathered with sched.bt.
func (p *Parser) Parse(r *bufio.Reader) error {
	lineNum := 0
	for {
		rowBytes, isPrefix, err := r.ReadLine()
		row := string(rowBytes)
		lineNum++
		if err := func() error {
			if isPrefix {
				return status.Errorf(codes.InvalidArgument, "read a fragment")
			}
			if err != nil {
				if err == io.EOF {
					return status.Error(codes.OutOfRange, "")
				}
				return status.Errorf(codes.Internal, "failed to ReadLine: %s", err)
			}
			if len(row) == 0 {
				return badRow(row)
			}
			parts := strings.Split(row, ":")
			if len(parts) == 0 {
				return nil
			}
			rowType := parts[0]
			parts = parts[1:]
			switch rowType {
			case "ST":
				return p.parseStartTimestamp(row, parts)
			case "S":
				return p.parseSwitch(row, parts)
			case "P":
				return p.parsePIDInfo(row, parts)
			case "M":
				return p.parseMigrate(row, parts)
			case "W":
				return p.parseWakeup(row, parts)
			default:
				// Skip unparsable rows.
				return nil
			}
		}(); err != nil {
			s, ok := status.FromError(err)
			if !ok {
				return status.Errorf(codes.Internal, "at line %d, %s", lineNum, err)
			}
			if s.Code() == codes.OutOfRange {
				break
			}
			return status.Errorf(s.Code(), "at line %d, %s", lineNum, err)
		}
	}
	return nil
}
