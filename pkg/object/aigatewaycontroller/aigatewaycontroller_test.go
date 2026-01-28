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

package aigatewaycontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/stretchr/testify/assert"
)

func setRequest(t *testing.T, ctx *context.Context, ns string, req *http.Request) {
	httpreq, err := httpprot.NewRequest(req)
	httpreq.FetchPayload(0)
	assert.Nil(t, err)
	ctx.SetRequest(ns, httpreq)
	ctx.UseNamespace(ns)
}

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestController(t *testing.T) {
	assert := assert.New(t)
	// test without backend
	{
		ctx := context.New(nil)
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
		controller := AIGatewayController{}
		controller.Init(spec)

		// no backend
		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/v1/models", nil)
		assert.Nil(err)
		setRequest(t, ctx, "controller", req)

		result := controller.Handle(ctx, "openai", nil)
		assert.Equal("internalError", result)

		resp := ctx.GetResponse("controller").(*httpprot.Response)
		assert.Equal(http.StatusInternalServerError, resp.StatusCode())

		// close controller
		controller.Close()
	}

	// test with backend
	{
		mockServer := httptest.NewServer(http.HandlerFunc(chatCompletionsHandler))
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
		controller := AIGatewayController{}
		controller.Init(spec)

		ctx := context.New(nil)
		req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:8080/v1/chat/completions", bytes.NewReader([]byte(`{"model": "gpt", "stream": false}`)))
		assert.Nil(err)
		setRequest(t, ctx, "backend", req)

		result := controller.Handle(ctx, "openai", nil)
		assert.Equal("", result)

		resp := ctx.GetResponse("backend").(*httpprot.Response)
		assert.Equal(http.StatusOK, resp.StatusCode())
		ctx.Finish()

		stats := controller.metricshub.GetStats()
		assert.Greater(len(stats), 0)

		controller.Close()
	}
}

func chatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	type ChatCompletionRequest struct {
		Model string `json:"model"`
	}

	var req ChatCompletionRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := getNonStreamBody(req.Model)
	json.NewEncoder(w).Encode(resp)
}

func getNonStreamBody(model string) any {
	return map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "Hello! How can I assist you today?",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     19,
			"completion_tokens": 10,
			"total_tokens":      29,
			"prompt_tokens_details": map[string]any{
				"cached_tokens": 0,
				"audio_tokens":  0,
			},
			"completion_tokens_details": map[string]any{
				"reasoning_tokens":           0,
				"audio_tokens":               0,
				"accepted_prediction_tokens": 0,
				"rejected_prediction_tokens": 0,
			},
		},
		"service_tier": "default",
	}
}
