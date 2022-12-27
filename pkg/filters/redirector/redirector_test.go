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

package redirector

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func getSpec(match string, part string, replace string, code int) *Spec {
	return &Spec{
		Match:       match,
		MatchPart:   part,
		Replacement: replace,
		StatusCode:  code,
	}
}

func TestRedirector(t *testing.T) {
	assert := assert.New(t)

	type match struct {
		reqURL       string
		expectedURL  string
		expectedCode int
		expectedBody string
	}

	getMatch := func(req string, expected string, code int, body string) match {
		return match{
			reqURL:       req,
			expectedURL:  expected,
			expectedCode: code,
			expectedBody: body,
		}
	}

	type testCase struct {
		spec    *Spec
		matches []match
	}

	getMsg := func(caseId int, matchId int) string {
		return fmt.Sprintf("case %d match %d failed.", caseId, matchId)
	}

	// test different spec configurations
	for i, t := range []testCase{
		{
			spec: getSpec("(.*)", "", "$1", 0), // use default, uri, 301
			matches: []match{
				getMatch("http://a.com:8080/foo/bar?baz=qux", "/foo/bar?baz=qux", 301, "Moved Permanently"),
			},
		},
		{
			spec: getSpec("(.*)", "uri", "$1", 301), // uri, 301
			matches: []match{
				getMatch("http://a.com:8080/foo/bar?baz=qux", "/foo/bar?baz=qux", 301, "Moved Permanently"),
			},
		},
		{
			spec: getSpec("(.*)", "full", "$1", 302), // full, 302
			matches: []match{
				getMatch("http://a.com:8080/foo/bar?baz=qux", "http://a.com:8080/foo/bar?baz=qux", 302, "Found"),
			},
		},
		{
			spec: getSpec("(.*)", "path", "$1", 302), // path, 302
			matches: []match{
				getMatch("http://a.com:8080/foo/bar?baz=qux", "/foo/bar", 302, "Found"),
			},
		},
	} {
		r := &Redirector{spec: t.spec}
		r.Init()
		for j, m := range t.matches {
			msg := getMsg(i, j)
			req, err := http.NewRequest(http.MethodGet, m.reqURL, nil)
			assert.Nil(err, msg)
			httpReq, err := httpprot.NewRequest(req)
			assert.Nil(err, msg)

			ctx := context.New(nil)
			ctx.SetInputRequest(httpReq)
			r.Handle(ctx)

			resp := ctx.GetOutputResponse().(*httpprot.Response)
			assert.Equal(m.expectedURL, resp.Header().Get("Location"), msg)
			assert.Equal(m.expectedCode, resp.StatusCode(), msg)
			assert.Equal(m.expectedBody, string(resp.RawPayload()), msg)
		}
	}

	// test invalid spec
	for i, t := range []*Spec{
		getSpec("(.*)", "all", "$1", 0),   // invalid match part
		getSpec("(.*)", "uri", "$1", 200), // invalid status code
		getSpec("(+)", "uri", "$1", 301),  // invalid regex
	} {
		msg := fmt.Sprintf("case %d failed.", i)
		r := &Redirector{spec: t}
		assert.Panics(func() { r.Init() }, msg)
	}

	// test complicated regex
	for i, t := range []testCase{
		{
			spec: getSpec("^/users/([0-9]+)", "path", "display?user=$1", 301),
			matches: []match{
				getMatch("http://a.com:8080/users/123", "display?user=123", 301, "Moved Permanently"),
				getMatch("http://a.com:8080/users/9", "display?user=9", 301, "Moved Permanently"),
				getMatch("http://a.com:8080/users/34", "display?user=34", 301, "Moved Permanently"),
				getMatch("http://a.com:8080/users/a123", "/users/a123", 301, "Moved Permanently"),
				getMatch("http://a.com:8080/profile/users/a123", "/profile/users/a123", 301, "Moved Permanently"),
			},
		},
		{
			spec: getSpec("^/users/([0-9]+)/status/([a-z0-9]+)", "path", "display?user=$1&status=$2", 301),
			matches: []match{
				getMatch("http://a.com:8080/users/123/status/info", "display?user=123&status=info", 301, "Moved Permanently"),
				getMatch("http://a.com:8080/users/9/status/work", "display?user=9&status=work", 301, "Moved Permanently"),
			},
		},
		{
			spec: getSpec("^/users/([0-9]+)", "path", "http://example.com/display?user=$1", 301),
			matches: []match{
				getMatch("http://a.com:8080/users/123", "http://example.com/display?user=123", 301, "Moved Permanently"),
			},
		},
	} {
		r := &Redirector{spec: t.spec}
		r.Init()
		for j, m := range t.matches {
			msg := getMsg(i, j)
			req, err := http.NewRequest(http.MethodGet, m.reqURL, nil)
			assert.Nil(err, msg)
			httpReq, err := httpprot.NewRequest(req)
			assert.Nil(err, msg)

			ctx := context.New(nil)
			ctx.SetInputRequest(httpReq)
			r.Handle(ctx)

			resp := ctx.GetOutputResponse().(*httpprot.Response)
			assert.Equal(m.expectedURL, resp.Header().Get("Location"), msg)
			assert.Equal(m.expectedCode, resp.StatusCode(), msg)
			assert.Equal(m.expectedBody, string(resp.RawPayload()), msg)
		}
	}
}
