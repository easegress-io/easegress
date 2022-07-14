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

package httpserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/stretchr/testify/assert"
)

func TestNewIPFilterChain(t *testing.T) {
	assert := assert.New(t)

	assert.Nil(newIPFilterChain(nil, nil))

	filters := newIPFilterChain(nil, &ipfilter.Spec{
		AllowIPs: []string{"192.168.1.0/24"},
	})
	assert.NotNil(filters)

	assert.NotNil(newIPFilterChain(filters, nil))
}

func TestNewIPFilter(t *testing.T) {
	assert := assert.New(t)
	assert.Nil(newIPFilter(nil))
	assert.NotNil(newIPFilter(&ipfilter.Spec{
		AllowIPs: []string{"192.168.1.0/24"},
	}))
}

func TestAllowIP(t *testing.T) {
	assert := assert.New(t)
	assert.True(allowIP(nil, "192.168.1.1"))
	filter := newIPFilter(&ipfilter.Spec{
		AllowIPs: []string{"192.168.1.0/24"},
		BlockIPs: []string{"192.168.2.0/24"},
	})
	assert.True(allowIP(filter, "192.168.1.1"))
	assert.False(allowIP(filter, "192.168.2.1"))
}

func TestMuxRule(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com:8080", nil)
	req, _ := httpprot.NewRequest(stdr)

	rule := newMuxRule(nil, &Rule{}, nil)
	assert.NotNil(rule)
	assert.True(rule.match(req))

	rule = newMuxRule(nil, &Rule{Host: "www.megaease.com"}, nil)
	assert.NotNil(rule)
	assert.True(rule.match(req))

	rule = newMuxRule(nil, &Rule{HostRegexp: `^[^.]+\.megaease\.com$`}, nil)
	assert.NotNil(rule)
	assert.True(rule.match(req))

	rule = newMuxRule(nil, &Rule{HostRegexp: `^[^.]+\.megaease\.cn$`}, nil)
	assert.NotNil(rule)
	assert.False(rule.match(req))
}

func TestMuxPath(t *testing.T) {
	assert := assert.New(t)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", nil)
	req, _ := httpprot.NewRequest(stdr)

	// 1. match path
	mp := newMuxPath(nil, &Path{})
	assert.NotNil(mp)
	assert.True(mp.matchPath(req))

	// exact match
	mp = newMuxPath(nil, &Path{Path: "/abc"})
	assert.NotNil(mp)
	assert.True(mp.matchPath(req))

	// prefix
	mp = newMuxPath(nil, &Path{PathPrefix: "/ab"})
	assert.NotNil(mp)
	assert.True(mp.matchPath(req))

	// regexp
	mp = newMuxPath(nil, &Path{PathRegexp: "/[a-z]+"})
	assert.NotNil(mp)
	assert.True(mp.matchPath(req))

	// invalid regexp
	mp = newMuxPath(nil, &Path{PathRegexp: "/[a-z+"})
	assert.NotNil(mp)
	assert.True(mp.matchPath(req))

	// not match
	mp = newMuxPath(nil, &Path{Path: "/xyz"})
	assert.NotNil(mp)
	assert.False(mp.matchPath(req))

	// 2. match method
	mp = newMuxPath(nil, &Path{})
	assert.NotNil(mp)
	assert.True(mp.matchMethod(req))

	mp = newMuxPath(nil, &Path{Methods: []string{http.MethodGet}})
	assert.NotNil(mp)
	assert.True(mp.matchMethod(req))

	mp = newMuxPath(nil, &Path{Methods: []string{http.MethodPut}})
	assert.NotNil(mp)
	assert.False(mp.matchMethod(req))

	// 3. match headers
	stdr.Header.Set("X-Test", "test1")

	mp = newMuxPath(nil, &Path{Headers: []*Header{{
		Key:    "X-Test",
		Values: []string{"test1", "test2"},
	}}})
	assert.True(mp.matchHeaders(req))

	mp = newMuxPath(nil, &Path{Headers: []*Header{{
		Key:    "X-Test",
		Regexp: "test[0-9]",
	}}})
	assert.True(mp.matchHeaders(req))

	mp = newMuxPath(nil, &Path{Headers: []*Header{{
		Key:    "X-Test2",
		Values: []string{"test1", "test2"},
	}}})
	assert.False(mp.matchHeaders(req))

	// 4. rewrite
	mp = newMuxPath(nil, &Path{Path: "/abc"})
	assert.NotNil(mp)
	mp.rewrite(req)
	assert.Equal("/abc", req.Path())

	mp = newMuxPath(nil, &Path{Path: "/abc", RewriteTarget: "/xyz"})
	assert.NotNil(mp)
	mp.rewrite(req)
	assert.Equal("/xyz", req.Path())

	mp = newMuxPath(nil, &Path{PathPrefix: "/xy", RewriteTarget: "/ab"})
	assert.NotNil(mp)
	mp.rewrite(req)
	assert.Equal("/abz", req.Path())

	mp = newMuxPath(nil, &Path{PathRegexp: "/([a-z]+)", RewriteTarget: "/1$1"})
	assert.NotNil(mp)
	mp.rewrite(req)
	assert.Equal("/1abz", req.Path())
}

func TestMuxReload(t *testing.T) {
	assert := assert.New(t)
	m := newMux(&httpstat.HTTPStat{}, &httpstat.TopN{}, nil)
	assert.NotNil(m)
	assert.NotNil(m.inst.Load())

	yamlSpec := `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
`
	superSpec, err := supervisor.NewSpec(yamlSpec)
	assert.NoError(err)
	assert.NotPanics(func() { m.reload(superSpec, nil) })

	yamlSpec = `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
cacheSize: 100
tracing:
  serviceName: test
  zipkin:
    serverURL: http://test.megaease.com/zipkin
    sampleRate: 0.1
rules:
- host: www.megaease.com
  paths:
  - path: /abc
    backend: abc-pipeline
- host: www.megaease.cn
  paths:
  - pathPrefix: /xyz
    backend: xyz-pipeline
`
	superSpec, err = supervisor.NewSpec(yamlSpec)
	assert.NoError(err)
	assert.NotPanics(func() { m.reload(superSpec, nil) })
	m.close()
}

func TestBuildFailureResponse(t *testing.T) {
	assert := assert.New(t)
	ctx := context.New(tracing.NoopSpan)
	resp := buildFailureResponse(ctx, http.StatusNotFound)
	assert.Equal(http.StatusNotFound, resp.StatusCode())
}

func TestAppendXForwardFor(t *testing.T) {
	const xForwardedFor = "X-Forwarded-For"

	assert := assert.New(t)
	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com/", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.1")

	req, _ := httpprot.NewRequest(stdr)

	appendXForwardedFor(req)

	assert.Equal("192.168.1.1", stdr.Header.Get(xForwardedFor))

	stdr.Header.Set("X-Real-Ip", "192.168.1.2")
	req, _ = httpprot.NewRequest(stdr)
	appendXForwardedFor(req)
	assert.True(strings.Contains(stdr.Header.Get(xForwardedFor), "192.168.1.2"))
}

func TestServerACME(t *testing.T) {
	assert := assert.New(t)

	mm := &contexttest.MockedMuxMapper{}
	m := newMux(httpstat.New(), httpstat.NewTopN(10), mm)
	assert.NotNil(m)
	assert.NotNil(m.inst.Load())

	yamlSpec := `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
`
	superSpec, err := supervisor.NewSpec(yamlSpec)
	assert.NoError(err)
	assert.NotPanics(func() { m.reload(superSpec, nil) })

	called := false
	mm.MockedGetHandler = func(name string) (context.Handler, bool) {
		called = true
		return nil, false
	}

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com/.well-known/acme-challenge/abc", http.NoBody)
	stdw := httptest.NewRecorder()
	m.ServeHTTP(stdw, stdr)
	assert.False(called)
	m.close()
}

func TestServeHTTP(t *testing.T) {
	assert := assert.New(t)

	mm := &contexttest.MockedMuxMapper{}
	m := newMux(httpstat.New(), httpstat.NewTopN(10), mm)
	assert.NotNil(m)
	assert.NotNil(m.inst.Load())

	yamlSpec := `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
cacheSize: 100
xForwardedFor: true
rules:
- host: www.megaease.com
  paths:
  - path: /abc
    backend: abc-pipeline
    rewriteTarget: /newabc
- host: www.megaease.cn
  paths:
  - pathPrefix: /xyz
    backend: xyz-pipeline
`
	superSpec, err := supervisor.NewSpec(yamlSpec)
	assert.NoError(err)
	assert.NotPanics(func() { m.reload(superSpec, mm) })

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com/", http.NoBody)
	stdw := httptest.NewRecorder()

	// route not found
	m.ServeHTTP(stdw, stdr)
	assert.Equal(http.StatusNotFound, stdw.Code)

	// do it again, for caching
	m.ServeHTTP(stdw, stdr)
	assert.Equal(http.StatusNotFound, stdw.Code)

	// backend not found
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdw = httptest.NewRecorder()
	m.ServeHTTP(stdw, stdr)
	assert.Equal(http.StatusServiceUnavailable, stdw.Code)

	// handler found
	mm.MockedGetHandler = func(name string) (context.Handler, bool) {
		return &contexttest.MockedHandler{}, true
	}
	m.ServeHTTP(stdw, stdr)
	assert.Equal(http.StatusServiceUnavailable, stdw.Code)

	// failed to read request body
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", iotest.ErrReader(fmt.Errorf("dummy")))
	stdr.ContentLength = -1
	stdw = httptest.NewRecorder()
	m.ServeHTTP(stdw, stdr)
	assert.Equal(http.StatusBadRequest, stdw.Code)
}

func TestMuxInstanceSearch(t *testing.T) {
	assert := assert.New(t)

	m := newMux(httpstat.New(), httpstat.NewTopN(10), nil)
	assert.NotNil(m)
	assert.NotNil(m.inst.Load())

	yamlSpec := `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
cacheSize: 100
xForwardedFor: true
ipFilter:
  blockIPs: [192.168.1.1]
rules:
- host: www.megaease.com
  ipFilter:
    blockIPs: [192.168.1.2]
  paths:
  - path: /abc
    backend: abc-pipeline
    ipFilter:
      blockIPs: [192.168.1.3]
  - path: /xyz
    methods: [PUT]
    backend: xyz-pipeline
  - path: /123
    methods: [GET]
    headers:
    - key: "X-Test"
      values: [test1, test2]
    backend: 123-pipeline
  - path: /headerAllMatch
    methods: [GET]
    headers:
    - key: "X-Test"
      values: [test1, test2]
    - key: "AllMatch"
      regexp: "^true$"
    matchAllHeader: true
    backend: 123-pipeline
`

	superSpec, err := supervisor.NewSpec(yamlSpec)
	assert.NoError(err)
	assert.NotPanics(func() { m.reload(superSpec, nil) })
	mi := m.inst.Load().(*muxInstance)

	// unknow host
	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.cn/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ := httpprot.NewRequest(stdr)
	assert.Equal(notFound, mi.search(req))

	// blocked IPs
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.1")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(forbidden, mi.search(req))

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.2")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(forbidden, mi.search(req))

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.3")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(forbidden, mi.search(req))

	// put to cache
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(req).code)

	// try again for cached result
	stdr.Header.Set("X-Real-Ip", "192.168.1.5")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(req).code)

	// cached result, but blocked by ip
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.1")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(forbidden, mi.search(req))

	// method not allowed
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/xyz", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(methodNotAllowed, mi.search(req))

	// has no required header
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/123", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(badRequest, mi.search(req))

	// success
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/123", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	stdr.Header.Set("X-Test", "test1")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(req).code)

	// header all matched
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/headerAllMatch", http.NoBody)
	stdr.Header.Set("X-Test", "test1")
	stdr.Header.Set("AllMatch", "true")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(req).code)

	// header all matched
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/headerAllMatch", http.NoBody)
	stdr.Header.Set("X-Test", "test1")
	stdr.Header.Set("AllMatch", "false")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(req).code)
}
