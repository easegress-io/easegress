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
	"github.com/megaease/easegress/v2/pkg/protocols"
)

const (
	// ResponseBuilderKind is the kind of ResponseBuilder.
	ResponseBuilderKind = "ResponseBuilder"
)

var responseBuilderKind = &filters.Kind{
	Name:        ResponseBuilderKind,
	Description: "ResponseBuilder builds a response",
	Results:     []string{resultBuildErr},
	DefaultSpec: func() filters.Spec {
		return &ResponseBuilderSpec{Protocol: "http"}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &ResponseBuilder{spec: spec.(*ResponseBuilderSpec)}
	},
}

func init() {
	filters.Register(responseBuilderKind)
}

type (
	// ResponseBuilder is filter ResponseBuilder.
	ResponseBuilder struct {
		spec *ResponseBuilderSpec
		Builder
	}

	// ResponseBuilderSpec is ResponseBuilder Spec.
	ResponseBuilderSpec struct {
		filters.BaseSpec `json:",inline"`
		Spec             `json:",inline"`
		SourceNamespace  string `json:"sourceNamespace,omitempty"`
		Protocol         string `json:"protocol,omitempty"`
	}
)

// Validate validates the ResponseBuilder Spec.
func (spec *ResponseBuilderSpec) Validate() error {
	if protocols.Get(spec.Protocol) == nil {
		return fmt.Errorf("unknown protocol: %s", spec.Protocol)
	}

	if spec.SourceNamespace == "" && spec.Template == "" {
		return fmt.Errorf("sourceNamespace or template must be specified")
	}

	if spec.SourceNamespace != "" && spec.Template != "" {
		return fmt.Errorf("sourceNamespace and template cannot be specified at the same time")
	}

	return spec.Spec.Validate()
}

// Name returns the name of the ResponseBuilder filter instance.
func (rb *ResponseBuilder) Name() string {
	return rb.spec.Name()
}

// Kind returns the kind of ResponseBuilder.
func (rb *ResponseBuilder) Kind() *filters.Kind {
	return responseBuilderKind
}

// Spec returns the spec used by the ResponseBuilder
func (rb *ResponseBuilder) Spec() filters.Spec {
	return rb.spec
}

// Init initializes ResponseBuilder.
func (rb *ResponseBuilder) Init() {
	rb.reload()
}

// Inherit inherits previous generation of ResponseBuilder.
func (rb *ResponseBuilder) Inherit(previousGeneration filters.Filter) {
	rb.Init()
}

func (rb *ResponseBuilder) reload() {
	if rb.spec.SourceNamespace == "" {
		rb.Builder.reload(&rb.spec.Spec)
	}
}

// Handle builds request.
func (rb *ResponseBuilder) Handle(ctx *context.Context) (result string) {
	if rb.spec.SourceNamespace != "" {
		ctx.CopyResponse(rb.spec.SourceNamespace)
		return ""
	}

	data, err := prepareBuilderData(ctx)
	if err != nil {
		logger.Warnf("prepareBuilderData failed: %v", err)
		return resultBuildErr
	}

	p := protocols.Get(rb.spec.Protocol)
	ri := p.NewResponseInfo()
	if err = rb.build(data, ri); err != nil {
		msgFmt := "ResponseBuilder(%s): failed to build response info: %v"
		logger.Warnf(msgFmt, rb.Name(), err)
		return resultBuildErr
	}

	resp, err := p.BuildResponse(ri)
	if err != nil {
		logger.Warnf(err.Error())
		return resultBuildErr
	}

	ctx.SetOutputResponse(resp)
	return ""
}
