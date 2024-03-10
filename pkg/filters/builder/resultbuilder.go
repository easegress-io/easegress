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

package builder

import (
	"fmt"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
)

const (
	// ResultBuilderKind is the kind of ResultBuilder.
	ResultBuilderKind = "ResultBuilder"

	maxResults    = 10
	resultUnknown = "unknown"
)

var resultBuilderKind = &filters.Kind{
	Name:        ResultBuilderKind,
	Description: "ResultBuilder generates a string as the filter result",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &ResultBuilderSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &ResultBuilder{spec: spec.(*ResultBuilderSpec)}
	},
}

func init() {
	// TODO: all results are predefined at present, this should be changed
	// later.
	results := []string{}
	for i := 0; i < maxResults; i++ {
		results = append(results, fmt.Sprintf("result%d", i))
	}
	results = append(results, resultBuildErr, resultUnknown)
	resultBuilderKind.Results = results

	filters.Register(resultBuilderKind)
}

type (
	// ResultBuilder is filter ResultBuilder.
	ResultBuilder struct {
		spec *ResultBuilderSpec
		Builder
	}

	// ResultBuilderSpec is ResultBuilder Spec.
	ResultBuilderSpec struct {
		filters.BaseSpec `json:",inline"`
		Spec             `json:",inline"`
	}
)

// Validate validates the ResultBuilder Spec.
func (spec *ResultBuilderSpec) Validate() error {
	return spec.Spec.Validate()
}

// Name returns the name of the ResultBuilder filter instance.
func (rb *ResultBuilder) Name() string {
	return rb.spec.Name()
}

// Kind returns the kind of ResultBuilder.
func (rb *ResultBuilder) Kind() *filters.Kind {
	return resultBuilderKind
}

// Spec returns the spec used by the ResultBuilder
func (rb *ResultBuilder) Spec() filters.Spec {
	return rb.spec
}

// Init initializes ResultBuilder.
func (rb *ResultBuilder) Init() {
	rb.reload()
}

// Inherit inherits previous generation of ResultBuilder.
func (rb *ResultBuilder) Inherit(previousGeneration filters.Filter) {
	rb.Init()
}

func (rb *ResultBuilder) reload() {
	rb.Builder.reload(&rb.spec.Spec)
}

// Handle builds result.
func (rb *ResultBuilder) Handle(ctx *context.Context) (result string) {
	data, err := prepareBuilderData(ctx)
	if err != nil {
		logger.Warnf("prepareBuilderData failed: %v", err)
		return resultBuildErr
	}

	var r string
	if err = rb.build(data, &r); err != nil {
		msgFmt := "ResultBuilder(%s): failed to build request info: %v"
		logger.Warnf(msgFmt, rb.Name(), err)
		return resultBuildErr
	}

	if r == "" {
		return ""
	}

	for i := 0; i < maxResults; i++ {
		if r == resultBuilderKind.Results[i] {
			return r
		}
	}

	return resultUnknown
}
