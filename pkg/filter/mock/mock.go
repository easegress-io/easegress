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

package mock

import (
	"strings"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
)

const (
	// Kind is the kind of Mock.
	Kind = "Mock"

	resultMocked = "mocked"
)

var results = []string{resultMocked}

func init() {
	httppipeline.Register(&Mock{})
}

type (
	// Mock is filter Mock.
	Mock struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec

		body []byte
	}

	// Spec describes the Mock.
	Spec struct {
		Rules []*Rule `yaml:"rules"`
	}

	// Rule is the mock rule.
	Rule struct {
		Path       string            `yaml:"path,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathPrefix string            `yaml:"pathPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		Code       int               `yaml:"code" jsonschema:"required,format=httpcode"`
		Headers    map[string]string `yaml:"headers" jsonschema:"omitempty"`
		Body       string            `yaml:"body" jsonschema:"omitempty"`
		Delay      string            `yaml:"delay" jsonschema:"omitempty,format=duration"`

		delay time.Duration
	}
)

// Kind returns the kind of Mock.
func (m *Mock) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of Mock.
func (m *Mock) DefaultSpec() interface{} {
	return &Spec{}
}

// Description returns the description of Mock.
func (m *Mock) Description() string {
	return "Mock mocks the response."
}

// Results returns the results of Mock.
func (m *Mock) Results() []string {
	return results
}

// Init initializes Mock.
func (m *Mock) Init(filterSpec *httppipeline.FilterSpec) {
	m.filterSpec, m.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	m.reload()
}

// Inherit inherits previous generation of Mock.
func (m *Mock) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	m.Init(filterSpec)
}

func (m *Mock) reload() {
	for _, r := range m.spec.Rules {
		if r.Delay == "" {
			continue
		}
		r.delay, _ = time.ParseDuration(r.Delay)
	}
}

// Handle mocks HTTPContext.
func (m *Mock) Handle(ctx context.HTTPContext) (result string) {
	result = m.handle(ctx)
	return ctx.CallNextHandler(result)
}

func (m *Mock) handle(ctx context.HTTPContext) (result string) {
	path := ctx.Request().Path()
	w := ctx.Response()

	mock := func(rule *Rule) {
		w.SetStatusCode(rule.Code)
		for key, value := range rule.Headers {
			w.Header().Set(key, value)
		}
		w.SetBody(strings.NewReader(rule.Body))
		result = resultMocked

		if rule.delay <= 0 {
			return
		}

		logger.Debugf("delay for %v ...", rule.delay)
		select {
		case <-ctx.Done():
			logger.Debugf("request cancelled in the middle of delay mocking")
		case <-time.After(rule.delay):
		}
	}

	for _, rule := range m.spec.Rules {
		if rule.Path == "" && rule.PathPrefix == "" {
			mock(rule)
			return
		}

		if rule.Path == path {
			mock(rule)
			return
		}

		if rule.PathPrefix != "" && strings.HasPrefix(path, rule.PathPrefix) {
			mock(rule)
			return
		}
	}

	return ""
}

// Status returns status.
func (m *Mock) Status() interface{} {
	return nil
}

// Close closes Mock.
func (m *Mock) Close() {}
