//go:build integration
// +build integration

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

package test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayAnthropicTools(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"data":[]}`)
			return
		}
		var request struct {
			Stream   bool `json:"stream"`
			Messages []struct {
				Role       string `json:"role"`
				Content    string `json:"content"`
				ToolCallID string `json:"tool_call_id"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if len(request.Messages) == 0 {
			t.Error("no upstream messages")
			w.WriteHeader(400)
			return
		}
		last := request.Messages[len(request.Messages)-1]
		if last.Role == "tool" {
			if last.ToolCallID != "call_weather" || last.Content != "sunny" {
				t.Error("tool result was not forwarded")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"chat_2","choices":[{"message":{"role":"assistant","content":"It is sunny."},"finish_reason":"stop"}]}`)
			return
		}
		if !request.Stream {
			reason := "stop"
			if last.Content == "length" {
				reason = "length"
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"id":"chat_1","choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_weather","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}]},"finish_reason":%q}]}`, reason)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"chat_1\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"Checking.\",\"tool_calls\":[{\"index\":0,\"id\":\"call_weather\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\"}}]}}]}\n\n")
		w.(http.Flusher).Flush()
		if last.Content == "cancel" {
			<-r.Context().Done()
			close(canceled)
			return
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"Paris\\\"}\"}}]},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":10}}\n\ndata: [DONE]\n\n")
	}))
	defer func() {
		unblock()
		upstream.Close()
	}()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	resources := []struct{ kind, name, yaml string }{
		{"AIGatewayController", "ai-tools-controller", fmt.Sprintf("kind: AIGatewayController\nname: ai-tools-controller\nproviders:\n- name: mock\n  providerType: openai\n  baseURL: %s\n  apiKey: test-only\n", upstream.URL)},
		{"Pipeline", "ai-tools-pipeline", "kind: Pipeline\nname: ai-tools-pipeline\nflow:\n- filter: ai\nfilters:\n- name: ai\n  kind: AIGatewayProxy\n  providerName: mock\n"},
		{"HTTPServer", "ai-tools-server", fmt.Sprintf("kind: HTTPServer\nname: ai-tools-server\nport: %d\nrules:\n- paths:\n  - pathPrefix: /\n    backend: ai-tools-pipeline\n", port)},
	}
	for _, resource := range resources {
		require.NoError(t, createResource(resource.yaml))
		kind, name := resource.kind, resource.name
		t.Cleanup(func() { require.NoError(t, deleteResource(kind, name)) })
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	require.Eventually(t, func() bool {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return false
		}
		c.Close()
		return true
	}, 5*time.Second, 20*time.Millisecond)
	client := &http.Client{Timeout: 5 * time.Second}
	send := func(body string) *http.Response {
		response, err := client.Post("http://"+addr+"/v1/messages", "application/json", strings.NewReader(body))
		require.NoError(t, err)
		require.Equal(t, 200, response.StatusCode)
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	tools := `"tools":[{"name":"get_weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}]`
	for _, tc := range []struct{ prompt, reason string }{{"weather", "tool_use"}, {"length", "max_tokens"}} {
		response := send(fmt.Sprintf(`{"model":"gpt-4o","max_tokens":100,"messages":[{"role":"user","content":%q}],%s}`, tc.prompt, tools))
		var message struct {
			StopReason string `json:"stop_reason"`
			Content    []struct {
				ID, Name, Type string
				Input          map[string]string
			}
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&message))
		response.Body.Close()
		require.Equal(t, tc.reason, message.StopReason)
		require.Len(t, message.Content, 1)
		require.Equal(t, "tool_use", message.Content[0].Type)
		require.Equal(t, "call_weather", message.Content[0].ID)
		require.Equal(t, "Paris", message.Content[0].Input["city"])
	}
	response := send(fmt.Sprintf(`{"model":"gpt-4o","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"weather"}],%s}`, tools))
	require.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))
	require.Empty(t, response.Header.Get("Content-Length"))
	scanner := bufio.NewScanner(response.Body)
	var events []map[string]interface{}
	arguments := ""
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event))
		events = append(events, event)
		if event["type"] == "content_block_delta" {
			delta := event["delta"].(map[string]interface{})
			if delta["type"] == "input_json_delta" {
				arguments += delta["partial_json"].(string)
				unblock()
			}
		}
	}
	require.NoError(t, scanner.Err())
	response.Body.Close()
	require.JSONEq(t, `{"city":"Paris"}`, arguments)
	require.Equal(t, "message_start", events[0]["type"])
	require.Equal(t, "message_stop", events[len(events)-1]["type"])
	final := events[len(events)-2]
	require.Equal(t, "tool_use", final["delta"].(map[string]interface{})["stop_reason"])
	require.Equal(t, float64(10), final["usage"].(map[string]interface{})["output_tokens"])
	starts := 0
	for _, event := range events {
		if event["type"] == "message_start" {
			starts++
		}
	}
	require.Equal(t, 1, starts)
	response = send(`{"model":"gpt-4o","max_tokens":100,"messages":[{"role":"user","content":"weather"},{"role":"assistant","content":[{"type":"tool_use","id":"call_weather","name":"get_weather","input":{"city":"Paris"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_weather","content":"sunny"}]}]}`)
	var finalMessage struct {
		StopReason string `json:"stop_reason"`
		Content    []struct{ Text string }
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&finalMessage))
	response.Body.Close()
	require.Equal(t, "end_turn", finalMessage.StopReason)
	require.Equal(t, "It is sunny.", finalMessage.Content[0].Text)
	response = send(`{"model":"gpt-4o","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"cancel"}]}`)
	response.Body.Close()
	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream request was not canceled after client disconnect")
	}
}
