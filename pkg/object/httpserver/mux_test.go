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

package httpserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/option"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/context/contexttest"
	_ "github.com/megaease/easegress/v2/pkg/object/httpserver/routers/ordered"
	_ "github.com/megaease/easegress/v2/pkg/object/httpserver/routers/radixtree"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

func TestMuxReload(t *testing.T) {
	assert := assert.New(t)
	m := newMux(&httpstat.HTTPStat{}, &httpstat.TopN{}, newMockMetrics(), nil)
	assert.NotNil(m)
	assert.NotNil(m.inst.Load())

	yamlConfig := `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
`
	superSpec, err := supervisor.NewSpec(yamlConfig)
	assert.NoError(err)
	assert.NotPanics(func() { m.reload(superSpec, nil) })

	yamlConfig = `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
cacheSize: 100
tracing:
  serviceName: test
  sampleRate: 0.1
  spanLimits:
    attributeCountLimit: 20
  exporter:
    zipkin:
      endpoint: http://test.megaease.com/zipkin

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
	superSpec, err = supervisor.NewSpec(yamlConfig)
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
	// NOTE: For loading system controller AutoCertManager.
	etcdDirName, err := os.MkdirTemp("", "autocertmanager-test")
	if err != nil {
		t.Errorf(err.Error())
	}
	defer os.RemoveAll(etcdDirName)

	cls := cluster.CreateClusterForTest(etcdDirName)
	supervisor.MustNew(&option.Options{}, cls)

	assert := assert.New(t)

	mm := &contexttest.MockedMuxMapper{}
	m := newMux(httpstat.New(), httpstat.NewTopN(10), newMockMetrics(), mm)
	assert.NotNil(m)
	assert.NotNil(m.inst.Load())

	yamlConfig := `
kind: HTTPServer
name: test
port: 8080
keepAlive: true
https: false
`
	superSpec, err := supervisor.NewSpec(yamlConfig)
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
	m := newMux(httpstat.New(), httpstat.NewTopN(10), newMockMetrics(), mm)
	assert.NotNil(m)
	assert.NotNil(m.inst.Load())

	yamlConfig := `
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
	superSpec, err := supervisor.NewSpec(yamlConfig)
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

	m := newMux(httpstat.New(), httpstat.NewTopN(10), newMockMetrics(), nil)
	assert.NotNil(m)
	assert.NotNil(m.inst.Load())

	yamlConfig := `
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
  - path: /headerAllMatch2
    methods: [GET]
    matchAllQuery: true
    headers:
    - key: "X-Test"
      values: [test1, test2]
    - key: "AllMatch"
      values: ["true"]
    matchAllHeader: true
    backend: 123-pipeline
  - path: /queryParams
    methods: [GET]
    matchAllQuery: true
    queries:
    - key: "q"
      values: ["v1", "v2"]
    backend: 123-pipeline
  - path: /queryParamsMultiKey
    methods: [GET]
    matchAllQuery: true
    queries:
    - key: "q"
      values: ["v1", "v2"]
    - key: "q2"
      values: ["v3", "v4"]
    backend: 123-pipeline
  - path: /queryParamsRegexp
    methods: [GET]
    matchAllQuery: true
    queries:
    - key: "q2"
      regexp: "^v[0-9]$"
    backend: 123-pipeline
  - path: /queryParamsRegexpAndValues
    methods: [GET]
    matchAllQuery: true
    queries:
    - key: "q3"
      values: ["v1", "v2"]
      regexp: "^v[0-9]$"
    backend: 123-pipeline
  - path: /queryParamsRegexpAndValues2
    methods: [GET]
    queries:
    - key: "id"
      values: ["011"]
      regexp: "[0-9]+"
    backend: 123-pipeline
  - path: /clientIPsWithBlockIPs
    backend: abc-pipeline
    ipFilter:
      allowIPs: [192.168.1.2]
      blockIPs: [192.168.1.3]
  - path: /clientIPsWithBlockIPs
    backend: abc-pipeline-3
    ipFilter:
      allowIPs: [192.168.1.3]
      blockIPs: [192.168.1.4]
  - path: /clientIPsWithBlockIPs
    backend: abc-pipeline
    ipFilter:
      allowIPs: [192.168.1.5]
      blockIPs: [192.168.1.5]
  - path: /clientIPsWithBlockIPs
    backend: abc-pipeline
    ipFilter:
      allowIPs: [192.168.1.4]
      blockIPs: [192.168.1.6]
  - path: /clientIPsWithBlockIPs2
    backend: abc-pipeline
    ipFilter:
      blockIPs: [192.168.1.5]
  - path: /clientIPsWithAllowIPs2
    backend: abc-pipeline
    ipFilter:
      blockIPs: [192.168.1.5,192.168.1.9]
  - path: /clientIPsWithAllowIPs2
    backend: abc-pipeline
    ipFilter:
      allowIPs: [192.168.1.6]
  - path: /clientIPsWithAllowIPs2
    backend: abc-pipeline-default
  - path: /clientIPsWithAllowIPs3
    backend: abc-pipeline
    ipFilter:
      allowIPs: [192.168.1.7]
  - path: /clientIPsWithAllowIPs3
    backend: 123-pipeline
    ipFilter:
      allowIPs: [192.168.1.8]
- host: 1.megaease.com
  ipFilter:
    blockIPs: [192.168.1.2]
  paths:
  - path: /abc
    backend: host2-abc-pipeline
    ipFilter:
      blockIPs: [192.168.1.5]
- host: 1.megaease.com
  ipFilter:
    blockIPs: [192.168.1.3]
  paths:
  - path: /abc
    backend: host2-abc-pipeline
    ipFilter:
      blockIPs: [192.168.1.5]
`

	superSpec, err := supervisor.NewSpec(yamlConfig)
	assert.NoError(err)
	assert.NotPanics(func() { m.reload(superSpec, nil) })
	mi := m.inst.Load().(*muxInstance)

	// unknow host
	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.cn/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ := httpprot.NewRequest(stdr)
	routeCtx := routers.NewContext(req)
	assert.Equal(notFound, mi.search(routeCtx))

	// blocked IPs
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.1")
	req, _ = httpprot.NewRequest(stdr)
	routeCtx = routers.NewContext(req)
	assert.Equal(forbidden, mi.search(routeCtx))

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.2")
	req, _ = httpprot.NewRequest(stdr)
	routeCtx = routers.NewContext(req)
	assert.Equal(forbidden, mi.search(routeCtx))

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.3")
	req, _ = httpprot.NewRequest(stdr)
	routeCtx = routers.NewContext(req)
	assert.Equal(forbidden, mi.search(routeCtx))

	// put to cache
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ = httpprot.NewRequest(stdr)
	routeCtx = routers.NewContext(req)
	assert.Equal(0, mi.search(routeCtx).code)

	// try again for cached result
	stdr.Header.Set("X-Real-Ip", "192.168.1.5")
	req, _ = httpprot.NewRequest(stdr)
	routeCtx = routers.NewContext(req)
	assert.Equal(0, mi.search(routeCtx).code)

	// cached result, but blocked by ip
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.1")
	req, _ = httpprot.NewRequest(stdr)
	routeCtx = routers.NewContext(req)
	assert.Equal(forbidden, mi.search(routeCtx))

	// method not allowed
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/xyz", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(methodNotAllowed, mi.search(routers.NewContext(req)))

	// has no required header
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/123", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(badRequest, mi.search(routers.NewContext(req)))

	// success
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/123", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	stdr.Header.Set("X-Test", "test1")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// header all matched
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/headerAllMatch", http.NoBody)
	stdr.Header.Set("X-Test", "test1")
	stdr.Header.Set("AllMatch", "true")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// header all matched
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/headerAllMatch", http.NoBody)
	stdr.Header.Set("X-Test", "test1")
	stdr.Header.Set("AllMatch", "false")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// header all matched
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/headerAllMatch2", http.NoBody)
	stdr.Header.Set("X-Test", "test1")
	stdr.Header.Set("AllMatch", "false")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// query string single key
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParams", http.NoBody)
	v := url.Values{"q": []string{"v1"}}
	stdr.URL.RawQuery = v.Encode()
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// query string single key
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParams", http.NoBody)
	v = url.Values{"q": []string{"v1", "v2"}}
	stdr.URL.RawQuery = v.Encode()
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// query string single key
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParams", http.NoBody)
	stdr.URL.RawQuery = "q=v1"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// query string multi key
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsMultiKey", http.NoBody)
	v = url.Values{"q": []string{"v1", "v3"}, "q2": []string{"v6"}}
	stdr.URL.RawQuery = v.Encode()
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// query string multi key
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsMultiKey", http.NoBody)
	v = url.Values{"q": []string{"v1", "v3"}}
	stdr.URL.RawQuery = v.Encode()
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// query string multi key
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsMultiKey", http.NoBody)
	v = url.Values{"q": []string{"v1", "v3"}, "q2": []string{"v3"}}
	stdr.URL.RawQuery = v.Encode()
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// query string regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexp", http.NoBody)
	stdr.URL.RawQuery = "q2=v1"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// query string regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexp", http.NoBody)
	stdr.URL.RawQuery = "q2=vv"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// query string values and regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexpAndValues", http.NoBody)
	stdr.URL.RawQuery = "q3=v2"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// query string values and regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexpAndValues", http.NoBody)
	stdr.URL.RawQuery = "q3=v1&q3=v4"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// query string values and regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexpAndValues", http.NoBody)
	stdr.URL.RawQuery = "q3=v4"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// query string values and regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexpAndValues", http.NoBody)
	stdr.URL.RawQuery = "q3=v4"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// query string values and regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexpAndValues2", http.NoBody)
	stdr.URL.RawQuery = "id=011&&id=baz"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// query string values and regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexpAndValues2", http.NoBody)
	stdr.URL.RawQuery = "id=baz&&id=011"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// query string values and regexp
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/queryParamsRegexpAndValues2", http.NoBody)
	stdr.URL.RawQuery = "id=baz"
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(400, mi.search(routers.NewContext(req)).code)

	// client ip with blockIPs
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithBlockIPs", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.4")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithBlockIPs", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.3")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)
	assert.Equal("abc-pipeline-3", mi.search(routers.NewContext(req)).route.GetBackend())

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithBlockIPs", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.2")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(403, mi.search(routers.NewContext(req)).code)

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithBlockIPs", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.5")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithBlockIPs2", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.3")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	// client ip with allowIPs
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithAllowIPs2", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.5")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)
	assert.Equal("abc-pipeline-default", mi.search(routers.NewContext(req)).route.GetBackend())

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithAllowIPs2", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.6")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithAllowIPs2", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.9")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal("abc-pipeline-default", mi.search(routers.NewContext(req)).route.GetBackend())

	// client ip
	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithAllowIPs3", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.6")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(403, mi.search(routers.NewContext(req)).code)

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithAllowIPs3", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.7")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)
	assert.Equal("abc-pipeline", mi.search(routers.NewContext(req)).route.GetBackend())

	stdr, _ = http.NewRequest(http.MethodGet, "http://www.megaease.com/clientIPsWithAllowIPs3", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.8")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)
	assert.Equal("123-pipeline", mi.search(routers.NewContext(req)).route.GetBackend())

	stdr, _ = http.NewRequest(http.MethodGet, "http://1.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.2")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)
	assert.Equal("host2-abc-pipeline", mi.search(routers.NewContext(req)).route.GetBackend())

	stdr, _ = http.NewRequest(http.MethodGet, "http://1.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.3")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(0, mi.search(routers.NewContext(req)).code)
	assert.Equal("host2-abc-pipeline", mi.search(routers.NewContext(req)).route.GetBackend())

	stdr, _ = http.NewRequest(http.MethodGet, "http://1.megaease.com/abc", http.NoBody)
	stdr.Header.Set("X-Real-Ip", "192.168.1.5")
	req, _ = httpprot.NewRequest(stdr)
	assert.Equal(403, mi.search(routers.NewContext(req)).code)
}

func TestAccessLog(t *testing.T) {
	log := &accessLog{
		Method:  "GET",
		URI:     "127.0.0.1",
		ReqSize: 100,
	}
	formatter := newAccessLogFormatter("{{Method}} {{URI}} [{{ReqSize}}]")
	s := formatter.format(log)
	assert.Equal(t, "GET 127.0.0.1 [100]", s)
}

func TestPrintHeader(t *testing.T) {
	h := http.Header{}
	h.Set("a", "1")
	h.Set("b", "2")
	s := printHeader(h)

	if s != "A: [1], B: [2]" && s != "B: [2], A: [1]" {
		t.Fail()
	}
}
