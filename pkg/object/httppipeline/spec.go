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

	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/yamltool"
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
		Name     string `yaml:"name" jsonschema:"required,format=urlname"`
		Kind     string `yaml:"kind" jsonschema:"required"`
		Pipeline string `yaml:"-" jsonschema:"-"`
	}
)

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
	s.rawSpec = rawSpec
	s.yamlConfig = yamlConfig
	s.rootFilter = rootFilter

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
