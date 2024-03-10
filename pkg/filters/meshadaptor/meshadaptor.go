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

// Package meshadaptor provides MeshAdaptor filter.
package meshadaptor

import (
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	proxy "github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpheader"
)

const (
	// Kind is the kind of MeshAdaptor.
	Kind = "MeshAdaptor"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "MeshAdaptor adapts requests for mesh traffic.",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &MeshAdaptor{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// MeshAdaptor is filter MeshAdaptor.
	MeshAdaptor struct {
		spec *Spec
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		filters.BaseSpec `json:",inline"`
		ServiceCanaries  []*ServiceCanaryAdaptor `json:"serviceCanaries,omitempty"`
	}

	// ServiceCanaryAdaptor is the service canary adaptor.
	ServiceCanaryAdaptor struct {
		Header *httpheader.AdaptSpec     `json:"header" jsonschema:"required"`
		Filter *proxy.RequestMatcherSpec `json:"filter" jsonschema:"required"`

		filter proxy.RequestMatcher
	}
)

// Name returns the name of the MeshAdaptor filter instance.
func (ra *MeshAdaptor) Name() string {
	return ra.spec.Name()
}

// Kind returns the kind of MeshAdaptor.
func (ra *MeshAdaptor) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the MeshAdaptor
func (ra *MeshAdaptor) Spec() filters.Spec {
	return ra.spec
}

// Init initializes MeshAdaptor.
func (ra *MeshAdaptor) Init() {
	ra.reload()
}

// Inherit inherits previous generation of MeshAdaptor.
func (ra *MeshAdaptor) Inherit(previousGeneration filters.Filter) {
	ra.Init()
}

func (ra *MeshAdaptor) reload() {
	for _, serviceCanary := range ra.spec.ServiceCanaries {
		serviceCanary.filter = proxy.NewRequestMatcher(serviceCanary.Filter)
	}
}

// Handle adapts request.
func (ra *MeshAdaptor) Handle(ctx *context.Context) string {
	httpreq := ctx.GetInputRequest().(*httpprot.Request)
	for _, serviceCanary := range ra.spec.ServiceCanaries {
		if serviceCanary.filter.Match(httpreq) {
			h := httpheader.New(httpreq.HTTPHeader())
			h.Adapt(serviceCanary.Header)
		}
	}

	return ""
}

// Status returns status.
func (ra *MeshAdaptor) Status() interface{} { return nil }

// Close closes MeshAdaptor.
func (ra *MeshAdaptor) Close() {}
