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

package responseadaptor

import (
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
)

const (
	// Kind is the kind of ResponseAdaptor.
	Kind = "ResponseAdaptor"

	resultResponseNotFound = "responseNotFound"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "ResponseAdaptor adapts response.",
	Results:     []string{resultResponseNotFound},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &ResponseAdaptor{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// ResponseAdaptor is filter ResponseAdaptor.
	ResponseAdaptor struct {
		spec *Spec
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		Header *httpheader.AdaptSpec `yaml:"header" jsonschema:"required"`
		Body   string                `yaml:"body" jsonschema:"omitempty"`
	}
)

// Name returns the name of the ResponseAdaptor filter instance.
func (ra *ResponseAdaptor) Name() string {
	return ra.spec.Name()
}

// Kind returns the kind of ResponseAdaptor.
func (ra *ResponseAdaptor) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the ResponseAdaptor
func (ra *ResponseAdaptor) Spec() filters.Spec {
	return ra.spec
}

// Init initializes ResponseAdaptor.
func (ra *ResponseAdaptor) Init() {
	ra.reload()
}

// Inherit inherits previous generation of ResponseAdaptor.
func (ra *ResponseAdaptor) Inherit(previousGeneration filters.Filter) {
	previousGeneration.Close()
	ra.reload()
}

func (ra *ResponseAdaptor) reload() {
	// Nothing to do.
}

func adaptHeader(req *httpprot.Response, as *httpheader.AdaptSpec) {
	h := req.Std().Header
	for _, key := range as.Del {
		h.Del(key)
	}
	for key, value := range as.Set {
		h.Set(key, value)
	}
	for key, value := range as.Add {
		h.Add(key, value)
	}
}

// Handle adapts response.
func (ra *ResponseAdaptor) Handle(ctx *context.Context) string {
	resp := ctx.GetResponse(ctx.TargetResponseID())
	if resp == nil {
		return resultResponseNotFound
	}
	httpresp := resp.(*httpprot.Response)
	adaptHeader(httpresp, ra.spec.Header)

	if len(ra.spec.Body) != 0 {
		httpresp.SetPayload([]byte(ra.spec.Body))
	}
	return ""
}

// Status returns status.
func (ra *ResponseAdaptor) Status() interface{} { return nil }

// Close closes ResponseAdaptor.
func (ra *ResponseAdaptor) Close() {}
