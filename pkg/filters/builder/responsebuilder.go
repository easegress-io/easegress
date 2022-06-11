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

package builder

import (
	"net/http"
	"runtime/debug"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
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
		return &ResponseBuilderSpec{}
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
		filters.BaseSpec `yaml:",inline"`
		Spec             `yaml:",inline"`
	}

	// ResponseInfo stores the information of a response.
	ResponseInfo struct {
		StatusCode int                 `yaml:"statusCode" jsonshema:"omitempty"`
		Headers    map[string][]string `yaml:"headers" jsonschema:"omitempty"`
		Body       string              `yaml:"body" jsonschema:"omitempty"`
	}
)

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
	rb.Builder.reload(&rb.spec.Spec)
}

// Handle builds request.
func (rb *ResponseBuilder) Handle(ctx *context.Context) (result string) {
	if rb.spec.SourceNamespace != "" {
		ctx.CopyResponse(rb.spec.SourceNamespace)
		return ""
	}

	defer func() {
		if err := recover(); err != nil {
			msgFmt := "panic: %s, stacktrace: %s\n"
			logger.Errorf(msgFmt, err, string(debug.Stack()))
			result = resultBuildErr
		}
	}()

	data, err := prepareBuilderData(ctx)
	if err != nil {
		logger.Warnf("prepareBuilderData failed: %v", err)
		return resultBuildErr
	}

	var ri ResponseInfo
	if err = rb.build(data, &ri); err != nil {
		msgFmt := "ResponseBuilder(%s): failed to build response info: %v"
		logger.Warnf(msgFmt, rb.Name(), err)
		return resultBuildErr
	}

	if ri.StatusCode == 0 {
		ri.StatusCode = http.StatusOK
	} else if ri.StatusCode < 200 || ri.StatusCode >= 600 {
		logger.Warnf("invalid status code: %d", ri.StatusCode)
		return resultBuildErr
	}

	stdResp := &http.Response{Header: http.Header{}, Body: http.NoBody}
	stdResp.StatusCode = ri.StatusCode

	for k, vs := range ri.Headers {
		for _, v := range vs {
			stdResp.Header.Add(k, v)
		}
	}

	// build body
	resp, _ := httpprot.NewResponse(stdResp)
	resp.SetPayload([]byte(ri.Body))

	ctx.SetOutputResponse(resp)
	return ""
}
