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

package fallback

import (
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/util/fallback"
)

const (
	// Kind is the kind of Fallback.
	Kind = "Fallback"

	resultFallback = "fallback"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Fallback do the fallback.",
	Results:     []string{resultFallback},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func() filters.Filter {
		return &Fallback{}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// Fallback is filter Fallback.
	Fallback struct {
		spec *Spec
		f    *fallback.Fallback
	}

	// Spec describes the Fallback.
	Spec struct {
		filters.BaseSpec `yaml:",inline"`

		fallback.Spec `yaml:",inline"`
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
func (f *Fallback) Init(spec filters.Spec) {
	f.spec = spec.(*Spec)
	f.reload()
}

// Inherit inherits previous generation of Fallback.
func (f *Fallback) Inherit(spec filters.Spec, previousGeneration filters.Filter) {
	previousGeneration.Close()
	f.Init(spec)
}

func (f *Fallback) reload() {
	f.f = fallback.New(&f.spec.Spec)
}

// Handle fallbacks HTTPContext.
// It always returns fallback.
func (f *Fallback) Handle(ctx context.HTTPContext) string {
	f.f.Fallback(ctx)
	return ctx.CallNextHandler(resultFallback)
}

// Status returns Status.
func (f *Fallback) Status() interface{} {
	return nil
}

// Close closes Fallback.
func (f *Fallback) Close() {}
