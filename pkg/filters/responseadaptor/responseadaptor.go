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
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/httpheader"
)

const (
	// Kind is the kind of ResponseAdaptor.
	Kind = "ResponseAdaptor"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "ResponseAdaptor adapts response.",
	Results:     []string{},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func() filters.Filter {
		return &ResponseAdaptor{}
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
func (ra *ResponseAdaptor) Kind() string {
	return Kind
}

// Init initializes ResponseAdaptor.
func (ra *ResponseAdaptor) Init(spec filters.Spec) {
	ra.spec = spec.(*Spec)
	ra.reload()
}

// Inherit inherits previous generation of ResponseAdaptor.
func (ra *ResponseAdaptor) Inherit(spec filters.Spec, previousGeneration filters.Filter) {
	previousGeneration.Close()
	ra.Init(spec)
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
