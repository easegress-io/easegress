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

package mock

import (
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/stretchr/testify/assert"
)

func setRequest(t *testing.T, ctx *context.Context, ns string, req *http.Request) {
	httpreq, err := httpprot.NewRequest(req)
	assert.Nil(t, err)
	ctx.SetRequest(ns, httpreq)
}

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestMock(t *testing.T) {
	assert := assert.New(t)
	const yamlConfig = `
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
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)

	spec, e := filters.NewSpec(nil, "", rawSpec)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	m := kind.CreateInstance(spec)
	m.Init()

	ctx := context.New(nil)
	{
		req, err := http.NewRequest(http.MethodGet, "http://example.com/login/1", nil)
		assert.Nil(err)
		setRequest(t, ctx, "id1", req)

		ctx.UseNamespace("id1")
		m.Handle(ctx)

		resp := ctx.GetResponse("id1").(*httpprot.Response)
		assert.Equal(202, resp.StatusCode())
	}

	{
		req, err := http.NewRequest(http.MethodGet, "http://example.com/sales", nil)
		assert.Nil(err)
		setRequest(t, ctx, "id2", req)

		ctx.UseNamespace("id2")
		m.Handle(ctx)

		resp := ctx.GetResponse("id2").(*httpprot.Response)
		assert.Equal(203, resp.StatusCode())
		body, err := io.ReadAll(resp.GetPayload())
		assert.Nil(err)
		assert.Equal("mocked body", string(body))
		assert.Equal("test2", resp.Std().Header.Get("X-Test"))
	}

	{
		req, err := http.NewRequest(http.MethodGet, "http://example.com/pets", nil)
		assert.Nil(err)
		req.Header.Set("X-Mock", "mock")
		setRequest(t, ctx, "id3", req)

		ctx.UseNamespace("id3")
		m.Handle(ctx)

		resp := ctx.GetResponse("id3").(*httpprot.Response)
		assert.Equal(205, resp.StatusCode())
		body, err := io.ReadAll(resp.GetPayload())
		assert.Nil(err)
		assert.Equal("mocked body", string(body))

		req.Header.Set("X-Mock", "mock1")
		m.Handle(ctx)
		resp = ctx.GetResponse("id3").(*httpprot.Response)
		assert.Equal(204, resp.StatusCode())
	}

	{
		req, err := http.NewRequest(http.MethodGet, "http://example.com/customers", nil)
		assert.Nil(err)
		req.Header = http.Header{
			"X-Mock": []string{},
		}
		setRequest(t, ctx, "id4", req)

		ctx.UseNamespace("id4")
		m.Handle(ctx)
		resp := ctx.GetResponse("id4").(*httpprot.Response)
		assert.Equal(206, resp.StatusCode())
		body, err := io.ReadAll(resp.GetPayload())
		assert.Nil(err)
		assert.Equal("mocked body", string(body))
	}

	{
		req, err := http.NewRequest(http.MethodGet, "http://example.com/vets", nil)
		assert.Nil(err)
		req.Header.Set("X-Mock", "mock")
		setRequest(t, ctx, "id5", req)

		ctx.UseNamespace("id5")
		m.Handle(ctx)

		resp := ctx.GetResponse("id5").(*httpprot.Response)
		assert.Equal(207, resp.StatusCode())
		body, err := io.ReadAll(resp.GetPayload())
		assert.Nil(err)
		assert.Equal("mocked body", string(body))

		req.Header.Set("X-Mock", "mock1")

		m.Handle(ctx)

		resp = ctx.GetResponse("id5").(*httpprot.Response)
		assert.Equal(204, resp.StatusCode())
	}

	{
		req, err := http.NewRequest(http.MethodGet, "http://example.com/customer", nil)
		assert.Nil(err)
		setRequest(t, ctx, "id6", req)

		spec, _ = filters.NewSpec(nil, "", rawSpec)
		newM := kind.CreateInstance(spec)
		newM.Inherit(m)
		m.Close()

		ctx.UseNamespace("id6")
		newM.Handle(ctx)

		resp := ctx.GetResponse("id6").(*httpprot.Response)
		assert.Equal(204, resp.StatusCode())
	}
}
