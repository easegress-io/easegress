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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var globalLatency = 0 * time.Millisecond

func init() {
	val := os.Getenv("EG_LLM_TEST_LATENCY")
	if val == "" {
		return
	}
	ms, err := time.ParseDuration(val)
	if err != nil {
		panic(fmt.Errorf("invalid EG_LLM_TEST_LATENCY value: %s, should be a valid duration", val))
	}
	globalLatency = ms
}

type ChatCompletionRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func getNonStreamBody(req *ChatCompletionRequest) any {
	return map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   req.Model,
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

func getStreamBody(req *ChatCompletionRequest) []any {
	id := fmt.Sprintf("chatcmpl-%d", time.Now().Unix())

	contexts := strings.Split("Hello! How can I assist you today?", " ")
	res := []any{
		map[string]any{
			"id":                 id,
			"object":             "chat.completion.chunk",
			"created":            time.Now().Unix(),
			"model":              req.Model,
			"system_fingerprint": "fp_abc123",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{"role": "assistant", "content": ""},
				},
			},
		},
	}
	for _, text := range contexts {
		res = append(res, map[string]any{
			"id":                 id,
			"object":             "chat.completion.chunk",
			"created":            time.Now().Unix(),
			"model":              req.Model,
			"system_fingerprint": "fp_abc123",
			"choices": []any{
				map[string]any{
					"index":         0,
					"delta":         map[string]any{"content": text},
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
		"model":              req.Model,
		"system_fingerprint": "fp_abc123",
		"choices": []any{
			map[string]any{"index": 0, "delta": map[string]any{}, "logprobs": nil, "finish_reason": "stop"},
		},
	})
	return res

}

func chatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received request: %+v\n", r)

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
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

	if req.Stream {
		chunks := getStreamBody(&req)
		latency := globalLatency / time.Duration(len(chunks))

		w.Header().Set("Content-Type", "text/event-stream")

		for _, chunk := range chunks {
			if latency > 0 {
				time.Sleep(latency)
			}

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

	if globalLatency > 0 {
		time.Sleep(globalLatency)
	}
	w.Header().Set("Content-Type", "application/json")
	resp := getNonStreamBody(&req)
	json.NewEncoder(w).Encode(resp)
}

func main() {
	http.HandleFunc("/v1/chat/completions", chatCompletionsHandler)
	fmt.Println("Server is running on port 19999")
	http.ListenAndServe(":19999", nil)
}
