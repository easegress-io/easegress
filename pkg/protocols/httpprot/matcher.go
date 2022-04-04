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

package httpprot

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/util/stringtool"

	yaml "gopkg.in/yaml.v2"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	policyIPHash     = "ipHash"
	policyHeaderHash = "headerHash"
	policyRandom     = "random"
)

// TrafficMatcherSpec describe TrafficMatcher
type TrafficMatcherSpec struct {
	MatchAllHeaders bool                      `yaml:"matchAllHeaders" jsonschema:"omitempty"`
	Headers         map[string]*StringMatcher `yaml:"headers" jsonschema:"omitempty"`
	URLs            []*MethodAndURLMatcher    `yaml:"urls" jsonschema:"omitempty"`
	Probability     *ProbabilitySpec          `yaml:"probability,omitempty" jsonschema:"omitempty"`
}

// ProbabilitySpec defines rule to match traffic by probability.
type ProbabilitySpec struct {
	PerMill       uint32 `yaml:"perMill" jsonschema:"required,minimum=1,maximum=1000"`
	Policy        string `yaml:"policy" jsonschema:"required,enum=ipHash,enum=headerHash,enum=random"`
	HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
}

// Validate validtes the TrafficMatcherSpec.
func (s *TrafficMatcherSpec) Validate() error {
	if len(s.Headers) == 0 && s.Probability == nil {
		return fmt.Errorf("none of headers and probability is specified")
	}

	if len(s.Headers) > 0 && s.Probability != nil {
		return fmt.Errorf("both headers and probability are specified")
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

	if s.Probability != nil {
		return s.Probability.Validate()
	}

	return nil
}

// Validate validates the ProbabilitySpec.
func (ps *ProbabilitySpec) Validate() error {
	if ps.Policy == policyHeaderHash && ps.HeaderHashKey == "" {
		return fmt.Errorf("headerHash needs to speficy headerHashKey")
	}

	return nil
}

// NewTrafficMatcher creates a new traffic matcher according to spec.
func NewTrafficMatcher(spec interface{}) (protocols.TrafficMatcher, error) {
	tms, ok := spec.(*TrafficMatcherSpec)
	if !ok {
		data, err := yaml.Marshal(spec)
		if err != nil {
			return nil, err
		}
		tms := &TrafficMatcherSpec{}
		if err = yaml.Unmarshal(data, tms); err != nil {
			return nil, err
		}
	}

	if err := tms.Validate(); err != nil {
		return nil, err
	}

	if len(tms.Headers) > 0 {
		matcher := &generalMatcher{
			matchAllHeaders: tms.MatchAllHeaders,
			headers:         tms.Headers,
			urls:            tms.URLs,
		}
		matcher.init()
		return matcher, nil
	}

	switch tms.Probability.Policy {
	case policyIPHash:
		return &ipHashMatcher{permill: tms.Probability.PerMill}, nil
	case policyHeaderHash:
		return &headerHashMatcher{
			permill:       tms.Probability.PerMill,
			headerHashKey: tms.Probability.HeaderHashKey,
		}, nil
	case policyRandom:
		return &randomMatcher{permill: tms.Probability.PerMill}, nil
	}

	logger.Errorf("BUG: unsupported probability policy: %s", tms.Probability.Policy)
	return &ipHashMatcher{permill: tms.Probability.PerMill}, nil
}

// randomMatcher implements random request matcher.
// TODO: move to package protocols??
type randomMatcher struct {
	permill uint32
}

// Match implements protocols.Matcher.
func (rm randomMatcher) Match(req protocols.Request) bool {
	return rand.Uint32()%1000 < rm.permill
}

// headerHashMatcher implements header hash request matcher.
type headerHashMatcher struct {
	permill       uint32
	headerHashKey string
}

// Match implements protocols.Matcher.
func (hhm headerHashMatcher) Match(req protocols.Request) bool {
	v := req.(*Request).HTTPHeader().Get(hhm.headerHashKey)
	hash := fnv.New32()
	hash.Write([]byte(v))
	return hash.Sum32()%1000 < hhm.permill
}

// ipHashMatcher implements IP address hash matcher.
type ipHashMatcher struct {
	permill uint32
}

// Match implements protocols.Matcher.
func (iphm ipHashMatcher) Match(req protocols.Request) bool {
	ip := req.(*Request).RealIP()
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
func (gm *generalMatcher) Match(r protocols.Request) bool {
	req := r.(*Request)

	matched := false
	if gm.matchAllHeaders {
		matched = gm.matchOneHeader(req)
	} else {
		matched = gm.matchAllHeader(req)
	}

	if matched && len(gm.urls) > 0 {
		matched = gm.matchURL(req)
	}

	return matched
}

func (gm *generalMatcher) matchOneHeader(req *Request) bool {
	h := req.HTTPHeader()

	for key, rule := range gm.headers {
		values := h.Values(key)

		if len(values) == 0 {
			if rule.Match("") {
				return true
			}
			continue
		}

		for _, v := range values {
			if rule.Match(v) {
				return true
			}
		}
	}

	return false
}

func (gm *generalMatcher) matchAllHeader(req *Request) bool {
	h := req.HTTPHeader()

OUTER_LOOP:
	for key, rule := range gm.headers {
		values := h.Values(key)

		if len(values) == 0 {
			if rule.Match("") {
				continue
			}
			return false
		}

		for _, v := range values {
			if rule.Match(v) {
				continue OUTER_LOOP
			}
		}

		return false
	}

	return true
}

func (gm *generalMatcher) matchURL(req *Request) bool {
	for _, url := range gm.urls {
		if url.match(req) {
			return true
		}
	}
	return false
}

// MethodAndURLMatcher defines the match rule of a http request
type MethodAndURLMatcher struct {
	Methods []string       `yaml:"methods" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
	URL     *StringMatcher `yaml:"url" jsonschema:"required"`
}

// Validate validates the MethodAndURLMatcher.
func (r *MethodAndURLMatcher) Validate() error {
	if r.URL != nil {
		return r.URL.Validate()
	}
	return nil
}

func (r *MethodAndURLMatcher) init() {
	if r.URL != nil {
		r.URL.init()
	}
}

// Match matches a request.
func (r *MethodAndURLMatcher) Match(req protocols.Request) bool {
	return r.match(req.(*Request))
}

func (r *MethodAndURLMatcher) match(req *Request) bool {
	if len(r.Methods) > 0 {
		if !stringtool.StrInSlice(req.Method(), r.Methods) {
			return false
		}
	}

	return r.URL.Match(req.URL().Path)
}

// StringMatcher defines the match rule of a string
type StringMatcher struct {
	Exact  string `yaml:"exact" jsonschema:"omitempty"`
	Prefix string `yaml:"prefix" jsonschema:"omitempty"`
	RegEx  string `yaml:"regex" jsonschema:"omitempty,format=regexp"`
	Empty  bool   `yaml:"empty" jsonschema:"omitempty"`
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
