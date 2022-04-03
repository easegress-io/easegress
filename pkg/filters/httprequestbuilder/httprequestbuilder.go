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

package requestbuilder

import (
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
)

const (
	// Kind is the kind of HTTPRequestBuilder.
	Kind = "HTTPRequestBuilder"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "HTTPRequestBuilder builds an HTTP request",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &HTTPRequestBuilder{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// HTTPRequestBuilder is filter HTTPRequestBuilder.
	HTTPRequestBuilder struct {
		spec *Spec
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		// Body is the request body of the target request.
		Body string `yaml:"body" jsonschema:"omitempty"`
	}
)

// Name returns the name of the HTTPRequestBuilder filter instance.
func (rb *HTTPRequestBuilder) Name() string {
	return rb.spec.Name()
}

// Kind returns the kind of HTTPRequestBuilder.
func (rb *HTTPRequestBuilder) Kind() *filters.Kind {
	return kind
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
	// Nothing to do.
}

// Handle builds request.
func (rb *HTTPRequestBuilder) Handle(ctx context.Context) string {
	// TODO: finish it later
	panic("finish this when ready")
	return ""
}

// Status returns status.
func (rb *HTTPRequestBuilder) Status() interface{} { return nil }

// Close closes HTTPRequestBuilder.
func (rb *HTTPRequestBuilder) Close() {}
