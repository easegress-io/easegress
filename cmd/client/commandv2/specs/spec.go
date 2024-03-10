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

package specs

import (
	"github.com/megaease/easegress/v2/cmd/client/general"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/filters/builder"
	"github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/object/autocertmanager"
	"github.com/megaease/easegress/v2/pkg/object/httpserver"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

// HTTPServerSpec is the spec of HTTPServer.
type HTTPServerSpec struct {
	Name string `json:"name"`
	Kind string `json:"kind"`

	httpserver.Spec `json:",inline"`
}

// PipelineSpec is the spec of Pipeline.
type PipelineSpec struct {
	Name string `json:"name"`
	Kind string `json:"kind"`

	pipeline.Spec `json:",inline"`
}

// SetFilters sets the filters of PipelineSpec.
func (p *PipelineSpec) SetFilters(filters []filters.Spec) {
	data := codectool.MustMarshalYAML(filters)
	maps, _ := general.UnmarshalMapInterface(data, true)
	p.Filters = maps
}

// NewHTTPServerSpec returns a new HTTPServerSpec.
func NewHTTPServerSpec(name string) *HTTPServerSpec {
	spec := &HTTPServerSpec{
		Name: name,
		Kind: httpserver.Kind,
		Spec: *getDefaultHTTPServerSpec(),
	}
	spec.Spec.Certs = map[string]string{}
	spec.Spec.Keys = map[string]string{}
	return spec
}

// NewPipelineSpec returns a new PipelineSpec.
func NewPipelineSpec(name string) *PipelineSpec {
	return &PipelineSpec{
		Name: name,
		Kind: pipeline.Kind,
		Spec: *getDefaultPipelineSpec(),
	}
}

func getDefaultHTTPServerSpec() *httpserver.Spec {
	return (&httpserver.HTTPServer{}).DefaultSpec().(*httpserver.Spec)
}

func getDefaultPipelineSpec() *pipeline.Spec {
	return (&pipeline.Pipeline{}).DefaultSpec().(*pipeline.Spec)
}

func getDefaultAutoCertManagerSpec() *autocertmanager.Spec {
	return (&autocertmanager.AutoCertManager{}).DefaultSpec().(*autocertmanager.Spec)
}

// NewProxyFilterSpec returns a new ProxyFilterSpec.
func NewProxyFilterSpec(name string) *httpproxy.Spec {
	spec := GetDefaultFilterSpec(httpproxy.Kind).(*httpproxy.Spec)
	spec.BaseSpec.MetaSpec.Name = name
	spec.BaseSpec.MetaSpec.Kind = httpproxy.Kind
	return spec
}

// NewWebsocketFilterSpec returns a new WebsocketFilterSpec.
func NewWebsocketFilterSpec(name string) *httpproxy.WebSocketProxySpec {
	spec := GetDefaultFilterSpec(httpproxy.WebSocketProxyKind).(*httpproxy.WebSocketProxySpec)
	spec.BaseSpec.MetaSpec.Name = name
	spec.BaseSpec.MetaSpec.Kind = httpproxy.WebSocketProxyKind
	return spec
}

// NewRequestAdaptorFilterSpec returns a new RequestAdaptorFilterSpec.
func NewRequestAdaptorFilterSpec(name string) *builder.RequestAdaptorSpec {
	spec := GetDefaultFilterSpec(builder.RequestAdaptorKind).(*builder.RequestAdaptorSpec)
	spec.BaseSpec.MetaSpec.Name = name
	spec.BaseSpec.MetaSpec.Kind = builder.RequestAdaptorKind
	return spec
}

// GetDefaultFilterSpec returns the default filter spec of the kind.
func GetDefaultFilterSpec(kind string) filters.Spec {
	return filters.GetKind(kind).DefaultSpec()
}

// PipelineSpec is the spec of Pipeline.
type AutoCertManagerSpec struct {
	Name string `json:"name"`
	Kind string `json:"kind"`

	autocertmanager.Spec `json:",inline"`
}

// NewAutoCertManagerSpec returns a new AutoCertManagerSpec.
func NewAutoCertManagerSpec() *AutoCertManagerSpec {
	return &AutoCertManagerSpec{
		Name: "autocertmanager",
		Kind: autocertmanager.Kind,
		Spec: *getDefaultAutoCertManagerSpec(),
	}
}

// AddOrUpdateDomain adds or updates a domain.
func (a *AutoCertManagerSpec) AddOrUpdateDomain(domain *autocertmanager.DomainSpec) {
	for i, d := range a.Domains {
		if d.Name == domain.Name {
			a.Domains[i] = *domain
			return
		}
	}
	a.Domains = append(a.Domains, *domain)
}
