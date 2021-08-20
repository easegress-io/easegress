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
	"bytes"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

const (
	// Kind is the kind of ResponseAdaptor.
	Kind = "ResponseAdaptor"
)

var results = []string{}

func init() {
	httppipeline.Register(&ResponseAdaptor{})
}

type (
	// ResponseAdaptor is filter ResponseAdaptor.
	ResponseAdaptor struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		Header *httpheader.AdaptSpec `yaml:"header" jsonschema:"required"`

		Body string `yaml:"body" jsonschema:"omitempty"`
	}
)

// Kind returns the kind of ResponseAdaptor.
func (ra *ResponseAdaptor) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of ResponseAdaptor.
func (ra *ResponseAdaptor) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of ResponseAdaptor.
func (ra *ResponseAdaptor) Description() string {
	return "ResponseAdaptor adapts response."
}

// Results returns the results of ResponseAdaptor.
func (ra *ResponseAdaptor) Results() []string {
	return results
}

// Init initializes ResponseAdaptor.
func (ra *ResponseAdaptor) Init(filterSpec *httppipeline.FilterSpec) {
	ra.filterSpec, ra.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	ra.reload()
}

// Inherit inherits previous generation of ResponseAdaptor.
func (ra *ResponseAdaptor) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	ra.Init(filterSpec)
}

func (ra *ResponseAdaptor) reload() {
	// Nothing to do.
}

// Handle adapts response.
func (ra *ResponseAdaptor) Handle(ctx context.HTTPContext) string {
	result := ra.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (ra *ResponseAdaptor) handle(ctx context.HTTPContext) string {
	hte := ctx.Template()
	ctx.Response().Header().Adapt(ra.spec.Header, hte)

	if len(ra.spec.Body) == 0 {
		return ""
	}

	if !hte.HasTemplates(ra.spec.Body) {
		ctx.Response().SetBody(bytes.NewReader([]byte(ra.spec.Body)))
	} else if body, err := hte.Render(ra.spec.Body); err != nil {
		logger.Errorf("BUG responseadaptor render body failed, template %s , err %v", ra.spec.Body, err)
	} else {
		ctx.Response().SetBody(bytes.NewReader([]byte(body)))
	}

	return ""
}

// Status returns status.
func (ra *ResponseAdaptor) Status() interface{} { return nil }

// Close closes ResponseAdaptor.
func (ra *ResponseAdaptor) Close() {}
