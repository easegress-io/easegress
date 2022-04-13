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

package resilience

import (
	"fmt"

	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/v"
	"gopkg.in/yaml.v2"
)

type (
	// Kind contains the meta data and functions of a resilience kind.
	Kind struct {
		// Name is the name of the resilience kind.
		Name string

		// DefaultPolicy returns a spec for the resilience, with default values. The
		// function should always return a new spec copy, because the caller
		// may modify the returned spec.
		DefaultPolicy func() Policy
	}

	// Policy is the common interface of resilience policies
	Policy interface {
		// Name returns name.
		Name() string

		// Kind returns kind.
		Kind() string

		// Pipeline returns the name of the pipeline this filter belongs to.
		Pipeline() string

		// YAMLConfig returns the config in yaml format.
		YAMLConfig() string

		// baseSpec returns the pointer to the BaseSpec of the spec instance,
		// it is an internal function.
		baseSpec() *BaseSpec
	}

	// BaseSpec is the universal spec for all filters.
	BaseSpec struct {
		supervisor.MetaSpec `yaml:",inline"`
		pipeline            string
		yamlConfig          string
	}
)

// NewPolicy creates a resilience policy and validates it.
func NewPolicy(pipeline string, rawSpec interface{}) (policy Policy, err error) {
	defer func() {
		if r := recover(); r != nil {
			policy = nil
			err = fmt.Errorf("%v", r)
		}
	}()

	yamlBuff, err := yaml.Marshal(rawSpec)
	if err != nil {
		return nil, err
	}

	// Meta part.
	meta := supervisor.MetaSpec{}
	if err = yaml.Unmarshal(yamlBuff, &meta); err != nil {
		return nil, err
	}
	if vr := v.Validate(&meta); !vr.Valid() {
		return nil, fmt.Errorf("%v", vr)
	}

	// Resilience self part.
	kind := GetKind(meta.Kind)
	if kind == nil {
		return nil, fmt.Errorf("kind %s not found", meta.Kind)
	}
	policy = kind.DefaultPolicy()
	if err = yaml.Unmarshal(yamlBuff, policy); err != nil {
		return nil, err
	}
	if vr := v.Validate(policy); !vr.Valid() {
		return nil, fmt.Errorf("%v", vr)
	}

	yamlBuff, err = yaml.Marshal(policy)
	if err != nil {
		return nil, err
	}

	baseSpec := policy.baseSpec()
	baseSpec.pipeline = pipeline
	baseSpec.yamlConfig = string(yamlBuff)
	return
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

// YAMLConfig returns the config in yaml format.
func (s *BaseSpec) YAMLConfig() string {
	return s.yamlConfig
}

// baseSpec returns the pointer to the BaseSpec of the spec instance, it is an
// internal function. baseSpec returns the receiver directly, it is existed for
// getting the corresponding BaseSpec from a Spec interface.
func (s *BaseSpec) baseSpec() *BaseSpec {
	return s
}
