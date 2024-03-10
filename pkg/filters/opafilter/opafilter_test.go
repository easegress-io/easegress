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

package opafilter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

func setRequest(t *testing.T, ctx *context.Context, stdReq *http.Request) {
	req, err := httpprot.NewRequest(stdReq)
	assert.Nil(t, err)
	ctx.SetInputRequest(req)
}

func createOPAFilter(yamlConfig string, prev *OPAFilter, supervisor *supervisor.Supervisor) *OPAFilter {
	rawSpec := make(map[string]interface{})
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)
	spec, err := filters.NewSpec(supervisor, "", rawSpec)
	if err != nil {
		panic(err.Error())
	}
	v := &OPAFilter{spec: spec.(*Spec)}
	if prev == nil {
		v.Init()
	} else {
		v.Inherit(prev)
	}
	return v
}

type testCase struct {
	req             func() *http.Request
	status          int
	shouldRegoError bool
	readBody        bool
	policy          string
	defaultStatus   int
	includedHeaders string
}

func TestOpaPolicyInFilter(t *testing.T) {
	testCases := map[string]testCase{
		"allow": {
			policy: `     
                        package http
						allow = true`,
			status: 200,
		},
		"deny": {
			policy: `
						package http
						allow = false`,
			status: 403,
		},
		"allow with path": {
			policy: `
						package http
						default allow = false

						allow {
							input.request.path_parts[0] == "allowed"
						}
						`,
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "https://example.com/allowed", nil)
			},
			status: 200,
		},
		"deny with path": {
			policy: `
						package http
						default allow = false

						allow {
							input.request.path_parts[0] == "allowed"
						}
						`,
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "https://example.com/forbidden", nil)
			},
			status: 403,
		},
		"allow when includedHeaders not configured": {
			policy: `
						package http
						default allow = true

						allow  {
							input.request.headers["x-bad-header"] == "1"
						}
						`,
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
				r.Header.Add("x-bad-header", "1")
				return r
			},
			status: 200,
		},
		"deny when includedHeaders configured": {
			policy: `
						package http
						default allow = true

						allow = { "status_code": 403 } {
							input.request.headers["X-Bad-Header"] == "1"
						}
					`,
			includedHeaders: "x-bad-header",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
				r.Header.Add("X-BAD-HEADER", "1")
				return r
			},
			status: 403,
		},
		"err on bad allow type": {
			policy: `
						package http
						allow = 1`,
			shouldRegoError: true,
		},
		"err on bad package": {
			policy: `
						package http.authz
						allow = true`,
			shouldRegoError: true,
		},
		"status config": {
			policy: `
						package http
						allow = false`,
			defaultStatus: 500,
			status:        500,
		},
		"body is not read by default": {
			// `"readBody": "false"` is the default value
			policy: `
						package http
						default allow = false
						allow {
							input.request.body == "allow"
						}
						`,
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "https://example.com", strings.NewReader("allow"))
				r.Header.Add("content-type", "text/plain; charset=utf8")
				return r
			},
			status: 403,
		},
		"reject when multiple headers included with space": {
			policy: `
						package http
						default allow = false
						allow {
							input.request.headers["X-Jwt-Header"]
							input.request.headers["X-My-Custom-Header"]
						}
						`,
			includedHeaders: "x-my-custom-header, x-jwt-header",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
				r.Header.Add("x-jwt-header", "1")
				r.Header.Add("x-bad-header", "2")
				return r
			},
			status: 403,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			yamlConf := buildYamlConf(t, &tc)
			opaFilter := createOPAFilter(yamlConf, nil, nil)
			var r *http.Request
			if tc.req != nil {
				r = tc.req()
			} else {
				r = httptest.NewRequest(http.MethodGet, "https://example.com", nil)
			}

			ctx := context.New(nil)
			setRequest(t, ctx, r)

			result := opaFilter.Handle(ctx)

			httpResp, _ := ctx.GetOutputResponse().(*httpprot.Response)
			stdResp := httpResp.Response
			if tc.shouldRegoError {
				assert.Equal(t, 403, stdResp.StatusCode)
				assert.Equal(t, resultFiltered, result)
				assert.Equal(t, "true", stdResp.Header.Get(opaErrorHeaderKey))
				return
			}
			assert.Equal(t, tc.status, stdResp.StatusCode)

			if tc.status == 200 {
				assert.Equal(t, "", result)
			} else {
				assert.Equal(t, resultFiltered, result)
			}
		})
	}
}

func buildYamlConf(t *testing.T, tc *testCase) string {
	buf, err := codectool.MarshalYAML(struct {
		Name            string `yaml:"name"`
		Kind            string `yaml:"kind"`
		Policy          string `yaml:"policy"`
		DefaultStatus   int    `yaml:"defaultStatus"`
		IncludedHeaders string `yaml:"includedHeaders"`
		ReadBody        bool   `yaml:"readBody"`
	}{
		Name:            "opaFilter",
		Kind:            "OPAFilter",
		Policy:          tc.policy,
		DefaultStatus:   tc.defaultStatus,
		IncludedHeaders: tc.includedHeaders,
		ReadBody:        tc.readBody,
	})
	if err != nil {
		t.Error(err)
	}
	return string(buf)
}
