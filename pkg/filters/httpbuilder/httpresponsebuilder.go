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
	"strconv"

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
		spec              *HTTPResponseBuilderSpec
		statusCode        int
		statusCodeBuilder *builder
		HTTPBuilder
	}

	// HTTPResponseBuilderSpec is HTTPResponseBuilder Spec.
	HTTPResponseBuilderSpec struct {
		filters.BaseSpec `yaml:",inline"`
		Spec             `yaml:",inline"`

		StatusCode string `yaml:"statusCode" jsonschema:"omitempty"`
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
	if rb.spec.StatusCode == "" {
		rb.statusCode = http.StatusOK
	} else if code, err := strconv.Atoi(rb.spec.StatusCode); err == nil {
		rb.statusCode = code
	} else {
		rb.statusCodeBuilder = newBuilder(rb.spec.StatusCode)
	}

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

	resp := &http.Response{}

	if rb.statusCodeBuilder == nil {
		resp.StatusCode = rb.statusCode
	} else if s, err := rb.statusCodeBuilder.buildString(data); err != nil {
		logger.Warnf("status code failed: %v", err)
		return resultBuildErr
	} else if code, err := strconv.Atoi(s); err != nil {
		logger.Warnf("status code is not an integer: %v", err)
		return resultBuildErr
	} else {
		resp.StatusCode = code
	}

	// build headers
	if h, err := rb.buildHeader(data); err != nil {
		return resultBuildErr
	} else {
		resp.Header = h
	}

	// build body
	egresp, _ := httpprot.NewResponse(resp)
	if body, err := rb.buildBody(data); err != nil {
		return resultBuildErr
	} else {
		egresp.SetPayload(body)
	}

	ctx.SetResponse(ctx.TargetResponseID(), egresp)
	return ""
}
