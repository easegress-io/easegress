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

package httpfilter

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/hashtool"
	"github.com/megaease/easegress/pkg/util/urlrule"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	policyIPHash     = "ipHash"
	policyHeaderHash = "headerHash"
	policyRandom     = "random"
)

type (
	// Spec describes HTTPFilter.
	Spec struct {
		MatchAllHeaders bool                            `yaml:"matchAllHeaders" jsonschema:"omitempty"`
		Headers         map[string]*urlrule.StringMatch `yaml:"headers" jsonschema:"omitempty"`
		URLs            []*urlrule.URLRule              `yaml:"urls" jsonschema:"omitempty"`
		Probability     *Probability                    `yaml:"probability,omitempty" jsonschema:"omitempty"`
	}

	// HTTPFilter filters HTTP traffic.
	HTTPFilter struct {
		spec *Spec
	}

	// Probability filters HTTP traffic by probability.
	Probability struct {
		PerMill       uint32 `yaml:"perMill" jsonschema:"required,minimum=1,maximum=1000"`
		Policy        string `yaml:"policy" jsonschema:"required,enum=ipHash,enum=headerHash,enum=random"`
		HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
	}
)

// Validate validates Probability.
func (p Probability) Validate() error {
	if p.Policy == policyHeaderHash && p.HeaderHashKey == "" {
		return fmt.Errorf("headerHash needs to speficy headerHashKey")
	}

	return nil
}

// Validate validates Spec
func (s Spec) Validate() error {
	if len(s.Headers) == 0 && s.Probability == nil {
		return fmt.Errorf("none of headers and probability is specified")
	}

	if len(s.Headers) > 0 && s.Probability != nil {
		return fmt.Errorf("both headers and probability are specified")
	}

	return nil
}

// New creates an HTTPFilter.
func New(spec *Spec) *HTTPFilter {
	hf := &HTTPFilter{
		spec: spec,
	}

	for _, stringMatcher := range spec.Headers {
		stringMatcher.Init()
	}

	for _, url := range spec.URLs {
		url.Init()
	}

	return hf
}

// Filter filters HTTPContext.
func (hf *HTTPFilter) Filter(req *httpprot.Request) bool {
	if len(hf.spec.Headers) > 0 {
		matchHeader := hf.filterHeader(req)
		if matchHeader && len(hf.spec.URLs) > 0 {
			return hf.filterURL(req)
		}
		return matchHeader
	}

	return hf.filterProbability(req)
}

func (hf *HTTPFilter) filterHeader(req *httpprot.Request) bool {
	h := req.HTTPHeader()
	headerMatchNum := 0
	for key, matchRule := range hf.spec.Headers {
		// NOTE: Quickly break for performance.
		if headerMatchNum >= 1 && !hf.spec.MatchAllHeaders {
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
			if matchRule.Match(value) {
				headerMatchNum++
				break
			}
		}
	}

	needHeaderMatchNum := 1
	if hf.spec.MatchAllHeaders {
		needHeaderMatchNum = len(hf.spec.Headers)
	}

	if headerMatchNum >= needHeaderMatchNum {
		return true
	}

	return false
}

func (hf *HTTPFilter) filterURL(req *httpprot.Request) bool {
	urlMatch := false
	for _, url := range hf.spec.URLs {
		if url.Match(req) {
			urlMatch = true
			break
		}
	}
	return urlMatch
}

func (hf *HTTPFilter) filterProbability(req *httpprot.Request) bool {
	prob := hf.spec.Probability

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
