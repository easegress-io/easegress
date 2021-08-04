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

package circuitbreaker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestCircuitBreaker(t *testing.T) {
	const yamlSpec = `
kind: CircuitBreaker
name: circuitbreaker
policies:
- name: default
  slowCallRateThreshold: 1
  failureRateThreshold: 50
  slidingWindowType: COUNT_BASED
  slidingWindowSize: 10
  minimumNumberOfCalls: 5
  failureStatusCodes: [500, 503]
defaultPolicyRef: default
urls:
- methods: []
  url:
    exact: /circuitbreak
    prefix:
    regex:
`
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := httppipeline.NewFilterSpec(rawSpec, nil)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	cb := &CircuitBreaker{}
	cb.Init(spec)

	resp := httptest.NewRecorder()
	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedMethod = func() string {
		return http.MethodGet
	}
	ctx.MockedRequest.MockedPath = func() string {
		return "/circuitbreak"
	}
	ctx.MockedResponse.MockedStd = func() http.ResponseWriter {
		return resp
	}
	ctx.MockedResponse.MockedStatusCode = func() int {
		return http.StatusInternalServerError
	}

	for i := 0; i < 5; i++ {
		result := cb.Handle(ctx)
		if result == resultShortCircuited {
			t.Error("should not be short circuited")
		}
	}

	result := cb.Handle(ctx)
	if result != resultShortCircuited {
		t.Error("should be short circuited")
	}

	ctx.MockedRequest.MockedPath = func() string {
		return "/notcircuitbreak"
	}
	result = cb.Handle(ctx)
	if result == resultShortCircuited {
		t.Error("should not be short circuited")
	}

	ctx.MockedRequest.MockedPath = func() string {
		return "/circuitbreak"
	}
	newCb := &CircuitBreaker{}
	spec, _ = httppipeline.NewFilterSpec(rawSpec, nil)
	newCb.Inherit(spec, cb)
	cb.Close()
	result = newCb.Handle(ctx)
	if result != resultShortCircuited {
		t.Error("new circuit breaker should be short circuited")
	}
}
