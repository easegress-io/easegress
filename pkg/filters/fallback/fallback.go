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

// Package fallback implements the fallback filter.
package fallback

import (
	"io"
	"strconv"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

const (
	// Kind is the kind of Fallback.
	Kind = "Fallback"

	resultFallback         = "fallback"
	resultResponseNotFound = "responseNotFound"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Fallback do the fallback.",
	Results:     []string{resultFallback, resultResponseNotFound},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Fallback{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// Fallback is filter Fallback.
	Fallback struct {
		spec       *Spec
		mockBody   []byte
		bodyLength string
	}

	// Spec describes the Fallback.
	Spec struct {
		filters.BaseSpec `json:",inline"`

		MockCode    int               `json:"mockCode" jsonschema:"required,format=httpcode"`
		MockHeaders map[string]string `json:"mockHeaders,omitempty"`
		MockBody    string            `json:"mockBody,omitempty"`
	}
)

// Name returns the name of the Fallback filter instance.
func (f *Fallback) Name() string {
	return f.spec.Name()
}

// Kind returns the kind of Fallback.
func (f *Fallback) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Fallback
func (f *Fallback) Spec() filters.Spec {
	return f.spec
}

// Init initializes Fallback.
func (f *Fallback) Init() {
	f.reload()
}

// Inherit inherits previous generation of Fallback.
func (f *Fallback) Inherit(previousGeneration filters.Filter) {
	f.Init()
}

func (f *Fallback) reload() {
	f.mockBody = []byte(f.spec.MockBody)
	f.bodyLength = strconv.Itoa(len(f.mockBody))
}

// Handle fallbacks HTTPContext.
// It always returns fallback.
func (f *Fallback) Handle(ctx *context.Context) string {
	resp := ctx.GetInputResponse().(*httpprot.Response)
	if resp == nil {
		return resultResponseNotFound
	}

	resp.SetStatusCode(f.spec.MockCode)
	resp.HTTPHeader().Set("Content-Length", f.bodyLength)
	for key, value := range f.spec.MockHeaders {
		resp.HTTPHeader().Set(key, value)
	}

	if resp.IsStream() {
		if c, ok := resp.GetPayload().(io.Closer); ok {
			c.Close()
		}
	}

	resp.SetPayload(f.mockBody)
	return resultFallback
}

// Status returns Status.
func (f *Fallback) Status() interface{} {
	return nil
}

// Close closes Fallback.
func (f *Fallback) Close() {
}
