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

package requestadaptor

import (
	"bytes"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Kind is the kind of RequestAdaptor.
	Kind = "RequestAdaptor"
)

var (
	results = []string{}
)

func init() {
	httppipeline.Register(&RequestAdaptor{})
}

type (
	// RequestAdaptor is filter RequestAdaptor.
	RequestAdaptor struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		pa *pathadaptor.PathAdaptor
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		Host   string                `yaml:"host" jsonschema:"omitempty"`
		Method string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path   *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
		Body   string                `yaml:"body" jsonschema:"omitempty"`
	}
)

// Kind returns the kind of RequestAdaptor.
func (ra *RequestAdaptor) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of RequestAdaptor.
func (ra *RequestAdaptor) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of RequestAdaptor.
func (ra *RequestAdaptor) Description() string {
	return "RequestAdaptor adapts request."
}

// Results returns the results of RequestAdaptor.
func (ra *RequestAdaptor) Results() []string {
	return results
}

// Init initializes RequestAdaptor.
func (ra *RequestAdaptor) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	ra.pipeSpec, ra.spec, ra.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	ra.reload()
}

// Inherit inherits previous generation of RequestAdaptor.
func (ra *RequestAdaptor) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	previousGeneration.Close()
	ra.Init(pipeSpec, super)
}

func (ra *RequestAdaptor) reload() {
	if ra.spec.Path != nil {
		ra.pa = pathadaptor.New(ra.spec.Path)
	}
}

// Handle adapts request.
func (ra *RequestAdaptor) Handle(ctx context.HTTPContext) string {
	result := ra.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (ra *RequestAdaptor) handle(ctx context.HTTPContext) string {
	r := ctx.Request()
	method, path, header := r.Method(), r.Path(), r.Header()

	if ra.spec.Method != "" && ra.spec.Method != method {
		ctx.AddTag(stringtool.Cat("requestAdaptor: method ",
			method, " adapted to ", ra.spec.Method))
		r.SetMethod(ra.spec.Method)
	}

	if ra.pa != nil {
		adaptedPath := ra.pa.Adapt(path)
		if adaptedPath != path {
			ctx.AddTag(stringtool.Cat("requestAdaptor: path ",
				path, " adapted to ", adaptedPath))
		}
		r.SetPath(adaptedPath)
	}
	hte := ctx.Template()
	if ra.spec.Header != nil {
		header.Adapt(ra.spec.Header, hte)
	}

	if len(ra.spec.Body) != 0 {
		if hte.HasTemplates(ra.spec.Body) {
			if body, err := hte.Render(ra.spec.Body); err != nil {
				logger.Errorf("BUG request render body failed, template %s, err %v",
					ra.spec.Body, err)
			} else {
				ctx.Request().SetBody(bytes.NewReader([]byte(body)))
			}
		} else {
			ctx.Request().SetBody(bytes.NewReader([]byte(ra.spec.Body)))
		}
	}

	if len(ra.spec.Host) != 0 {
		if hte.HasTemplates(ra.spec.Host) {
			if host, err := hte.Render(ra.spec.Host); err != nil {
				logger.Errorf("BUG request render host failed, template %s, err %v",
					ra.spec.Host, err)
			} else {
				ctx.Request().SetHost(host)
			}
		} else {
			ctx.Request().SetHost(ra.spec.Host)
		}
	}
	return ""
}

// Status returns status.
func (ra *RequestAdaptor) Status() interface{} { return nil }

// Close closes RequestAdaptor.
func (ra *RequestAdaptor) Close() {}
