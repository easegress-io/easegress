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
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

const (
	// HTTPRequestBuilderKind is the kind of HTTPRequestBuilder.
	HTTPRequestBuilderKind = "HTTPRequestBuilder"
)

var methods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodPost:    {},
	http.MethodPut:     {},
	http.MethodPatch:   {},
	http.MethodDelete:  {},
	http.MethodConnect: {},
	http.MethodOptions: {},
	http.MethodTrace:   {},
}

var httpRequestBuilderKind = &filters.Kind{
	Name:        HTTPRequestBuilderKind,
	Description: "HTTPRequestBuilder builds an HTTP request",
	Results:     []string{resultBuildErr},
	DefaultSpec: func() filters.Spec {
		return &HTTPRequestBuilderSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &HTTPRequestBuilder{spec: spec.(*HTTPRequestBuilderSpec)}
	},
}

func init() {
	filters.Register(httpRequestBuilderKind)
}

type (
	// HTTPRequestBuilder is filter HTTPRequestBuilder.
	HTTPRequestBuilder struct {
		spec          *HTTPRequestBuilderSpec
		methodBuilder *builder
		urlBuilder    *builder
		HTTPBuilder
	}

	// HTTPRequestBuilderSpec is HTTPRequestBuilder Spec.
	HTTPRequestBuilderSpec struct {
		filters.BaseSpec `yaml:",inline"`
		Spec             `yaml:",inline"`

		Method string `yaml:"method" jsonschema:"required"`
		URL    string `yaml:"url" jsonschema:"required"`
	}
)

// Name returns the name of the HTTPRequestBuilder filter instance.
func (rb *HTTPRequestBuilder) Name() string {
	return rb.spec.Name()
}

// Kind returns the kind of HTTPRequestBuilder.
func (rb *HTTPRequestBuilder) Kind() *filters.Kind {
	return httpRequestBuilderKind
}

// Spec returns the spec used by the HTTPRequestBuilder
func (rb *HTTPRequestBuilder) Spec() filters.Spec {
	return rb.spec
}

// Init initializes HTTPRequestBuilder.
func (rb *HTTPRequestBuilder) Init() {
	rb.reload()
}

// Inherit inherits previous generation of HTTPRequestBuilder.
func (rb *HTTPRequestBuilder) Inherit(previousGeneration filters.Filter) {
	previousGeneration.Close()
	rb.Init()
}

func (rb *HTTPRequestBuilder) reload() {
	rb.methodBuilder = newBuilder(rb.spec.Method)
	if rb.methodBuilder.template == nil {
		rb.methodBuilder.value = strings.ToUpper(rb.spec.Method)
		if _, ok := methods[rb.methodBuilder.value]; !ok {
			msgFmt := "invalid method for HTTPRequestBuilder %v"
			panic(fmt.Errorf(msgFmt, rb.spec.Method))
		}
	}

	rb.urlBuilder = newBuilder(rb.spec.URL)
	rb.HTTPBuilder.reload(&rb.spec.Spec)
}

// Handle builds request.
func (rb *HTTPRequestBuilder) Handle(ctx *context.Context) (result string) {
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

	// build method
	method, err := rb.methodBuilder.buildString(data)
	if err != nil {
		logger.Warnf("build method failed: %v", err)
		return resultBuildErr
	}

	// build url
	url, err := rb.urlBuilder.buildString(data)
	if err != nil {
		logger.Warnf("build url failed: %v", err)
		return resultBuildErr
	}

	// build request, use SetPayload to set body
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		logger.Warnf("build request failed: %v", err)
		return resultBuildErr
	}

	// build headers
	if h, err := rb.buildHeader(data); err != nil {
		return resultBuildErr
	} else {
		req.Header = h
	}

	// build body
	egreq, _ := httpprot.NewRequest(req)
	if body, err := rb.buildBody(data); err != nil {
		return resultBuildErr
	} else {
		egreq.SetPayload([]byte(body))
	}

	ctx.SetRequest(ctx.TargetRequestID(), egreq)
	return ""
}
