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

// Package resilience implements the resilience policies.
package resilience

import (
	"context"
	"fmt"

	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/v"
)

type (
	// Kind contains the meta data and functions of a resilience kind.
	Kind struct {
		// Name is the name of the resilience kind.
		Name string

		// DefaultPolicy returns a spec for the resilience, with default values.
		// The function should always return a new spec copy, because the caller
		// may modify the returned spec.
		DefaultPolicy func() Policy
	}

	// HandlerFunc is the handler function to be wrapped by resilience policies.
	HandlerFunc func(ctx context.Context) error

	// Wrapper is the wrapper to wrap a handler function.
	Wrapper interface {
		// Wrap wraps the handler function with the policy.
		Wrap(HandlerFunc) HandlerFunc
	}

	// Policy is the common interface of resilience policies
	Policy interface {
		// Name returns name.
		Name() string

		// Kind returns kind.
		Kind() string

		// CreateWrapper creates a Wrapper.
		CreateWrapper() Wrapper
	}

	// BaseSpec is the universal spec for all resilience policies.
	BaseSpec struct {
		supervisor.MetaSpec `json:",inline"`
	}
)

// NewPolicy creates a resilience policy and validates it.
func NewPolicy(rawSpec interface{}) (policy Policy, err error) {
	defer func() {
		if r := recover(); r != nil {
			policy = nil
			err = fmt.Errorf("%v", r)
		}
	}()

	jsonBuff, err := codectool.MarshalJSON(rawSpec)
	if err != nil {
		return nil, err
	}

	// Meta part.
	meta := supervisor.MetaSpec{Version: supervisor.DefaultSpecVersion}
	if err = codectool.Unmarshal(jsonBuff, &meta); err != nil {
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
	if err = codectool.Unmarshal(jsonBuff, policy); err != nil {
		return nil, err
	}
	if vr := v.Validate(policy); !vr.Valid() {
		return nil, fmt.Errorf("%v", vr)
	}

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
