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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

type openAIStreamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		Input  int `json:"prompt_tokens"`
		Output int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicStreamBlock struct {
	index     int
	id        string
	name      string
	arguments strings.Builder
	started   bool
	closed    bool
}

// anthropicStream converts SSE events as the client reads them. Keeping conversion
// in Read avoids buffering the full response or leaving a writer goroutine behind
// when the client disconnects. The provider owns and closes the upstream body.
type anthropicStream struct {
	source       *bufio.Reader
	pending      bytes.Buffer
	model        string
	started      bool
	finishReason string
	inputTokens  int
	outputTokens int
	blocks       []*anthropicStreamBlock
	text         *anthropicStreamBlock
	tools        map[int]*anthropicStreamBlock
	err          error
}

func newAnthropicStream(source io.Reader, model string) *anthropicStream {
	return &anthropicStream{source: bufio.NewReader(source), model: model, tools: make(map[int]*anthropicStreamBlock)}
}

func (s *anthropicStream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for s.pending.Len() == 0 && s.err == nil {
		data, err := s.readEvent()
		if err != nil {
			if err == io.EOF && s.finishReason != "" {
				s.finish()
			} else {
				s.fail(err)
			}
			break
		}
		if data == "[DONE]" {
			if s.finishReason == "" {
				s.fail(io.ErrUnexpectedEOF)
			} else {
				s.finish()
			}
			break
		}
		if err := s.convert(data); err != nil {
			s.fail(err)
		}
	}
	if s.pending.Len() > 0 {
		return s.pending.Read(p)
	}
	return 0, s.err
}

// readEvent accepts SSE comments, named events, CRLF and multiline data without
// imposing bufio.Scanner's token size limit on large tool argument fragments.
func (s *anthropicStream) readEvent() (string, error) {
	var data []string
	for {
		line, err := s.source.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
		if (line == "" || err != nil) && len(data) > 0 {
			return strings.Join(data, "\n"), nil
		}
		if err != nil {
			return "", err
		}
	}
}

func (s *anthropicStream) emit(event map[string]interface{}) {
	fmt.Fprintf(&s.pending, "event: %s\ndata: ", event["type"])
	// Events contain only JSON values decoded from the upstream or built here.
	_ = json.NewEncoder(&s.pending).Encode(event)
	s.pending.WriteByte('\n')
}

func (s *anthropicStream) convert(data string) error {
	var chunk openAIStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return fmt.Errorf("invalid upstream stream: %w", err)
	}
	if chunk.Error != nil {
		return fmt.Errorf("upstream stream error: %s", chunk.Error.Message)
	}
	if chunk.Usage != nil {
		s.inputTokens = chunk.Usage.Input
		s.outputTokens = chunk.Usage.Output
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	choice := chunk.Choices[0]
	if choice.Index != 0 {
		return nil
	}
	if !s.started {
		id := chunk.ID
		if id == "" {
			id = "msg_" + uuid.NewString()
		}
		s.emit(map[string]interface{}{"type": EventMessageStart, "message": map[string]interface{}{
			"id": id, "type": "message", "role": RoleAssistant, "model": s.model, "content": []interface{}{},
			"stop_reason": nil, "stop_sequence": nil, "usage": map[string]int{"input_tokens": s.inputTokens, "output_tokens": 0},
		}})
		s.started = true
	}
	if s.finishReason != "" {
		return nil
	}
	if choice.Delta.Content != "" {
		if s.text == nil {
			s.text = s.newBlock()
			s.text.started = true
			s.emit(map[string]interface{}{"type": EventContentBlockStart, "index": s.text.index, "content_block": map[string]interface{}{"type": ContentText, "text": ""}})
		}
		s.emit(map[string]interface{}{"type": EventContentBlockDelta, "index": s.text.index, "delta": map[string]interface{}{"type": "text_delta", "text": choice.Delta.Content}})
	}
	for _, call := range choice.Delta.ToolCalls {
		if s.text != nil {
			s.closeBlock(s.text)
			s.text = nil
		}
		tool := s.tools[call.Index]
		if tool == nil {
			tool = s.newBlock()
			s.tools[call.Index] = tool
		}
		if call.ID != "" {
			if tool.started && call.ID != tool.id {
				return fmt.Errorf("upstream changed tool call ID")
			}
			tool.id = call.ID
		}
		if call.Function.Name != "" && call.Function.Name != tool.name {
			if tool.started {
				return fmt.Errorf("upstream changed tool call name")
			}
			tool.name += call.Function.Name
		}
		tool.arguments.WriteString(call.Function.Arguments)
		if tool.name != "" && tool.id != "" {
			s.startTool(tool)
		}
		if tool.started && tool.arguments.Len() > 0 {
			s.emit(map[string]interface{}{"type": EventContentBlockDelta, "index": tool.index, "delta": map[string]interface{}{"type": "input_json_delta", "partial_json": tool.arguments.String()}})
			tool.arguments.Reset()
		}
	}
	if choice.FinishReason != "" {
		s.finishReason = choice.FinishReason
		// Close content now, but wait for the optional usage-only chunk before
		// sending message_delta and message_stop.
		return s.closeBlocks()
	}
	return nil
}

func (s *anthropicStream) newBlock() *anthropicStreamBlock {
	block := &anthropicStreamBlock{index: len(s.blocks)}
	s.blocks = append(s.blocks, block)
	return block
}

func (s *anthropicStream) startTool(tool *anthropicStreamBlock) {
	if tool.started {
		return
	}
	if tool.id == "" {
		tool.id = "tool_" + uuid.NewString()
	}
	tool.started = true
	s.emit(map[string]interface{}{"type": EventContentBlockStart, "index": tool.index, "content_block": map[string]interface{}{"type": ContentToolUse, "id": tool.id, "name": tool.name, "input": map[string]interface{}{}}})
}

func (s *anthropicStream) closeBlock(block *anthropicStreamBlock) {
	if block.closed {
		return
	}
	s.emit(map[string]interface{}{"type": EventContentBlockStop, "index": block.index})
	block.closed = true
}

func (s *anthropicStream) closeBlocks() error {
	for _, block := range s.blocks {
		if block.closed {
			continue
		}
		if !block.started {
			if block.name == "" {
				return fmt.Errorf("upstream tool call has no name")
			}
			s.startTool(block)
			if block.arguments.Len() > 0 {
				s.emit(map[string]interface{}{"type": EventContentBlockDelta, "index": block.index, "delta": map[string]interface{}{"type": "input_json_delta", "partial_json": block.arguments.String()}})
				block.arguments.Reset()
			}
		}
		s.closeBlock(block)
	}
	return nil
}

func (s *anthropicStream) finish() {
	s.emit(map[string]interface{}{"type": EventMessageDelta, "delta": map[string]interface{}{"stop_reason": anthropicStopReason(s.finishReason, len(s.tools) > 0), "stop_sequence": nil}, "usage": map[string]int{"input_tokens": s.inputTokens, "output_tokens": s.outputTokens}})
	s.emit(map[string]interface{}{"type": EventMessageStop})
	s.err = io.EOF
}

func (s *anthropicStream) fail(err error) {
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	s.emit(map[string]interface{}{"type": "error", "error": map[string]string{"type": "api_error", "message": err.Error()}})
	s.err = err
}
