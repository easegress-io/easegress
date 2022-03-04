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

package meshadaptor

import (
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/httpfilter"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
)

const (
	// Kind is the kind of MeshAdaptor.
	Kind = "MeshAdaptor"
)

var results = []string{}

func init() {
	httppipeline.Register(&MeshAdaptor{})
}

type (
	// MeshAdaptor is filter MeshAdaptor.
	MeshAdaptor struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec

		pa *pathadaptor.PathAdaptor
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		ServiceCanaries []*ServiceCanaryAdaptor `yaml:"serviceCanaries" jsonschema:"omitempty"`
	}

	// ServiceCanaryAdaptor is the service canary adaptor.
	ServiceCanaryAdaptor struct {
		Header *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"required"`
		Filter *httpfilter.Spec      `yaml:"filter" jsonschema:"required"`

		filter *httpfilter.HTTPFilter
	}
)

// Kind returns the kind of MeshAdaptor.
func (ra *MeshAdaptor) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of MeshAdaptor.
func (ra *MeshAdaptor) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of MeshAdaptor.
func (ra *MeshAdaptor) Description() string {
	return "MeshAdaptor adapts requests for mesh traffic."
}

// Results returns the results of MeshAdaptor.
func (ra *MeshAdaptor) Results() []string {
	return results
}

// Init initializes MeshAdaptor.
func (ra *MeshAdaptor) Init(filterSpec *httppipeline.FilterSpec) {
	ra.filterSpec, ra.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	ra.reload()
}

// Inherit inherits previous generation of MeshAdaptor.
func (ra *MeshAdaptor) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	ra.Init(filterSpec)
}

func (ra *MeshAdaptor) reload() {
	for _, serviceCanary := range ra.spec.ServiceCanaries {
		serviceCanary.filter = httpfilter.New(serviceCanary.Filter)
	}
}

// Handle adapts request.
func (ra *MeshAdaptor) Handle(ctx context.HTTPContext) string {
	for _, serviceCanary := range ra.spec.ServiceCanaries {
		if serviceCanary.filter.Filter(ctx) {
			ctx.Request().Header().Adapt(serviceCanary.Header, ctx.Template())
		}
	}

	result := ""
	return ctx.CallNextHandler(result)
}

// Status returns status.
func (ra *MeshAdaptor) Status() interface{} { return nil }

// Close closes MeshAdaptor.
func (ra *MeshAdaptor) Close() {}
