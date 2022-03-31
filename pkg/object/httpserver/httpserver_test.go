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

	status := httpServer.runtime.Status()
	assert.Equal(status.Error, "")

	newServer := HTTPServer{}
	newServer.Inherit(superSpec, &httpServer, mux)
	httpServer.Close()

	// test HTTP3
	superSpecYaml = `
name: http-server-test
kind: HTTPServer
port: 10080
http3: true
https: true
keyBase64: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRQ1M3YVJXYXNOTi9GVWgKRXBZcHFVRmhHL2k1SGY2dFl4REZ0djhvcUxBRGNjaThabWlZSFczWHJ3aVFENXdia2prZFpMMkRrZFdUbVFnUwpkVUYxS2NzREhIc2UyNDFlbGw0U3BVSXIrVndMZXplUnMwSm1PQ3FYWWxzTHJCUElyRzh6MXZzY0lnZ2tNVVBaCnl1aUJsc0FUWDJLenk1Rm9oZjJRcUthQmllZFA2cmtZU1YrV2Jqc0drcTlVODEyT0UrTFhONUtKaGhEQnF6TS8KNUlnSlgycnY0TXRjU0JwelRNS0h1YmcvdG5hVlM3Ykc1Wmpyc0ZDcWc5bTNJbUc0Ti9BbFVtMTRrU1M0UC9CRgpXMGFBcC9BYjNxT0hVbzlzaXRyK040aTJOckhjQWkyc3BFR29nTGgzS1JTN2ZvVldKSFkrY2hTTGRyMGZmRGxJCng3dXhvQWVWQWdNQkFBRUNnZ0VBQW9FVVpQaXEzWUJvZndqUEVHUzNIWTJaZnFZNU9nRlBQdDl3bCtQUUpDN2oKU2ZyQTI1N2N5V2xOVHc5RkROOUFJL1VjbWNwNWhtdDhUTHc4NGw5VSszZVh6WjNXV2Y5Y0dSdEI5bmZvanJXSgo2K3pQTytqSEtROWZGK0xWNzN5bzVJeE1lVjFISUQ3S3RrS1VGZWxZMnJ1c2RmNEpPMnZWTjRyNFU0cmpLMlNCCktLTkp1U0hxZ01LV21rVmZsS1RhSGlyY2ZBV09sc1ZxWjlzUnpKdTJpcUc1NmtzVFBnRG9LK05obnNyaEc5cEwKT0pwQzRCeDVaQ2J5eUdTYXpjWXNDOTRUQnV6a1E5cGJvcjJTMkl2dC9pc00zaFdhUFhKVk1uNVVpUEhQYWlpNgpvVFpDcFhWTlJidkJyOFVuZFo1ODZQRmJJUWVsN2wzODRGZ2I4Zk95b1FLQmdRRERFY1JsZUIvc0FBWTZjbzAxCkRLVEdXaG9JMFZKRUYwZDhBWlRkR0s1ZEc0R1RnTGhOWFpEZDA2blFITDhxenlwMnppWlNjQTVVbXRsTUFKaWcKenhXNVFDaHZyNGdzT0ZNKzVVWk5sNjU1SU9mL0VQVG1wbUI4ZjhGaDNFT1paRkpaa3h1eVY0eXVoZWpkZnZEYQozeVVzYnFEOU5WZURReWpRZFNzT3U0WHF2UUtCZ1FEQTBtUlpsNkNyZGlRWUdMaXk3ZmY5VUtpQkNZMjZ0c08wCk02am1YMy81V21KTkt0eXZSZFFsbkJtbExMVGEyTlN2WVNVbk9uYklXZGd0SmRWVzhMUEp5ejBKcy9mM3BEam0KUzJoQzZNdjJZb3VhemtQcWlmelh4ZGl4NTJSMXdNT1dPN1VONUJvbE5SVkRDSkNYdVQ1c0YwTDYxRG1YWFdBdwpKRW5mbU9mSnVRS0JnQ3ZxT2c2bDVublkzNDRVNzlrN2lYVG1IK3BRUlhieXpyTUtJQnRPVFNMRTZIenVnNDlYCk94L1ZZT3RyTFZaVDRUbHgyNHEvazFwVXFnckVMNWcwUnEyMzFlS2UzOGNrdndqdjBNM3pFZUpQR0N1Q0E4QlIKUUhPR3gyQmltQTFXV251ejlJNUh5M0lXejMvZDdoYzRHVVJSZTRqRmszZ0hqSTZ4Y2dvVkNXYjVBb0dBRFNrTwo4bEo0QTl2ZllNbW5LWWMyYXRLcmZZc2lZa0VCSUhaNks2Y08rL3pnUXJZUE0rTkhOSDN2L2ljTC9QZlpwRkswCkQzWmREeFdhdkpJZGVuNlpOc2VwVmRVenNuSkI4KzNub3RGeXdsRTlpQVpWK2xjS3E4dDBHOGhZUWZVekpEalYKQmFxdzRpTTZYVVhqWUllakxBdDJaZHBBU0FWMmdES3AzQm42ai9rQ2dZRUFubEtSLzNRUFROTEtkUTE3cWhHVApYYjBUUGtRdWlQQkU0aWJmTDN1NTZDaVNzd3h1RlM4S2szTkYxMTVrUVh4VWR2T3ZrU0lBOFlwU1dzWGdxZzh0CnRTQjBDcGlhTzNLOWxOWEh3ZmNzcHlYVENWaFg5NElvOWhEenZ1bGtqRmIzaG41ams5SFovM3FIWCs3Z1ErbXYKeU5iS3pZcnVlRUhzWWpMVzJmZlpSeXc9Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0K
certBase64: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURJVENDQWdtZ0F3SUJBZ0lRU1FIYU5pMUlzd2hKcnZwcDBrRytRREFOQmdrcWhraUc5dzBCQVFzRkFEQVMKTVJBd0RnWURWUVFLRXdkQlkyMWxJRU52TUI0WERURTJNREV3TVRFMU1EUXdOVm9YRFRJMU1USXlPVEUxTURRdwpOVm93RWpFUU1BNEdBMVVFQ2hNSFFXTnRaU0JEYnpDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDCkFRb0NnZ0VCQUpMdHBGWnF3MDM4VlNFU2xpbXBRV0ViK0xrZC9xMWpFTVcyL3lpb3NBTnh5THhtYUpnZGJkZXYKQ0pBUG5CdVNPUjFrdllPUjFaT1pDQkoxUVhVcHl3TWNleDdialY2V1hoS2xRaXY1WEF0N041R3pRbVk0S3BkaQpXd3VzRThpc2J6UFcreHdpQ0NReFE5bks2SUdXd0JOZllyUExrV2lGL1pDb3BvR0o1MC9xdVJoSlg1WnVPd2FTCnIxVHpYWTRUNHRjM2tvbUdFTUdyTXova2lBbGZhdS9neTF4SUduTk13b2U1dUQrMmRwVkx0c2JsbU91d1VLcUQKMmJjaVliZzM4Q1ZTYlhpUkpMZy84RVZiUm9DbjhCdmVvNGRTajJ5SzJ2NDNpTFkyc2R3Q0xheWtRYWlBdUhjcApGTHQraFZZa2RqNXlGSXQydlI5OE9Vakh1N0dnQjVVQ0F3RUFBYU56TUhFd0RnWURWUjBQQVFIL0JBUURBZ0trCk1CTUdBMVVkSlFRTU1Bb0dDQ3NHQVFVRkJ3TUJNQThHQTFVZEV3RUIvd1FGTUFNQkFmOHdIUVlEVlIwT0JCWUUKRkdNb0xXMW5tWkU0LzhodVhyTkJjRzdHK0txcU1Cb0dBMVVkRVFRVE1CR0NDV3h2WTJGc2FHOXpkSWNFZndBQQpBVEFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBaGlOK1VUQjlpMUd5QzZnUTRCeDZ4VlJIUzJnYjFoVUlKU3pJClhXT1h5cEMxVjlNUnpHeWJsQTdQYzhJUHllWlRGQkkyUS9xWUNMaWh2S3hRbzZuUk5zQU5zdVFqRGtNakpVUkYKQldhQzZwQzNWRVZ5YURtNnYzelVYcWczZllSbDJvalc0dkZQbDhrdkkxbGxWaGxZZEs0VjVVVTI5R1h0WklJZgpPQTlJa0JVZDJwMVBmQ0J0QWRrc21qTVBWczZUalBzVHdHd2dKa0FqVUlwV1ZjRTUzT1JSQ1JsRDZLK2xDc1RLClBqVGRteXpMeEUyQTltT2xhMEVac3JaNGh5ZmVlMW9rU1dQMFFRUmNVQU1MNDlPR2JMOXY5RXZPaFVac29lczYKWUgzcUVjYWVhaFlJSG00ZnZ3aUJkRUpyT0RaUmt2V2l2ZDlVUU9Lc25BSzI0TENWcEE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
cacheSize: 200
rules:
  - paths:
    - pathPrefix: /api
`
	superSpec, err = supervisor.NewSpec(superSpecYaml)
	assert.Nil(err)
	httpServer = HTTPServer{}
	mux = &muxMapperMock{&handlerMock{}}
	httpServer.Init(superSpec, mux)
	//httpServer.Close()
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
