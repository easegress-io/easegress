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

package proxies

import (
	"fmt"
	"hash/fnv"
	"math/rand"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

// RequestMatcher is the interface to match requests.
type RequestMatcher interface {
	Match(req protocols.Request) bool
}

// RequestMatcherBaseSpec describe RequestMatcher
type RequestMatcherBaseSpec struct {
	Policy          string                               `json:"policy,omitempty" jsonschema:"enum=,enum=general,enum=ipHash,enum=headerHash,enum=random"`
	MatchAllHeaders bool                                 `json:"matchAllHeaders,omitempty"`
	Headers         map[string]*stringtool.StringMatcher `json:"headers,omitempty"`
	Permil          uint32                               `json:"permil,omitempty" jsonschema:"minimum=0,maximum=1000"`
	HeaderHashKey   string                               `json:"headerHashKey,omitempty"`
}

// Validate validates the RequestMatcherBaseSpec.
func (s *RequestMatcherBaseSpec) Validate() error {
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

	if s.Policy == "headerHash" && s.HeaderHashKey == "" {
		return fmt.Errorf("headerHash needs to specify headerHashKey")
	}

	return nil
}

// NewRequestMatcher creates a new traffic matcher according to spec.
func NewRequestMatcher(spec *RequestMatcherBaseSpec) RequestMatcher {
	switch spec.Policy {
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
	v := req.Header().Get(hhm.headerHashKey).(string)
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
	ip := req.RealIP()
	hash := fnv.New32()
	hash.Write([]byte(ip))
	return hash.Sum32()%1000 < iphm.permill
}
