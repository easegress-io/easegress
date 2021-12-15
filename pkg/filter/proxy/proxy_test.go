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
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/httpfilter"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/memorycache"
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
      "X-Mirror":
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
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		header := http.Header{}
		header.Set("X-Mirror", "mirror")
		return httpheader.New(header)
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

	fnSendRequest = func(r *http.Request, client *http.Client) (*http.Response, error) {
		return &http.Response{
			Body: io.NopCloser(strings.NewReader("this is the body")),
		}, nil
	}

	result := proxy.Handle(ctx)
	if result != "" {
		t.Error("proxy.Handle should succeeded")
	}
	ctx.Finish()

	fnSendRequest = func(r *http.Request, client *http.Client) (*http.Response, error) {
		return nil, fmt.Errorf("mocked error")
	}

	result = proxy.Handle(ctx)
	if result == "" {
		t.Error("proxy.Handle should fail")
	}

	time.Sleep(10 * time.Millisecond)
	proxy.Close()
	time.Sleep(10 * time.Millisecond)
}

func TestSpecValidate(t *testing.T) {
	spec := Spec{}

	if spec.Validate() == nil {
		t.Error("validate should fail")
	}

	spec.MainPool = &PoolSpec{}
	spec.MainPool.Filter = &httpfilter.Spec{}
	if spec.Validate() == nil {
		t.Error("validate should fail")
	}

	spec.MainPool.Filter = nil
	if spec.Validate() != nil {
		t.Error("validate should succeed")
	}

	spec.CandidatePools = append(spec.CandidatePools, &PoolSpec{})
	if spec.Validate() == nil {
		t.Error("validate should fail")
	}

	spec.CandidatePools[0].Filter = &httpfilter.Spec{}
	if spec.Validate() != nil {
		t.Error("validate should succeed")
	}

	spec.MirrorPool = &PoolSpec{}
	if spec.Validate() == nil {
		t.Error("validate should fail")
	}

	spec.MirrorPool.Filter = &httpfilter.Spec{}
	if spec.Validate() != nil {
		t.Error("validate should succeed")
	}

	spec.MirrorPool.MemoryCache = &memorycache.Spec{}
	if spec.Validate() == nil {
		t.Error("validate should fail")
	}
	spec.MirrorPool.MemoryCache = nil

	spec.Fallback = &FallbackSpec{}
	if spec.Validate() == nil {
		t.Error("validate should fail")
	}

	spec.FailureCodes = []int{500}
	if spec.Validate() != nil {
		t.Error("validate should succeed")
	}
}

func TestPoolSpecValidate(t *testing.T) {
	spec := PoolSpec{}

	if spec.Validate() == nil {
		t.Error("validate should fail")
	}

	servers := []*Server{
		{
			URL:  "http://127.0.0.1:9090",
			Tags: []string{"d1", "v1", "green"},
		},
		{
			URL:    "http://127.0.0.1:9091",
			Tags:   []string{"v1", "d1", "green"},
			Weight: 2,
		},
	}
	spec.Servers = servers
	if spec.Validate() == nil {
		t.Error("validate should fail")
	}

	servers[0].Weight = 1
	spec.ServersTags = []string{"v2"}
	if spec.Validate() == nil {
		t.Error("validate should fail")
	}

	spec.ServersTags = []string{"v1"}
	if spec.Validate() != nil {
		t.Error("validate should succeed")
	}
}
