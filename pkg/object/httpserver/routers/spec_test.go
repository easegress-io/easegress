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

package routers

import (
	"net/http"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/ipfilter"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestRuleInit(t *testing.T) {
	assert := assert.New(t)

	rules := Rules{
		&Rule{
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
		},
		&Rule{
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
		},
	}

	rules.Init()

	rule := rules[0]
	assert.NotNil(rule.Hosts[1].re)
	assert.NotNil(rule.ipFilter)
	assert.Equal(len(rule.Paths), 1)

	rule = rules[1]
	assert.NotNil(rule.Hosts[1].re)
	assert.NotNil(rule.ipFilter)

	assert.Equal(len(rule.Paths), 1)

	rule = &Rule{
		Host:       "www.megaease.com",
		HostRegexp: `^([^.]+\.megaease\.com$`,
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
	rule.Init()
	assert.Nil(rule.Hosts[1].re)
}

func TestRuleMatch(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com:8080", nil)
	req, _ := httpprot.NewRequest(stdr)
	ctx := NewContext(req)

	rule := &Rule{}
	rule.Init()

	assert.NotNil(rule)
	assert.True(rule.MatchHost(ctx))

	rule = &Rule{Host: "www.megaease.com"}
	rule.Init()
	assert.NotNil(rule)
	assert.True(rule.MatchHost(ctx))

	rule = &Rule{HostRegexp: `^[^.]+\.megaease\.com$`}
	rule.Init()
	assert.NotNil(rule)
	assert.True(rule.MatchHost(ctx))

	rule = &Rule{HostRegexp: `^[^.]+\.megaease\.cn$`}
	rule.Init()
	assert.NotNil(rule)
	assert.False(rule.MatchHost(ctx))

	testCases := []struct {
		request string
		value   string
		result  bool
	}{
		{request: "http://www.megaease.com:8080", value: "www.megaease.com", result: true},
		{request: "http://www.megaease.com:8080", value: "*.megaease.com", result: true},
		{request: "http://www.sub.megaease.com:8080", value: "*.megaease.com", result: true},
		{request: "http://www.example.megaease.com:8080", value: "*.megaease.com", result: true},
		{request: "http://www.megaease.com:8080", value: "www.megaease.*", result: true},
		{request: "http://www.megaease.cn:8080", value: "www.megaease.*", result: true},
		{request: "http://www.google.com:8080", value: "*.megaease.com", result: false},
	}
	for _, tc := range testCases {
		stdr, _ := http.NewRequest(http.MethodGet, tc.request, nil)
		req, _ := httpprot.NewRequest(stdr)
		ctx := NewContext(req)

		rule = &Rule{Hosts: []Host{{Value: tc.value}}}
		rule.Init()
		assert.Equal(tc.result, rule.MatchHost(ctx))
	}
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

	rule.Init()

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
		Backend:           "foo",
		ClientMaxBodySize: 1000,
	}

	path.Init(nil)

	assert.NotNil(path.ipFilter)
	assert.Equal(path.method, MALL)
	assert.NotNil(path.Headers[0].re)
	assert.NotNil(path.Queries[0].re)
	assert.Equal("foo", path.GetBackend())
	assert.EqualValues(1000, path.GetClientMaxBodySize())

	path.Methods = []string{"GET", "POST"}
	path.Init(nil)
	assert.True(path.method&mGET != 0)
	assert.True(path.method&mPOST != 0)
	assert.True(path.method&mDELETE == 0)
}

func TestPathValidate(t *testing.T) {
	p := &Path{RewriteTarget: "abc"}
	assert.Error(t, p.Validate())

	p.Path = "foo"
	assert.NoError(t, p.Validate())

	p.Path = ""
	p.PathPrefix = "foo"
	assert.NoError(t, p.Validate())

	p.PathPrefix = ""
	p.PathRegexp = "^foo"
	assert.NoError(t, p.Validate())

	p.PathRegexp = ""
	p.RewriteTarget = ""
	assert.NoError(t, p.Validate())
}

func TestPathInit2(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
	}

	path.Init(nil)
	assert.True(path.cacheable)
	assert.False(path.matchable)

	path = &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
		IPFilterSpec: &ipfilter.Spec{
			BlockByDefault: true,
			AllowIPs:       []string{"192.168.1.0/24"},
		},
	}

	path.Init(nil)
	assert.False(path.cacheable)
	assert.True(path.matchable)

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
	assert.True(path.matchable)
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
			result: false,
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
		assert.Equal(test.ipNotAllowed, ctx.IPMismatch)

	}
}

func TestPathMatch1(t *testing.T) {
	assert := assert.New(t)

	path := &Path{
		Path:       "/api/task/check",
		PathRegexp: `^[^.]+\.megaease\.com$`,
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

func TestHeadersValidate(t *testing.T) {
	headers := Headers{
		{
			Key: "X-Test1",
		},
		{
			Key:    "X-Test2",
			Regexp: `^abc$`,
		},
	}

	assert.Error(t, headers.Validate())

	headers[0].Values = []string{"abc"}
	assert.NoError(t, headers.Validate())
}

func TestHeadersMatch(t *testing.T) {
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
		{
			headers: map[string][]string{
				"X-Test2": {"test2"},
				"X-Test1": {"test1"},
				"X-Test3": {"test4"},
			},
			matchAll: true,
			result:   false,
		},
	}

	assert := assert.New(t)

	headers := Headers{}
	headers.init()

	for _, test := range tests {
		result := headers.Match(test.headers, test.matchAll)
		assert.True(result)
	}

	headers = Headers{
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

func TestQueriesValidate(t *testing.T) {
	qs := Queries{
		{
			Key: "X-Test1",
		},
		{
			Key:    "X-Test2",
			Regexp: `^abc$`,
		},
	}

	assert.Error(t, qs.Validate())

	qs[0].Values = []string{"abc"}
	assert.NoError(t, qs.Validate())
}

func TestQueriesMatch(t *testing.T) {
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
		{
			queries: map[string][]string{
				"X-Test2": {"test2"},
				"X-Test1": {"test1"},
				"X-Test3": {"test4"},
			},
			matchAll: true,
			result:   false,
		},
	}

	assert := assert.New(t)

	queries := Queries{}
	queries.init()
	for _, test := range tests {
		result := queries.Match(test.queries, test.matchAll)
		assert.True(result)
	}

	queries = Queries{
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

	for _, test := range tests {
		result := queries.Match(test.queries, test.matchAll)
		assert.Equal(test.result, result)
	}
}
