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

// Package filters implements common functionality of filters.
package filters

import (
	"fmt"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/resilience"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/v"
)

type (
	// Kind contains the meta data and functions of a filter kind.
	Kind struct {
		// Name is the name of the filter kind.
		Name string

		// Description is the description of the filter.
		Description string

		// Results list all possible results of the filter, except the normal
		// result (i.e. empty string).
		Results []string

		// CreateInstance creates a new filter instance of the kind.
		CreateInstance func(spec Spec) Filter

		// DefaultSpec returns a spec for the filter, with default values. The
		// function should always return a new spec copy, because the caller
		// may modify the returned spec.
		DefaultSpec func() Spec
	}

	// Filter is the interface of filters handling traffic of various protocols.
	Filter interface {
		// Name returns the name of the filter.
		Name() string

		// Kind returns the kind of the filter, caller should never modify the
		// return value.
		Kind() *Kind

		// Spec returns the Spec of the filter instance.
		Spec() Spec

		// Init initializes the Filter.
		Init()

		// Inherit also initializes the Filter, the difference from Init is it
		// inherit something from the previousGeneration, but Inherit does NOT
		// handle the lifecycle of previousGeneration.
		Inherit(previousGeneration Filter)

		// Handle handles one HTTP request, all possible results
		// need be registered in Results.
		Handle(*context.Context) (result string)

		// Status returns its runtime status.
		// It could return nil.
		Status() interface{}

		// Close closes itself.
		Close()
	}

	// Resiliencer is the interface of objects that accept resilience policies.
	Resiliencer interface {
		InjectResiliencePolicy(policies map[string]resilience.Policy)
	}

	// Spec is the common interface of filter specs
	Spec interface {
		// Super returns supervisor
		Super() *supervisor.Supervisor

		// Name returns name.
		Name() string

		// Kind returns kind.
		Kind() string

		// Pipeline returns the name of the pipeline this filter belongs to.
		Pipeline() string

		// JSONConfig returns the config in json format.
		JSONConfig() string

		// baseSpec returns the pointer to the BaseSpec of the spec instance,
		// it is an internal function.
		baseSpec() *BaseSpec
	}

	// BaseSpec is the universal spec for all filters.
	BaseSpec struct {
		supervisor.MetaSpec `json:",inline"`
		super               *supervisor.Supervisor
		pipeline            string
		jsonConfig          string
	}
)

// NewSpec creates a filter spec and validates it.
func NewSpec(super *supervisor.Supervisor, pipeline string, rawSpec interface{}) (spec Spec, err error) {
	defer func() {
		if r := recover(); r != nil {
			spec = nil
			err = fmt.Errorf("%v", r)
		}
	}()

	jsonConfig, err := codectool.MarshalJSON(rawSpec)
	if err != nil {
		return nil, err
	}

	// Meta part.
	meta := supervisor.MetaSpec{Version: supervisor.DefaultSpecVersion}
	if err = codectool.Unmarshal(jsonConfig, &meta); err != nil {
		return nil, err
	}
	if vr := v.Validate(&meta); !vr.Valid() {
		return nil, fmt.Errorf("%v", vr)
	}

	// Filter self part.
	kind := GetKind(meta.Kind)
	if kind == nil {
		return nil, fmt.Errorf("kind %s not found", meta.Kind)
	}
	spec = kind.DefaultSpec()
	if err = codectool.Unmarshal(jsonConfig, spec); err != nil {
		return nil, err
	}
	// TODO: Make the invalid part more accurate. e,g:
	// filters: jsonschemaErrs:
	// - 'policies.0: name is required'
	// to
	// filters: jsonschemaErrs:
	// - 'rateLimiter.policies.0: name is required'
	if vr := v.Validate(spec); !vr.Valid() {
		return nil, fmt.Errorf("%v", vr)
	}

	jsonConfig, err = codectool.MarshalJSON(spec)
	if err != nil {
		return nil, err
	}

	baseSpec := spec.baseSpec()
	baseSpec.super = super
	baseSpec.pipeline = pipeline
	baseSpec.jsonConfig = string(jsonConfig)
	return
}

// Super returns super.
func (s *BaseSpec) Super() *supervisor.Supervisor {
	return s.super
}

// Name returns name.
func (s *BaseSpec) Name() string {
	return s.MetaSpec.Name
}

// Kind returns kind.
func (s *BaseSpec) Kind() string {
	return s.MetaSpec.Kind
}

// Pipeline returns the name of the pipeline this filter belongs to.
func (s *BaseSpec) Pipeline() string {
	return s.pipeline
}

// JSONConfig returns the config in json format.
func (s *BaseSpec) JSONConfig() string {
	return s.jsonConfig
}

// baseSpec returns the pointer to the BaseSpec of the spec instance, it is an
// internal function. baseSpec returns the receiver directly, it is existed for
// getting the corresponding BaseSpec from a Spec interface.
func (s *BaseSpec) baseSpec() *BaseSpec {
	return s
}
