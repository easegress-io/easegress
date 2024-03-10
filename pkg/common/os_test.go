//go:build !windows
// +build !windows

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

package common

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

func TestNonZeroExit(t *testing.T) {
	if os.Getenv("BE_TestNonZeroExit") == "1" {
		Exit(1, "error")
		return
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command(os.Args[0], "-test.run=TestNonZeroExit")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "BE_TestNonZeroExit=1")
	err := cmd.Run()
	e, ok := err.(*exec.ExitError)
	if ok && !e.Success() && stderr.String() != "" && stdout.String() == "" {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 1", err)
}

func TestZeroExit(t *testing.T) {
	if os.Getenv("BE_TestZeroExit") == "1" {
		Exit(0, "everythingok")
		return
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command(os.Args[0], "-test.run=TestZeroExit")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "BE_TestZeroExit=1")
	err := cmd.Run()
	if err == nil && stdout.String() != "" && stderr.String() == "" {
		return
	}
	t.Fatalf("process ran with err %v, want exit status 0", err)
}
