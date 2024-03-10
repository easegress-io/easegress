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

// Package build contains utils for egbuilder build command.
package build

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

type Options struct {
	Compile Compile `json:"compile"`

	EGVersion    string `json:"egVersion"`
	RaceDetector bool   `json:"raceDetector"`
	SkipBuild    bool   `json:"skipBuild"`
	SkipCleanup  bool   `json:"skipCleanup"`

	BuildFlags []string `json:"buildFlags"`
	ModFlags   []string `json:"modFlags"`
}

type Config struct {
	Options `json:",inline"`

	Plugins []*Plugin `json:"plugins"`
	Output  string    `json:"output"`
}

// Plugin contains parameters for a plugin.
type Plugin struct {
	Module      string `json:"module"`
	Version     string `json:"version"`
	Replacement string `json:"replacement"`
}

// Compile contains parameters for compilation.
type Compile struct {
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
	ARM  string `json:"arm,omitempty"`
	Cgo  bool   `json:"cgo,omitempty"`
}

// CgoEnabled returns "1" if c.Cgo is true, "0" otherwise.
// This is used for setting the CGO_ENABLED env variable.
func (c Compile) CgoEnabled() string {
	if c.Cgo {
		return "1"
	}
	return "0"
}

// Init initializes the config. When use NewConfig, Init will be called automatically.
func (config *Config) Init() error {
	// check plugins
	for _, plugin := range config.Plugins {
		if plugin.Module == "" {
			return fmt.Errorf("empty module name in plugins")
		}
		if strings.HasPrefix(plugin.Replacement, ".") {
			repl, err := filepath.Abs(plugin.Replacement)
			if err != nil {
				return fmt.Errorf("get absolute path of %s in module %s failed: %v", plugin.Replacement, plugin.Module, err)
			}
			plugin.Replacement = repl
		}
	}

	// check output
	if config.Output == "" {
		output, err := getBuildOutputFile()
		if err != nil {
			return fmt.Errorf("get build output file failed: %v", err)
		}
		config.Output = output
	}
	if strings.HasPrefix(config.Output, ".") {
		output, err := filepath.Abs(config.Output)
		if err != nil {
			return fmt.Errorf("get absolute path of output file %s failed: %v", config.Output, err)
		}
		config.Output = output
	}

	// check compile
	if config.Compile.OS == "" {
		config.Compile.OS = os.Getenv("GOOS")
	}
	if config.Compile.Arch == "" {
		config.Compile.Arch = os.Getenv("GOARCH")
	}
	if config.Compile.ARM == "" {
		config.Compile.ARM = os.Getenv("GOARM")
	}

	fmt.Printf("Build easegress-server with config\n")
	fmt.Printf("  %#v\n", config.Options)
	for _, p := range config.Plugins {
		fmt.Printf("  plugin: %#v\n", p)
	}
	fmt.Printf("  output: %s\n", config.Output)
	return nil
}

func NewConfig(filename string) (*Config, error) {
	config := &Config{}
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read file %s failed: %v", filename, err)
	}
	err = codectool.UnmarshalYAML(data, config)
	if err != nil {
		return nil, fmt.Errorf("unmarshal yaml file %s failed: %v", filename, err)
	}

	err = config.Init()
	if err != nil {
		return nil, fmt.Errorf("init config failed: %v", err)
	}
	return config, nil
}

func getBuildOutputFile() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current working directory failed: %v", err)
	}
	output := filepath.Join(cwd, "easegress-server")
	if runtime.GOOS == "windows" {
		output += ".exe"
	}
	return output, nil
}
