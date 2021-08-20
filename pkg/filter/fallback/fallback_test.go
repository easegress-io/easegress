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

package fallback

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestFallback(t *testing.T) {
	const yamlSpec = `
kind: Fallback
name: fallback
mockCode: 203
mockHeaders:
  X-Mocked: true
mockBody: "mocked body"
`
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := httppipeline.NewFilterSpec(rawSpec, nil)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	fb := &Fallback{}
	fb.Init(spec)

	resp := httptest.NewRecorder()
	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedResponse.MockedSetStatusCode = func(code int) {
		resp.WriteHeader(code)
	}
	ctx.MockedResponse.MockedHeader = func() *httpheader.HTTPHeader {
		return httpheader.New(resp.Header())
	}
	ctx.MockedResponse.MockedSetBody = func(body io.Reader) {
		data, _ := io.ReadAll(body)
		resp.Write(data)
	}

	fb.Handle(ctx)
	if resp.Code != 203 {
		t.Error("status code is not correct")
	}
	if resp.Body.String() != "mocked body" {
		t.Error("body is not correct")
	}
	if resp.Header().Get("X-Mocked") != "true" {
		t.Error("header is not correct")
	}

	if fb.Status() != nil {
		t.Error("behavior changed, please update this case")
	}
	fb.Description()

	newFb := &Fallback{}
	spec, _ = httppipeline.NewFilterSpec(rawSpec, nil)
	newFb.Inherit(spec, fb)
	fb.Close()
	ctx.MockedRequest.MockedMethod = func() string {
		return http.MethodGet
	}

	resp = httptest.NewRecorder()
	newFb.Handle(ctx)
	if resp.Code != 203 {
		t.Error("status code is not correct")
	}
	if resp.Body.String() != "mocked body" {
		t.Error("body is not correct")
	}
	if resp.Header().Get("X-Mocked") != "true" {
		t.Error("header is not correct")
	}
}
