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

package mock

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestMock(t *testing.T) {
	const yamlSpec = `
kind: Mock
name: mock
rules:
- match:
    pathPrefix: /login/
  code: 202
  body: 'mocked body'
  headers:
    X-Test: test1
- match:
    path: /sales
  code: 203
  body: 'mocked body'
  headers:
    X-Test: test2
  delay: 1ms
- match:
    path: /pets
    headers:
      X-Mock:
        exact: mock
  code: 205
  body: 'mocked body'
  headers:
    X-Test: test2
- match:
    path: /customers
    headers:
      X-Mock:
        empty: true
  code: 206
  body: 'mocked body'
  headers:
    X-Test: test2
- match:
    path: /vets
    matchAllHeader: true
    headers:
      X-Mock:
        exact: mock
  code: 207
  body: 'mocked body'
  headers:
    X-Test: test2
- code: 204
  body: 'mocked body 2'
  headers:
    X-Test: test3
`
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := filters.NewSpec(nil, "", rawSpec)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	m := &Mock{}
	m.Init(spec)

	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedRequest.MockedMethod = func() string {
		return http.MethodGet
	}
	resp := httptest.NewRecorder()
	ctx.MockedResponse.MockedSetStatusCode = func(code int) {
		resp.WriteHeader(code)
	}
	ctx.MockedResponse.MockedSetBody = func(body io.Reader) {
		data, _ := io.ReadAll(body)
		resp.Write(data)
	}
	ctx.MockedResponse.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(resp.Header())
	}
	ctx.MockedCallNextHandler = func(lastResult string) string {
		return ""
	}

	resp = httptest.NewRecorder()
	ctx.MockedRequest.MockedPath = func() string {
		return "/login/1"
	}
	m.Handle(ctx)
	if resp.Code != 202 {
		t.Error("status code is not 202")
	}

	resp = httptest.NewRecorder()
	ctx.MockedRequest.MockedPath = func() string {
		return "/sales"
	}
	m.Handle(ctx)
	if resp.Code != 203 {
		t.Error("status code is not 203")
	}

	if resp.Body.String() != "mocked body" {
		t.Error("body should be 'mocked body'")
	}

	if resp.Header().Get("X-Test") != "test2" {
		t.Error("header 'X-Test' should be 'test2'")
	}

	resp = httptest.NewRecorder()
	ctx.MockedRequest.MockedPath = func() string {
		return "/pets"
	}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		h := http.Header{}
		h.Add("X-Mock", "mock")
		return httpheader.New(h)
	}
	m.Handle(ctx)
	if resp.Code != 205 {
		t.Error("status code is not 205")
	}
	if resp.Body.String() != "mocked body" {
		t.Error("body should be 'mocked body'")
	}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		h := http.Header{}
		h.Add("X-Mock", "mock1")
		return httpheader.New(h)
	}
	resp = httptest.NewRecorder()
	m.Handle(ctx)
	if resp.Code != 204 {
		t.Errorf("status code is %d, not 204", resp.Code)
	}

	resp = httptest.NewRecorder()
	ctx.MockedRequest.MockedPath = func() string {
		return "/customers"
	}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		h := http.Header{
			"X-Mock": []string{},
		}
		return httpheader.New(h)
	}
	m.Handle(ctx)
	if resp.Code != 206 {
		t.Errorf("status code is %d, not 206", resp.Code)
	}
	if body := resp.Body.String(); body != "mocked body" {
		t.Errorf("body is %q should be 'mocked body'", body)
	}

	resp = httptest.NewRecorder()
	ctx.MockedRequest.MockedPath = func() string {
		return "/vets"
	}
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		h := http.Header{}
		h.Add("X-Mock", "mock")
		return httpheader.New(h)
	}
	m.Handle(ctx)
	if resp.Code != 207 {
		t.Error("status code is not 207")
	}
	if resp.Body.String() != "mocked body" {
		t.Error("body should be 'mocked body'")
	}

	resp = httptest.NewRecorder()
	ctx.MockedRequest.MockedHeader = func() *httpheader.HTTPHeader {
		h := http.Header{}
		h.Add("X-Mock", "mock1")
		return httpheader.New(h)
	}
	m.Handle(ctx)
	if resp.Code != 204 {
		t.Errorf("status code is %d, not 204", resp.Code)
	}

	if m.Status() != nil {
		t.Error("behavior changed, please update this case")
	}
	m.Description()

	resp = httptest.NewRecorder()
	ctx.MockedRequest.MockedPath = func() string {
		return "/customer"
	}
	newM := &Mock{}
	spec, _ = filters.NewSpec(nil, "", rawSpec)
	newM.Inherit(spec, m)
	m.Close()
	newM.Handle(ctx)
	if resp.Code != 204 {
		t.Error("status code is not 204")
	}
}
