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

package grpcproxy

import (
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

// RequestMatcherSpec describe RequestMatcher
type RequestMatcherSpec struct {
	proxies.RequestMatcherBaseSpec `json:",inline"`
	Methods                        []*stringtool.StringMatcher `json:"methods,omitempty"`
}

// Validate validates the RequestMatcherSpec.
func (s *RequestMatcherSpec) Validate() error {
	if err := s.RequestMatcherBaseSpec.Validate(); err != nil {
		return err
	}

	for _, r := range s.Methods {
		if err := r.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// NewRequestMatcher creates a new traffic matcher according to spec.
func NewRequestMatcher(spec *RequestMatcherSpec) RequestMatcher {
	switch spec.Policy {
	case "", "general":
		matcher := &generalMatcher{
			matchAllHeaders: spec.MatchAllHeaders,
			headers:         spec.Headers,
			methods:         spec.Methods,
		}
		matcher.init()
		return matcher
	default:
		return proxies.NewRequestMatcher(&spec.RequestMatcherBaseSpec)
	}
}

// generalMatcher implements general grpc matcher.
type generalMatcher struct {
	matchAllHeaders bool
	headers         map[string]*stringtool.StringMatcher
	methods         []*stringtool.StringMatcher
}

func (gm *generalMatcher) init() {
	for _, h := range gm.headers {
		h.Init()
	}

	for _, m := range gm.methods {
		m.Init()
	}
}

// Match implements protocols.Matcher.
func (gm *generalMatcher) Match(req protocols.Request) bool {
	grpcreq, ok := req.(*grpcprot.Request)
	if !ok {
		panic("not a grpc request")
	}

	matched := false
	if gm.matchAllHeaders {
		matched = gm.matchAllHeader(grpcreq)
	} else {
		matched = gm.matchOneHeader(grpcreq)
	}

	if matched && len(gm.methods) > 0 {
		matched = gm.matchMethod(grpcreq)
	}

	return matched
}

func (gm *generalMatcher) matchOneHeader(req *grpcprot.Request) bool {
	h := req.RawHeader()

	for key, rule := range gm.headers {
		values := h.RawGet(key)

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

func (gm *generalMatcher) matchAllHeader(req *grpcprot.Request) bool {
	h := req.RawHeader()

	for key, rule := range gm.headers {
		values := h.RawGet(key)

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

func (gm *generalMatcher) matchMethod(req *grpcprot.Request) bool {
	for _, m := range gm.methods {
		if m.Match(req.FullMethod()) {
			return true
		}
	}
	return false
}
