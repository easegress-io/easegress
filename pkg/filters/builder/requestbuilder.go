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
	"strings"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

const (
	// RequestBuilderKind is the kind of RequestBuilder.
	RequestBuilderKind = "RequestBuilder"
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

var requestBuilderKind = &filters.Kind{
	Name:        RequestBuilderKind,
	Description: "RequestBuilder builds a request",
	Results:     []string{resultBuildErr},
	DefaultSpec: func() filters.Spec {
		return &RequestBuilderSpec{}
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
	rb.Builder.reload(&rb.spec.Spec)
}

// Handle builds request.
func (rb *RequestBuilder) Handle(ctx *context.Context) (result string) {
	if rb.spec.SourceNamespace != "" {
		ctx.CopyRequest(rb.spec.SourceNamespace)
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

	var ri RequestInfo
	if err = rb.build(data, &ri); err != nil {
		msgFmt := "RequestBuilder(%s): failed to build request info: %v"
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

	stdReq, err := http.NewRequest(ri.Method, ri.URL, http.NoBody)
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

	ctx.SetOutputRequest(req)
	return ""
}
