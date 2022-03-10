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
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/util/httpfilter"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
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
	CreateInstance: func() filters.Filter {
		return &MeshAdaptor{}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// MeshAdaptor is filter MeshAdaptor.
	MeshAdaptor struct {
		spec *Spec

		pa *pathadaptor.PathAdaptor
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`
		ServiceCanaries  []*ServiceCanaryAdaptor `yaml:"serviceCanaries" jsonschema:"omitempty"`
	}

	// ServiceCanaryAdaptor is the service canary adaptor.
	ServiceCanaryAdaptor struct {
		Header *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"required"`
		Filter *httpfilter.Spec      `yaml:"filter" jsonschema:"required"`

		filter *httpfilter.HTTPFilter
	}
)

// Name returns the name of the MeshAdaptor filter instance.
func (ra *MeshAdaptor) Name() string {
	return ra.spec.Name()
}

// Kind returns the kind of MeshAdaptor.
func (ra *MeshAdaptor) Kind() string {
	return Kind
}

// Init initializes MeshAdaptor.
func (ra *MeshAdaptor) Init(spec filters.Spec) {
	ra.spec = spec.(*Spec)
	ra.reload()
}

// Inherit inherits previous generation of MeshAdaptor.
func (ra *MeshAdaptor) Inherit(spec filters.Spec, previousGeneration filters.Filter) {
	previousGeneration.Close()
	ra.Init(spec)
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
