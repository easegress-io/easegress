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

package builder

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitMock()
}

func getResponseBuilder(spec *ResponseBuilderSpec) *ResponseBuilder {
	spec.Protocol = "http"
	rb := &ResponseBuilder{spec: spec}
	rb.Init()
	return rb
}

func TestStatusCode(t *testing.T) {
	assert := assert.New(t)

	// set status code directly
	yamlConfig := `template: |
  statusCode: 200
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
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
	yamlConfig = `template: |
  statusCode: {{.responses.response1.StatusCode}}
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
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
	yamlConfig := `template: |
  headers:
    "X-Request": [{{index (index .requests.request1.Header "X-Request") 0}}]
    "X-Response": [{{index (index .responses.response1.Header "X-Response") 0}}]
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
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
	yamlConfig := `template: |
  body: body
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
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
	yamlConfig = `template: |
  body: body {{ .requests.request1.Body }}
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
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
	yamlConfig = `template: |
  body: body {{ .requests.request1.JSONBody.field1 }} {{ .requests.request1.JSONBody.field2 }}
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
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
	yamlConfig = `template: |
  body: body {{ .requests.request1.YAMLBody.field1 }} {{ .requests.request1.YAMLBody.field2 }}
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
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
	yamlConfig = `template: |
  statusCode: 800
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.NotEmpty(res)
	}
}

func TestResponseBuilder(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(&ResponseBuilderSpec{Protocol: "http"}, responseBuilderKind.DefaultSpec())
	yamlConfig := `
name: responseBuilder
kind: ResponseBuilder
template: |
  statusCode: 200
`
	rawSpec := map[string]interface{}{}
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)
	spec, err := filters.NewSpec(nil, "pipeline1", rawSpec)
	assert.Nil(err)
	responseBuilder := responseBuilderKind.CreateInstance(spec).(*ResponseBuilder)
	assert.Equal("responseBuilder", responseBuilder.Name())
	assert.Equal(responseBuilderKind, responseBuilder.Kind())
	assert.Equal(spec, responseBuilder.Spec())
	responseBuilder.Init()

	newResponseBuilder := responseBuilderKind.CreateInstance(spec)
	newResponseBuilder.Inherit(responseBuilder)
	assert.Nil(newResponseBuilder.Status())
}

func TestRespSourceNamespace(t *testing.T) {
	assert := assert.New(t)

	// set status code directly
	yamlConfig := `
sourceNamespace: response1
`
	{
		spec := &ResponseBuilderSpec{}
		codectool.MustUnmarshal([]byte(yamlConfig), spec)
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)
		stdResp := httptest.NewRecorder().Result()
		resp, err := httpprot.NewResponse(stdResp)
		assert.Nil(err)
		ctx.SetResponse("response1", resp)

		ctx.UseNamespace("test")
		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response).Std()
		assert.Equal(stdResp, testResp)
	}
}

func TestResponseBuilderSpecValidate(t *testing.T) {
	assert := assert.New(t)

	// invalid protocol
	yamlConfig := `
name: responseBuilder
kind: ResponseBuilder
protocol: foo
`
	spec := &ResponseBuilderSpec{}
	codectool.MustUnmarshal([]byte(yamlConfig), spec)
	assert.Error(spec.Validate())

	// source namespace and template are both empty
	yamlConfig = `
name: responseBuilder
kind: ResponseBuilder
protocol: http
`
	spec = &ResponseBuilderSpec{}
	codectool.MustUnmarshal([]byte(yamlConfig), spec)
	assert.Error(spec.Validate())

	// source namespace and template are both specified
	yamlConfig = `
name: responseBuilder
kind: ResponseBuilder
protocol: http
sourceNamespace: request1
template: |
  method: Delete
`
	spec = &ResponseBuilderSpec{}
	codectool.MustUnmarshal([]byte(yamlConfig), spec)
	assert.Error(spec.Validate())
}
