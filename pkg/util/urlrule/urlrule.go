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

// Package urlrule impelments match rule for HTTP requests.
package urlrule

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

type (
	// URLRule defines the match rule of a http request
	URLRule struct {
		id        string
		Methods   []string                 `json:"methods,omitempty" jsonschema:"uniqueItems=true,format=httpmethod-array"`
		URL       stringtool.StringMatcher `json:"url" jsonschema:"required"`
		PolicyRef string                   `json:"policyRef,omitempty"`
	}
)

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
	r.URL.Init()
}

// Match matches a URL to the rule
func (r *URLRule) Match(req *http.Request) bool {
	if len(r.Methods) > 0 {
		if !stringtool.StrInSlice(req.Method, r.Methods) {
			return false
		}
	}

	return r.URL.Match(req.URL.Path)
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
