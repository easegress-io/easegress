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

package urlrule

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/util/stringtool"
)

type (
	// StringMatch defines the match rule of a string
	StringMatch struct {
		Exact  string `yaml:"exact" jsonschema:"omitempty"`
		Prefix string `yaml:"prefix" jsonschema:"omitempty"`
		RegEx  string `yaml:"regex" jsonschema:"omitempty,format=regexp"`
		re     *regexp.Regexp
	}

	// URLRule defines the match rule of a http request
	URLRule struct {
		id        string
		Methods   []string    `yaml:"methods" jsonschema:"omitempty,uniqueItems=true,format=httpmethod-array"`
		URL       StringMatch `yaml:"url" jsonschema:"required"`
		PolicyRef string      `yaml:"policyRef" jsonschema:"omitempty"`
	}
)

// Validate validates the StringMatch object
func (sm StringMatch) Validate() error {
	if sm.Exact != "" {
		return nil
	}

	if sm.Prefix != "" {
		return nil
	}

	if sm.RegEx != "" {
		return nil
	}

	return fmt.Errorf("at least one pattern must be configured")
}

// Init intizlize an StringMatch
func (sm *StringMatch) Init() {
	if sm.RegEx != "" {
		sm.re = regexp.MustCompile(sm.RegEx)
	}
}

// Match matches a string to the pattern
func (sm *StringMatch) Match(value string) bool {
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

// ID returns the ID of the URLRule.
// ID is the first valid one of Exact, Prefix, RegEx.
func (r *URLRule) ID() string {
	return r.id
}

// Init initialize an URLRule
func (r *URLRule) Init() {
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

// Match matches a URL to the rule
func (r *URLRule) Match(req context.HTTPRequest) bool {
	if len(r.Methods) > 0 {
		if !stringtool.StrInSlice(req.Method(), r.Methods) {
			return false
		}
	}

	return r.URL.Match(req.Path())
}

// DeepEqual returns true if r deep equal with r1 and false otherwise
func (r *URLRule) DeepEqual(r1 *URLRule) bool {
	if len(r.Methods) != len(r1.Methods) {
		return false
	}
	for i := 0; i < len(r.Methods); i++ {
		if r.Methods[i] != r1.Methods[i] {
			return false
		}
	}

	if r.URL.Exact != r1.URL.Exact {
		return false
	}

	if r.URL.Prefix != r1.URL.Prefix {
		return false
	}

	if r.URL.RegEx != r1.URL.RegEx {
		return false
	}

	return r.PolicyRef == r1.PolicyRef
}
