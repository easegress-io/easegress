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
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/stretchr/testify/assert"
)

func TestAnthropicReqMessageToOpenAI(t *testing.T) {
	// Test case 1: Comprehensive golden path with all supported content types
	anthropicReq := &anthropic.MessageNewParams{
		Model: "claude-3-opus-20240229",
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewTextBlock("What is the weather in San Francisco?"),
					anthropic.NewImageBlockBase64("image/jpeg", "base64-encoded-image-data"),
				},
			},
			{
				Role: "assistant",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewToolUseBlock("toolu_1234", map[string]interface{}{"location": "San Francisco"}, "get_weather"),
				},
			},
			{
				Role: "user",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewToolResultBlock("toolu_1234", `{"temperature": "72F"}`, false),
				},
			},
			{
				Role: "assistant",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewThinkingBlock("thinking_signature", "I am thinking about the weather."),
					anthropic.NewTextBlock("The weather in San Francisco is 72F."),
				},
			},
		},
		System: []anthropic.TextBlockParam{
			{Text: "You are a helpful assistant."},
		},
		MaxTokens:     1024,
		Metadata:      anthropic.MetadataParam{UserID: anthropic.String("user-123")},
		StopSequences: []string{"\n"},
		Temperature:   anthropic.Float(0.7),
		TopP:          anthropic.Float(0.9),
		Tools: []anthropic.ToolUnionParam{
			anthropic.ToolUnionParamOfTool(
				anthropic.ToolInputSchemaParam{
					Type: "object",
					Properties: map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state, e.g. San Francisco, CA",
						},
					},
					Required: []string{"location"},
				},
				"get_weather",
			),
		},
		// Don't set ToolChoice to test default behavior
	}

	// Print human-readable JSON input
	fmt.Println("=== TEST CASE 1: Comprehensive Golden Path ===")
	fmt.Println("--- Anthropic Request (Input) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicReq, "", "  ")
	fmt.Println(string(anthropicJSON))

	// Convert the request
	openaiReq, err := AnthropicReqMessageToOpenAI(anthropicReq)
	assert.NoError(t, err)
	assert.NotNil(t, openaiReq)

	// Print human-readable JSON output
	fmt.Println("\n--- OpenAI Request (Output) ---")
	openaiJSON, _ := json.MarshalIndent(openaiReq, "", "  ")
	fmt.Println(string(openaiJSON))

	// Assertions to validate the conversion
	assert.Equal(t, "claude-3-opus-20240229", openaiReq.Model)
	assert.Equal(t, 1024, openaiReq.MaxCompletionTokens)
	assert.Equal(t, float32(0.7), openaiReq.Temperature)
	assert.Equal(t, float32(0.9), openaiReq.TopP)
	assert.Equal(t, []string{"\n"}, openaiReq.Stop)
	assert.Equal(t, "user-123", openaiReq.Metadata["user_id"])

	// Validate system message
	assert.Len(t, openaiReq.Messages, 5)
	assert.Equal(t, openai.ChatMessageRoleSystem, openaiReq.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", openaiReq.Messages[0].Content)

	// Validate first user message (text + image)
	userMsg1 := openaiReq.Messages[1]
	assert.Equal(t, openai.ChatMessageRoleUser, userMsg1.Role)
	assert.Len(t, userMsg1.MultiContent, 2)
	assert.Equal(t, openai.ChatMessagePartTypeText, userMsg1.MultiContent[0].Type)
	assert.Equal(t, "What is the weather in San Francisco?", userMsg1.MultiContent[0].Text)
	assert.Equal(t, openai.ChatMessagePartTypeImageURL, userMsg1.MultiContent[1].Type)
	assert.Equal(t, "data:image/jpeg;base64,base64-encoded-image-data", userMsg1.MultiContent[1].ImageURL.URL)

	// Validate first assistant message (tool use)
	assistantMsg1 := openaiReq.Messages[2]
	assert.Equal(t, openai.ChatMessageRoleAssistant, assistantMsg1.Role)
	assert.Len(t, assistantMsg1.ToolCalls, 1)
	assert.Equal(t, "toolu_1234", assistantMsg1.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", assistantMsg1.ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"location":"San Francisco"}`, assistantMsg1.ToolCalls[0].Function.Arguments)

	// Validate second user message (tool result)
	userMsg2 := openaiReq.Messages[3]
	assert.Equal(t, openai.ChatMessageRoleTool, userMsg2.Role)
	assert.Equal(t, "toolu_1234", userMsg2.ToolCallID)
	assert.Equal(t, `{"temperature": "72F"}`, userMsg2.Content)

	// Validate second assistant message (thinking + text)
	assistantMsg2 := openaiReq.Messages[4]
	assert.Equal(t, openai.ChatMessageRoleAssistant, assistantMsg2.Role)
	assert.Equal(t, "[Thinking: I am thinking about the weather.]The weather in San Francisco is 72F.", assistantMsg2.Content)

	// Validate tools
	assert.Len(t, openaiReq.Tools, 1)
	assert.Equal(t, openai.ToolTypeFunction, openaiReq.Tools[0].Type)
	assert.Equal(t, "get_weather", openaiReq.Tools[0].Function.Name)
	assert.NotNil(t, openaiReq.Tools[0].Function.Parameters)
	schema, ok := openaiReq.Tools[0].Function.Parameters.(*jsonschema.Definition)
	assert.True(t, ok)
	assert.Equal(t, jsonschema.Object, schema.Type)
	assert.Equal(t, []string{"location"}, schema.Required)
	assert.Contains(t, schema.Properties, "location")

	// Validate tool choice (may be nil if not set)
	if openaiReq.ToolChoice != nil {
		toolChoice, ok := openaiReq.ToolChoice.(*openai.ToolChoice)
		if ok {
			assert.Equal(t, openai.ToolTypeFunction, toolChoice.Type)
		}
	}
}

func TestAnthropicReqMessageToOpenAI_NilInput(t *testing.T) {
	fmt.Println("\n=== TEST CASE 2: Nil Input ===")
	fmt.Println("--- Anthropic Request (Input) ---")
	fmt.Println("nil")

	result, err := AnthropicReqMessageToOpenAI(nil)
	assert.NoError(t, err)
	assert.Nil(t, result)

	fmt.Println("--- OpenAI Request (Output) ---")
	fmt.Println("nil")
}

func TestAnthropicReqMessageToOpenAI_MinimalInput(t *testing.T) {
	fmt.Println("\n=== TEST CASE 3: Minimal Input ===")

	anthropicReq := &anthropic.MessageNewParams{
		Model: "claude-3-haiku-20240307",
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewTextBlock("Hello"),
				},
			},
		},
	}

	fmt.Println("--- Anthropic Request (Input) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicReq, "", "  ")
	fmt.Println(string(anthropicJSON))

	openaiReq, err := AnthropicReqMessageToOpenAI(anthropicReq)
	assert.NoError(t, err)
	assert.NotNil(t, openaiReq)

	fmt.Println("\n--- OpenAI Request (Output) ---")
	openaiJSON, _ := json.MarshalIndent(openaiReq, "", "  ")
	fmt.Println(string(openaiJSON))

	assert.Equal(t, "claude-3-haiku-20240307", openaiReq.Model)
	assert.Len(t, openaiReq.Messages, 1)
	assert.Equal(t, openai.ChatMessageRoleUser, openaiReq.Messages[0].Role)
	assert.Equal(t, "Hello", openaiReq.Messages[0].Content)
}

func TestAnthropicReqMessageToOpenAI_ImageURL(t *testing.T) {
	fmt.Println("\n=== TEST CASE 4: Image URL ===")

	anthropicReq := &anthropic.MessageNewParams{
		Model: "claude-3-opus-20240229",
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewImageBlockBase64("image/jpeg", "base64-image-data"),
				},
			},
		},
	}

	fmt.Println("--- Anthropic Request (Input) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicReq, "", "  ")
	fmt.Println(string(anthropicJSON))

	openaiReq, err := AnthropicReqMessageToOpenAI(anthropicReq)
	assert.NoError(t, err)
	assert.NotNil(t, openaiReq)

	fmt.Println("\n--- OpenAI Request (Output) ---")
	openaiJSON, _ := json.MarshalIndent(openaiReq, "", "  ")
	fmt.Println(string(openaiJSON))

	assert.Len(t, openaiReq.Messages, 1)
	assert.Len(t, openaiReq.Messages[0].MultiContent, 1)
	assert.Equal(t, openai.ChatMessagePartTypeImageURL, openaiReq.Messages[0].MultiContent[0].Type)
	assert.Equal(t, "data:image/jpeg;base64,base64-image-data", openaiReq.Messages[0].MultiContent[0].ImageURL.URL)
}

func TestAnthropicReqMessageToOpenAI_DocumentContent(t *testing.T) {
	fmt.Println("\n=== TEST CASE 5: Document Content ===")

	anthropicReq := &anthropic.MessageNewParams{
		Model: "claude-3-opus-20240229",
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{
						Data:      "base64-pdf-data",
						MediaType: "application/pdf",
					}),
				},
			},
		},
	}

	fmt.Println("--- Anthropic Request (Input) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicReq, "", "  ")
	fmt.Println(string(anthropicJSON))

	openaiReq, err := AnthropicReqMessageToOpenAI(anthropicReq)
	assert.NoError(t, err)
	assert.NotNil(t, openaiReq)

	fmt.Println("\n--- OpenAI Request (Output) ---")
	openaiJSON, _ := json.MarshalIndent(openaiReq, "", "  ")
	fmt.Println(string(openaiJSON))

	assert.Len(t, openaiReq.Messages, 1)
	assert.Contains(t, openaiReq.Messages[0].Content, "[Document content not directly convertible]")
}

func TestAnthropicReqMessageToOpenAI_RedactedThinking(t *testing.T) {
	fmt.Println("\n=== TEST CASE 6: Redacted Thinking ===")

	anthropicReq := &anthropic.MessageNewParams{
		Model: "claude-3-opus-20240229",
		Messages: []anthropic.MessageParam{
			{
				Role: "assistant",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewRedactedThinkingBlock("Redacted thinking content"),
				},
			},
		},
	}

	fmt.Println("--- Anthropic Request (Input) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicReq, "", "  ")
	fmt.Println(string(anthropicJSON))

	openaiReq, err := AnthropicReqMessageToOpenAI(anthropicReq)
	assert.NoError(t, err)
	assert.NotNil(t, openaiReq)

	fmt.Println("\n--- OpenAI Request (Output) ---")
	openaiJSON, _ := json.MarshalIndent(openaiReq, "", "  ")
	fmt.Println(string(openaiJSON))

	assert.Len(t, openaiReq.Messages, 1)
	assert.Equal(t, "[Redacted thinking content]", openaiReq.Messages[0].Content)
}

func TestAnthropicReqMessageToOpenAI_ToolChoiceSpecific(t *testing.T) {
	fmt.Println("\n=== TEST CASE 7: Specific Tool Choice ===")

	anthropicReq := &anthropic.MessageNewParams{
		Model: "claude-3-opus-20240229",
		Messages: []anthropic.MessageParam{
			{
				Role: "user",
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewTextBlock("Use the weather tool"),
				},
			},
		},
		Tools: []anthropic.ToolUnionParam{
			anthropic.ToolUnionParamOfTool(
				anthropic.ToolInputSchemaParam{
					Type: "object",
					Properties: map[string]interface{}{
						"location": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"get_weather",
			),
		},
		ToolChoice: anthropic.ToolChoiceParamOfTool("get_weather"),
	}

	fmt.Println("--- Anthropic Request (Input) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicReq, "", "  ")
	fmt.Println(string(anthropicJSON))

	openaiReq, err := AnthropicReqMessageToOpenAI(anthropicReq)
	assert.NoError(t, err)
	assert.NotNil(t, openaiReq)

	fmt.Println("\n--- OpenAI Request (Output) ---")
	openaiJSON, _ := json.MarshalIndent(openaiReq, "", "  ")
	fmt.Println(string(openaiJSON))

	assert.NotNil(t, openaiReq.ToolChoice)
	toolChoice, ok := openaiReq.ToolChoice.(*openai.ToolChoice)
	assert.True(t, ok)
	assert.Equal(t, openai.ToolTypeFunction, toolChoice.Type)
	assert.Equal(t, "get_weather", toolChoice.Function.Name)
}

func TestOpenAIToAnthropicRespMessage(t *testing.T) {
	// Test case 1: Comprehensive OpenAI response with tool calls
	openaiResp := &openai.ChatCompletionResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1677652288,
		Model:   "gpt-3.5-turbo",
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Role: "assistant",
					ToolCalls: []openai.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: openai.FunctionCall{
								Name:      "get_weather",
								Arguments: `{"location":"San Francisco"}`,
							},
						},
					},
				},
				FinishReason: openai.FinishReasonToolCalls,
			},
		},
		Usage: openai.Usage{
			PromptTokens:     50,
			CompletionTokens: 25,
			TotalTokens:      75,
		},
	}

	fmt.Println("\n=== TEST CASE 8: OpenAI to Anthropic Response (Tool Calls) ===")
	fmt.Println("--- OpenAI Response (Input) ---")
	openaiJSON, _ := json.MarshalIndent(openaiResp, "", "  ")
	fmt.Println(string(openaiJSON))

	anthropicMsg, err := OpenAIToAnthropicRespMessage(openaiResp)
	assert.NoError(t, err)
	assert.NotNil(t, anthropicMsg)

	fmt.Println("\n--- Anthropic Message (Output) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicMsg, "", "  ")
	fmt.Println(string(anthropicJSON))

	// Validate conversion
	assert.Equal(t, "chatcmpl-123", anthropicMsg.ID)
	assert.Equal(t, anthropic.Model("gpt-3.5-turbo"), anthropicMsg.Model)
	assert.Equal(t, "assistant", string(anthropicMsg.Role))
	assert.Equal(t, anthropic.StopReasonToolUse, anthropicMsg.StopReason)
	assert.Equal(t, int64(50), anthropicMsg.Usage.InputTokens)
	assert.Equal(t, int64(25), anthropicMsg.Usage.OutputTokens)

	// Validate content blocks
	assert.Len(t, anthropicMsg.Content, 1)
	assert.Equal(t, "tool_use", anthropicMsg.Content[0].Type)
	assert.Equal(t, "call_123", anthropicMsg.Content[0].ID)
	assert.Equal(t, "get_weather", anthropicMsg.Content[0].Name)
}

func TestOpenAIToAnthropicRespMessage_TextResponse(t *testing.T) {
	// Test case 2: Simple text response
	openaiResp := &openai.ChatCompletionResponse{
		ID:      "chatcmpl-456",
		Object:  "chat.completion",
		Created: 1677652288,
		Model:   "gpt-4",
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "Hello! How can I help you today?",
				},
				FinishReason: openai.FinishReasonStop,
			},
		},
		Usage: openai.Usage{
			PromptTokens:     10,
			CompletionTokens: 8,
			TotalTokens:      18,
		},
	}

	fmt.Println("\n=== TEST CASE 9: OpenAI to Anthropic Response (Text) ===")
	fmt.Println("--- OpenAI Response (Input) ---")
	openaiJSON, _ := json.MarshalIndent(openaiResp, "", "  ")
	fmt.Println(string(openaiJSON))

	anthropicMsg, err := OpenAIToAnthropicRespMessage(openaiResp)
	assert.NoError(t, err)
	assert.NotNil(t, anthropicMsg)

	fmt.Println("\n--- Anthropic Message (Output) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicMsg, "", "  ")
	fmt.Println(string(anthropicJSON))

	// Validate conversion
	assert.Equal(t, "chatcmpl-456", anthropicMsg.ID)
	assert.Equal(t, anthropic.Model("gpt-4"), anthropicMsg.Model)
	assert.Equal(t, anthropic.StopReasonEndTurn, anthropicMsg.StopReason)

	// Validate content
	assert.Len(t, anthropicMsg.Content, 1)
	assert.Equal(t, "text", anthropicMsg.Content[0].Type)
	assert.Equal(t, "Hello! How can I help you today?", anthropicMsg.Content[0].Text)
}

func TestOpenAIToAnthropicRespMessage_ThinkingContent(t *testing.T) {
	// Test case 3: Response with thinking patterns (from our own conversion)
	openaiResp := &openai.ChatCompletionResponse{
		ID:      "chatcmpl-789",
		Object:  "chat.completion",
		Created: 1677652288,
		Model:   "gpt-4",
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "[Thinking: Let me analyze this problem]The answer is 42.",
				},
				FinishReason: openai.FinishReasonStop,
			},
		},
		Usage: openai.Usage{
			PromptTokens:     15,
			CompletionTokens: 12,
			TotalTokens:      27,
		},
	}

	fmt.Println("\n=== TEST CASE 10: OpenAI to Anthropic Response (Thinking) ===")
	fmt.Println("--- OpenAI Response (Input) ---")
	openaiJSON, _ := json.MarshalIndent(openaiResp, "", "  ")
	fmt.Println(string(openaiJSON))

	anthropicMsg, err := OpenAIToAnthropicRespMessage(openaiResp)
	assert.NoError(t, err)
	assert.NotNil(t, anthropicMsg)

	fmt.Println("\n--- Anthropic Message (Output) ---")
	anthropicJSON, _ := json.MarshalIndent(anthropicMsg, "", "  ")
	fmt.Println(string(anthropicJSON))

	// Validate conversion
	assert.Len(t, anthropicMsg.Content, 2)

	// First block should be thinking
	assert.Equal(t, "thinking", anthropicMsg.Content[0].Type)
	assert.Equal(t, "Let me analyze this problem", anthropicMsg.Content[0].Thinking)
	assert.Equal(t, "converted_from_openai", anthropicMsg.Content[0].Signature)

	// Second block should be text
	assert.Equal(t, "text", anthropicMsg.Content[1].Type)
	assert.Equal(t, "The answer is 42.", anthropicMsg.Content[1].Text)
}

func TestOpenAIToAnthropicRespMessage_NilInput(t *testing.T) {
	fmt.Println("\n=== TEST CASE 11: OpenAI to Anthropic Response (Nil) ===")
	fmt.Println("--- OpenAI Response (Input) ---")
	fmt.Println("nil")

	result, err := OpenAIToAnthropicRespMessage(nil)
	assert.NoError(t, err)
	assert.Nil(t, result)

	fmt.Println("--- Anthropic Message (Output) ---")
	fmt.Println("nil")
}
