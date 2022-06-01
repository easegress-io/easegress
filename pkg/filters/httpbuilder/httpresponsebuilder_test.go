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

package httpbuilder

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/yamltool"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func init() {
	logger.InitMock()
}

func getResponseBuilder(spec *HTTPResponseBuilderSpec) *HTTPResponseBuilder {
	rb := &HTTPResponseBuilder{spec: spec}
	rb.Init()
	return rb
}

func setRequest(t *testing.T, ctx *context.Context, ns string, req *http.Request) {
	r, err := httpprot.NewRequest(req)
	r.FetchPayload(1024 * 1024)
	assert.Nil(t, err)
	ctx.SetRequest(ns, r)
}

func TestStatusCode(t *testing.T) {
	assert := assert.New(t)

	// set status code directly
	yml := `template: |
  statusCode: 200
`
	{
		spec := &HTTPResponseBuilderSpec{}
		yaml.Unmarshal([]byte(yml), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetResponse("test").(*httpprot.Response).Std()
		assert.Equal(200, testReq.StatusCode)
	}

	// set status code from other response
	yml = `template: |
  statusCode: {{.responses.response1.StatusCode}}
`
	{
		spec := &HTTPResponseBuilderSpec{}
		yaml.Unmarshal([]byte(yml), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		resp, _ := httpprot.NewResponse(nil)
		resp.SetStatusCode(http.StatusBadRequest)
		ctx.SetResponse("response1", resp)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response).Std()
		assert.Equal(400, testResp.StatusCode)
	}
}

func TestResponseHeader(t *testing.T) {
	assert := assert.New(t)

	// get header from request and response
	yml := `template: |
  headers:
    "X-Request": [{{index (index .requests.request1.Header "X-Request") 0}}]
    "X-Response": [{{index (index .responses.response1.Header "X-Response") 0}}]
`
	{
		spec := &HTTPResponseBuilderSpec{}
		yaml.Unmarshal([]byte(yml), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com?field1=value1&field2=value2", nil)
		assert.Nil(err)
		req1.Header.Add("X-Request", "from-request1")
		setRequest(t, ctx, "request1", req1)

		resp1 := &http.Response{}
		resp1.Header = http.Header{}
		resp1.Header.Add("X-Response", "from-response1")
		httpresp1, err := httpprot.NewResponse(resp1)
		assert.Nil(err)
		ctx.SetResponse("response1", httpresp1)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response).Std()
		assert.Equal("from-request1", testResp.Header.Get("X-Request"))
		assert.Equal("from-response1", testResp.Header.Get("X-Response"))
		assert.Equal(200, testResp.StatusCode)
	}
}

func TestResponseBody(t *testing.T) {
	assert := assert.New(t)

	// directly set body
	yml := `template: |
  body: body
`
	{
		spec := &HTTPResponseBuilderSpec{}
		yaml.Unmarshal([]byte(yml), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetResponse("test").(*httpprot.Response)
		data, err := io.ReadAll(testReq.GetPayload())
		assert.Nil(err)
		assert.Equal("body", string(data))
	}

	// set body by using other body
	yml = `template: |
  body: body {{ .requests.request1.Body }}
`
	{
		spec := &HTTPResponseBuilderSpec{}
		yaml.Unmarshal([]byte(yml), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com", strings.NewReader("123"))
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response)
		data, err := io.ReadAll(testResp.GetPayload())
		assert.Nil(err)
		assert.Equal("body 123", string(data))
	}

	// set body by using other body json map
	yml = `template: |
  body: body {{ .requests.request1.JSONBody.field1 }} {{ .requests.request1.JSONBody.field2 }}
`
	{
		spec := &HTTPResponseBuilderSpec{}
		yaml.Unmarshal([]byte(yml), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com", strings.NewReader(`{"field1":"value1", "field2": "value2"}`))
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response)
		data, err := io.ReadAll(testResp.GetPayload())
		assert.Nil(err)
		assert.Equal("body value1 value2", string(data))
	}

	// set body by using other body yaml map
	yml = `template: |
  body: body {{ .requests.request1.YAMLBody.field1 }} {{ .requests.request1.YAMLBody.field2 }}
`
	{
		spec := &HTTPResponseBuilderSpec{}
		yaml.Unmarshal([]byte(yml), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com", strings.NewReader(`
field1: value1
field2: value2
`))
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response)
		data, err := io.ReadAll(testResp.GetPayload())
		assert.Nil(err)
		assert.Equal("body value1 value2", string(data))
	}

	// invalid status code
	yml = `template: |
  statusCode: 800
`
	{
		spec := &HTTPResponseBuilderSpec{}
		yaml.Unmarshal([]byte(yml), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.NotEmpty(res)
	}
}

func TestHTTPResponseBuilder(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(&HTTPResponseBuilderSpec{}, httpResponseBuilderKind.DefaultSpec())
	yamlStr := `
name: responseBuilder 
kind: HTTPResponseBuilder 
template: |
  statusCode: 200 
`
	rawSpec := map[string]interface{}{}
	yamltool.Unmarshal([]byte(yamlStr), &rawSpec)
	spec, err := filters.NewSpec(nil, "pipeline1", rawSpec)
	assert.Nil(err)
	responseBuilder := httpResponseBuilderKind.CreateInstance(spec).(*HTTPResponseBuilder)
	assert.Equal("responseBuilder", responseBuilder.Name())
	assert.Equal(httpResponseBuilderKind, responseBuilder.Kind())
	assert.Equal(spec, responseBuilder.Spec())
	responseBuilder.Init()

	newResponseBuilder := httpResponseBuilderKind.CreateInstance(spec)
	newResponseBuilder.Inherit(responseBuilder)
	assert.Nil(newResponseBuilder.Status())
}
