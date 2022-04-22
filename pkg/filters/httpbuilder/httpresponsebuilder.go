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
		return &ResponseSpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &HTTPResponseBuilder{spec: spec.(*ResponseSpec)}
	},
}

func init() {
	filters.Register(httpResponseBuilderKind)
}

type (
	// HTTPResponseBuilder is filter HTTPResponseBuilder.
	HTTPResponseBuilder struct {
		spec           *ResponseSpec
		bodyBuilder    *builder
		headerBuilders []*headerBuilder
	}

	// ResponseSpec is HTTPResponseBuilder Spec.
	ResponseSpec struct {
		filters.BaseSpec `yaml:",inline"`

		ID         string      `yaml:"id" jsonschema:"required"`
		StatusCode *StatusCode `yaml:"statusCode" jsonschema:"required"`
		Headers    []Header    `yaml:"headers" jsonschema:"omitempty"`
		Body       *BodySpec   `yaml:"body" jsonschema:"omitempty"`
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
	if rb.spec.Body != nil {
		rb.bodyBuilder = getBuilder(rb.spec.Body.Body)
	}

	for _, header := range rb.spec.Headers {
		keyBuilder := getBuilder(header.Key)
		valueBuilder := getBuilder(header.Value)
		rb.headerBuilders = append(rb.headerBuilders, &headerBuilder{keyBuilder, valueBuilder})
	}
}

func (rb *HTTPResponseBuilder) setStatusCode(ctx *context.Context, resp *httpprot.Response) {
	code := 200
	// set status code
	if rb.spec.StatusCode != nil {
		if rb.spec.StatusCode.Code != 0 {
			code = rb.spec.StatusCode.Code

		} else if rb.spec.StatusCode.CopyResponseID != "" {
			r := ctx.GetResponse(rb.spec.StatusCode.CopyResponseID)
			if r != nil {
				httpresp := r.(*httpprot.Response)
				code = httpresp.StatusCode()
			}
		}
	}
	resp.SetStatusCode(code)
}

// Handle builds request.
func (rb *HTTPResponseBuilder) Handle(ctx *context.Context) (result string) {
	// defer func() {
	// 	if err := recover(); err != nil {
	// 		logger.Errorf("panic: %v", err)
	// 		result = resultBuildErr
	// 	}
	// }()

	templateCtx, err := getTemplateContext(ctx)
	if err != nil {
		logger.Errorf("getTemplateContext failed: %v", err)
		return resultBuildErr
	}

	resp, _ := httpprot.NewResponse(nil)

	rb.setStatusCode(ctx, resp)

	// build body
	var body string
	if rb.bodyBuilder != nil {
		body, err = rb.bodyBuilder.build(templateCtx)
		if err != nil {
			logger.Errorf("build body failed: %v", err)
			return resultBuildErr
		}
	}
	resp.SetPayload([]byte(body))

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
		resp.Std().Header.Add(key, value)
	}

	ctx.SetResponse(rb.spec.ID, resp)
	return ""
}

// Status returns status.
func (rb *HTTPResponseBuilder) Status() interface{} { return nil }

// Close closes HTTPRequestBuilder.
func (rb *HTTPResponseBuilder) Close() {}
