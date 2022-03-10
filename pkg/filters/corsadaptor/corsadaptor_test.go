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

package corsadaptor

import (
	"net/http"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestCORSAdaptor(t *testing.T) {
	t.Run("CORS preflight-request", func(t *testing.T) {
		const yamlSpec = `
kind: CORSAdaptor
name: cors
`
		rawSpec := make(map[string]interface{})
		yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

		spec, e := filters.NewSpec(nil, "", rawSpec)
		if e != nil {
			t.Errorf("unexpected error: %v", e)
		}

		cors := &CORSAdaptor{}
		cors.Init(spec)

		header := http.Header{}
		ctx := &contexttest.MockedHTTPContext{}
		ctx.MockedRequest.MockedMethod = func() string {
			return http.MethodOptions
		}
		ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
			return httpheader.New(header)
		}

		result := cors.Handle(ctx)
		if result == resultPreflighted {
			t.Error("request should not be preflighted")
		}

		header.Add("Access-Control-Request-Method", "abc")
		result = cors.Handle(ctx)
		if result != resultPreflighted {
			t.Error("request should be preflighted")
		}

		newCors := &CORSAdaptor{}
		spec, _ = filters.NewSpec(nil, "", rawSpec)
		newCors.Inherit(spec, cors)
		cors.Close()
		ctx.MockedRequest.MockedMethod = func() string {
			return http.MethodGet
		}
		result = newCors.Handle(ctx)
		if result == resultPreflighted {
			t.Error("request should not be preflighted")
		}
	})
	t.Run("CORS request", func(t *testing.T) {
		const yamlSpec = `
kind: CORSAdaptor
name: cors
supportCORSRequest: true
allowedOrigins:
  - test.orig.test
`
		rawSpec := make(map[string]interface{})
		yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

		spec, e := filters.NewSpec(nil, "", rawSpec)
		if e != nil {
			t.Errorf("unexpected error: %v", e)
		}

		cors := &CORSAdaptor{}
		cors.Init(spec)
		cors.Status()
		header := http.Header{}
		ctx := &contexttest.MockedHTTPContext{}
		ctx.MockedRequest.MockedMethod = func() string {
			return http.MethodOptions
		}
		ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
			return httpheader.New(header)
		}
		result := cors.Handle(ctx)
		if result == resultPreflighted {
			t.Error("request should not be preflighted")
		}
		header.Add("Origin", "test.orig.test")
		header.Add("Access-Control-Request-Method", "get")
		result = cors.Handle(ctx)
		if result != resultPreflighted {
			t.Error("request should be preflighted")
		}

		header = http.Header{}
		header.Add("Origin", "test.orig.test")
		ctx = &contexttest.MockedHTTPContext{}
		ctx.MockedRequest.MockedMethod = func() string {
			return http.MethodGet
		}
		ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
			return httpheader.New(header)
		}
		result = cors.Handle(ctx)
		if result == resultPreflighted {
			t.Error("request should not be preflighted")
		}
	})
}
