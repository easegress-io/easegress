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

package httpproxy

import (
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

// RequestMatcherSpec describe RequestMatcher
type RequestMatcherSpec struct {
	proxies.RequestMatcherBaseSpec `json:",inline"`
	URLs                           []*MethodAndURLMatcher `json:"urls,omitempty"`
}

// Validate validates the RequestMatcherSpec.
func (s *RequestMatcherSpec) Validate() error {
	if err := s.RequestMatcherBaseSpec.Validate(); err != nil {
		return err
	}

	for _, r := range s.URLs {
		if err := r.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// NewRequestMatcher creates a new traffic matcher according to spec.
func NewRequestMatcher(spec *RequestMatcherSpec) proxies.RequestMatcher {
	switch spec.Policy {
	case "", "general":
		matcher := &generalMatcher{
			matchAllHeaders: spec.MatchAllHeaders,
			headers:         spec.Headers,
			urls:            spec.URLs,
		}
		matcher.init()
		return matcher
	default:
		return proxies.NewRequestMatcher(&spec.RequestMatcherBaseSpec)
	}
}

// generalMatcher implements general HTTP matcher.
type generalMatcher struct {
	matchAllHeaders bool
	headers         map[string]*stringtool.StringMatcher
	urls            []*MethodAndURLMatcher
}

func (gm *generalMatcher) init() {
	for _, h := range gm.headers {
		h.Init()
	}

	for _, url := range gm.urls {
		url.init()
	}
}

// Match implements protocols.Matcher.
func (gm *generalMatcher) Match(req protocols.Request) bool {
	httpreq, ok := req.(*httpprot.Request)
	if !ok {
		panic("BUG: not a http request")
	}

	matched := false
	if gm.matchAllHeaders {
		matched = gm.matchAllHeader(httpreq)
	} else {
		matched = gm.matchOneHeader(httpreq)
	}

	if matched && len(gm.urls) > 0 {
		matched = gm.matchURL(httpreq)
	}

	return matched
}

func (gm *generalMatcher) matchOneHeader(req *httpprot.Request) bool {
	h := req.HTTPHeader()

	for key, rule := range gm.headers {
		values := h.Values(key)

		if len(values) == 0 {
			if rule.Match("") {
				return true
			}
		} else {
			for _, v := range values {
				if rule.Match(v) {
					return true
				}
			}
		}
	}
	return false
}

func (gm *generalMatcher) matchAllHeader(req *httpprot.Request) bool {
	h := req.HTTPHeader()

	for key, rule := range gm.headers {
		values := h.Values(key)

		if len(values) == 0 {
			if !rule.Match("") {
				return false
			}
		} else {
			if !rule.MatchAny(values) {
				return false
			}
		}
	}
	return true
}

func (gm *generalMatcher) matchURL(req *httpprot.Request) bool {
	for _, url := range gm.urls {
		if url.Match(req) {
			return true
		}
	}
	return false
}

// MethodAndURLMatcher defines the match rule of a http request
type MethodAndURLMatcher struct {
	Methods []string                  `json:"methods,omitempty" jsonschema:"uniqueItems=true,format=httpmethod-array"`
	URL     *stringtool.StringMatcher `json:"url" jsonschema:"required"`
}

// Validate validates the MethodAndURLMatcher.
func (r *MethodAndURLMatcher) Validate() error {
	return r.URL.Validate()
}

func (r *MethodAndURLMatcher) init() {
	r.URL.Init()
}

// Match matches a request.
func (r *MethodAndURLMatcher) Match(req *httpprot.Request) bool {
	if len(r.Methods) > 0 {
		if !stringtool.StrInSlice(req.Method(), r.Methods) {
			return false
		}
	}

	return r.URL.Match(req.URL().Path)
}
