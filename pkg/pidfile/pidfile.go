/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package pidfile provides pidfile related functions.
package pidfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
)

const (
	pidfileName = "easegress.pid"
)

var pidfilePath string

// Write writes pidfile.
func Write(opt *option.Options) error {
	pidfilePath = filepath.Join(opt.AbsHomeDir, pidfileName)

	err := os.WriteFile(pidfilePath, []byte(fmt.Sprintf("%d", os.Getpid())), 0o644)
	if err != nil {
		logger.Errorf("write %s failed: %s", pidfilePath, err)
		return err
	}

	return nil
}

// Read reads pidfile and return its value.
func Read(opt *option.Options) (int, error) {
	pidfilePath = filepath.Join(opt.AbsHomeDir, pidfileName)

	data, err := os.ReadFile(pidfilePath)
	if err != nil {
		logger.Errorf("read %s failed: %s", pidfilePath, err)
		return 0, err
	}

	return strconv.Atoi(string(data))
}
