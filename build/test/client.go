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

package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func egctlWithServer(server string, args ...string) *exec.Cmd {
	egctl := os.Getenv("EGCTL")
	if egctl == "" {
		egctl = "egctl"
	}
	cmd := exec.Command(egctl, args...)
	cmd.Args = append(cmd.Args, "--server", server)
	return cmd
}

func egctlCmd(args ...string) *exec.Cmd {
	return egctlWithServer("http://127.0.0.1:12381", args...)
}

func runCmd(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func createResource(yamlFile string) error {
	cmd := egctlCmd("create", "-f", "-")
	cmd.Stdin = strings.NewReader(yamlFile)
	_, stderr, err := runCmd(cmd)
	if err != nil || stderr != "" {
		return fmt.Errorf("create resource failed\nstderr: %v\nerr: %v", stderr, err)
	}
	return nil
}

func applyResource(yamlFile string) error {
	cmd := egctlCmd("apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yamlFile)
	_, stderr, err := runCmd(cmd)
	if err != nil || stderr != "" {
		return fmt.Errorf("apply resource failed\nstderr: %v\nerr: %v", stderr, err)
	}
	return nil
}

func deleteResource(kind string, args ...string) error {
	cmd := egctlCmd("delete", kind)
	if len(args) > 0 {
		cmd.Args = append(cmd.Args, args...)
	}
	_, stderr, err := runCmd(cmd)
	if err != nil || stderr != "" {
		return fmt.Errorf("delete resource failed\nstderr: %v\nerr: %v", stderr, err)
	}
	return nil
}

func describeResource(kind string, args ...string) (string, error) {
	cmd := egctlCmd("describe", kind)
	if len(args) > 0 {
		cmd.Args = append(cmd.Args, args...)
	}
	stdout, stderr, err := runCmd(cmd)
	if err != nil || stderr != "" {
		return "", fmt.Errorf("describe resource failed\nstderr: %v\nerr: %v", stderr, err)
	}
	return stdout, nil
}

func getResource(kind string, args ...string) (string, error) {
	cmd := egctlCmd("get", kind)
	if len(args) > 0 {
		cmd.Args = append(cmd.Args, args...)
	}
	stdout, stderr, err := runCmd(cmd)
	if err != nil || stderr != "" {
		return "", fmt.Errorf("describe resource failed\nstderr: %v\nerr: %v", stderr, err)
	}
	return stdout, nil
}

func matchTable(array []string, output string) bool {
	// Join the elements of the array with the regular expression pattern to match one or more tab characters
	pattern := strings.Join(array, `\t+`)

	// Compile the regular expression pattern
	re := regexp.MustCompile(pattern)

	// Check if the regular expression matches the output string
	return re.MatchString(output)
}

func egbuilderCmd(args ...string) *exec.Cmd {
	egbuilder := os.Getenv("EGBUILDER")
	if egbuilder == "" {
		egbuilder = "egbuilder"
	}
	cmd := exec.Command(egbuilder, args...)
	return cmd
}
