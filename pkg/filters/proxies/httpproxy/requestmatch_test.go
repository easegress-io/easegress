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
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
	"github.com/stretchr/testify/assert"
)

func TestRequestMatcherSpecValidate(t *testing.T) {
	assert := assert.New(t)

	spec := &RequestMatcherSpec{}
	assert.Error(spec.Validate())

	spec.Policy = "headerHash"
	spec.Permil = 100
	assert.Error(spec.Validate())

	spec.Headers = map[string]*stringtool.StringMatcher{}
	spec.Headers["X-Test"] = &stringtool.StringMatcher{
		Empty: true,
		Exact: "abc",
	}
	assert.Error(spec.Validate())

	spec.Headers["X-Test"] = &stringtool.StringMatcher{Exact: "abc"}
	spec.URLs = append(spec.URLs, &MethodAndURLMatcher{
		URL: &stringtool.StringMatcher{
			Empty: true,
			Exact: "abc",
		},
	})
	assert.Error(spec.Validate())

	spec.URLs[0] = &MethodAndURLMatcher{
		URL: &stringtool.StringMatcher{Empty: true},
	}
	assert.Error(spec.Validate())

	spec.HeaderHashKey = "X-Test"
	assert.NoError(spec.Validate())
}

func TestGeneralMatche(t *testing.T) {
	assert := assert.New(t)

	// match all headers
	rm := NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			MatchAllHeaders: true,
			Headers: map[string]*stringtool.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Exact: "test2"},
			},
		},
	})

	stdr, _ := http.NewRequest(http.MethodGet, "http://megaease.com/abc", nil)
	req, _ := httpprot.NewRequest(stdr)

	stdr.Header.Set("X-Test1", "test1")
	assert.False(rm.Match(req))

	stdr.Header.Set("X-Test2", "not-test2")
	assert.False(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			MatchAllHeaders: true,
			Headers: map[string]*stringtool.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Empty: true, Exact: "test2"},
			},
		},
	})

	stdr.Header.Del("X-Test2")
	assert.True(rm.Match(req))

	// match one header
	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			Headers: map[string]*stringtool.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Empty: true, Exact: "test2"},
			},
		},
	})
	assert.True(rm.Match(req))

	stdr.Header.Del("X-Test1")
	assert.True(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			Headers: map[string]*stringtool.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Exact: "test2"},
			},
		},
	})
	assert.False(rm.Match(req))

	// match urls
	stdr.Header.Set("X-Test1", "test1")

	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			Headers: map[string]*stringtool.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Exact: "test2"},
			},
		},
		URLs: []*MethodAndURLMatcher{
			{
				Methods: []string{http.MethodGet},
				URL: &stringtool.StringMatcher{
					Exact: "/abc",
				},
			},
		},
	})
	assert.True(rm.Match(req))

	rm = NewRequestMatcher(&RequestMatcherSpec{
		RequestMatcherBaseSpec: proxies.RequestMatcherBaseSpec{
			Headers: map[string]*stringtool.StringMatcher{
				"X-Test1": {Exact: "test1"},
				"X-Test2": {Exact: "test2"},
			},
		},
		URLs: []*MethodAndURLMatcher{
			{
				Methods: []string{http.MethodGet},
				URL: &stringtool.StringMatcher{
					Exact: "/abcd",
				},
			},
		},
	})
	assert.False(rm.Match(req))
}

func TestMethodAndURLMatcher(t *testing.T) {
	assert := assert.New(t)

	m := &MethodAndURLMatcher{
		URL: &stringtool.StringMatcher{
			Exact: "/abc",
		},
	}
	assert.NoError(m.Validate())

	m.init()

	stdr, _ := http.NewRequest(http.MethodGet, "http://megaease.com/abc", nil)
	req, _ := httpprot.NewRequest(stdr)

	assert.True(m.Match(req))

	m.Methods = append(m.Methods, http.MethodGet)
	assert.True(m.Match(req))

	m.Methods = []string{http.MethodPost}
	assert.False(m.Match(req))
}
