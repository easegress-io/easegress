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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/stretchr/testify/require"
)

func streamChunk(delta map[string]interface{}, finish string) string {
	data, _ := json.Marshal(map[string]interface{}{"id": "chat_1", "choices": []interface{}{map[string]interface{}{"index": 0, "delta": delta, "finish_reason": finish}}})
	return "data: " + string(data) + "\n\n"
}

func toolDelta(index int, id, name, args string) map[string]interface{} {
	return map[string]interface{}{"index": index, "id": id, "type": "function", "function": map[string]interface{}{"name": name, "arguments": args}}
}

func decodeAnthropicEvents(t *testing.T, body string) []map[string]interface{} {
	t.Helper()
	var events []map[string]interface{}
	for _, frame := range strings.Split(body, "\n\n") {
		if frame == "" {
			continue
		}
		parts := strings.SplitN(frame, "\ndata: ", 2)
		require.Len(t, parts, 2)
		var event map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(parts[1]), &event))
		require.Equal(t, "event: "+event["type"].(string), parts[0])
		events = append(events, event)
	}
	return events
}

func TestAnthropicStreamingTools(t *testing.T) {
	for _, reason := range []string{"stop", "tool_calls", "length"} {
		t.Run(reason, func(t *testing.T) {
			// Include content with the initial role, repeated roles, parallel calls,
			// arguments split across chunks, and usage after the finish chunk.
			input := streamChunk(map[string]interface{}{"role": "assistant", "content": "Checking "}, "") +
				streamChunk(map[string]interface{}{"role": "assistant", "content": "weather."}, "") +
				streamChunk(map[string]interface{}{"role": "assistant", "tool_calls": []interface{}{
					toolDelta(0, "call_1", "get_weather", `{"city":`), toolDelta(1, "call_2", "get_time", `{"zone":`),
				}}, "") +
				streamChunk(map[string]interface{}{"tool_calls": []interface{}{
					toolDelta(1, "", "", `"UTC"}`), toolDelta(0, "", "", `"Paris"}`),
				}}, "") +
				streamChunk(map[string]interface{}{"role": "assistant"}, reason) +
				"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":10}}\n\ndata: [DONE]\n\n"
			ctx := &Context{RespType: ResponseTypeMessage, ReqInfo: &protocol.GeneralRequest{Stream: true, Model: "upstream-model"}, ClaudeMessagesRequest: &ClaudeMessagesRequest{Model: "client-model"}}
			ctx.SetResponse(&Response{StatusCode: 200, Header: http.Header{"Content-Length": []string{"999"}}, BodyReader: strings.NewReader(input)})
			response := ctx.GetResponse()
			require.Equal(t, int64(-1), response.ContentLength)
			require.Empty(t, response.Header.Get("Content-Length"))
			require.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))
			body, err := io.ReadAll(response.BodyReader)
			require.NoError(t, err)
			require.NotContains(t, string(body), "[DONE]")
			events := decodeAnthropicEvents(t, string(body))
			require.Equal(t, EventMessageStart, events[0]["type"])
			require.Equal(t, "client-model", events[0]["message"].(map[string]interface{})["model"])
			require.Equal(t, EventMessageStop, events[len(events)-1]["type"])
			starts, stops := 0, 0
			blocks := map[int]map[string]interface{}{}
			arguments := map[int]string{}
			closed := map[int]bool{}
			text := ""
			for _, event := range events {
				switch event["type"] {
				case EventMessageStart:
					starts++
				case EventMessageStop:
					stops++
				case EventContentBlockStart:
					index := int(event["index"].(float64))
					require.NotContains(t, blocks, index)
					require.Equal(t, len(blocks), index)
					blocks[index] = event["content_block"].(map[string]interface{})
				case EventContentBlockDelta:
					index := int(event["index"].(float64))
					require.Contains(t, blocks, index)
					require.False(t, closed[index])
					delta := event["delta"].(map[string]interface{})
					if delta["type"] == "text_delta" {
						text += delta["text"].(string)
					} else {
						arguments[index] += delta["partial_json"].(string)
					}
				case EventContentBlockStop:
					index := int(event["index"].(float64))
					require.Contains(t, blocks, index)
					require.False(t, closed[index])
					closed[index] = true
				}
			}
			require.Equal(t, 1, starts)
			require.Equal(t, 1, stops)
			require.Len(t, closed, 3)
			require.Equal(t, "Checking weather.", text)
			require.Equal(t, "call_1", blocks[1]["id"])
			require.Equal(t, "get_weather", blocks[1]["name"])
			require.JSONEq(t, `{"city":"Paris"}`, arguments[1])
			require.JSONEq(t, `{"zone":"UTC"}`, arguments[2])
			final := events[len(events)-2]
			expected := StopToolUse
			if reason == "length" {
				expected = StopMaxTokens
			}
			require.Equal(t, expected, final["delta"].(map[string]interface{})["stop_reason"])
			require.Equal(t, float64(5), final["usage"].(map[string]interface{})["input_tokens"])
			require.Equal(t, float64(10), final["usage"].(map[string]interface{})["output_tokens"])
		})
	}
}

func TestAnthropicStreamEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		wantError   bool
		stop        string
	}{
		{"text with finish in same chunk", streamChunk(map[string]interface{}{"role": "assistant", "content": "hello"}, "stop"), false, StopEndTurn},
		{"empty text", streamChunk(map[string]interface{}{"role": "assistant"}, "stop") + "data:[DONE]\n\n", false, StopEndTurn},
		{"large argument fragment", streamChunk(map[string]interface{}{"tool_calls": []interface{}{toolDelta(0, "call_1", "echo", `{"text":"`+strings.Repeat("中", 30000)+`"}`)}}, "stop"), false, StopToolUse},
		{"no arguments", streamChunk(map[string]interface{}{"tool_calls": []interface{}{toolDelta(0, "call_1", "now", "")}}, "stop"), false, StopToolUse},
		{"missing id", streamChunk(map[string]interface{}{"tool_calls": []interface{}{toolDelta(0, "", "now", "{}")}}, "stop"), false, StopToolUse},
		{"malformed JSON", "data: {broken}\n\n", true, ""},
		{"upstream error", "data: {\"error\":{\"message\":\"rate limited\"}}\n\n", true, ""},
		{"truncated stream", streamChunk(map[string]interface{}{"role": "assistant", "content": "partial"}, ""), true, ""},
		{"done without finish", streamChunk(map[string]interface{}{"role": "assistant"}, "") + "data: [DONE]\n\n", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := io.ReadAll(newAnthropicStream(strings.NewReader(tc.input), "model"))
			events := decodeAnthropicEvents(t, string(body))
			if tc.wantError {
				require.Error(t, err)
				require.Equal(t, "error", events[len(events)-1]["type"])
				return
			}
			require.NoError(t, err)
			require.Equal(t, EventMessageStop, events[len(events)-1]["type"])
			final := events[len(events)-2]["delta"].(map[string]interface{})
			require.Equal(t, tc.stop, final["stop_reason"])
		})
	}
}

func TestAnthropicSSEFraming(t *testing.T) {
	input := ": heartbeat\r\nevent: completion\r\nid: 1\r\ndata: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\r\ndata: \"finish_reason\":\"stop\"}]}\r\n\r\ndata:[DONE]"
	body, err := io.ReadAll(newAnthropicStream(strings.NewReader(input), "model"))
	require.NoError(t, err)
	events := decodeAnthropicEvents(t, string(body))
	require.Equal(t, EventMessageStop, events[len(events)-1]["type"])
	require.NotContains(t, string(body), "heartbeat")
}

func TestAnthropicRequestPreservesStream(t *testing.T) {
	stream := true
	request := &ClaudeMessagesRequest{Model: "gpt-4o", MaxTokens: 100, Stream: &stream, Messages: []ClaudeMessage{{Role: RoleUser, Content: "hello"}}}
	converted, err := ConvertClaudeToOpenAI(request, GlobalModelManager, GlobalAnthropicConfig)
	require.NoError(t, err)
	require.Equal(t, true, converted["stream"])
	require.Equal(t, map[string]bool{"include_usage": true}, converted["stream_options"])
}

func TestAnthropicToolStopReason(t *testing.T) {
	for _, reason := range []string{"stop", "tool_calls", "length", ""} {
		t.Run(fmt.Sprintf("reason_%s", reason), func(t *testing.T) {
			input := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":"checking","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{}"}}]},"finish_reason":%q}]}`, reason)
			var response map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(input), &response))
			converted, err := ConvertOpenAIToClaudeResponse(response, &ClaudeMessagesRequest{Model: "model"})
			require.NoError(t, err)
			expected := StopToolUse
			if reason == "length" {
				expected = StopMaxTokens
			}
			require.Equal(t, expected, converted["stop_reason"])
		})
	}
}

func TestAnthropicErrorResponseUnchanged(t *testing.T) {
	for _, stream := range []bool{false, true} {
		ctx := &Context{RespType: ResponseTypeMessage, ReqInfo: &protocol.GeneralRequest{Stream: stream}}
		body := []byte(`{"error":{"message":"bad key"}}`)
		ctx.SetResponse(&Response{StatusCode: 401, Header: make(http.Header), BodyBytes: body})
		require.True(t, bytes.Equal(body, ctx.GetResponse().BodyBytes))
		require.Nil(t, ctx.GetResponse().BodyReader)
	}
}

// finalReadError models a transport returning its last bytes and a read error
// together, followed by EOF on subsequent reads.
type finalReadError struct{ data string }

func (r *finalReadError) Read(p []byte) (int, error) {
	if r.data == "" {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	if r.data == "" {
		return n, errors.New("upstream connection reset")
	}
	return n, nil
}

func TestAnthropicStreamReadError(t *testing.T) {
	input := strings.TrimRight(streamChunk(map[string]interface{}{"content": "partial"}, "stop"), "\n")
	body, err := io.ReadAll(newAnthropicStream(&finalReadError{data: input}, "model"))
	require.ErrorContains(t, err, "upstream connection reset")
	require.NotContains(t, string(body), "event: message_stop")
	require.Contains(t, string(body), "event: error")
}
