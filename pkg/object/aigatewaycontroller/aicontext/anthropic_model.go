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
	"encoding/json"
	"fmt"
)

// ClaudeContentBlockText represents a text content block
type ClaudeContentBlockText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ClaudeContentBlockImage represents an image content block
type ClaudeContentBlockImage struct {
	Type   string                 `json:"type"`
	Source map[string]interface{} `json:"source"`
}

// ClaudeContentBlockToolUse represents a tool use content block
type ClaudeContentBlockToolUse struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ClaudeContentBlockToolResult represents a tool result content block
type ClaudeContentBlockToolResult struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"` // Can be string, []map[string]interface{}, or map[string]interface{}
}

// ClaudeContentBlock represents any content block (union type)
type ClaudeContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	Source    map[string]interface{} `json:"source,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   interface{}            `json:"content,omitempty"`
}

// ClaudeSystemContent represents system content
type ClaudeSystemContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ClaudeMessage represents a message in the conversation
type ClaudeMessage struct {
	Role    string      `json:"role"`    // "user" or "assistant"
	Content interface{} `json:"content"` // Can be string or []ClaudeContentBlock
}

// UnmarshalJSON implements custom JSON unmarshaling for ClaudeMessage
func (m *ClaudeMessage) UnmarshalJSON(data []byte) error {
	type Alias ClaudeMessage
	aux := &struct {
		Content json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Try to unmarshal content as string first
	var contentStr string
	if err := json.Unmarshal(aux.Content, &contentStr); err == nil {
		m.Content = contentStr
		return nil
	}

	// Try to unmarshal as single content block
	var contentBlock ClaudeContentBlock
	if err := json.Unmarshal(aux.Content, &contentBlock); err == nil {
		m.Content = []ClaudeContentBlock{contentBlock}
		return nil
	}

	// Try to unmarshal as array of content blocks
	var contentBlocks []ClaudeContentBlock
	if err := json.Unmarshal(aux.Content, &contentBlocks); err == nil {
		m.Content = contentBlocks
		return nil
	}

	return fmt.Errorf("invalid content format")
}

// MarshalJSON implements custom JSON marshaling for ClaudeMessage
func (m ClaudeMessage) MarshalJSON() ([]byte, error) {
	type Alias ClaudeMessage
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(&m),
	})
}

// ClaudeTool represents a tool definition
type ClaudeTool struct {
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ClaudeThinkingConfig represents thinking configuration
type ClaudeThinkingConfig struct {
	Enabled bool `json:"enabled"` // Default: true in Python
}

// NewClaudeThinkingConfig creates a new ClaudeThinkingConfig with default values
func NewClaudeThinkingConfig() *ClaudeThinkingConfig {
	return &ClaudeThinkingConfig{
		Enabled: true, // Default value from Python
	}
}

// ClaudeMessagesRequest represents the request for Claude messages API
type ClaudeMessagesRequest struct {
	Model         string                 `json:"model"`
	MaxTokens     int                    `json:"max_tokens"`
	Messages      []ClaudeMessage        `json:"messages"`
	System        interface{}            `json:"system,omitempty"` // Can be string or []ClaudeSystemContent
	StopSequences []string               `json:"stop_sequences,omitempty"`
	Stream        *bool                  `json:"stream,omitempty"`      // Default: false in Python
	Temperature   *float64               `json:"temperature,omitempty"` // Default: 1.0 in Python
	TopP          *float64               `json:"top_p,omitempty"`       // Default: None in Python
	TopK          *int                   `json:"top_k,omitempty"`       // Default: None in Python
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Tools         []ClaudeTool           `json:"tools,omitempty"`
	ToolChoice    map[string]interface{} `json:"tool_choice,omitempty"`
	Thinking      *ClaudeThinkingConfig  `json:"thinking,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for ClaudeMessagesRequest
func (r *ClaudeMessagesRequest) UnmarshalJSON(data []byte) error {
	type Alias ClaudeMessagesRequest
	aux := &struct {
		System   json.RawMessage `json:"system,omitempty"`
		Messages json.RawMessage `json:"messages"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle System field - can be string or []ClaudeSystemContent
	if len(aux.System) > 0 {
		var systemStr string
		if err := json.Unmarshal(aux.System, &systemStr); err == nil {
			r.System = systemStr
		} else {
			var systemContents []ClaudeSystemContent
			if err := json.Unmarshal(aux.System, &systemContents); err == nil {
				r.System = systemContents
			} else {
				return fmt.Errorf("invalid system format")
			}
		}
	}

	// Handle Messages field with content normalization
	var rawMessages []json.RawMessage
	if err := json.Unmarshal(aux.Messages, &rawMessages); err != nil {
		return err
	}

	r.Messages = make([]ClaudeMessage, len(rawMessages))
	for i, rawMsg := range rawMessages {
		// Use the custom unmarshaling for ClaudeMessage
		if err := json.Unmarshal(rawMsg, &r.Messages[i]); err != nil {
			return err
		}
	}

	// Set default values for fields that weren't provided
	if r.Temperature == nil {
		temp := 1.0 // Default temperature from Python
		r.Temperature = &temp
	}

	if r.Stream == nil {
		stream := false // Default stream from Python
		r.Stream = &stream
	}

	if r.Thinking == nil {
		r.Thinking = NewClaudeThinkingConfig() // Default thinking config
	}

	return nil
}

// ClaudeTokenCountRequest represents the request for Claude token count API
type ClaudeTokenCountRequest struct {
	Model      string                 `json:"model"`
	Messages   []ClaudeMessage        `json:"messages"`
	System     interface{}            `json:"system,omitempty"` // Can be string or []ClaudeSystemContent
	Tools      []ClaudeTool           `json:"tools,omitempty"`
	Thinking   *ClaudeThinkingConfig  `json:"thinking,omitempty"`
	ToolChoice map[string]interface{} `json:"tool_choice,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for ClaudeTokenCountRequest
func (r *ClaudeTokenCountRequest) UnmarshalJSON(data []byte) error {
	type Alias ClaudeTokenCountRequest
	aux := &struct {
		System   json.RawMessage `json:"system,omitempty"`
		Messages json.RawMessage `json:"messages"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle System field - can be string or []ClaudeSystemContent
	if len(aux.System) > 0 {
		var systemStr string
		if err := json.Unmarshal(aux.System, &systemStr); err == nil {
			r.System = systemStr
		} else {
			var systemContents []ClaudeSystemContent
			if err := json.Unmarshal(aux.System, &systemContents); err == nil {
				r.System = systemContents
			} else {
				return fmt.Errorf("invalid system format")
			}
		}
	}

	// Handle Messages field with content normalization
	var rawMessages []json.RawMessage
	if err := json.Unmarshal(aux.Messages, &rawMessages); err != nil {
		return err
	}

	r.Messages = make([]ClaudeMessage, len(rawMessages))
	for i, rawMsg := range rawMessages {
		// Use the custom unmarshaling for ClaudeMessage
		if err := json.Unmarshal(rawMsg, &r.Messages[i]); err != nil {
			return err
		}
	}

	return nil
}

// Helper methods for ClaudeMessage

// GetContentAsString returns the content as a string if it's a string type
func (m *ClaudeMessage) GetContentAsString() (string, bool) {
	if str, ok := m.Content.(string); ok {
		return str, true
	}
	return "", false
}

// GetContentAsBlocks returns the content as content blocks if it's an array type
func (m *ClaudeMessage) GetContentAsBlocks() ([]ClaudeContentBlock, bool) {
	if blocks, ok := m.Content.([]ClaudeContentBlock); ok {
		return blocks, true
	}
	return nil, false
}

// SetContentAsString sets the content as a string
func (m *ClaudeMessage) SetContentAsString(content string) {
	m.Content = content
}

// SetContentAsBlocks sets the content as content blocks
func (m *ClaudeMessage) SetContentAsBlocks(blocks []ClaudeContentBlock) {
	m.Content = blocks
}

// Helper methods for ClaudeMessagesRequest

// GetSystemAsString returns the system as a string if it's a string type
func (r *ClaudeMessagesRequest) GetSystemAsString() (string, bool) {
	if str, ok := r.System.(string); ok {
		return str, true
	}
	return "", false
}

// GetSystemAsContents returns the system as content blocks if it's an array type
func (r *ClaudeMessagesRequest) GetSystemAsContents() ([]ClaudeSystemContent, bool) {
	if contents, ok := r.System.([]ClaudeSystemContent); ok {
		return contents, true
	}
	return nil, false
}

// SetSystemAsString sets the system as a string
func (r *ClaudeMessagesRequest) SetSystemAsString(system string) {
	r.System = system
}

// SetSystemAsContents sets the system as content blocks
func (r *ClaudeMessagesRequest) SetSystemAsContents(contents []ClaudeSystemContent) {
	r.System = contents
}

// GetTemperature returns the temperature value with default (1.0)
func (r *ClaudeMessagesRequest) GetTemperature() float64 {
	if r.Temperature != nil {
		return *r.Temperature
	}
	return 1.0 // Default value from Python
}

// SetTemperature sets the temperature value
func (r *ClaudeMessagesRequest) SetTemperature(temp float64) {
	r.Temperature = &temp
}

// GetStream returns the stream value with default (false)
func (r *ClaudeMessagesRequest) GetStream() bool {
	if r.Stream != nil {
		return *r.Stream
	}
	return false // Default value from Python
}

// SetStream sets the stream value
func (r *ClaudeMessagesRequest) SetStream(stream bool) {
	r.Stream = &stream
}

// GetTopP returns the top_p value (no default, can be nil)
func (r *ClaudeMessagesRequest) GetTopP() *float64 {
	return r.TopP
}

// SetTopP sets the top_p value
func (r *ClaudeMessagesRequest) SetTopP(topP float64) {
	r.TopP = &topP
}

// GetTopK returns the top_k value (no default, can be nil)
func (r *ClaudeMessagesRequest) GetTopK() *int {
	return r.TopK
}

// SetTopK sets the top_k value
func (r *ClaudeMessagesRequest) SetTopK(topK int) {
	r.TopK = &topK
}

// NewClaudeMessagesRequest creates a new ClaudeMessagesRequest with default values
func NewClaudeMessagesRequest(model string, maxTokens int, messages []ClaudeMessage) *ClaudeMessagesRequest {
	temp := 1.0     // Default temperature
	stream := false // Default stream
	return &ClaudeMessagesRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		Messages:    messages,
		Temperature: &temp,
		Stream:      &stream,
	}
}

// Helper methods for ClaudeTokenCountRequest

// GetSystemAsString returns the system as a string if it's a string type
func (r *ClaudeTokenCountRequest) GetSystemAsString() (string, bool) {
	if str, ok := r.System.(string); ok {
		return str, true
	}
	return "", false
}

// GetSystemAsContents returns the system as content blocks if it's an array type
func (r *ClaudeTokenCountRequest) GetSystemAsContents() ([]ClaudeSystemContent, bool) {
	if contents, ok := r.System.([]ClaudeSystemContent); ok {
		return contents, true
	}
	return nil, false
}

// SetSystemAsString sets the system as a string
func (r *ClaudeTokenCountRequest) SetSystemAsString(system string) {
	r.System = system
}

// SetSystemAsContents sets the system as content blocks
func (r *ClaudeTokenCountRequest) SetSystemAsContents(contents []ClaudeSystemContent) {
	r.System = contents
}

// NewClaudeTokenCountRequest creates a new ClaudeTokenCountRequest
func NewClaudeTokenCountRequest(model string, messages []ClaudeMessage) *ClaudeTokenCountRequest {
	return &ClaudeTokenCountRequest{
		Model:    model,
		Messages: messages,
	}
}
