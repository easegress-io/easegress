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

package routers

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/stretchr/testify/assert"
)

func TestRuleInit(t *testing.T) {
	assert := assert.New(t)

	rule := &Rule{
		Host:       "www.megaease.com",
		HostRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			AllowIPs: []string{"192.168.1.0/24"},
		},
		Paths: []*Path{
			{
				Path: "/api/test",
			},
		},
	}

	rule.Init(nil)

	assert.NotNil(rule.hostRE)
	assert.NotNil(rule.ipFilter)
	assert.NotNil(rule.ipFilterChain)
	assert.Equal(len(rule.ipFilterChain.Filters()), 1)
	assert.Equal(len(rule.Paths), 1)
	assert.NotNil(rule.Paths[0].ipFilterChain)
	assert.Equal(len(rule.Paths[0].ipFilterChain.Filters()), 1)

	rule2 := &Rule{
		Host:       "www.megaease.com",
		HostRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			AllowIPs: []string{"192.168.1.0/24"},
		},
		Paths: []*Path{
			{
				Path: "/api/test",
				IPFilterSpec: &ipfilter.Spec{
					AllowIPs: []string{"192.168.1.0/24"},
				},
			},
		},
	}

	rule2.Init(ipfilter.NewIPFilterChain(nil, &ipfilter.Spec{
		AllowIPs: []string{"192.168.1.0/24"},
	}))

	assert.NotNil(rule2.hostRE)
	assert.NotNil(rule2.ipFilter)
	assert.NotNil(rule2.ipFilterChain)
	assert.Equal(len(rule2.ipFilterChain.Filters()), 2)
	assert.Equal(len(rule2.Paths), 1)
	assert.NotNil(rule2.Paths[0].ipFilterChain)
	assert.Equal(len(rule2.Paths[0].ipFilterChain.Filters()), 3)
}

func TestRuleMatch(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com:8080", nil)
	req, _ := httpprot.NewRequest(stdr)

	rule := &Rule{}
	rule.Init(nil)

	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{Host: "www.megaease.com"}
	rule.Init(nil)
	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{HostRegexp: `^[^.]+\.megaease\.com$`}
	rule.Init(nil)
	assert.NotNil(rule)
	assert.True(rule.Match(req))

	rule = &Rule{HostRegexp: `^[^.]+\.megaease\.cn$`}
	rule.Init(nil)
	assert.NotNil(rule)
	assert.False(rule.Match(req))
}

func TestRuleAllowIP(t *testing.T) {
	assert := assert.New(t)

	rule := &Rule{
		Host:       "www.megaease.com",
		HostRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
		Paths: []*Path{
			{
				Path: "/api/test",
			},
		},
	}

	rule.Init(nil)

	assert.True(rule.AllowIP("192.168.1.1"))
	assert.False(rule.AllowIP("10.168.1.1"))

	rule.ipFilter = nil
	assert.True(rule.AllowIP("192.168.1.1"))
	assert.True(rule.AllowIP("10.168.1.1"))
}

func TestPathInit(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			AllowIPs: []string{"192.168.1.0/24"},
		},
		Headers: []*Header{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
		Queries: []*Query{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
	}

	path.Init(nil)

	assert.NotNil(path.ipFilter)
	assert.NotNil(path.ipFilterChain)
	assert.Equal(len(path.ipFilterChain.Filters()), 1)
	assert.Equal(path.method, httpprot.MALL)
	assert.NotNil(path.Headers[0].re)
	assert.NotNil(path.Queries[0].re)

	path.Methods = []string{"GET", "POST"}
	path.Init(nil)
	assert.True(path.method&httpprot.MGET != 0)
	assert.True(path.method&httpprot.MPOST != 0)
	assert.True(path.method&httpprot.MDELETE == 0)
}

func TestPathInit2(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
	}

	path.Init(nil)
	assert.True(path.cacheable)
	assert.True(path.noMatchable)

	path = &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
	}

	path.Init(nil)
	assert.True(path.cacheable)
	assert.False(path.noMatchable)

	path = &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
		Headers: []*Header{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
		Queries: []*Query{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
	}

	path.Init(nil)
	assert.False(path.cacheable)
	assert.False(path.noMatchable)
}

func TestPathAllowIP(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
		Headers: []*Header{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
		Queries: []*Query{
			{
				Regexp: `^[^.]+\.megaease\.com$`,
			},
		},
	}

	path.Init(nil)

	assert.True(path.AllowIP("192.168.1.1"))
	assert.False(path.AllowIP("10.168.1.1"))

	path.ipFilter = nil
	assert.True(path.AllowIP("192.168.1.1"))
	assert.True(path.AllowIP("10.168.1.1"))
}

func TestPathAllowIPChain(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
	}
	path.Init(ipfilter.NewIPFilterChain(nil, &ipfilter.Spec{
		BlockByDefault: true,
		AllowIPs:       []string{"10.0.0.1/24"},
	}))

	assert.False(path.AllowIPChain("192.168.1.1"))
	assert.False(path.AllowIPChain("10.0.0.1"))
	assert.False(path.AllowIPChain("2.168.1.1"))

	path.ipFilterChain = nil
	assert.True(path.AllowIPChain("192.168.1.1"))
	assert.True(path.AllowIPChain("10.168.1.1"))
	assert.True(path.AllowIPChain("2.168.1.1"))
}

func TestPathMatch(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
		Methods: []string{"GET", "POST"},
		Headers: []*Header{
			{
				Key:    "X-Test",
				Values: []string{"abc", "123"},
			},
			{
				Key:    "X-Test1",
				Regexp: `^abc$`,
			},
		},
		Queries: []*Query{
			{
				Key:    "q1",
				Values: []string{"abc", "123"},
			},
			{
				Key:    "q2",
				Regexp: `^abc$`,
			},
		},
	}
	path.Init(nil)

	tests := []struct {
		method                                                                     string
		headers                                                                    map[string][]string
		matchAllheader                                                             bool
		query                                                                      string
		matchAllQuery                                                              bool
		ip                                                                         string
		result, methodMismatch, headerMismatch, queryMismatch, cache, ipNotAllowed bool
	}{
		{
			method:         http.MethodDelete,
			result:         false,
			methodMismatch: true,
		},

		{
			method: http.MethodGet,
			result: false,
			headers: map[string][]string{
				"X-Test": {"spec"},
			},
			matchAllheader: false,
			headerMismatch: true,
		},
		{
			method: http.MethodPost,
			result: false,
			headers: map[string][]string{
				"X-Test": {"abc"},
			},
			matchAllheader: true,
			headerMismatch: true,
		},

		{
			method: http.MethodPost,
			result: false,
			headers: map[string][]string{
				"X-Test": {"abc"},
			},
			query:         "q1=spec",
			queryMismatch: true,
		},

		{
			method: http.MethodPost,
			result: false,
			headers: map[string][]string{
				"X-Test": {"abc"},
			},
			query:         "q1=abc",
			matchAllQuery: true,
			queryMismatch: true,
		},

		{
			method: http.MethodPost,
			result: true,
			headers: map[string][]string{
				"X-Test":    {"abc"},
				"X-Real-Ip": {"10.168.1.0"},
			},
			query:         "q1=abc",
			queryMismatch: true,
			ipNotAllowed:  true,
		},

		{
			method: http.MethodPost,
			result: true,
			headers: map[string][]string{
				"X-Test":    {"abc"},
				"X-Real-Ip": {"192.168.1.0"},
			},
			query:         "q1=abc",
			queryMismatch: true,
		},
	}

	for _, test := range tests {
		stdr, _ := http.NewRequest(test.method, "/api/test?"+test.query, nil)
		stdr.Header = test.headers
		req, _ := httpprot.NewRequest(stdr)
		ctx := NewContext(req)

		path.MatchAllHeader = test.matchAllheader
		path.MatchAllQuery = test.matchAllQuery
		result := path.Match(ctx)

		assert.Equal(test.result, result)
		assert.Equal(test.methodMismatch, ctx.MethodMismatch)
		assert.Equal(test.headerMismatch, ctx.HeaderMismatch)
		assert.Equal(test.cache, ctx.Cacheable)
		assert.Equal(test.ipNotAllowed, ctx.IPNotAllowed)

	}
}

func TestPathMatch1(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
	}
	path.Init(nil)

	stdr, _ := http.NewRequest(http.MethodGet, "/api/test?", nil)
	req, _ := httpprot.NewRequest(stdr)

	ctx := NewContext(req)
	result := path.Match(ctx)

	assert.True(result)
	assert.True(ctx.Cacheable)
}

func TestHeadersInit(t *testing.T) {
	var headers Headers = []*Header{
		{
			Key:    "X-Test1",
			Regexp: `^abc$`,
		},
		{
			Key:    "X-Test2",
			Regexp: `^abc$`,
		},
	}

	headers.init()
	assert := assert.New(t)

	for _, h := range headers {
		assert.NotNil(h.re)
	}
}

func TestHeadersMatch(t *testing.T) {
	var headers Headers = []*Header{
		{
			Key:    "X-Test1",
			Regexp: `^test1$`,
		},
		{
			Key:    "X-Test2",
			Regexp: `^test2$`,
		},
		{
			Key:    "X-Test3",
			Values: []string{"test3"},
		},
	}

	headers.init()

	tests := []struct {
		headers          map[string][]string
		result, matchAll bool
	}{
		{
			headers: map[string][]string{
				"X-Test": {"abc"},
			},
			matchAll: false,
			result:   false,
		},
		{
			headers: map[string][]string{
				"X-Test3": {"abc"},
			},
			matchAll: false,
			result:   false,
		},
		{
			headers: map[string][]string{
				"X-Test3": {"test3"},
			},
			matchAll: false,
			result:   true,
		},
		{
			headers: map[string][]string{
				"X-Test1": {"test1"},
			},
			matchAll: false,
			result:   true,
		},
		{
			headers: map[string][]string{
				"X-Test2": {"test2"},
			},
			matchAll: false,
			result:   true,
		},

		{
			headers: map[string][]string{
				"X-Test2": {"test2"},
			},
			matchAll: true,
			result:   false,
		},
		{
			headers: map[string][]string{
				"X-Test2": {"test2"},
				"X-Test1": {"test1"},
				"X-Test3": {"test3"},
			},
			matchAll: true,
			result:   true,
		},
	}

	assert := assert.New(t)
	for _, test := range tests {
		result := headers.Match(test.headers, test.matchAll)
		assert.Equal(test.result, result)
	}
}

func TestQueriesInit(t *testing.T) {
	var queries Queries = []*Query{
		{
			Key:    "q1",
			Regexp: `^abc$`,
		},
		{
			Key:    "q2",
			Regexp: `^abc$`,
		},
	}

	queries.init()
	assert := assert.New(t)

	for _, q := range queries {
		assert.NotNil(q.re)
	}
}

func TestQueriesMatch(t *testing.T) {
	var queries Queries = []*Query{
		{
			Key:    "X-Test1",
			Regexp: `^test1$`,
		},
		{
			Key:    "X-Test2",
			Regexp: `^test2$`,
		},
		{
			Key:    "X-Test3",
			Values: []string{"test3"},
		},
	}

	queries.init()

	tests := []struct {
		queries          map[string][]string
		result, matchAll bool
	}{
		{
			queries: map[string][]string{
				"X-Test": {"abc"},
			},
			matchAll: false,
			result:   false,
		},
		{
			queries: map[string][]string{
				"X-Test3": {"abc"},
			},
			matchAll: false,
			result:   false,
		},
		{
			queries: map[string][]string{
				"X-Test3": {"test3"},
			},
			matchAll: false,
			result:   true,
		},
		{
			queries: map[string][]string{
				"X-Test1": {"test1"},
			},
			matchAll: false,
			result:   true,
		},
		{
			queries: map[string][]string{
				"X-Test2": {"test2"},
			},
			matchAll: false,
			result:   true,
		},

		{
			queries: map[string][]string{
				"X-Test2": {"test2"},
			},
			matchAll: true,
			result:   false,
		},
		{
			queries: map[string][]string{
				"X-Test2": {"test2"},
				"X-Test1": {"test1"},
				"X-Test3": {"test3"},
			},
			matchAll: true,
			result:   true,
		},
	}

	assert := assert.New(t)
	for _, test := range tests {
		result := queries.Match(test.queries, test.matchAll)
		assert.Equal(test.result, result)
	}
}
