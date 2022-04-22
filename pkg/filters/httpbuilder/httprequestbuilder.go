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

func checkMethod(method string) bool {
	_, ok := methods[method]
	return ok
}

var httpRequestBuilderKind = &filters.Kind{
	Name:        HTTPRequestBuilderKind,
	Description: "HTTPRequestBuilder builds an HTTP request",
	Results:     []string{resultBuildErr},
	DefaultSpec: func() filters.Spec {
		return &RequestSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &HTTPRequestBuilder{spec: spec.(*RequestSpec)}
	},
}

func init() {
	filters.Register(httpRequestBuilderKind)
}

type (
	// HTTPRequestBuilder is filter HTTPRequestBuilder.
	HTTPRequestBuilder struct {
		spec           *RequestSpec
		methodBuilder  *builder
		urlBuilder     *builder
		bodyBuilder    *builder
		headerBuilders []*headerBuilder
	}

	// RequestSpec is HTTPRequestBuilder Spec.
	RequestSpec struct {
		filters.BaseSpec `yaml:",inline"`

		ID      string   `yaml:"id" jsonschema:"required"`
		Method  string   `yaml:"method" jsonschema:"required"`
		URL     string   `yaml:"url" jsonschema:"required"`
		Headers []Header `yaml:"headers" jsonschema:"omitempty"`
		Body    string   `yaml:"body" jsonschema:"omitempty"`
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
	rb.methodBuilder = getBuilder(rb.spec.Method)
	if rb.methodBuilder.template == nil {
		rb.methodBuilder.value = strings.ToUpper(rb.spec.Method)
		if !checkMethod(rb.methodBuilder.value) {
			panic(fmt.Errorf("invalid method for HTTPRequestBuilder %v", rb.spec.Method))
		}
	}

	rb.urlBuilder = getBuilder(rb.spec.URL)
	rb.bodyBuilder = getBuilder(rb.spec.Body)

	for _, header := range rb.spec.Headers {
		keyBuilder := getBuilder(header.Key)
		valueBuilder := getBuilder(header.Value)
		rb.headerBuilders = append(rb.headerBuilders, &headerBuilder{keyBuilder, valueBuilder})
	}
}

// Handle builds request.
func (rb *HTTPRequestBuilder) Handle(ctx *context.Context) (result string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("panic: %s, stacktrace: %s\n", err, string(debug.Stack()))
			result = resultBuildErr
		}
	}()

	templateCtx, err := getTemplateContext(ctx)
	if err != nil {
		logger.Errorf("getTemplateContext failed: %v", err)
		return resultBuildErr
	}

	// build method
	method, err := rb.methodBuilder.build(templateCtx)
	if err != nil {
		logger.Errorf("build method failed: %v", err)
		return resultBuildErr
	}

	// build url
	url, err := rb.urlBuilder.build(templateCtx)
	if err != nil {
		logger.Errorf("build url failed: %v", err)
		return resultBuildErr
	}

	// build body
	var body string
	if rb.bodyBuilder != nil {
		body, err = rb.bodyBuilder.build(templateCtx)
		if err != nil {
			logger.Errorf("build body failed: %v", err)
			return resultBuildErr
		}
	}

	// build request
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		logger.Errorf("build request failed: %v", err)
		return resultBuildErr
	}

	// build headers
	for _, headerBuilder := range rb.headerBuilders {
		key, err := headerBuilder.key.build(templateCtx)
		if err != nil {
			logger.Errorf("build header key failed: %v", err)
			return resultBuildErr
		}
		value, err := headerBuilder.value.build(templateCtx)
		if err != nil {
			logger.Errorf("build header value failed: %v", err)
			return resultBuildErr
		}
		req.Header.Add(key, value)
	}

	// build context
	httpreq, err := httpprot.NewRequest(req)
	if err != nil {
		logger.Errorf("build context failed: %v", err)
		return resultBuildErr
	}
	ctx.SetRequest(rb.spec.ID, httpreq)
	return ""
}

// Status returns status.
func (rb *HTTPRequestBuilder) Status() interface{} { return nil }

// Close closes HTTPRequestBuilder.
func (rb *HTTPRequestBuilder) Close() {}
