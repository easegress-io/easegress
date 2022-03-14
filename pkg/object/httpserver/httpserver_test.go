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
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

type testCase struct {
	path           string
	method         string
	headers        map[string]string
	realIP         string
	rules          []*muxRule
	expectedResult SearchResult
}

func (tc *testCase) toCtx() *contexttest.MockedHTTPContext {
	ctx := &contexttest.MockedHTTPContext{}
	header := http.Header{}
	for k, v := range tc.headers {
		header.Add(k, v)
	}
	ctx.MockedRequest.MockedPath = func() string {
		return tc.path
	}
	ctx.MockedRequest.MockedMethod = func() string {
		return tc.method
	}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(header)
	}
	ctx.MockedRequest.MockedRealIP = func() string { return tc.realIP }
	return ctx
}

func TestSearchPath(t *testing.T) {
	assert := assert.New(t)
	emptyHeaders := make(map[string]string)
	jsonHeader := make(map[string]string)
	jsonHeader["content-type"] = "application/json"
	tests := []testCase{
		{
			"/path/1", http.MethodGet, emptyHeaders, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{Path: "/path/2"}),
				}),
			}, NotFound,
		},
		{
			"/path/1", http.MethodGet, emptyHeaders, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{Path: "/path/1"}),
				}),
			}, Found,
		},
		{
			"/path/1", http.MethodGet, emptyHeaders, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/path/1", Methods: []string{http.MethodPost},
					}),
				}),
			}, MethodNotAllowed,
		},
		{
			"/otherpath", http.MethodPost, emptyHeaders, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/otherpath", Methods: []string{http.MethodPost},
					}),
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/blaa", Methods: []string{http.MethodPost},
					}),
				}),
			}, Found,
		},
		{
			"/otherpath", http.MethodPost, emptyHeaders, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{},
					&Rule{IPFilter: &ipfilter.Spec{BlockByDefault: true}}, []*MuxPath{
						newMuxPath(
							&ipfilter.IPFilters{},
							&Path{Path: "/otherpath", Methods: []string{http.MethodPost}},
						),
					},
				),
			}, IPNotAllowed,
		},
		{
			"/route", http.MethodGet, jsonHeader, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path:    "/route",
						Headers: []*Header{&Header{Key: "content-type", Values: []string{"application/json"}}},
					}),
				}),
			}, FoundSkipCache,
		},
		{
			"/route", http.MethodGet, jsonHeader, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path:    "/route",
						Headers: []*Header{&Header{Key: "content-type", Values: []string{"application/csv"}}},
					}),
				}),
			}, MethodNotAllowed,
		},
		{
			"/multimethod", http.MethodPut, emptyHeaders, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/multimethod", Methods: []string{http.MethodGet},
					}),
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/multimethod", Methods: []string{http.MethodPost},
					}),
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/multimethod", Methods: []string{http.MethodPut},
					}),
				}),
			}, Found,
		},
		{
			"/multiheader", http.MethodPut, jsonHeader, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/multiheader", Methods: []string{http.MethodPut},
						Headers: []*Header{&Header{Key: "content-type", Values: []string{"application/csv"}}},
					}),
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/multiheader", Methods: []string{http.MethodPut},
						Headers: []*Header{&Header{Key: "content-type", Values: []string{"application/json"}}},
					}),
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/multiheader", Methods: []string{http.MethodPut},
						Headers: []*Header{&Header{Key: "content-type", Values: []string{"application/txt"}}},
					}),
				}),
			}, FoundSkipCache,
		},
	}

	for i := 0; i < len(tests); i++ {
		t.Run("test case "+fmt.Sprint(i), func(t *testing.T) {
			testcase := tests[i]
			result, _ := SearchPath(testcase.toCtx(), testcase.rules)
			assert.NotNil(result)

			assert.Equal(result, testcase.expectedResult)
		})
	}
}

func TestSearchPathHeadersAndIPs(t *testing.T) {
	assert := assert.New(t)
	ipfilter1 := &ipfilter.Spec{AllowIPs: []string{"8.8.8.8"}, BlockByDefault: true}
	ipfilter2 := &ipfilter.Spec{AllowIPs: []string{"9.9.9.9"}, BlockByDefault: true}
	path1 := newMuxPath(&ipfilter.IPFilters{}, &Path{
		IPFilter: ipfilter1,
		Path:     "/pipeline", Methods: []string{http.MethodPost},
		Headers: []*Header{&Header{Key: "X-version", Values: []string{"v1"}}},
	})
	path2 := newMuxPath(&ipfilter.IPFilters{}, &Path{
		IPFilter: ipfilter2,
		Path:     "/pipeline", Methods: []string{http.MethodPost},
		Headers: []*Header{&Header{Key: "X-version", Values: []string{"v2"}}},
	})
	muxRules := []*muxRule{newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{path1, path2})}

	hdr := make(map[string]string)
	hdr["X-version"] = "v1"
	testCaseA := testCase{
		path:    "/pipeline",
		method:  http.MethodPost,
		headers: hdr,
		realIP:  "8.8.8.8",
	}

	result, pathRes := SearchPath(testCaseA.toCtx(), muxRules)
	assert.NotNil(result)
	assert.NotNil(pathRes)
	assert.Equal(result, FoundSkipCache)
	assert.Equal(pathRes, path1)

	hdr["X-version"] = "v2"
	testCaseB := testCase{
		path:    "/pipeline",
		method:  http.MethodPost,
		headers: hdr,
		realIP:  "9.9.9.9",
	}

	result, pathRes = SearchPath(testCaseB.toCtx(), muxRules)
	assert.NotNil(result)
	assert.NotNil(pathRes)
	assert.Equal(result, FoundSkipCache)
	assert.Equal(pathRes, path2)

	hdr["X-version"] = "v3"
	testCaseC := testCase{
		path:    "/pipeline",
		method:  http.MethodPost,
		headers: hdr,
		realIP:  "9.9.9.9",
	}

	result, _ = SearchPath(testCaseC.toCtx(), muxRules)
	assert.NotNil(result)
	assert.Equal(result, MethodNotAllowed)

	hdr["X-version"] = "v1"
	testCaseD := testCase{
		path:    "/pipeline",
		method:  http.MethodPost,
		headers: hdr,
		realIP:  "9.1.1.9",
	}

	result, _ = SearchPath(testCaseD.toCtx(), muxRules)
	assert.NotNil(result)
	assert.Equal(result, IPNotAllowed)

	testCaseE := testCase{
		path:   "/pipeline",
		method: http.MethodPost,
		realIP: "9.9.9.9",
	}

	result, _ = SearchPath(testCaseE.toCtx(), muxRules)
	assert.NotNil(result)
	assert.Equal(result, MethodNotAllowed) // Missing required header

	// This might be unintuitive but as IP is used only to validate request and
	// not route/choose between path, this request produces IPNotAllowed.
	// 1st path is mathed and it has IPFilter "8.8.8.8" which does not match with the request.
	hdr["X-version"] = "v1"
	testCaseF := testCase{
		path:    "/pipeline",
		method:  http.MethodPost,
		headers: hdr,
		realIP:  "9.9.9.9",
	}

	result, _ = SearchPath(testCaseF.toCtx(), muxRules)
	assert.NotNil(result)
	assert.Equal(result, IPNotAllowed)
}

type handlerMock struct{}

func (hm *handlerMock) Handle(ctx context.HTTPContext) string {
	return "test"
}

type muxMapperMock struct {
	hm *handlerMock
}

func (mmm *muxMapperMock) GetHandler(name string) (protocol.HTTPHandler, bool) {
	handler := mmm.hm
	return handler, true
}

func TestServeHTTP(t *testing.T) {
	assert := assert.New(t)
	superSpecYaml := `
name: http-server-test
kind: HTTPServer
port: 10080
cacheSize: 200
rules:
  - paths:
    - pathPrefix: /api
`

	superSpec, err := supervisor.NewSpec(superSpecYaml)
	assert.Nil(err)
	assert.NotNil(superSpec.ObjectSpec())
	mux := &muxMapperMock{&handlerMock{}}
	httpServer := HTTPServer{}
	httpServer.Init(superSpec, mux)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cacheKey := stringtool.Cat(r.Host, r.Method, r.URL.Path)
		cacheItem := httpServer.runtime.mux.rules.Load().(*muxRules).cache.get(cacheKey)
		assert.Nil(cacheItem)

		httpServer.runtime.mux.ServeHTTP(w, r)

		cacheItem = httpServer.runtime.mux.rules.Load().(*muxRules).cache.get(cacheKey)
		assert.NotNil(cacheItem)
		assert.Equal(true, cacheItem.notFound)
	}))
	res, err := http.Get(ts.URL + "/unknown-path")
	assert.Nil(err)
	assert.Equal("404 Not Found", res.Status)
	ts.Close()

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cacheKey := stringtool.Cat(r.Host, r.Method, r.URL.Path)

		httpServer.runtime.mux.ServeHTTP(w, r)

		cacheItem := httpServer.runtime.mux.rules.Load().(*muxRules).cache.get(cacheKey)
		assert.NotNil(cacheItem)
		assert.Equal(false, cacheItem.notFound)
		assert.Equal("/api", cacheItem.path.pathPrefix)
	}))
	res, err = http.Get(ts.URL + "/api")
	assert.Nil(err)
	assert.Equal("200 OK", res.Status)
	ts.Close()

	httpServer.Close()
}
