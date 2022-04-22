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
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func init() {
	logger.InitMock()
}

func getResponseBuilder(spec *ResponseSpec) *HTTPResponseBuilder {
	rb := &HTTPResponseBuilder{spec: spec}
	rb.Init()
	return rb
}

func setRequest(t *testing.T, ctx *context.Context, id string, req *http.Request) {
	r, err := httpprot.NewRequest(req)
	assert.Nil(t, err)
	ctx.SetRequest(id, r)
}

func TestStatusCode(t *testing.T) {
	assert := assert.New(t)

	// set status code directly
	{
		spec := &ResponseSpec{
			ID: "test",
			StatusCode: &StatusCode{
				Code: http.StatusOK,
			},
		}
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetResponse("test").(*httpprot.Response).Std()
		assert.Equal(200, testReq.StatusCode)
	}

	// set status code from other response
	{
		spec := &ResponseSpec{
			ID: "test",
			StatusCode: &StatusCode{
				CopyResponseID: "response1",
			},
		}
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		resp, _ := httpprot.NewResponse(nil)
		resp.SetStatusCode(http.StatusBadRequest)
		ctx.SetResponse("response1", resp)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response).Std()
		assert.Equal(400, testResp.StatusCode)
	}
}

func TestResponseHeader(t *testing.T) {
	assert := assert.New(t)

	// get header from request and response
	{
		spec := &ResponseSpec{
			ID: "test",
			Headers: []Header{
				{"X-Request", `{{index (index .Requests.request1.Header "X-Request") 0}}`},
				{"X-Response", `{{index (index .Responses.response1.Header "X-Response") 0}}`},
			},
		}
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
	{
		spec := &ResponseSpec{
			ID:   "test",
			Body: "body",
		}
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testReq := ctx.GetResponse("test").(*httpprot.Response).Std()
		data, err := io.ReadAll(testReq.Body)
		assert.Nil(err)
		assert.Equal("body", string(data))
	}

	// set body by using other body
	{
		spec := &ResponseSpec{
			ID:   "test",
			Body: "body {{ .RequestBodies.request1.String }}",
		}
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com", strings.NewReader("123"))
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response).Std()
		data, err := io.ReadAll(testResp.Body)
		assert.Nil(err)
		assert.Equal("body 123", string(data))
	}

	// set body by using other body json map
	{
		spec := &ResponseSpec{
			ID:   "test",
			Body: "body {{ .RequestBodies.request1.JsonMap.field1 }} {{ .RequestBodies.request1.JsonMap.field2 }}",
		}
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com", strings.NewReader(`{"field1":"value1", "field2": "value2"}`))
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response).Std()
		data, err := io.ReadAll(testResp.Body)
		assert.Nil(err)
		assert.Equal("body value1 value2", string(data))
	}

	// set body by using other body yaml map
	{
		spec := &ResponseSpec{
			ID:   "test",
			Body: "body {{ .RequestBodies.request1.YamlMap.field1 }} {{ .RequestBodies.request1.YamlMap.field2 }}",
		}
		rb := getResponseBuilder(spec)
		defer rb.Close()

		ctx := context.New(nil)

		req1, err := http.NewRequest(http.MethodDelete, "http://www.google.com", strings.NewReader(`
field1: value1
field2: value2
`))
		assert.Nil(err)
		setRequest(t, ctx, "request1", req1)

		res := rb.Handle(ctx)
		assert.Empty(res)
		testResp := ctx.GetResponse("test").(*httpprot.Response).Std()
		data, err := io.ReadAll(testResp.Body)
		assert.Nil(err)
		assert.Equal("body value1 value2", string(data))
	}
}
