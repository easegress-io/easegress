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
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// Constants for content types (matching Python Constants class)
const (
	ContentText       = "text"
	ContentImage      = "image"
	ContentToolUse    = "tool_use"
	ContentToolResult = "tool_result"

	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"

	ToolFunction = "function"

	// Stop reasons
	StopEndTurn   = "end_turn"
	StopMaxTokens = "max_tokens"
	StopToolUse   = "tool_use"

	// Event types for streaming
	EventMessageStart      = "message_start"
	EventContentBlockStart = "content_block_start"
	EventContentBlockDelta = "content_block_delta"
	EventContentBlockStop  = "content_block_stop"
	EventMessageDelta      = "message_delta"
	EventMessageStop       = "message_stop"
	EventPing              = "ping"

	// Delta types
	DeltaText = "text_delta"
)

// ModelManager interface to map Claude models to OpenAI models
type ModelManager interface {
	MapClaudeModelToOpenAI(claudeModel string) string
}

// ConvertClaudeToOpenAI converts Claude API request format to OpenAI format
// This is a precise translation of the Python convert_claude_to_openai function
func ConvertClaudeToOpenAI(claudeRequest *ClaudeMessagesRequest, modelManager ModelManager, config *AnthropicConfig) (map[string]interface{}, error) {
	// Map model
	openaiModel := modelManager.MapClaudeModelToOpenAI(claudeRequest.Model)

	// Convert messages
	var openaiMessages []openai.ChatCompletionMessage

	// Add system message if present
	if claudeRequest.System != nil {
		systemText := ""
		if systemStr, ok := claudeRequest.GetSystemAsString(); ok {
			systemText = systemStr
		} else if systemContents, ok := claudeRequest.GetSystemAsContents(); ok {
			var textParts []string
			for _, block := range systemContents {
				if block.Type == ContentText {
					textParts = append(textParts, block.Text)
				}
			}
			systemText = strings.Join(textParts, "\n\n")
		}

		if strings.TrimSpace(systemText) != "" {
			openaiMessages = append(openaiMessages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: strings.TrimSpace(systemText),
			})
		}
	}

	// Process Claude messages
	i := 0
	for i < len(claudeRequest.Messages) {
		msg := claudeRequest.Messages[i]

		if msg.Role == RoleUser {
			openaiMessage, err := convertClaudeUserMessage(&msg)
			if err != nil {
				return nil, fmt.Errorf("failed to convert user message: %w", err)
			}
			openaiMessages = append(openaiMessages, *openaiMessage)
		} else if msg.Role == RoleAssistant {
			openaiMessage, err := convertClaudeAssistantMessage(&msg)
			if err != nil {
				return nil, fmt.Errorf("failed to convert assistant message: %w", err)
			}
			openaiMessages = append(openaiMessages, *openaiMessage)

			// Check if next message contains tool results
			if i+1 < len(claudeRequest.Messages) {
				nextMsg := claudeRequest.Messages[i+1]
				if nextMsg.Role == RoleUser {
					if blocks, ok := nextMsg.GetContentAsBlocks(); ok {
						hasToolResult := false
						for _, block := range blocks {
							if block.Type == ContentToolResult {
								hasToolResult = true
								break
							}
						}
						if hasToolResult {
							// Process tool results
							i++ // Skip to tool result message
							toolResults, err := convertClaudeToolResults(&nextMsg)
							if err != nil {
								return nil, fmt.Errorf("failed to convert tool results: %w", err)
							}
							openaiMessages = append(openaiMessages, toolResults...)
						}
					}
				}
			}
		}
		i++
	}

	// Build OpenAI request
	minTokens := config.GetMinTokensLimit()
	maxTokens := config.GetMaxTokensLimit()

	// Apply token limits with bounds checking
	finalMaxTokens := claudeRequest.MaxTokens
	if finalMaxTokens < minTokens {
		finalMaxTokens = minTokens
	}
	if finalMaxTokens > maxTokens {
		finalMaxTokens = maxTokens
	}

	openaiRequest := map[string]interface{}{
		"model":                 openaiModel,
		"messages":              openaiMessages,
		"max_completion_tokens": finalMaxTokens, // Updated to match Python
		"temperature":           1,              // Default temperature as per Python
		// NOTICE: OpenAI needs verified organization to use stream,
		// so set stream to false for now.
		"stream": false, // claudeRequest.GetStream()
	}

	// Add optional parameters
	if len(claudeRequest.StopSequences) > 0 {
		openaiRequest["stop"] = claudeRequest.StopSequences
	}
	if claudeRequest.TopP != nil {
		openaiRequest["top_p"] = *claudeRequest.TopP
	}

	// Convert tools
	if len(claudeRequest.Tools) > 0 {
		var openaiTools []openai.Tool
		for _, tool := range claudeRequest.Tools {
			if tool.Name != "" && strings.TrimSpace(tool.Name) != "" {
				description := ""
				if tool.Description != nil {
					description = *tool.Description
				}
				openaiTools = append(openaiTools, openai.Tool{
					Type: ToolFunction,
					Function: &openai.FunctionDefinition{
						Name:        tool.Name,
						Description: description,
						Parameters:  tool.InputSchema,
					},
				})
			}
		}
		if len(openaiTools) > 0 {
			openaiRequest["tools"] = openaiTools
		}
	}

	// Convert tool choice
	if claudeRequest.ToolChoice != nil {
		choiceType, hasType := claudeRequest.ToolChoice["type"]
		if hasType {
			switch choiceType {
			case "auto":
				openaiRequest["tool_choice"] = "auto"
			case "any":
				openaiRequest["tool_choice"] = "auto"
			case "tool":
				if name, hasName := claudeRequest.ToolChoice["name"]; hasName {
					openaiRequest["tool_choice"] = map[string]interface{}{
						"type": ToolFunction,
						ToolFunction: map[string]interface{}{
							"name": name,
						},
					}
				} else {
					openaiRequest["tool_choice"] = "auto"
				}
			default:
				openaiRequest["tool_choice"] = "auto"
			}
		} else {
			openaiRequest["tool_choice"] = "auto"
		}
	}

	return openaiRequest, nil
}

// convertClaudeUserMessage converts Claude user message to OpenAI format
// This is a precise translation of the Python convert_claude_user_message function
func convertClaudeUserMessage(msg *ClaudeMessage) (*openai.ChatCompletionMessage, error) {
	if msg.Content == nil {
		return &openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "",
		}, nil
	}

	if contentStr, ok := msg.GetContentAsString(); ok {
		return &openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: contentStr,
		}, nil
	}

	// Handle multimodal content
	if blocks, ok := msg.GetContentAsBlocks(); ok {
		var openaiContent []openai.ChatMessagePart
		for _, block := range blocks {
			switch block.Type {
			case ContentText:
				openaiContent = append(openaiContent, openai.ChatMessagePart{
					Type: "text",
					Text: block.Text,
				})
			case ContentImage:
				// Convert Claude image format to OpenAI format
				if block.Source != nil {
					if sourceType, hasType := block.Source["type"]; hasType && sourceType == "base64" {
						if mediaType, hasMediaType := block.Source["media_type"]; hasMediaType {
							if data, hasData := block.Source["data"]; hasData {
								imageURL := fmt.Sprintf("data:%v;base64,%v", mediaType, data)
								openaiContent = append(openaiContent, openai.ChatMessagePart{
									Type: "image_url",
									ImageURL: &openai.ChatMessageImageURL{
										URL: imageURL,
									},
								})
							}
						}
					}
				}
			}
		}

		if len(openaiContent) == 1 && openaiContent[0].Type == "text" {
			return &openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: openaiContent[0].Text,
			}, nil
		}
		return &openai.ChatCompletionMessage{
			Role:         openai.ChatMessageRoleUser,
			MultiContent: openaiContent,
		}, nil
	}

	return &openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "",
	}, nil
}

// convertClaudeAssistantMessage converts Claude assistant message to OpenAI format
// This is a precise translation of the Python convert_claude_assistant_message function
func convertClaudeAssistantMessage(msg *ClaudeMessage) (*openai.ChatCompletionMessage, error) {
	var textParts []string
	var toolCalls []openai.ToolCall

	if msg.Content == nil {
		return &openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: "",
		}, nil
	}

	if contentStr, ok := msg.GetContentAsString(); ok {
		return &openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: contentStr,
		}, nil
	}

	if blocks, ok := msg.GetContentAsBlocks(); ok {
		for _, block := range blocks {
			switch block.Type {
			case ContentText:
				textParts = append(textParts, block.Text)
			case ContentToolUse:
				inputJSON, err := json.Marshal(block.Input)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool input: %w", err)
				}
				toolCalls = append(toolCalls, openai.ToolCall{
					ID:   block.ID,
					Type: ToolFunction,
					Function: openai.FunctionCall{
						Name:      block.Name,
						Arguments: string(inputJSON),
					},
				})
			}
		}
	}

	openaiMessage := &openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
	}

	// Set content
	if len(textParts) > 0 {
		openaiMessage.Content = strings.Join(textParts, "")
	} else {
		openaiMessage.Content = ""
		// Note: In OpenAI Go SDK, empty content for tool calls should be empty string, not nil
	}

	// Set tool calls
	if len(toolCalls) > 0 {
		openaiMessage.ToolCalls = toolCalls
	}

	return openaiMessage, nil
}

// convertClaudeToolResults converts Claude tool results to OpenAI format
// This is a precise translation of the Python convert_claude_tool_results function
func convertClaudeToolResults(msg *ClaudeMessage) ([]openai.ChatCompletionMessage, error) {
	var toolMessages []openai.ChatCompletionMessage

	if blocks, ok := msg.GetContentAsBlocks(); ok {
		for _, block := range blocks {
			if block.Type == ContentToolResult {
				content, err := parseToolResultContent(block.Content)
				if err != nil {
					return nil, fmt.Errorf("failed to parse tool result content: %w", err)
				}
				toolMessages = append(toolMessages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    content,
					ToolCallID: block.ToolUseID,
				})
			}
		}
	}

	return toolMessages, nil
}

// parseToolResultContent parses and normalizes tool result content into a string format
// This is a precise translation of the Python parse_tool_result_content function
func parseToolResultContent(content interface{}) (string, error) {
	if content == nil {
		return "No content provided", nil
	}

	if contentStr, ok := content.(string); ok {
		return contentStr, nil
	}

	if contentList, ok := content.([]interface{}); ok {
		var resultParts []string
		for _, item := range contentList {
			if itemDict, ok := item.(map[string]interface{}); ok {
				if itemType, hasType := itemDict["type"]; hasType && itemType == ContentText {
					if text, hasText := itemDict["text"]; hasText {
						if textStr, ok := text.(string); ok {
							resultParts = append(resultParts, textStr)
						}
					}
				} else if text, hasText := itemDict["text"]; hasText {
					if textStr, ok := text.(string); ok {
						resultParts = append(resultParts, textStr)
					}
				} else {
					jsonBytes, err := json.Marshal(itemDict)
					if err != nil {
						resultParts = append(resultParts, fmt.Sprintf("%v", item))
					} else {
						resultParts = append(resultParts, string(jsonBytes))
					}
				}
			} else if itemStr, ok := item.(string); ok {
				resultParts = append(resultParts, itemStr)
			} else {
				resultParts = append(resultParts, fmt.Sprintf("%v", item))
			}
		}
		return strings.TrimSpace(strings.Join(resultParts, "\n")), nil
	}

	if contentDict, ok := content.(map[string]interface{}); ok {
		if contentType, hasType := contentDict["type"]; hasType && contentType == ContentText {
			if text, hasText := contentDict["text"]; hasText {
				if textStr, ok := text.(string); ok {
					return textStr, nil
				}
			}
		}
		jsonBytes, err := json.Marshal(contentDict)
		if err != nil {
			return fmt.Sprintf("%v", content), nil
		}
		return string(jsonBytes), nil
	}

	return fmt.Sprintf("%v", content), nil
}

// ConvertOpenAIToClaudeResponse converts OpenAI response to Claude format
// This is a precise translation of the Python convert_openai_to_claude_response function
func ConvertOpenAIToClaudeResponse(openaiResponse map[string]interface{}, originalRequest *ClaudeMessagesRequest) (map[string]interface{}, error) {
	// Extract response data
	choices, hasChoices := openaiResponse["choices"]
	if !hasChoices {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	choicesList, ok := choices.([]interface{})
	if !ok || len(choicesList) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	choice, ok := choicesList[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid choice format in OpenAI response")
	}

	message, hasMessage := choice["message"]
	if !hasMessage {
		return nil, fmt.Errorf("no message in OpenAI choice")
	}

	messageMap, ok := message.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid message format in OpenAI choice")
	}

	// Build Claude content blocks
	var contentBlocks []map[string]interface{}

	// Add text content
	textContent, hasTextContent := messageMap["content"]
	if hasTextContent && textContent != nil {
		if textStr, ok := textContent.(string); ok && textStr != "" {
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type": ContentText,
				"text": textStr,
			})
		}
	}

	// Add tool calls
	toolCalls, hasToolCalls := messageMap["tool_calls"]
	if hasToolCalls && toolCalls != nil {
		if toolCallsList, ok := toolCalls.([]interface{}); ok {
			for _, toolCall := range toolCallsList {
				if toolCallMap, ok := toolCall.(map[string]interface{}); ok {
					if toolType, hasType := toolCallMap["type"]; hasType && toolType == ToolFunction {
						if functionData, hasFunction := toolCallMap[ToolFunction]; hasFunction {
							if functionMap, ok := functionData.(map[string]interface{}); ok {
								var arguments map[string]interface{}
								if argsStr, hasArgs := functionMap["arguments"]; hasArgs {
									if argsString, ok := argsStr.(string); ok {
										if err := json.Unmarshal([]byte(argsString), &arguments); err != nil {
											// If JSON parsing fails, use raw arguments
											arguments = map[string]interface{}{
												"raw_arguments": argsString,
											}
										}
									}
								}
								if arguments == nil {
									arguments = make(map[string]interface{})
								}

								toolID, _ := toolCallMap["id"].(string)
								if toolID == "" {
									toolID = fmt.Sprintf("tool_%s", generateUUID())
								}

								functionName, _ := functionMap["name"].(string)

								contentBlocks = append(contentBlocks, map[string]interface{}{
									"type":  ContentToolUse,
									"id":    toolID,
									"name":  functionName,
									"input": arguments,
								})
							}
						}
					}
				}
			}
		}
	}

	// Ensure at least one content block
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, map[string]interface{}{
			"type": ContentText,
			"text": "",
		})
	}

	// Map finish reason
	finishReason, _ := choice["finish_reason"].(string)
	var stopReason string
	switch finishReason {
	case "stop":
		stopReason = StopEndTurn
	case "length":
		stopReason = StopMaxTokens
	case "tool_calls", "function_call":
		stopReason = StopToolUse
	default:
		stopReason = StopEndTurn
	}

	// Extract usage information
	var inputTokens, outputTokens int
	if usage, hasUsage := openaiResponse["usage"]; hasUsage {
		if usageMap, ok := usage.(map[string]interface{}); ok {
			if promptTokens, hasPrompt := usageMap["prompt_tokens"]; hasPrompt {
				if tokens, ok := promptTokens.(float64); ok {
					inputTokens = int(tokens)
				}
			}
			if completionTokens, hasCompletion := usageMap["completion_tokens"]; hasCompletion {
				if tokens, ok := completionTokens.(float64); ok {
					outputTokens = int(tokens)
				}
			}
		}
	}

	// Build Claude response
	responseID, _ := openaiResponse["id"].(string)
	if responseID == "" {
		responseID = fmt.Sprintf("msg_%s", generateUUID())
	}

	claudeResponse := map[string]interface{}{
		"id":            responseID,
		"type":          "message",
		"role":          RoleAssistant,
		"model":         originalRequest.Model,
		"content":       contentBlocks,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage": map[string]interface{}{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}

	return claudeResponse, nil
}

// generateUUID generates a simple UUID-like string
// Note: This is a simplified implementation since we don't have external UUID library
func generateUUID() string {
	// This is a simplified UUID generation - in production, use a proper UUID library
	// For now, using a combination of timestamp and random-like string
	return fmt.Sprintf("%x", []byte(fmt.Sprintf("%d", len(fmt.Sprintf("%p", &[]int{})))))[:8]
}

// StreamingConversionState holds state for streaming conversion
type StreamingConversionState struct {
	MessageID        string
	TextBlockIndex   int
	ToolBlockCounter int
	CurrentToolCalls map[int]*ToolCallState
	FinalStopReason  string
}

// ToolCallState tracks the state of a tool call during streaming
type ToolCallState struct {
	ID          string
	Name        string
	ArgsBuffer  string
	JSONSent    bool
	ClaudeIndex *int
	Started     bool
}
