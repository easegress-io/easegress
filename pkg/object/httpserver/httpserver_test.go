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

	moreHeader := make(map[string]string)
	moreHeader["content-type"] = "application/json"
	moreHeader["accept-encoding"] = "gzip"

	allMatchHeader := make(map[string]string)
	allMatchHeader["content-type"] = "application/json"
	allMatchHeader["accept-language"] = "zh-CN"
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
						Headers: []*Header{{Key: "content-type", Values: []string{"application/json"}}},
					}),
				}),
			}, FoundSkipCache,
		},
		{
			"/route", http.MethodGet, jsonHeader, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path:    "/route",
						Headers: []*Header{{Key: "content-type", Values: []string{"application/csv"}}},
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
						Headers: []*Header{{Key: "content-type", Values: []string{"application/csv"}}},
					}),
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/multiheader", Methods: []string{http.MethodPut},
						Headers: []*Header{{Key: "content-type", Values: []string{"application/json"}}},
					}),
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/multiheader", Methods: []string{http.MethodPut},
						Headers: []*Header{{Key: "content-type", Values: []string{"application/txt"}}},
					}),
				}),
			}, FoundSkipCache,
		},
		{
			"/matchallheader", http.MethodGet, moreHeader, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/matchallheader", Methods: []string{http.MethodGet},
						Headers: []*Header{{Key: "content-type", Values: []string{"application/json"}}},
					}),
				}),
			}, FoundSkipCache,
		},
		//todo: When the pipeline refactoring is complete, the expected results `MethodNotAllowed` need to be adapted
		{
			"/matchallheader", http.MethodGet, jsonHeader, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/matchallheader", Methods: []string{http.MethodGet},
						Headers: []*Header{
							{Key: "content-type", Values: []string{"application/json"}},
							{Key: "accept-language", Values: []string{"zh-CN"}},
						},

						MatchAllHeader: true,
					}),
				}),
			}, MethodNotAllowed,
		},
		{
			"/matchallheader", http.MethodGet, moreHeader, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/matchallheader", Methods: []string{http.MethodGet},
						Headers: []*Header{
							{Key: "content-type", Values: []string{"application/json"}},
							{Key: "accept-language", Values: []string{"zh-CN"}},
						},
						MatchAllHeader: true,
					}),
				}),
			}, MethodNotAllowed,
		},
		{
			"/matchallheader", http.MethodGet, allMatchHeader, "", []*muxRule{
				newMuxRule(&ipfilter.IPFilters{}, &Rule{}, []*MuxPath{
					newMuxPath(&ipfilter.IPFilters{}, &Path{
						Path: "/matchallheader", Methods: []string{http.MethodGet},
						Headers: []*Header{
							{Key: "content-type", Values: []string{"application/json"}},
							{Key: "accept-language", Values: []string{"zh-CN"}},
						},
						MatchAllHeader: true,
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
		Headers: []*Header{{Key: "X-version", Values: []string{"v1"}}},
	})
	path2 := newMuxPath(&ipfilter.IPFilters{}, &Path{
		IPFilter: ipfilter2,
		Path:     "/pipeline", Methods: []string{http.MethodPost},
		Headers: []*Header{{Key: "X-version", Values: []string{"v2"}}},
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
  - hostRegexp: 127.0.[0|1].1
    paths:
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

	status := httpServer.runtime.Status()
	assert.Equal(status.Error, "")

	superSpecYaml = `
name: http-server-test2
kind: HTTPServer
port: 10081
cacheSize: 201
rules:
  - paths:
    - pathPrefix: /api
`

	superSpec, err = supervisor.NewSpec(superSpecYaml)
	assert.Nil(err)
	newServer := HTTPServer{}
	newServer.Inherit(superSpec, &httpServer, mux)
	httpServer.Close()
}

func TestStartTwoServerInSamePort(t *testing.T) {
	assert := assert.New(t)
	superSpecYaml := `
name: http-server-test
kind: HTTPServer
port: 10082
cacheSize: 200
rules:
  - paths:
    - pathPrefix: /api
`
	superSpec1, err := supervisor.NewSpec(superSpecYaml)
	assert.Nil(err)
	assert.NotNil(superSpec1.ObjectSpec())
	superSpec2, err := supervisor.NewSpec(superSpecYaml)
	assert.Nil(err)
	assert.NotNil(superSpec2.ObjectSpec())
	mux1 := &muxMapperMock{&handlerMock{}}
	mux2 := &muxMapperMock{&handlerMock{}}
	httpServer1 := HTTPServer{}
	httpServer1.Init(superSpec1, mux1)
	httpServer2 := HTTPServer{}
	httpServer2.Init(superSpec2, mux2)

	httpServer1.Close()
	httpServer2.Close()
	_, err = http.Get("http://127.0.0.1:10082/api")
	assert.NotNil(err)
}

func TestMatchPath(t *testing.T) {
	assert := assert.New(t)

	originPath := "/tianji/exchange/activity/claim"

	type MuxPathTestCase struct {
		path           string
		pathPrefix     string
		pathRegexp     string
		expectedResult bool
	}

	toCtx := func(tc *MuxPathTestCase) *contexttest.MockedHTTPContext {
		ctx := &contexttest.MockedHTTPContext{}
		header := http.Header{}
		ctx.MockedRequest.MockedPath = func() string {
			return originPath
		}
		ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
			return httpheader.New(header)
		}
		return ctx
	}

	tests := []MuxPathTestCase{
		// matched, but path and prefix and regexp are empty
		{
			expectedResult: true,
		},

		// path
		{
			path:           originPath + "/a",
			pathPrefix:     "",
			pathRegexp:     "",
			expectedResult: false,
		},

		{
			path:           originPath,
			pathPrefix:     "",
			pathRegexp:     "",
			expectedResult: true,
		},

		// prefix
		{
			path:           "",
			pathPrefix:     "/1tianji/exchange/activity",
			pathRegexp:     "",
			expectedResult: false,
		},
		{
			path:           "",
			pathPrefix:     "/tianji/exchange/activity",
			pathRegexp:     "",
			expectedResult: true,
		},

		// regexp
		{
			path:           "",
			pathPrefix:     "",
			pathRegexp:     "^/1tianji/exchange/activity/(.*)$",
			expectedResult: false,
		},

		{
			path:           "",
			pathPrefix:     "",
			pathRegexp:     "^/tianji/exchange/activity/(.*)$",
			expectedResult: true,
		},
	}

	for i := 0; i < len(tests); i++ {
		t.Run("test case "+fmt.Sprint(i), func(t *testing.T) {
			testcase := tests[i]

			mp := newMuxPath(&ipfilter.IPFilters{}, &Path{
				Path:       testcase.path,
				PathPrefix: testcase.pathPrefix,
				PathRegexp: testcase.pathRegexp,
				Headers:    []*Header{{Key: "content-type", Values: []string{"application/csv"}}},
			})

			ctx := toCtx(&testcase)
			r := mp.matchPath(ctx)

			assert.Equal(testcase.expectedResult, r)
		})
	}
}

func TestHandleRewrite(t *testing.T) {
	assert := assert.New(t)

	exceptRewritePath := "/api/activity/ex/claim"

	originPath := "/tianji/exchange/activity/claim"

	type MuxPathTestCase struct {
		path           string
		pathPrefix     string
		pathRegexp     string
		rewriteTarget  string
		expectedResult string
	}

	toCtx := func(tc *MuxPathTestCase) *contexttest.MockedHTTPContext {
		ctx := &contexttest.MockedHTTPContext{}
		header := http.Header{}
		setFlag := false
		var setPath string
		ctx.MockedRequest.MockedPath = func() string {
			if setFlag {
				return setPath
			}
			return originPath
		}
		ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
			return httpheader.New(header)
		}
		ctx.MockedRequest.MockedSetPath = func(path string) {
			setFlag = true
			setPath = path
		}
		return ctx
	}

	tests := []MuxPathTestCase{
		// rewriteTarget is empty
		{
			rewriteTarget:  "",
			expectedResult: originPath,
		},

		// path is not empty
		{
			path:           originPath,
			pathPrefix:     "",
			pathRegexp:     "",
			rewriteTarget:  "/api/activity/ex/claim",
			expectedResult: exceptRewritePath,
		},
		// prefix is not empty
		{
			path:           "",
			pathPrefix:     "/tianji/exchange/activity",
			pathRegexp:     "",
			rewriteTarget:  "/api/activity/ex",
			expectedResult: exceptRewritePath,
		},

		// regexp is not empty
		{
			path:           "",
			pathPrefix:     "",
			pathRegexp:     "^/tianji/exchange/activity/(.*)$",
			rewriteTarget:  "/api/activity/ex/$1",
			expectedResult: exceptRewritePath,
		},
	}

	for i := 0; i < len(tests); i++ {
		t.Run("test case "+fmt.Sprint(i), func(t *testing.T) {
			testcase := tests[i]

			mp := newMuxPath(&ipfilter.IPFilters{}, &Path{
				Path:          testcase.path,
				PathPrefix:    testcase.pathPrefix,
				PathRegexp:    testcase.pathRegexp,
				RewriteTarget: testcase.rewriteTarget,
				Headers:       []*Header{{Key: "content-type", Values: []string{"application/csv"}}},
			})

			ctx := toCtx(&testcase)
			mp.handleRewrite(ctx)

			assert.Equal(testcase.expectedResult, ctx.Request().Path())
		})
	}
}
