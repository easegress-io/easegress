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

package aicontext

import (
	"bytes"
	"net/http"
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func setRequest(t *testing.T, ctx *context.Context, ns string, req *http.Request) {
	httpreq, err := httpprot.NewRequest(req)
	httpreq.FetchPayload(0)
	assert.Nil(t, err)
	ctx.SetRequest(ns, httpreq)
	ctx.UseNamespace(ns)
}

func TestContext(t *testing.T) {
	assert := assert.New(t)

	spec := &ProviderSpec{
		Name:         "openai",
		ProviderType: "openai",
		APIKey:       "test-api-key",
	}

	{
		// invalid path
		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/invalid-path", nil)
		assert.Nil(err)
		setRequest(t, ctx, "invalid.path", req)
		_, err = New(ctx, spec)
		assert.NotNil(err)
	}

	{
		// models
		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/models", nil)
		assert.Nil(err)
		setRequest(t, ctx, "models", req)
		aiCtx, err := New(ctx, spec)
		assert.Nil(err)
		assert.Equal(ResponseTypeModels, aiCtx.RespType)
	}

	{
		// invalid body
		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/chat/completions", bytes.NewReader([]byte(`{"model": 123}`)))
		assert.Nil(err)
		setRequest(t, ctx, "invlaid.body", req)
		_, err = New(ctx, spec)
		assert.NotNil(err)
	}

	{
		// chat completions
		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/chat/completions", bytes.NewReader([]byte(`{"model": "gpt"}`)))
		assert.Nil(err)
		setRequest(t, ctx, "chat.completions", req)

		// parse part
		aiCtx, err := New(ctx, spec)
		assert.Nil(err)
		assert.Equal(ResponseTypeChatCompletions, aiCtx.RespType)
		assert.False(aiCtx.ReqInfo.Stream)

		// callback
		assert.Zero(len(aiCtx.Callbacks()))
		aiCtx.AddCallBack(func(fc *FinishContext) {
		})
		assert.Equal(1, len(aiCtx.Callbacks()))

		// set response
		resp := &Response{
			StatusCode: 200,
		}
		aiCtx.SetResponse(resp)
		assert.Equal(200, aiCtx.GetResponse().StatusCode)

		// check stop
		assert.False(aiCtx.IsStopped())
		assert.Equal(ResultOk, aiCtx.Result())
		aiCtx.Stop(ResultInternalError)
		assert.True(aiCtx.IsStopped())
		assert.Equal(ResultInternalError, aiCtx.Result())
	}

}
