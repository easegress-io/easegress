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

package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestGetLastChunkFromOpenAIStream(t *testing.T) {
	assert := assert.New(t)
	cases := []struct {
		input  string
		output string
		err    metricshub.MetricError
	}{
		{input: "data: 123\n\ndata: 234\n\ndata: [DONE]\n\n", output: "234", err: ""},
		{input: "data: 123\n\ndata: 234\n\ndata: [DONE]", output: "234", err: ""},
		{input: "data: 123\n\ndata: 234\n\n", output: "", err: metricshub.MetricMarshalError},
	}

	for _, c := range cases {
		output, err := getLastChunkFromOpenAIStream([]byte(c.input))
		assert.Equal(c.err, err, c)
		if c.output == "" {
			assert.Nil(output, c)
		} else {
			assert.Equal(c.output, string(output), c)
		}
	}
}

func TestBaseProvider(t *testing.T) {
	assert := assert.New(t)
	mockServer := httptest.NewServer(http.HandlerFunc(chatCompletionsHandler))
	defer mockServer.Close()

	providerSpec := &aicontext.ProviderSpec{
		Name:         "openai",
		ProviderType: "openai",
		BaseURL:      mockServer.URL,
		APIKey:       "test-api-key",
	}
	provider := &BaseProvider{}
	provider.init(providerSpec)

	{
		// stream
		content := "Hello, how are you?"
		tokens := strings.Split(content, " ")
		ctx := context.New(nil)
		req, err := createChatCompletionRequest("gpt-5", true, content)
		assert.Nil(err)
		setRequest(t, ctx, "chat.completions", req)
		aiCtx, err := aicontext.New(ctx, providerSpec)
		assert.Nil(err)
		provider.Handle(aiCtx)

		resp := aiCtx.GetResponse()
		assert.Equal(200, resp.StatusCode)

		data, err := io.ReadAll(resp.BodyReader)
		assert.Nil(err)
		metrics := aiCtx.ParseMetricFn(&aicontext.FinishContext{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			RespBody:   data,
			Duration:   100,
		})
		assert.NotNil(metrics)
		assert.True(metrics.Success)
		assert.Equal("openai", metrics.Provider)
		assert.Equal("openai", metrics.ProviderType)
		assert.Equal("gpt-5", metrics.Model)
		assert.Equal("/v1/chat/completions", metrics.ResponseType)
		assert.Equal(int64(len(tokens)), metrics.InputTokens)
		assert.Equal(int64(len(tokens)), metrics.OutputTokens)

		for _, cb := range aiCtx.Callbacks() {
			fc := &aicontext.FinishContext{
				StatusCode: resp.StatusCode,
				Header:     resp.Header,
				RespBody:   data,
				Duration:   100,
			}
			cb(fc)
		}
	}

	{
		// non-stream
		content := "This a a test message of non-stream response."
		tokens := strings.Split(content, " ")
		ctx := context.New(nil)
		req, err := createChatCompletionRequest("gpt-5", false, content)
		assert.Nil(err)
		setRequest(t, ctx, "chat.completions", req)
		aiCtx, err := aicontext.New(ctx, providerSpec)
		assert.Nil(err)
		provider.Handle(aiCtx)

		resp := aiCtx.GetResponse()
		assert.Equal(200, resp.StatusCode)

		data, err := io.ReadAll(resp.BodyReader)
		assert.Nil(err)
		metrics := aiCtx.ParseMetricFn(&aicontext.FinishContext{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			RespBody:   data,
			Duration:   100,
		})
		assert.NotNil(metrics)
		assert.True(metrics.Success)
		assert.Equal("openai", metrics.Provider)
		assert.Equal("openai", metrics.ProviderType)
		assert.Equal("gpt-5", metrics.Model)
		assert.Equal("/v1/chat/completions", metrics.ResponseType)
		assert.Equal(int64(len(tokens)), metrics.InputTokens)
		assert.Equal(int64(len(tokens)), metrics.OutputTokens)

		for _, cb := range aiCtx.Callbacks() {
			fc := &aicontext.FinishContext{
				StatusCode: resp.StatusCode,
				Header:     resp.Header,
				RespBody:   data,
				Duration:   100,
			}
			cb(fc)
		}
	}
}

func setRequest(t *testing.T, ctx *context.Context, ns string, req *http.Request) {
	httpreq, err := httpprot.NewRequest(req)
	httpreq.FetchPayload(0)
	assert.Nil(t, err)
	ctx.SetRequest(ns, httpreq)
	ctx.UseNamespace(ns)
}

func createChatCompletionRequest(model string, stream bool, content string) (*http.Request, error) {
	reqBody := map[string]any{
		"model":  model,
		"stream": stream,
		"messages": []map[string]string{
			{"role": "user", "content": content},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func chatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type ChatCompletionRequest struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
		Stream   bool      `json:"stream"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON
	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	content := req.Messages[len(req.Messages)-1].Content

	if req.Stream {
		chunks := getStreamBody(req.Model, content)
		w.Header().Set("Content-Type", "text/event-stream")

		for _, chunk := range chunks {
			w.Write([]byte("data: "))
			data, err := json.Marshal(chunk)
			if err != nil {
				break
			}
			w.Write(data)
			w.Write([]byte("\n\n"))
			w.(http.Flusher).Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		w.(http.Flusher).Flush()
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := getNonStreamBody(req.Model, content)
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func getNonStreamBody(model string, content string) any {
	tokens := strings.Split(content, " ")
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
					"content": content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     len(tokens),
			"completion_tokens": len(tokens),
			"total_tokens":      len(tokens) * 2, // Assuming prompt and completion tokens are equal
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

func getStreamBody(model string, content string) []any {
	id := fmt.Sprintf("chatcmpl-%d", time.Now().Unix())
	tokens := strings.Split(content, " ")

	res := []any{
		map[string]any{
			"id":                 id,
			"object":             "chat.completion.chunk",
			"created":            time.Now().Unix(),
			"model":              model,
			"system_fingerprint": "fp_abc123",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{"role": "assistant", "content": ""},
				},
			},
		},
	}
	for _, token := range tokens {
		res = append(res, map[string]any{
			"id":                 id,
			"object":             "chat.completion.chunk",
			"created":            time.Now().Unix(),
			"model":              model,
			"system_fingerprint": "fp_abc123",
			"choices": []any{
				map[string]any{
					"index":         0,
					"delta":         map[string]any{"content": token},
					"logprobs":      nil,
					"finish_reason": nil,
				},
			},
		})
	}
	res = append(res, map[string]any{
		"id":                 id,
		"object":             "chat.completion.chunk",
		"created":            time.Now().Unix(),
		"model":              model,
		"system_fingerprint": "fp_abc123",
		"choices": []any{
			map[string]any{"index": 0, "delta": map[string]any{}, "logprobs": nil, "finish_reason": "stop"},
		},
		"usage": map[string]any{
			"prompt_tokens":     len(tokens),
			"completion_tokens": len(tokens),
			"total_tokens":      len(tokens) * 2, // Assuming prompt and completion
		},
	})
	return res

}
