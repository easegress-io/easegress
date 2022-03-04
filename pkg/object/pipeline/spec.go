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

package pipeline

import (
	"fmt"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/yamltool"
	"github.com/megaease/easegress/pkg/v"
)

type (
	// Spec describes the Pipeline.
	Spec struct {
		Name     string                   `yaml:"-" jsonschema:"-"`
		Protocol context.Protocol         `yaml:"protocol" jsonschema:"required"`
		Flow     []Flow                   `yaml:"flow" jsonschema:"omitempty"`
		Filters  []map[string]interface{} `yaml:"filters" jsonschema:"required"`
	}

	// Flow controls the flow of pipeline.
	Flow struct {
		Filter string `yaml:"filter" jsonschema:"required,format=urlname"`
	}

	// FilterSpec is the universal spec for all filters.
	FilterSpec struct {
		super *supervisor.Supervisor

		yamlConfig string
		meta       *FilterMetaSpec
		filterSpec interface{}
	}

	// FilterMetaSpec is metadata for all specs.
	FilterMetaSpec struct {
		Name     string           `yaml:"name" jsonschema:"required,format=urlname"`
		Kind     string           `yaml:"kind" jsonschema:"required"`
		Pipeline string           `yaml:"-" jsonschema:"-"`
		Protocol context.Protocol `yaml:"-" jsonschema:"-"`
	}

	// Status is the status of HTTPPipeline.
	Status struct {
		Health string `yaml:"health"`

		Filters map[string]interface{} `yaml:"filters"`
	}
)

func extractFiltersData(config []byte) interface{} {
	var whole map[string]interface{}
	yamltool.Unmarshal(config, &whole)
	return whole["filters"]
}

// creates FilterSpecs from a list of filters
func filtersToFilterSpecs(filters []map[string]interface{}, super *supervisor.Supervisor) (map[string]*FilterSpec, []string) {
	filterMap := make(map[string]*FilterSpec)
	filterNames := []string{}
	for _, filter := range filters {
		spec, err := NewFilterSpec(filter, super)
		if err != nil {
			panic(err)
		}
		if _, exists := filterMap[spec.Name()]; exists {
			panic(fmt.Errorf("conflict name: %s", spec.Name()))
		}
		filterMap[spec.Name()] = spec
		filterNames = append(filterNames, spec.Name())
	}
	return filterMap, filterNames
}

// Validate validates Spec.
func (s Spec) Validate() (err error) {
	errPrefix := "filters"
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: %s", errPrefix, r)
		}
	}()

	config := yamltool.Marshal(s)

	filtersData := extractFiltersData(config)
	if filtersData == nil {
		return fmt.Errorf("filters is required")
	}

	filterSpecs, _ := filtersToFilterSpecs(
		s.Filters,
		nil, /*NOTE: Nil supervisor is fine in spec validating phrase.*/
	)

	for _, spec := range filterSpecs {
		kind := spec.Kind()
		filter, exists := filterRegistry[kind]
		if !exists {
			panic(fmt.Errorf("kind %s not found", kind))
		}
		protocols, err := getProtocols(filter)
		if err != nil {
			panic(fmt.Errorf("filter %v get protocols failed, %v", kind, err))
		}
		if _, ok := protocols[s.Protocol]; !ok {
			panic(fmt.Errorf("filter %v not support pipeline protocol %s", spec.Name(), s.Protocol))
		}
	}

	errPrefix = "flow"
	filters := make(map[string]struct{})
	for _, f := range s.Flow {
		if _, exists := filters[f.Filter]; exists {
			panic(fmt.Errorf("repeated filter %s", f.Filter))
		}
	}
	return nil
}

// NewFilterSpec creates a filter spec and validates it.
func NewFilterSpec(originalRawSpec map[string]interface{}, super *supervisor.Supervisor) (
	s *FilterSpec, err error) {

	s = &FilterSpec{super: super}

	defer func() {
		if r := recover(); r != nil {
			s = nil
			err = fmt.Errorf("%v", r)
		} else {
			err = nil
		}
	}()

	yamlBuff := yamltool.Marshal(originalRawSpec)

	// Meta part.
	meta := &FilterMetaSpec{}
	yamltool.Unmarshal(yamlBuff, meta)
	verr := v.Validate(meta)
	if !verr.Valid() {
		panic(verr)
	}

	// Filter self part.
	rootFilter, exists := filterRegistry[meta.Kind]
	if !exists {
		panic(fmt.Errorf("kind %s not found", meta.Kind))
	}
	filterSpec := rootFilter.DefaultSpec()
	yamltool.Unmarshal(yamlBuff, filterSpec)
	verr = v.Validate(filterSpec)
	if !verr.Valid() {
		// TODO: Make the invalid part more accurate. e,g:
		// filters: jsonschemaErrs:
		// - 'policies.0: name is required'
		// to
		// filters: jsonschemaErrs:
		// - 'rateLimiter.policies.0: name is required'
		panic(verr)
	}

	// Build final yaml config and raw spec.
	var rawSpec map[string]interface{}
	filterBuff := yamltool.Marshal(filterSpec)
	yamltool.Unmarshal(filterBuff, &rawSpec)

	metaBuff := yamltool.Marshal(meta)
	yamltool.Unmarshal(metaBuff, &rawSpec)

	yamlConfig := string(yamltool.Marshal(rawSpec))

	s.meta = meta
	s.filterSpec = filterSpec
	s.yamlConfig = yamlConfig

	return
}

// Super returns super
func (s *FilterSpec) Super() *supervisor.Supervisor {
	return s.super
}

// Name returns name.
func (s *FilterSpec) Name() string { return s.meta.Name }

// Kind returns kind.
func (s *FilterSpec) Kind() string { return s.meta.Kind }

// Pipeline returns the name of the pipeline this filter belongs to.
func (s *FilterSpec) Pipeline() string { return s.meta.Pipeline }

// Protocol return protocol for this filter
func (s *FilterSpec) Protocol() context.Protocol { return s.meta.Protocol }

// YAMLConfig returns the config in yaml format.
func (s *FilterSpec) YAMLConfig() string {
	return s.yamlConfig
}

// FilterSpec returns the filter spec in its own type.
func (s *FilterSpec) FilterSpec() interface{} {
	return s.filterSpec
}
