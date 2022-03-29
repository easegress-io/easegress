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
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/util/hashtool"
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

// Matcher is HTTP implementation for protocols.TrafficMatcher
type Matcher struct {
	spec *MatcherSpec
	http.Request
}

// MatcherSpec describe Matcher
type MatcherSpec struct {
	MatchAllHeaders bool                           `yaml:"matchAllHeaders" jsonschema:"omitempty"`
	Headers         map[string]*MatcherStringMatch `yaml:"headers" jsonschema:"omitempty"`
	URLs            []*MatcherURLRule              `yaml:"urls" jsonschema:"omitempty"`
	Probability     *MatcherProbability            `yaml:"probability,omitempty" jsonschema:"omitempty"`
}

func (s *MatcherSpec) validate() error {
	if len(s.Headers) == 0 && s.Probability == nil {
		return fmt.Errorf("none of headers and probability is specified")
	}

	if len(s.Headers) > 0 && s.Probability != nil {
		return fmt.Errorf("both headers and probability are specified")
	}

	for _, v := range s.Headers {
		if err := v.validate(); err != nil {
			return err
		}
	}

	for _, r := range s.URLs {
		if err := r.validate(); err != nil {
			return err
		}
	}

	if err := s.Probability.validate(); err != nil {
		return err
	}
	return nil
}

var _ protocols.TrafficMatcher = (*Matcher)(nil)

func NewMatcher(spec interface{}) (protocols.TrafficMatcher, error) {
	data, err := yaml.Marshal(spec)
	if err != nil {
		return nil, err
	}
	matcherSpec := &MatcherSpec{}
	err = yaml.Unmarshal(data, matcherSpec)
	if err != nil {
		return nil, err
	}

	err = matcherSpec.validate()
	if err != nil {
		return nil, err
	}

	matcher := &Matcher{spec: matcherSpec}
	for _, stringMatcher := range matcherSpec.Headers {
		stringMatcher.init()
	}

	for _, url := range matcherSpec.URLs {
		url.init()
	}
	return matcher, nil
}

func (m *Matcher) Match(r protocols.Request) bool {
	req := r.(*Request)
	return m.match(req)
}

func (m *Matcher) match(req *Request) bool {
	if len(m.spec.Headers) > 0 {
		matchHeader := m.filterHeader(req)
		if matchHeader && len(m.spec.URLs) > 0 {
			return m.filterURL(req)
		}
		return matchHeader
	}
	return m.filterProbability(req)
}

func (m *Matcher) filterHeader(req *Request) bool {
	h := req.HTTPHeader()
	headerMatchNum := 0
	for key, matchRule := range m.spec.Headers {
		// NOTE: Quickly break for performance.
		if headerMatchNum >= 1 && !m.spec.MatchAllHeaders {
			break
		}

		values := h.Values(key)

		if len(values) == 0 && matchRule.Empty {
			headerMatchNum++
			continue
		}

		// NOTE: So even matchRule.RegEx match empty string,
		// it won't reach here when len(values) == 0.
		for _, value := range values {
			if matchRule.match(value) {
				headerMatchNum++
				break
			}
		}
	}

	needHeaderMatchNum := 1
	if m.spec.MatchAllHeaders {
		needHeaderMatchNum = len(m.spec.Headers)
	}

	if headerMatchNum >= needHeaderMatchNum {
		return true
	}

	return false
}

func (m *Matcher) filterURL(req *Request) bool {
	urlMatch := false
	for _, url := range m.spec.URLs {
		if url.match(req) {
			urlMatch = true
			break
		}
	}
	return urlMatch
}

func (m *Matcher) filterProbability(req *Request) bool {
	prob := m.spec.Probability

	var result uint32
	switch prob.Policy {
	case policyIPHash:
		result = hashtool.Hash32(req.RealIP())
	case policyHeaderHash:
		result = hashtool.Hash32(req.HTTPHeader().Get(prob.HeaderHashKey))
	case policyRandom:
		result = uint32(rand.Int31n(1000))
	default:
		logger.Errorf("BUG: unsupported probability policy: %s", prob.Policy)
		result = hashtool.Hash32(req.RealIP())
	}

	return result%1000 < prob.PerMill
}

// MatcherProbability filters HTTP traffic by probability.
type MatcherProbability struct {
	PerMill       uint32 `yaml:"perMill" jsonschema:"required,minimum=1,maximum=1000"`
	Policy        string `yaml:"policy" jsonschema:"required,enum=ipHash,enum=headerHash,enum=random"`
	HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
}

func (p *MatcherProbability) validate() error {
	if p.Policy == policyHeaderHash && p.HeaderHashKey == "" {
		return fmt.Errorf("headerHash needs to speficy headerHashKey")
	}

	return nil
}

// MatcherURLRule defines the match rule of a http request
type MatcherURLRule struct {
	id        string
	Methods   []string            `yaml:"methods" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
	URL       *MatcherStringMatch `yaml:"url" jsonschema:"required"`
	PolicyRef string              `yaml:"policyRef" jsonschema:"omitempty"`
}

func (r *MatcherURLRule) validate() error {
	if r.URL != nil {
		return r.URL.validate()
	}
	return nil
}

func (r *MatcherURLRule) init() {
	if r.URL.Exact != "" {
		r.id = r.URL.Exact
	} else if r.URL.Prefix != "" {
		r.id = r.URL.Prefix
	} else {
		r.id = r.URL.RegEx
	}
	if r.URL.RegEx != "" {
		r.URL.re = regexp.MustCompile(r.URL.RegEx)
	}
}

func (r *MatcherURLRule) match(req *Request) bool {
	if len(r.Methods) > 0 {
		if !stringtool.StrInSlice(req.Method(), r.Methods) {
			return false
		}
	}

	return r.URL.match(req.URL().Path)
}

// MatcherStringMatch defines the match rule of a string
type MatcherStringMatch struct {
	Exact  string `yaml:"exact" jsonschema:"omitempty"`
	Prefix string `yaml:"prefix" jsonschema:"omitempty"`
	RegEx  string `yaml:"regex" jsonschema:"omitempty,format=regexp"`
	Empty  bool   `yaml:"empty" jsonschema:"omitempty"`
	re     *regexp.Regexp
}

func (sm *MatcherStringMatch) validate() error {
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

	return fmt.Errorf("all patterns is empty")
}

func (sm *MatcherStringMatch) init() {
	if sm.RegEx != "" {
		sm.re = regexp.MustCompile(sm.RegEx)
	}
}

func (sm *MatcherStringMatch) match(value string) bool {
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
