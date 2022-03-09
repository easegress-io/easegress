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

package pathsearch

import (
	//"fmt"

	"net/http"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitNop()
}

type (
	testRule struct {
		ipOk  bool
		paths []MuxPathInterface
	}
	testPath struct {
		path       string
		method     string
		hasheaders bool
	}
)

func (tr *testRule) Pass(ctx context.HTTPContext) bool  { return tr.ipOk }
func (tr *testRule) Match(ctx context.HTTPContext) bool { return true }
func (tr *testRule) Paths() []MuxPathInterface          { return tr.paths }

func (tp *testPath) Pass(ctx context.HTTPContext) bool { return true }
func (tp *testPath) MatchPath(ctx context.HTTPContext) bool {
	return ctx.Request().Path() == tp.path
}
func (tp *testPath) MatchMethod(ctx context.HTTPContext) bool {
	return ctx.Request().Method() == tp.method
}
func (tp *testPath) MatchHeaders(ctx context.HTTPContext) bool { return true }
func (tp *testPath) HasHeaders() bool                          { return tp.hasheaders }
func (tp *testPath) GetIPFilterChain() *ipfilter.IPFilters     { return &ipfilter.IPFilters{} }

type testCase struct {
	path           string
	method         string
	rules          []MuxRuleInterface
	expectedResult SearchResult
}

func TestSearchPath(t *testing.T) {
	assert := assert.New(t)
	tests := []testCase{
		{
			"/path/1", http.MethodGet, []MuxRuleInterface{
				&testRule{
					ipOk: true,
					paths: []MuxPathInterface{
						&testPath{path: "/path/2", hasheaders: false},
					},
				},
			}, NotFound,
		},
		{
			"/path/1", http.MethodGet, []MuxRuleInterface{
				&testRule{
					ipOk: true,
					paths: []MuxPathInterface{
						&testPath{path: "/path/1", hasheaders: false},
					},
				},
			}, MethodNotAllowed,
		},
		{
			"/path/1", http.MethodGet, []MuxRuleInterface{
				&testRule{
					ipOk: true,
					paths: []MuxPathInterface{
						&testPath{path: "/path/1", method: http.MethodGet, hasheaders: false},
					},
				},
				&testRule{
					ipOk: true,
					paths: []MuxPathInterface{
						&testPath{path: "/blaa", method: http.MethodGet, hasheaders: false},
					},
				},
			}, Found,
		},
		{
			"/path/1", http.MethodGet, []MuxRuleInterface{
				&testRule{
					ipOk: false,
					paths: []MuxPathInterface{
						&testPath{path: "/path/1", method: http.MethodGet},
					},
				},
			}, IPNotAllowed,
		},
		{
			"/path/1", http.MethodPost, []MuxRuleInterface{
				&testRule{
					ipOk: true,
					paths: []MuxPathInterface{
						&testPath{path: "/path/1", method: http.MethodPost, hasheaders: true},
					},
				},
			}, FoundSkipCache,
		},
		{
			"/path/1", http.MethodPut, []MuxRuleInterface{
				&testRule{
					ipOk: true,
					paths: []MuxPathInterface{
						&testPath{path: "/path/1", method: http.MethodGet, hasheaders: false},
					},
				},
				&testRule{
					ipOk: true,
					paths: []MuxPathInterface{
						&testPath{path: "/path/1", method: http.MethodPost, hasheaders: false},
					},
				},
				&testRule{
					ipOk: true,
					paths: []MuxPathInterface{
						&testPath{path: "/path/1", method: http.MethodPut, hasheaders: false},
					},
				},
			}, Found,
		},
	}

	for i := 0; i < len(tests); i++ {
		testcase := tests[i]
		ctx := &contexttest.MockedHTTPContext{}
		header := http.Header{}
		//header.Add(testcase.headerKey, testcase.headerValue)
		ctx.MockedRequest.MockedPath = func() string {
			return testcase.path
		}
		ctx.MockedRequest.MockedMethod = func() string {
			return testcase.method
		}
		ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
			return httpheader.New(header)
		}
		result, _ := SearchPath(ctx, testcase.rules)
		assert.NotNil(result)

		assert.Equal(result, testcase.expectedResult)
	}
}
