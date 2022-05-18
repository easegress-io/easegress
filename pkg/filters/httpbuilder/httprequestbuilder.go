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
		spec *HTTPRequestBuilderSpec
		HTTPBuilder
	}

	// HTTPRequestBuilderSpec is HTTPRequestBuilder Spec.
	HTTPRequestBuilderSpec struct {
		filters.BaseSpec `yaml:",inline"`
		Spec             `yaml:",inline"`
	}

	// RequestInfo stores the information of a request.
	RequestInfo struct {
		Method  string              `json:"method" jsonschema:"omitempty"`
		URL     string              `json:"url" jsonschema:"omitempty"`
		Headers map[string][]string `yaml:"headers" jsonschema:"omitempty"`
		Body    string              `yaml:"body" jsonschema:"omitempty"`
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

	var ri RequestInfo
	if err = rb.build(data, &ri); err != nil {
		msgFmt := "HTTPRequestBuilder(%s): failed to build request info: %v"
		logger.Warnf(msgFmt, rb.Name(), err)
		return resultBuildErr
	}

	if ri.URL == "" {
		ri.URL = "/"
	}

	if ri.Method == "" {
		ri.Method = http.MethodGet
	} else {
		ri.Method = strings.ToUpper(ri.Method)
	}
	if _, ok := methods[ri.Method]; !ok {
		logger.Warnf("invalid method: %s", ri.Method)
		return resultBuildErr
	}

	stdReq, err := http.NewRequest(ri.Method, ri.URL, nil)
	if err != nil {
		logger.Warnf("failed to create new request: %v", err)
		return resultBuildErr
	}

	for k, vs := range ri.Headers {
		for _, v := range vs {
			stdReq.Header.Add(k, v)
		}
	}

	req, _ := httpprot.NewRequest(stdReq)
	req.SetPayload([]byte(ri.Body))

	ctx.SetRequest(ctx.TargetRequestID(), req)
	return ""
}
