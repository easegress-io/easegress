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

// Package gen generates codes for egbuilder.
package gen

import (
	"fmt"
	"os"
	"path"

	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	KindName       = "Kind"
	SpecName       = "Spec"
	configFileName = ".egbuilderrc"
	apiName        = "API"
	yourCodeHere   = "your code here\n"
)

// Config is the configuration for generate files.
type Config struct {
	Repo        string   `json:"repo"`
	Controllers []string `json:"controllers"`
	Filters     []string `json:"filters"`
}

// Save saves the config to file.
func (c *Config) Save(dir string) error {
	fileName := path.Join(dir, configFileName)
	yamlData, err := codectool.MarshalYAML(c)
	if err != nil {
		return fmt.Errorf("marshal config file failed, %s", err.Error())
	}
	return os.WriteFile(fileName, yamlData, os.ModePerm)
}

// Load loads the config from file.
func (c *Config) Load(dir string) error {
	fileName := path.Join(dir, configFileName)
	yamlData, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("read config file %s failed, %s", fileName, err.Error())
	}
	err = codectool.UnmarshalYAML(yamlData, c)
	if err != nil {
		return fmt.Errorf("unmarshal config file %s failed, %s", fileName, err.Error())
	}
	return nil
}

// CheckDuplicate checks if the config has duplicate controllers or filters.
func (c *Config) CheckDuplicate(conf *Config) error {
	arrToSet := func(arr []string) map[string]struct{} {
		set := make(map[string]struct{})
		for _, a := range arr {
			set[a] = struct{}{}
		}
		return set
	}

	filters := arrToSet(c.Filters)
	for _, f := range conf.Filters {
		if _, ok := filters[f]; ok {
			return fmt.Errorf("filter %s already exists", f)
		}
	}

	controllers := arrToSet(c.Controllers)
	for _, r := range conf.Controllers {
		if _, ok := controllers[r]; ok {
			return fmt.Errorf("controller %s already exists", r)
		}
	}
	return nil
}

func (c *Config) GenFilters(dir string) error {
	for _, f := range c.Filters {
		err := os.MkdirAll(getModulePath(dir, moduleFilter, f, false), os.ModePerm)
		if err != nil {
			return fmt.Errorf("make directory for filter %s failed, %s", f, err.Error())
		}
		filter := CreateFilter(f)
		err = filter.Save(getFileName(dir, moduleFilter, f, f))
		if err != nil {
			return fmt.Errorf("save filter %s failed, %s", f, err.Error())
		}
	}
	return nil
}

func (c *Config) GenControllers(dir string) error {
	for _, r := range c.Controllers {
		err := os.MkdirAll(getModulePath(dir, moduleController, r, false), os.ModePerm)
		if err != nil {
			return fmt.Errorf("make directory for controller %s failed, %s", r, err.Error())
		}
		controller := CreateController(r)
		err = controller.Save(getFileName(dir, moduleController, r, r))
		if err != nil {
			return fmt.Errorf("save controller %s failed, %s", r, err.Error())
		}
	}
	return nil
}

func (c *Config) GenRegistry(dir string) error {
	registryPath := getModulePath(dir, moduleRegistry, "", false)
	err := os.MkdirAll(registryPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("make directory %s failed: %s", registryPath, err.Error())
	}
	file := CreateRegistry(c)
	err = file.Save(getFileName(dir, moduleRegistry, "", "registry"))
	if err != nil {
		return fmt.Errorf("generate registry file failed: %s", err.Error())
	}
	return nil
}
