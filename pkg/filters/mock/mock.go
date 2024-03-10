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

// Package mock provides Mock filter.
package mock

import (
	"strings"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

const (
	// Kind is the kind of Mock.
	Kind = "Mock"

	resultMocked = "mocked"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Mock mocks the response.",
	Results:     []string{resultMocked},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Mock{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// Mock is filter Mock.
	Mock struct {
		spec *Spec
	}

	// Spec describes the Mock.
	Spec struct {
		filters.BaseSpec `json:",inline"`

		Rules []*Rule `json:"rules"`
	}

	// Rule is the mock rule.
	Rule struct {
		Match   MatchRule         `json:"match" jsonschema:"required"`
		Code    int               `json:"code" jsonschema:"required,format=httpcode"`
		Headers map[string]string `json:"headers,omitempty"`
		Body    string            `json:"body,omitempty"`
		Delay   string            `json:"delay,omitempty" jsonschema:"format=duration"`

		delay time.Duration
	}

	// MatchRule is the rule to match a request
	MatchRule struct {
		Path            string                               `json:"path,omitempty" jsonschema:"pattern=^/"`
		PathPrefix      string                               `json:"pathPrefix,omitempty" jsonschema:"pattern=^/"`
		Headers         map[string]*stringtool.StringMatcher `json:"headers,omitempty"`
		MatchAllHeaders bool                                 `json:"matchAllHeaders,omitempty"`
	}
)

// Name returns the name of the Mock filter instance.
func (m *Mock) Name() string {
	return m.spec.Name()
}

// Kind returns the kind of Mock.
func (m *Mock) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Mock
func (m *Mock) Spec() filters.Spec {
	return m.spec
}

// Init initializes Mock.
func (m *Mock) Init() {
	m.reload()
}

// Inherit inherits previous generation of Mock.
func (m *Mock) Inherit(previousGeneration filters.Filter) {
	m.Init()
}

func (m *Mock) reload() {
	for _, r := range m.spec.Rules {
		if r.Delay == "" {
			continue
		}
		r.delay, _ = time.ParseDuration(r.Delay)
	}
}

// Handle mocks Context.
func (m *Mock) Handle(ctx *context.Context) string {
	result := ""
	if rule := m.match(ctx); rule != nil {
		m.mock(ctx, rule)
		result = resultMocked
	}
	return result
}

func (m *Mock) match(ctx *context.Context) *Rule {
	req := ctx.GetInputRequest().(*httpprot.Request)
	path := req.Path()
	header := req.HTTPHeader()

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

	matchOneHeader := func(key string, rule *stringtool.StringMatcher) bool {
		values := header.Values(key)
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

func (m *Mock) mock(ctx *context.Context, rule *Rule) {
	resp, _ := httpprot.NewResponse(nil)

	resp.SetStatusCode(rule.Code)
	for key, value := range rule.Headers {
		resp.Std().Header.Set(key, value)
	}
	resp.SetPayload([]byte(rule.Body))
	ctx.SetOutputResponse(resp)

	if rule.delay <= 0 {
		return
	}

	req := ctx.GetInputRequest().(*httpprot.Request)
	logger.Debugf("delay for %v ...", rule.delay)
	select {
	case <-req.Context().Done():
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
