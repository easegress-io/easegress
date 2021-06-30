/*
 * Copyright (c) 2017, MegaEase
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

package httppipeline

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/v"
)

type (
	// FilterSpec is the universal spec for all filters.
	FilterSpec struct {
		super *supervisor.Supervisor

		rawSpec    map[string]interface{}
		yamlConfig string
		meta       *FilterMetaSpec
		filterSpec interface{}
		rootFilter Filter
	}

	// FilterMetaSpec is metadata for all specs.
	FilterMetaSpec struct {
		Name string `yaml:"name" jsonschema:"required,format=urlname"`
		Kind string `yaml:"kind" jsonschema:"required"`
	}
)

// NewFilterSpec creates a filter spec and validates it.
func NewFilterSpec(rawSpec map[string]interface{}, super *supervisor.Supervisor) (*FilterSpec, error) {
	s := &FilterSpec{
		super:   super,
		rawSpec: rawSpec,
	}

	yamlConfig, err := yaml.Marshal(rawSpec)
	if err != nil {
		return nil, fmt.Errorf("marshal %#v to yaml failed: %v", rawSpec, err)
	}

	s.yamlConfig = string(yamlConfig)

	meta := &FilterMetaSpec{}
	err = yaml.Unmarshal(yamlConfig, meta)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed: %v", err)
	}

	rootFilter, exists := filterRegistry[meta.Kind]
	if !exists {
		return nil, fmt.Errorf("kind %s not found", meta.Kind)
	}

	s.meta, s.filterSpec, s.rootFilter = meta, rootFilter.DefaultSpec(), rootFilter

	err = yaml.Unmarshal(yamlConfig, s.filterSpec)
	if err != nil {
		return nil, fmt.Errorf("unmarshal failed: %v", err)
	}

	vr := v.Validate(s.filterSpec, []byte(yamlConfig))
	if !vr.Valid() {
		return nil, fmt.Errorf("%v", vr.Error())
	}

	return s, nil
}

func (s *FilterSpec) Super() *supervisor.Supervisor {
	return s.super
}

// Name returns name.
func (s *FilterSpec) Name() string { return s.meta.Name }

// Kind returns kind.
func (s *FilterSpec) Kind() string { return s.meta.Kind }

// YAMLConfig returns the config in yaml format.
func (s *FilterSpec) YAMLConfig() string {
	return s.yamlConfig
}

// RawSpec returns raw spec in type map[string]interface{}.
func (s *FilterSpec) RawSpec() map[string]interface{} {
	return s.rawSpec
}

// FilterSpec returns the filter spec in its own type.
func (s *FilterSpec) FilterSpec() interface{} {
	return s.filterSpec
}

// RootFilter returns the root filter of the filter spec.
func (s *FilterSpec) RootFilter() Filter {
	return s.rootFilter
}
