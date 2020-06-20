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
package traceparser

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
)

var (
	cpuRe = regexp.MustCompile(`cpu(\d+)$`)
)

// WalkPerCPUDir walks the input directory looking for files of the format
// cpu\d+. For each file it calls process with a bufio.Reader and the number
// found.
func WalkPerCPUDir(traceDir string, errorOnUnknown bool, process func(reader *bufio.Reader, cpu int64) error) error {
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
			if err := process(reader, cpu); err != nil {
				return err
			}
		} else if errorOnUnknown {
			return fmt.Errorf("unknown file in trace directory: %s", filePath)
		}
		return nil
	})
	return err
}
