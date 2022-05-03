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

package httpbuilder

import (
	"net/http"
	"runtime/debug"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

const (
	// HTTPResponseBuilderKind is the kind of HTTPResponseBuilder.
	HTTPResponseBuilderKind = "HTTPResponseBuilder"
)

var httpResponseBuilderKind = &filters.Kind{
	Name:        HTTPResponseBuilderKind,
	Description: "HTTPResponseBuilder builds an HTTP response",
	Results:     []string{resultBuildErr},
	DefaultSpec: func() filters.Spec {
		return &HTTPResponseBuilderSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &HTTPResponseBuilder{spec: spec.(*HTTPResponseBuilderSpec)}
	},
}

func init() {
	filters.Register(httpResponseBuilderKind)
}

type (
	// HTTPResponseBuilder is filter HTTPResponseBuilder.
	HTTPResponseBuilder struct {
		spec *HTTPResponseBuilderSpec
		HTTPBuilder
	}

	// HTTPResponseBuilderSpec is HTTPResponseBuilder Spec.
	HTTPResponseBuilderSpec struct {
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

// Name returns the name of the HTTPResponseBuilder filter instance.
func (rb *HTTPResponseBuilder) Name() string {
	return rb.spec.Name()
}

// Kind returns the kind of HTTPResponseBuilder.
func (rb *HTTPResponseBuilder) Kind() *filters.Kind {
	return httpResponseBuilderKind
}

// Spec returns the spec used by the HTTPResponseBuilder
func (rb *HTTPResponseBuilder) Spec() filters.Spec {
	return rb.spec
}

// Init initializes HTTPResponseBuilder.
func (rb *HTTPResponseBuilder) Init() {
	rb.reload()
}

// Inherit inherits previous generation of HTTPResponseBuilder.
func (rb *HTTPResponseBuilder) Inherit(previousGeneration filters.Filter) {
	previousGeneration.Close()
	rb.Init()
}

func (rb *HTTPResponseBuilder) reload() {
	rb.HTTPBuilder.reload(&rb.spec.Spec)
}

// Handle builds request.
func (rb *HTTPResponseBuilder) Handle(ctx *context.Context) (result string) {
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
		msgFmt := "HTTPResponseBuilder(%s): failed to build response info: %v"
		logger.Warnf(msgFmt, rb.Name(), err)
		return resultBuildErr
	}

	if ri.StatusCode == 0 {
		ri.StatusCode = http.StatusOK
	} else if ri.StatusCode < 200 || ri.StatusCode >= 600 {
		logger.Warnf("invalid status code: %d", ri.StatusCode)
		return resultBuildErr
	}

	resp := &http.Response{Header: http.Header{}}
	resp.StatusCode = ri.StatusCode

	for k, vs := range ri.Headers {
		for _, v := range vs {
			resp.Header.Add(k, v)
		}
	}

	// build body
	egresp, _ := httpprot.NewResponse(resp)
	egresp.SetPayload([]byte(ri.Body))

	ctx.SetResponse(ctx.TargetResponseID(), egresp)
	return ""
}
