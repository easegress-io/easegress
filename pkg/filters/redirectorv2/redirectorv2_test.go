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

package redirectorv2

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"

	gwapis "sigs.k8s.io/gateway-api/apis/v1"
)

type fakeMuxPath struct {
	routers.Path
}

func (mp *fakeMuxPath) Protocol() string                      { return "http" }
func (mp *fakeMuxPath) Rewrite(context *routers.RouteContext) {}

func TestSpecValidate(t *testing.T) {
	tests := []struct {
		name      string
		spec      *Spec
		expectErr bool
		errMsg    string
	}{
		{
			name: "Invalid Scheme",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Scheme: new(string),
				},
			},
			expectErr: true,
			errMsg:    "invalid scheme of Redirector, only support http and https",
		},
		{
			name: "Valid Scheme (http)",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Scheme: func() *string { s := "http"; return &s }(),
				},
			},
			expectErr: false,
		},
		{
			name: "Valid Scheme (https)",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Scheme: func() *string { s := "https"; return &s }(),
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid Port",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Port: func() *gwapis.PortNumber { i := gwapis.PortNumber(70000); return &i }(),
				},
			},
			expectErr: true,
			errMsg:    "invalid port of Redirector, only support 1-65535",
		},
		{
			name: "Valid Port",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Port: func() *gwapis.PortNumber { i := gwapis.PortNumber(8080); return &i }(),
				},
			},
			expectErr: false,
		},
		{
			name: "Invalid Status Code",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					StatusCode: func() *int { i := 299; return &i }(),
				},
			},
			expectErr: true,
			errMsg:    "invalid status code of Redirector, support 300, 301, 302, 303, 304, 307, 308",
		},
		{
			name: "Valid Status Code",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					StatusCode: func() *int { i := 302; return &i }(),
				},
			},
			expectErr: false,
		},
		{
			name: "Missing replaceFullPath",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Path: &gwapis.HTTPPathModifier{
						Type: gwapis.FullPathHTTPPathModifier,
					},
				},
			},
			expectErr: true,
			errMsg:    "invalid path of Redirector, replaceFullPath can't be empty",
		},
		{
			name: "Missing pathPrefix",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Path: &gwapis.HTTPPathModifier{
						Type: gwapis.PrefixMatchHTTPPathModifier,
					},
				},
			},
			expectErr: true,
			errMsg:    "invalid path of Redirector, replacePrefixMatch can't be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()

			if tt.expectErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRedirectorHandle2(t *testing.T) {
	tests := []struct {
		name           string
		reqURL         string
		spec           *Spec
		expectedResult string
		expectedURL    string
		expectedStatus int
	}{
		{
			name:   "Redirect full path",
			reqURL: "http://localhost/user/data/profile",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Path: &gwapis.HTTPPathModifier{
						Type:            gwapis.FullPathHTTPPathModifier,
						ReplaceFullPath: new(string),
					},
				},
			},
			expectedResult: resultRedirected,
			expectedURL:    "http://localhost/newpath",
			expectedStatus: 302,
		},
		{
			name:   "Redirect prefix",
			reqURL: "http://localhost/user/data/profile",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Path: &gwapis.HTTPPathModifier{
						Type:               gwapis.PrefixMatchHTTPPathModifier,
						ReplacePrefixMatch: new(string),
					},
				},
			},
			expectedResult: resultRedirected,
			expectedURL:    "http://localhost/account/data/profile",
			expectedStatus: 302,
		},
		{
			name:   "Change scheme",
			reqURL: "http://localhost/user/data/profile",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Scheme: new(string),
				},
			},
			expectedResult: resultRedirected,
			expectedURL:    "https://localhost/user/data/profile",
			expectedStatus: 302,
		},
		{
			name:   "Redirect with hostname",
			reqURL: "http://localhost/user/data/profile",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					Hostname: new(gwapis.PreciseHostname),
				},
			},
			expectedResult: resultRedirected,
			expectedURL:    "http://example.com/user/data/profile",
			expectedStatus: 302,
		},
		{
			name:           "No redirection",
			reqURL:         "http://localhost/user/data/profile",
			spec:           &Spec{},
			expectedResult: "",
			expectedStatus: 0, // 0 indicates no redirection, so no status check
		},
		{
			name:   "Custom status code",
			reqURL: "http://localhost/user/data/profile",
			spec: &Spec{
				HTTPRequestRedirectFilter: gwapis.HTTPRequestRedirectFilter{
					StatusCode: new(int),
				},
			},
			expectedResult: "",
			expectedURL:    "http://localhost/user/data/profile",
			expectedStatus: 0, // 0 indicates no redirection, so no status check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdReq, _ := http.NewRequest("GET", tt.reqURL, nil)
			req, _ := httpprot.NewRequest(stdReq)
			ctx := context.New(nil)
			ctx.SetInputRequest(req)
			ctx.SetRoute(&fakeMuxPath{
				Path: routers.Path{PathPrefix: "/user/"},
			})

			if tt.spec.Path != nil && tt.spec.Path.ReplaceFullPath != nil {
				*tt.spec.Path.ReplaceFullPath = "/newpath"
			}

			if tt.spec.Path != nil && tt.spec.Path.ReplacePrefixMatch != nil {
				*tt.spec.Path.ReplacePrefixMatch = "/account/"
			}

			if tt.spec.Scheme != nil {
				*tt.spec.Scheme = "https"
			}

			if tt.spec.Hostname != nil {
				*tt.spec.Hostname = "example.com"
			}

			if tt.spec.StatusCode != nil {
				*tt.spec.StatusCode = 303
			}

			rd := &Redirector{spec: tt.spec}
			result := rd.Handle(ctx)
			assert.Equal(t, tt.expectedResult, result)

			if result == resultRedirected {
				resp := ctx.GetOutputResponse().(*httpprot.Response)
				location := resp.Header().Get("Location")
				assert.Equal(t, tt.expectedURL, location)
				assert.Equal(t, tt.expectedStatus, resp.StatusCode())
			}
		})
	}
}
