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
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
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
- pathPrefix: /login/
  code: 202
  body: 'mocked body'
  headers:
    X-Test: test1
- path: /sales
  code: 203
  body: 'mocked body'
  headers:
    X-Test: test2
  delay: 1ms
- code: 204
  body: 'mocked body 2'
  headers:
    X-Test: test3
`
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := httppipeline.NewFilterSpec(rawSpec, nil)
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

	if m.Status() != nil {
		t.Error("behavior changed, please update this case")
	}
	m.Description()

	resp = httptest.NewRecorder()
	ctx.MockedRequest.MockedPath = func() string {
		return "/customer"
	}
	newM := &Mock{}
	spec, _ = httppipeline.NewFilterSpec(rawSpec, nil)
	newM.Inherit(spec, m)
	m.Close()
	newM.Handle(ctx)
	if resp.Code != 204 {
		t.Error("status code is not 204")
	}
}
