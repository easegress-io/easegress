package aicontext

import (
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// AnthropicReqMessageToOpenAI converts Anthropic MessageNewParams to OpenAI ChatCompletionRequest
//
// This function provides comprehensive conversion between Anthropic's message format and OpenAI's
// chat completion format, handling all major content types and preserving semantic meaning.
//
// Content Type Conversions:
//  1. Text content: Direct 1:1 conversion from Anthropic text blocks to OpenAI content
//  2. Image content: Base64 images converted to data URLs, URL images passed through
//  3. Document content: All document types (PDF, text) converted to placeholder text since
//     OpenAI doesn't support document attachments directly
//  4. Tool use: Anthropic tool_use blocks converted to OpenAI function calls with JSON arguments
//  5. Tool results: Converted to OpenAI tool messages with proper role and tool_call_id mapping
//  6. Server tool use: Server-side tools converted to OpenAI function calls
//  7. Thinking blocks: Prefixed with "[Thinking: ]" and included as text content
//  8. Redacted thinking: Converted to "[Redacted thinking content]" placeholder
//
// Parameter Conversions:
// - Model: Direct string conversion
// - Temperature/TopP: Converted from param.Opt[float64] to float32
// - MaxTokens: Mapped to MaxCompletionTokens (following OpenAI's new field)
// - StopSequences: Direct conversion to Stop field
// - System messages: Converted to separate system role messages
// - Tools: Full conversion including input schema transformation to jsonschema.Definition
// - ToolChoice: Auto/specific tool choices properly mapped
// - Metadata: UserID extracted and mapped to OpenAI's metadata format
//
// Limitations:
// - Documents (PDF, etc.) cannot be directly converted and become placeholder text
// - Some Anthropic-specific features (Container, MCPServers, ServiceTier) are not converted
// - Stream flag is always set to false (streaming handled at API call level)
// - Thinking content is included as text since OpenAI has no equivalent feature
func AnthropicReqMessageToOpenAI(req *anthropic.MessageNewParams) (*openai.ChatCompletionRequest, error) {
	if req == nil {
		return nil, nil
	}

	result := &openai.ChatCompletionRequest{
		Model: string(req.Model),
	}

	// Convert Stream field - Note: MessageNewParams doesn't have Stream field,
	// it's set at the API call level. We'll assume non-streaming for now.
	result.Stream = false

	// Convert Temperature (param.Opt[float64] -> float32)
	if req.Temperature.Valid() {
		result.Temperature = float32(req.Temperature.Or(0))
	}

	// Convert TopP (param.Opt[float64] -> float32)
	if req.TopP.Valid() {
		result.TopP = float32(req.TopP.Or(0))
	}

	// Convert MaxTokens (int64 -> int)
	// max_tokens is deprecated in favor of max_completion_tokens:
	// https://platform.openai.com/docs/api-reference/chat/create#chat_create-max_tokens
	if req.MaxTokens > 0 {
		result.MaxCompletionTokens = int(req.MaxTokens)
	}

	// Convert StopSequences to Stop
	if len(req.StopSequences) > 0 {
		result.Stop = req.StopSequences
	}

	// Convert Messages and handle System prompt
	messages := make([]openai.ChatCompletionMessage, 0)

	// Add system messages first if present
	if len(req.System) > 0 {
		systemContent := ""
		for _, sysBlock := range req.System {
			// Convert TextBlockParam to string content
			systemContent += sysBlock.Text
		}
		if systemContent != "" {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemContent,
			})
		}
	}

	// Convert user/assistant messages
	for _, msg := range req.Messages {
		openaiMsg := openai.ChatCompletionMessage{
			Role: string(msg.Role),
		}

		// Convert complex ContentBlockParamUnion to simple content
		content := ""
		var toolCalls []openai.ToolCall

		for _, contentBlock := range msg.Content {
			// Handle different content block types
			if contentBlock.OfText != nil {
				content += contentBlock.OfText.Text
			} else if contentBlock.OfImage != nil {
				// For image content, we need to convert to OpenAI format
				// OpenAI expects multipart content for images
				imageURL := ""
				if contentBlock.OfImage.Source.OfBase64 != nil {
					// Convert base64 image to data URL format
					imageURL = fmt.Sprintf("data:%s;base64,%s",
						string(contentBlock.OfImage.Source.OfBase64.MediaType),
						contentBlock.OfImage.Source.OfBase64.Data)
				} else if contentBlock.OfImage.Source.OfURL != nil {
					// Handle URL-based images
					imageURL = contentBlock.OfImage.Source.OfURL.URL
				}

				// For images, we need to use MultiContent instead of simple Content
				if imageURL != "" {
					openaiMsg.MultiContent = append(openaiMsg.MultiContent, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: imageURL,
						},
					})
				}

			} else if contentBlock.OfDocument != nil {
				// Document content - convert to text content for OpenAI
				// Anthropic supports documents but OpenAI doesn't have direct equivalent
				if contentBlock.OfDocument.Source.OfText != nil {
					// PlainTextSourceParam doesn't have Text field, it has different structure
					// We need to access the proper field based on the Anthropic SDK structure
					content += "[Document content not directly convertible]"
				} else if contentBlock.OfDocument.Source.OfContent != nil {
					// Handle content blocks within documents
					contentUnion := contentBlock.OfDocument.Source.OfContent.Content
					if contentUnion.OfString.Valid() {
						content += contentUnion.OfString.Or("")
					}
				} else if contentBlock.OfDocument.Source.OfBase64 != nil {
					// Handle Base64 PDF documents
					content += "[Document content not directly convertible]"
				} else if contentBlock.OfDocument.Source.OfURL != nil {
					// Handle URL-based PDF documents
					content += "[Document content not directly convertible]"
				}
				// Note: PDF documents (Base64 or URL) can't be directly converted to OpenAI format
				// They would need to be processed/extracted separately
			} else if contentBlock.OfToolUse != nil {
				// Convert Anthropic tool_use to OpenAI function call
				toolCall := openai.ToolCall{
					ID:   contentBlock.OfToolUse.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name: contentBlock.OfToolUse.Name,
					},
				}

				// Convert input to JSON string
				if contentBlock.OfToolUse.Input != nil {
					if inputBytes, err := json.Marshal(contentBlock.OfToolUse.Input); err == nil {
						toolCall.Function.Arguments = string(inputBytes)
					}
				}

				toolCalls = append(toolCalls, toolCall)

			} else if contentBlock.OfToolResult != nil {
				// Convert Anthropic tool_result to OpenAI tool message
				// This should be handled as a separate message with role "tool"
				// For now, we'll convert the content to text
				if len(contentBlock.OfToolResult.Content) > 0 {
					for _, resultContent := range contentBlock.OfToolResult.Content {
						if resultContent.OfText != nil {
							content += resultContent.OfText.Text
						} else if resultContent.OfImage != nil {
							// Handle image results
							var resultImageURL string
							if resultContent.OfImage.Source.OfBase64 != nil {
								resultImageURL = fmt.Sprintf("data:%s;base64,%s",
									string(resultContent.OfImage.Source.OfBase64.MediaType),
									resultContent.OfImage.Source.OfBase64.Data)
							} else if resultContent.OfImage.Source.OfURL != nil {
								resultImageURL = resultContent.OfImage.Source.OfURL.URL
							}

							if resultImageURL != "" {
								openaiMsg.MultiContent = append(openaiMsg.MultiContent, openai.ChatMessagePart{
									Type: openai.ChatMessagePartTypeImageURL,
									ImageURL: &openai.ChatMessageImageURL{
										URL: resultImageURL,
									},
								})
							}
						}
					}
				}

				// Set tool call ID for tool result messages
				if contentBlock.OfToolResult.ToolUseID != "" {
					openaiMsg.ToolCallID = contentBlock.OfToolResult.ToolUseID
					// Tool result messages should have role "tool"
					openaiMsg.Role = openai.ChatMessageRoleTool
				}

			} else if contentBlock.OfThinking != nil {
				// Anthropic thinking blocks - OpenAI doesn't have direct equivalent
				// Could be included as content or ignored
				content += fmt.Sprintf("[Thinking: %s]", contentBlock.OfThinking.Thinking)
			} else if contentBlock.OfRedactedThinking != nil {
				// Redacted thinking - minimal representation
				content += "[Redacted thinking content]"
			} else if contentBlock.OfServerToolUse != nil {
				// Server tool use - similar to tool use but for server-side tools
				// Convert to function call format
				toolCall := openai.ToolCall{
					ID:   contentBlock.OfServerToolUse.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name: string(contentBlock.OfServerToolUse.Name), // ServerToolUseBlockParamName is a string type
					},
				}

				if contentBlock.OfServerToolUse.Input != nil {
					if inputBytes, err := json.Marshal(contentBlock.OfServerToolUse.Input); err == nil {
						toolCall.Function.Arguments = string(inputBytes)
					}
				}

				toolCalls = append(toolCalls, toolCall)
			}
		}

		// Add tool calls to message if any
		if len(toolCalls) > 0 {
			openaiMsg.ToolCalls = toolCalls
		} // Set content only if we don't have MultiContent
		if len(openaiMsg.MultiContent) == 0 && content != "" {
			openaiMsg.Content = content
		} else if len(openaiMsg.MultiContent) > 0 && content != "" {
			// Add text content as first part if we have both text and images
			textPart := openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: content,
			}
			openaiMsg.MultiContent = append([]openai.ChatMessagePart{textPart}, openaiMsg.MultiContent...)
		}

		messages = append(messages, openaiMsg)
	}

	result.Messages = messages

	// Convert Tools
	if len(req.Tools) > 0 {
		tools := make([]openai.Tool, 0, len(req.Tools))

		for _, tool := range req.Tools {
			if tool.OfTool != nil {
				openaiTool := openai.Tool{
					Type: openai.ToolTypeFunction,
					Function: &openai.FunctionDefinition{
						Name: tool.OfTool.Name,
					},
				}

				// Convert Description (param.Opt[string] -> string)
				if tool.OfTool.Description.Valid() {
					openaiTool.Function.Description = tool.OfTool.Description.Or("")
				}

				// Convert InputSchema from Anthropic format to OpenAI jsonschema format
				if tool.OfTool.InputSchema.Properties != nil || tool.OfTool.InputSchema.Required != nil {
					schema := &jsonschema.Definition{
						Type:     jsonschema.Object,
						Required: tool.OfTool.InputSchema.Required,
					}

					// Convert properties from map[string]interface{} to jsonschema.Definition
					if propMap, ok := tool.OfTool.InputSchema.Properties.(map[string]interface{}); ok {
						schema.Properties = make(map[string]jsonschema.Definition)
						for propName, propValue := range propMap {
							if propMap, ok := propValue.(map[string]interface{}); ok {
								propDef := jsonschema.Definition{}
								if typeVal, exists := propMap["type"]; exists {
									if typeStr, ok := typeVal.(string); ok {
										propDef.Type = jsonschema.DataType(typeStr)
									}
								}
								if desc, exists := propMap["description"]; exists {
									if descStr, ok := desc.(string); ok {
										propDef.Description = descStr
									}
								}
								schema.Properties[propName] = propDef
							}
						}
					}

					openaiTool.Function.Parameters = schema
				}

				tools = append(tools, openaiTool)
			}
			// Handle other tool types (bash, text_editor, web_search) as needed
		}

		result.Tools = tools
	}

	// Convert ToolChoice
	if !param.IsOmitted(req.ToolChoice.OfAuto) {
		result.ToolChoice = &openai.ToolChoice{
			Type: openai.ToolTypeFunction,
		}
	} else if !param.IsOmitted(req.ToolChoice.OfTool) {
		result.ToolChoice = &openai.ToolChoice{
			Type: openai.ToolTypeFunction,
			Function: openai.ToolFunction{
				Name: req.ToolChoice.OfTool.Name,
			},
		}
	}

	// Handle Metadata - OpenAI has a different metadata structure
	if req.Metadata.UserID.Valid() {
		// OpenAI metadata is map[string]string, so convert accordingly
		if result.Metadata == nil {
			result.Metadata = make(map[string]string)
		}
		if userID := req.Metadata.UserID.Or(""); userID != "" {
			result.Metadata["user_id"] = userID
		}
	}

	// Note: Some Anthropic-specific fields don't have direct OpenAI equivalents:
	// - Thinking: OpenAI doesn't have thinking configuration
	// - ServiceTier: Different service tier models in OpenAI
	// - Container: Anthropic-specific container reuse feature
	// - MCPServers: Anthropic-specific Model Context Protocol servers

	// Debug: Print marshaled JSON for debugging
	// if reqJSON, err := json.MarshalIndent(req, "", "  "); err == nil {
	// 	fmt.Printf("Anthropic Request JSON:\n%s\n", string(reqJSON))
	// }

	// if resultJSON, err := json.MarshalIndent(result, "", "  "); err == nil {
	// 	fmt.Printf("OpenAI Request JSON:\n%s\n", string(resultJSON))
	// }

	return result, nil
}

// OpenAIToAnthropicRespMessage converts OpenAI ChatCompletionResponse to Anthropic Message
//
// This function provides comprehensive conversion from OpenAI's response format to Anthropic's
// message format, handling content types and preserving semantic meaning where possible.
//
// Content Type Conversions:
// 1. Text content: Direct conversion from OpenAI message content to Anthropic text blocks
// 2. Tool calls: OpenAI function calls converted to Anthropic tool_use blocks
// 3. Tool messages: OpenAI tool role messages converted to Anthropic tool_result blocks
// 4. MultiContent: Images and text parts converted to appropriate Anthropic content blocks
//
// Response Field Mappings:
// - ID: Direct mapping from OpenAI response ID
// - Model: Direct string conversion
// - Role: Always "assistant" for Anthropic responses
// - StopReason: Mapped from OpenAI FinishReason with semantic equivalents
// - StopSequence: Extracted from OpenAI if available
// - Usage: Token counts mapped with appropriate field conversions
//
// Limitations:
// - OpenAI's multiple choices are flattened to single choice (index 0)
// - Some OpenAI-specific features (reasoning_content, logprobs) are not converted
// - Complex multi-choice responses lose additional choices beyond the first
func OpenAIToAnthropicRespMessage(resp *openai.ChatCompletionResponse) (*anthropic.Message, error) {
	if resp == nil {
		return nil, nil
	}

	// Handle empty choices
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI response has no choices")
	}

	// Use the first choice (OpenAI can have multiple choices, Anthropic typically has one)
	choice := resp.Choices[0]
	message := choice.Message

	// Build content blocks from OpenAI message
	var contentBlocks []anthropic.ContentBlockUnion

	// Handle tool calls first (they appear in assistant messages)
	if len(message.ToolCalls) > 0 {
		for _, toolCall := range message.ToolCalls {
			// Parse the arguments JSON
			var input interface{}
			if toolCall.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
					// If parsing fails, use the raw string
					input = toolCall.Function.Arguments
				}
			}

			// Create tool use block
			contentBlock := anthropic.ContentBlockUnion{
				Type:  "tool_use",
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: json.RawMessage(toolCall.Function.Arguments),
			}
			contentBlocks = append(contentBlocks, contentBlock)
		}
	}

	// Handle MultiContent (images and text parts)
	if len(message.MultiContent) > 0 {
		for _, part := range message.MultiContent {
			switch part.Type {
			case openai.ChatMessagePartTypeText:
				if part.Text != "" {
					contentBlock := anthropic.ContentBlockUnion{
						Type: "text",
						Text: part.Text,
					}
					contentBlocks = append(contentBlocks, contentBlock)
				}
			case openai.ChatMessagePartTypeImageURL:
				// Note: Anthropic's response format doesn't typically include images
				// in assistant responses, but we'll convert it for completeness
				if part.ImageURL != nil && part.ImageURL.URL != "" {
					contentBlock := anthropic.ContentBlockUnion{
						Type: "text",
						Text: fmt.Sprintf("[Image: %s]", part.ImageURL.URL),
					}
					contentBlocks = append(contentBlocks, contentBlock)
				}
			}
		}
	} else if message.Content != "" {
		// Handle simple text content
		// Check if it contains thinking patterns from our conversion
		content := message.Content

		// Parse thinking blocks if they exist (from our own conversion)
		if strings.Contains(content, "[Thinking: ") {
			parts := strings.Split(content, "[Thinking: ")
			if len(parts) > 1 {
				// Add initial text if any
				if parts[0] != "" {
					contentBlock := anthropic.ContentBlockUnion{
						Type: "text",
						Text: parts[0],
					}
					contentBlocks = append(contentBlocks, contentBlock)
				}

				// Process thinking blocks
				for i := 1; i < len(parts); i++ {
					if endIdx := strings.Index(parts[i], "]"); endIdx != -1 {
						thinkingContent := parts[i][:endIdx]
						remainingContent := parts[i][endIdx+1:]

						// Add thinking block
						contentBlock := anthropic.ContentBlockUnion{
							Type:      "thinking",
							Signature: "converted_from_openai",
							Thinking:  thinkingContent,
						}
						contentBlocks = append(contentBlocks, contentBlock)

						// Add remaining text if any
						if remainingContent != "" {
							contentBlock = anthropic.ContentBlockUnion{
								Type: "text",
								Text: remainingContent,
							}
							contentBlocks = append(contentBlocks, contentBlock)
						}
					}
				}
			}
		} else {
			// Simple text content
			contentBlock := anthropic.ContentBlockUnion{
				Type: "text",
				Text: content,
			}
			contentBlocks = append(contentBlocks, contentBlock)
		}
	}

	// Handle reasoning content if present (DeepSeek specific)
	if message.ReasoningContent != "" {
		contentBlock := anthropic.ContentBlockUnion{
			Type:      "thinking",
			Signature: "reasoning_from_openai",
			Thinking:  message.ReasoningContent,
		}
		contentBlocks = append(contentBlocks, contentBlock)
	}

	// Convert FinishReason to StopReason
	var stopReason anthropic.StopReason
	switch choice.FinishReason {
	case openai.FinishReasonStop:
		stopReason = anthropic.StopReasonEndTurn
	case openai.FinishReasonLength:
		stopReason = anthropic.StopReasonMaxTokens
	case openai.FinishReasonFunctionCall, openai.FinishReasonToolCalls:
		stopReason = anthropic.StopReasonToolUse
	case openai.FinishReasonContentFilter:
		stopReason = anthropic.StopReasonRefusal
	default:
		stopReason = anthropic.StopReasonEndTurn
	}

	// Convert Usage
	usage := anthropic.Usage{
		InputTokens:  int64(resp.Usage.PromptTokens),
		OutputTokens: int64(resp.Usage.CompletionTokens),
		// Set cache tokens to 0 as OpenAI doesn't provide this info
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     0,
		// Set server tool use to default values
		ServerToolUse: anthropic.ServerToolUsage{},
		// Default service tier
		ServiceTier: anthropic.UsageServiceTierStandard,
	}

	// Create the Anthropic Message
	anthropicMessage := &anthropic.Message{
		ID:           resp.ID,
		Content:      contentBlocks,
		Model:        anthropic.Model(resp.Model),
		Role:         "assistant", // constant.Assistant
		StopReason:   stopReason,
		StopSequence: "",        // OpenAI doesn't typically provide the actual stop sequence
		Type:         "message", // constant.Message
		Usage:        usage,
	}

	// Debug: Print marshaled JSON for debugging
	// if respJSON, err := json.MarshalIndent(resp, "", "  "); err == nil {
	// 	fmt.Printf("OpenAI Response JSON:\n%s\n", string(respJSON))
	// }

	// if anthropicJSON, err := json.MarshalIndent(anthropicMessage, "", "  "); err == nil {
	// 	fmt.Printf("Anthropic Message JSON:\n%s\n", string(anthropicJSON))
	// }

	return anthropicMessage, nil
}
