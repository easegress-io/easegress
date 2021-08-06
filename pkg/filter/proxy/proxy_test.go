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

package proxy

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestProxy(t *testing.T) {
	const yamlSpec = `
name: proxy
kind: Proxy
fallback:
  forCodes: true
  mockCode: 200
  mockHeaders:
    X-Mock: mocked
  mockBody: this is the mocked body
mainPool:
  servers:
  - url: http://127.0.0.1:9095
  - url: http://127.0.0.1:9096
  - url: http://127.0.0.1:9097
  loadBalance:
    policy: roundRobin
candidatePools:
- filter:
    headers:
      "X-Test":
        exact: testheader
  servers:
  - url: http://127.0.0.2:9095
  - url: http://127.0.0.2:9096
  - url: http://127.0.0.2:9097
  - url: http://127.0.0.2:9098
  loadBalance:
    policy: roundRobin
mirrorPool:
  filter:
    headers:
      "X-Test":
        exact: mirror
  servers:
  - url: http://127.0.0.3:9095
  - url: http://127.0.0.3:9096
  loadBalance:
    policy: roundRobin
compression:
  minLength: 1024
failureCodes: [503, 504]
`
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := httppipeline.NewFilterSpec(rawSpec, nil)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	proxy := &Proxy{}
	proxy.Init(spec)

	if len(proxy.candidatePools) != 1 {
		t.Error("length of candidate pools is incorrect")
	}
	if len(proxy.mirrorPool.spec.Servers) != 2 {
		t.Error("server count of mirror pool is incorrect")
	}

	status := proxy.Status()
	if status == nil {
		t.Error("status should not be nil")
	}

	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedResponse.MockedStatusCode = func() int {
		return http.StatusServiceUnavailable
	}
	ctx.MockedResponse.MockedHeader = func() *httpheader.HTTPHeader {
		header := http.Header{}
		return httpheader.New(header)
	}
	if !proxy.fallbackForCodes(ctx) {
		t.Error("fallback for 503 should be true")
	}
	ctx.MockedResponse.MockedStatusCode = func() int {
		return http.StatusInternalServerError
	}
	if proxy.fallbackForCodes(ctx) {
		t.Error("fallback for 500 should be false")
	}

	proxy.Close()
}
