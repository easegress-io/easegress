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

package responseadaptor

import (
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/megaease/easegress/pkg/context/contexttest"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/texttemplate"
	"github.com/megaease/easegress/pkg/util/yamltool"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestResponseAdaptor(t *testing.T) {
	yamlSpec := `
kind: ResponseAdaptor
name: ra
header:
  del: ["X-Del"]
  add:
    "[[headerName]]": "mockedHeaderValue"
body: "copyright [[name]]"
`

	ra := doTest(t, yamlSpec, nil)

	yamlSpec = `
kind: ResponseAdaptor
name: ra
header:
  del: ["X-Del"]
  add:
    "[[headerName]]": "mockedHeaderValue"
body: "copyright megaease"
`
	doTest(t, yamlSpec, ra)

	yamlSpec = `
kind: ResponseAdaptor
name: ra
header:
  del: ["X-Del"]
  add:
    "[[headerName]]": "mockedHeaderValue"
`
	doTest(t, yamlSpec, nil)
}

func doTest(t *testing.T, yamlSpec string, prev *ResponseAdaptor) *ResponseAdaptor {
	rawSpec := make(map[string]interface{})
	yamltool.Unmarshal([]byte(yamlSpec), &rawSpec)

	spec, e := httppipeline.NewFilterSpec(rawSpec, nil)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	ra := &ResponseAdaptor{}
	if prev == nil {
		ra.Init(spec)
	} else {
		ra.Inherit(spec, prev)
	}

	ctx := &contexttest.MockedHTTPContext{}
	ctx.MockedTemplate = func() texttemplate.TemplateEngine {
		tt, _ := texttemplate.NewDefault([]string{"name", "headerName"})
		tt.SetDict("name", "megaease")
		tt.SetDict("headerName", "mockedHeader")
		return tt
	}
	resp := httptest.NewRecorder()
	resp.Header().Add("X-Del", "deleted")

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

	ra.Handle(ctx)

	if v := resp.Header().Get("mockedHeader"); v != "mockedHeaderValue" {
		t.Error("unexpected header name or value: ", v)
	}

	if v := resp.Header().Get("X-Del"); v != "" {
		t.Error("header 'X-Del' should not exist")
	}

	if v := resp.Body.String(); ra.spec.Body != "" && v != "copyright megaease" {
		t.Error("unexpected body:", v)
	}

	ra.Status()
	ra.Description()
	return ra
}
