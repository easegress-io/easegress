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
	// RequestBuilderKind is the kind of RequestBuilder.
	RequestBuilderKind = "RequestBuilder"
)

var requestBuilderKind = &filters.Kind{
	Name:        RequestBuilderKind,
	Description: "RequestBuilder builds a request",
	Results:     []string{resultBuildErr},
	DefaultSpec: func() filters.Spec {
		return &RequestBuilderSpec{Protocol: "http"}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &RequestBuilder{spec: spec.(*RequestBuilderSpec)}
	},
}

func init() {
	filters.Register(requestBuilderKind)
}

type (
	// RequestBuilder is filter RequestBuilder.
	RequestBuilder struct {
		spec *RequestBuilderSpec
		Builder
	}

	// RequestBuilderSpec is RequestBuilder Spec.
	RequestBuilderSpec struct {
		filters.BaseSpec `json:",inline"`
		Spec             `json:",inline"`
		SourceNamespace  string `json:"sourceNamespace,omitempty"`
		Protocol         string `json:"protocol,omitempty"`
	}
)

// Validate validates the RequestBuilder Spec.
func (spec *RequestBuilderSpec) Validate() error {
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

// Name returns the name of the RequestBuilder filter instance.
func (rb *RequestBuilder) Name() string {
	return rb.spec.Name()
}

// Kind returns the kind of RequestBuilder.
func (rb *RequestBuilder) Kind() *filters.Kind {
	return requestBuilderKind
}

// Spec returns the spec used by the RequestBuilder
func (rb *RequestBuilder) Spec() filters.Spec {
	return rb.spec
}

// Init initializes RequestBuilder.
func (rb *RequestBuilder) Init() {
	rb.reload()
}

// Inherit inherits previous generation of RequestBuilder.
func (rb *RequestBuilder) Inherit(previousGeneration filters.Filter) {
	rb.Init()
}

func (rb *RequestBuilder) reload() {
	if rb.spec.SourceNamespace == "" {
		rb.Builder.reload(&rb.spec.Spec)
	}
}

// Handle builds request.
func (rb *RequestBuilder) Handle(ctx *context.Context) (result string) {
	if rb.spec.SourceNamespace != "" {
		ctx.CopyRequest(rb.spec.SourceNamespace)
		return ""
	}

	data, err := prepareBuilderData(ctx)
	if err != nil {
		logger.Warnf("prepareBuilderData failed: %v", err)
		return resultBuildErr
	}

	p := protocols.Get(rb.spec.Protocol)
	ri := p.NewRequestInfo()
	if err = rb.build(data, ri); err != nil {
		msgFmt := "RequestBuilder(%s): failed to build request info: %v"
		logger.Warnf(msgFmt, rb.Name(), err)
		return resultBuildErr
	}

	req, err := p.BuildRequest(ri)
	if err != nil {
		logger.Warnf(err.Error())
		return resultBuildErr
	}

	ctx.SetOutputRequest(req)
	return ""
}
