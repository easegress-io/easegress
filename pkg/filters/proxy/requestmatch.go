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

package proxy

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"strings"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

// RequestMatcher is the interface to match requests.
type RequestMatcher interface {
	Match(req *httpprot.Request) bool
}

// RequestMatcherSpec describe RequestMatcher
type RequestMatcherSpec struct {
	Policy          string                    `json:"policy" jsonschema:"omitempty,enum=,enum=general,enum=ipHash,enum=headerHash,enum=random"`
	MatchAllHeaders bool                      `json:"matchAllHeaders" jsonschema:"omitempty"`
	Headers         map[string]*StringMatcher `json:"headers" jsonschema:"omitempty"`
	URLs            []*MethodAndURLMatcher    `json:"urls" jsonschema:"omitempty"`
	Permil          uint32                    `json:"permil" jsonschema:"omitempty,minimum=0,maximum=1000"`
	HeaderHashKey   string                    `json:"headerHashKey" jsonschema:"omitempty"`
}

// Validate validtes the RequestMatcherSpec.
func (s *RequestMatcherSpec) Validate() error {
	if s.Policy == "general" || s.Policy == "" {
		if len(s.Headers) == 0 {
			return fmt.Errorf("headers is not specified")
		}
	} else if s.Permil == 0 {
		return fmt.Errorf("permil is not specified")
	}

	for _, v := range s.Headers {
		if err := v.Validate(); err != nil {
			return err
		}
	}

	for _, r := range s.URLs {
		if err := r.Validate(); err != nil {
			return err
		}
	}

	if s.Policy == "headerHash" && s.HeaderHashKey == "" {
		return fmt.Errorf("headerHash needs to specify headerHashKey")
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
			urls:            spec.URLs,
		}
		matcher.init()
		return matcher
	case "ipHash":
		return &ipHashMatcher{permill: spec.Permil}
	case "headerHash":
		return &headerHashMatcher{
			permill:       spec.Permil,
			headerHashKey: spec.HeaderHashKey,
		}
	case "random":
		return &randomMatcher{permill: spec.Permil}
	}

	logger.Errorf("BUG: unsupported probability policy: %s", spec.Policy)
	return &ipHashMatcher{permill: spec.Permil}
}

// randomMatcher implements random request matcher.
type randomMatcher struct {
	permill uint32
}

// Match implements protocols.Matcher.
func (rm randomMatcher) Match(req *httpprot.Request) bool {
	return rand.Uint32()%1000 < rm.permill
}

// headerHashMatcher implements header hash request matcher.
type headerHashMatcher struct {
	permill       uint32
	headerHashKey string
}

// Match implements protocols.Matcher.
func (hhm headerHashMatcher) Match(req *httpprot.Request) bool {
	v := req.HTTPHeader().Get(hhm.headerHashKey)
	hash := fnv.New32()
	hash.Write([]byte(v))
	return hash.Sum32()%1000 < hhm.permill
}

// ipHashMatcher implements IP address hash matcher.
type ipHashMatcher struct {
	permill uint32
}

// Match implements protocols.Matcher.
func (iphm ipHashMatcher) Match(req *httpprot.Request) bool {
	ip := req.RealIP()
	hash := fnv.New32()
	hash.Write([]byte(ip))
	return hash.Sum32()%1000 < iphm.permill
}

// generalMatcher implements general HTTP matcher.
type generalMatcher struct {
	matchAllHeaders bool
	headers         map[string]*StringMatcher
	urls            []*MethodAndURLMatcher
}

func (gm *generalMatcher) init() {
	for _, h := range gm.headers {
		h.init()
	}

	for _, url := range gm.urls {
		url.init()
	}
}

// Match implements protocols.Matcher.
func (gm *generalMatcher) Match(req *httpprot.Request) bool {
	matched := false
	if gm.matchAllHeaders {
		matched = gm.matchAllHeader(req)
	} else {
		matched = gm.matchOneHeader(req)
	}

	if matched && len(gm.urls) > 0 {
		matched = gm.matchURL(req)
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
	Methods []string       `json:"methods" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
	URL     *StringMatcher `json:"url" jsonschema:"required"`
}

// Validate validates the MethodAndURLMatcher.
func (r *MethodAndURLMatcher) Validate() error {
	return r.URL.Validate()
}

func (r *MethodAndURLMatcher) init() {
	r.URL.init()
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

// StringMatcher defines the match rule of a string
type StringMatcher struct {
	Exact  string `json:"exact" jsonschema:"omitempty"`
	Prefix string `json:"prefix" jsonschema:"omitempty"`
	RegEx  string `json:"regex" jsonschema:"omitempty,format=regexp"`
	Empty  bool   `json:"empty" jsonschema:"omitempty"`
	re     *regexp.Regexp
}

// Validate validates the StringMatcher.
func (sm *StringMatcher) Validate() error {
	if sm.Empty {
		if sm.Exact != "" || sm.Prefix != "" || sm.RegEx != "" {
			return fmt.Errorf("empty is conflict with other patterns")
		}
		return nil
	}

	if sm.Exact != "" {
		return nil
	}

	if sm.Prefix != "" {
		return nil
	}

	if sm.RegEx != "" {
		return nil
	}

	return fmt.Errorf("all patterns are empty")
}

func (sm *StringMatcher) init() {
	if sm.RegEx != "" {
		sm.re = regexp.MustCompile(sm.RegEx)
	}
}

// Match matches a string.
func (sm *StringMatcher) Match(value string) bool {
	if sm.Empty && value == "" {
		return true
	}

	if sm.Exact != "" && value == sm.Exact {
		return true
	}

	if sm.Prefix != "" && strings.HasPrefix(value, sm.Prefix) {
		return true
	}

	if sm.re == nil {
		return false
	}

	return sm.re.MatchString(value)
}

// MatchAny return true if any of the values matches.
func (sm *StringMatcher) MatchAny(values []string) bool {
	for _, v := range values {
		if sm.Match(v) {
			return true
		}
	}
	return false
}
