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
	"github.com/megaease/easegress/pkg/util/urlrule"
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
	}

	// Spec describes the Mock.
	Spec struct {
		Rules []*Rule `yaml:"rules"`
	}

	// Rule is the mock rule.
	Rule struct {
		Match   MatchRule         `yaml:"match" jsonschema:"required"`
		Code    int               `yaml:"code" jsonschema:"required,format=httpcode"`
		Headers map[string]string `yaml:"headers" jsonschema:"omitempty"`
		Body    string            `yaml:"body" jsonschema:"omitempty"`
		Delay   string            `yaml:"delay" jsonschema:"omitempty,format=duration"`

		delay time.Duration
	}

	// MatchRule is the rule to match a request
	MatchRule struct {
		Path            string                          `yaml:"path,omitempty" jsonschema:"omitempty,pattern=^/"`
		PathPrefix      string                          `yaml:"pathPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		Headers         map[string]*urlrule.StringMatch `yaml:"headers" jsonschema:"omitempty"`
		MatchAllHeaders bool                            `yaml:"matchAllHeaders" jsonschema:"omitempty"`
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
func (m *Mock) Handle(ctx context.HTTPContext) string {
	result := ""
	if rule := m.match(ctx); rule != nil {
		m.mock(ctx, rule)
		result = resultMocked
	}
	return ctx.CallNextHandler(result)
}

func (m *Mock) match(ctx context.HTTPContext) *Rule {
	path := ctx.Request().Path()
	header := ctx.Request().Header()

	matchPath := func(rule *Rule) bool {
		if rule.Match.Path == "" && rule.Match.PathPrefix == "" {
			return true
		}

		if rule.Match.Path == path {
			return true
		}

		if rule.Match.PathPrefix == "" {
			return false
		}

		return strings.HasPrefix(path, rule.Match.PathPrefix)
	}

	matchOneHeader := func(key string, rule *urlrule.StringMatch) bool {
		values := header.GetAll(key)
		if len(values) == 0 {
			return rule.Empty
		}
		if rule.Empty {
			return false
		}

		for _, v := range values {
			if rule.Match(v) {
				return true
			}
		}

		return false
	}

	matchHeader := func(rule *Rule) bool {
		if len(rule.Match.Headers) == 0 {
			return true
		}

		for key, r := range rule.Match.Headers {
			if matchOneHeader(key, r) {
				if !rule.Match.MatchAllHeaders {
					return true
				}
			} else {
				if rule.Match.MatchAllHeaders {
					return false
				}
			}
		}

		return rule.Match.MatchAllHeaders
	}

	for _, rule := range m.spec.Rules {
		if matchPath(rule) && matchHeader(rule) {
			return rule
		}
	}

	return nil
}

func (m *Mock) mock(ctx context.HTTPContext, rule *Rule) {
	w := ctx.Response()
	w.SetStatusCode(rule.Code)
	for key, value := range rule.Headers {
		w.Header().Set(key, value)
	}
	w.SetBody(strings.NewReader(rule.Body))

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

// Status returns status.
func (m *Mock) Status() interface{} {
	return nil
}

// Close closes Mock.
func (m *Mock) Close() {
}
