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

package aigatewayproxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
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

func TestProxy(t *testing.T) {
	assert := assert.New(t)
	yamlConfig := `
kind: AIGatewayProxy
name: aigatewayproxy
providerName: openai
`
	rawSpec := make(map[string]interface{})
	codectool.MustUnmarshal([]byte(yamlConfig), &rawSpec)

	spec, e := filters.NewSpec(nil, "", rawSpec)
	if e != nil {
		t.Errorf("unexpected error: %v", e)
	}

	p := kind.CreateInstance(spec)
	p.Init()
	defer p.Close()

	ctx := context.New(nil)
	// test no controller
	{
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/chat/completions", nil)
		assert.Nil(err)
		setRequest(t, ctx, "mock", req)

		ctx.UseNamespace("mock")
		result := p.Handle(ctx)
		assert.Equal(resultNoController, result)

		resp := ctx.GetResponse("mock").(*httpprot.Response)
		assert.Equal(http.StatusInternalServerError, resp.StatusCode())
	}

	// test with controller
	{
		controllerConfig := `
kind: AIGatewayController
name: aigatewaycontroller
providers:
- name: openai
  providerType: openai
  baseURL: http://localhost:19876
  apiKey: mock
`
		super := supervisor.NewMock(option.New(), nil, nil,
			nil, false, nil, nil)
		spec, err := super.NewSpec(controllerConfig)
		assert.Nil(err)
		controller := aigatewaycontroller.AIGatewayController{}
		controller.Init(spec)

		// no backend
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/models", nil)
		assert.Nil(err)
		setRequest(t, ctx, "controller", req)

		ctx.UseNamespace("controller")
		result := p.Handle(ctx)
		assert.Equal("internalError", result)

		resp := ctx.GetResponse("controller").(*httpprot.Response)
		assert.Equal(http.StatusInternalServerError, resp.StatusCode())

		// close controller
		controller.Close()
	}

	// test with backend
	{
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
  "object": "list",
  "data": [
    {
      "id": "model-id-0",
      "object": "model",
      "created": 1686935002,
      "owned_by": "organization-owner"
    },
    {
      "id": "model-id-1",
      "object": "model",
      "created": 1686935002,
      "owned_by": "organization-owner",
    },
    {
      "id": "model-id-2",
      "object": "model",
      "created": 1686935002,
      "owned_by": "openai"
    },
  ],
  "object": "list"
}`))
		}))
		defer mockServer.Close()

		controllerConfig := `
kind: AIGatewayController
name: aigatewaycontroller
providers:
- name: openai
  providerType: openai
  baseURL: %s
  apiKey: mock
`
		controllerConfig = fmt.Sprintf(controllerConfig, mockServer.URL)
		super := supervisor.NewMock(option.New(), nil, nil,
			nil, false, nil, nil)
		spec, err := super.NewSpec(controllerConfig)
		assert.Nil(err)
		controller := aigatewaycontroller.AIGatewayController{}
		controller.Init(spec)

		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/models", nil)
		assert.Nil(err)
		setRequest(t, ctx, "backend", req)

		ctx.UseNamespace("backend")
		result := p.Handle(ctx)
		assert.Equal("", result)

		resp := ctx.GetResponse("backend").(*httpprot.Response)
		assert.Equal(http.StatusOK, resp.StatusCode())

		controller.Close()
	}
}
