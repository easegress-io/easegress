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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

// ResponseType defines the type of response for AI requests.
type ResponseType string

const (
	// Anthropic Response Types.
	// ResponseTypeMessage is used for message requests.
	ResponseTypeMessage ResponseType = "/v1/messages"

	// OpenAI Response Types.
	// ResponseTypeCompletions is used for standard completion requests.
	ResponseTypeCompletions ResponseType = "/v1/completions"
	// ResponseTypeChatCompletions is used for chat completion requests.
	ResponseTypeChatCompletions ResponseType = "/v1/chat/completions"
	// ResponseTypeEmbeddings is used for embedding requests.
	ResponseTypeEmbeddings ResponseType = "/v1/embeddings"
	// ResponseTypeModels is used for model listing requests.
	ResponseTypeModels ResponseType = "/v1/models"
	// ResponseTypeImageGenerations is used for image generation requests.
	ResponseTypeImageGenerations ResponseType = "/v1/images/generations"
)

type ResultError string

const (
	// ResultOk indicates a successful request with no errors.
	// In easegress, it is represented as an empty string.
	ResultOk               ResultError = ""
	ResultInternalError    ResultError = "internalError"
	ResultClientError      ResultError = "clientError"
	ResultServerError      ResultError = "serverError"
	ResultFailureCodeError ResultError = "failureCodeError"
	ResultProviderError    ResultError = "providerError"
	ResultMiddlewareError  ResultError = "middlewareError"
)

func ContextResults() []string {
	return []string{
		string(ResultInternalError),
		string(ResultClientError),
		string(ResultServerError),
		string(ResultFailureCodeError),
		string(ResultProviderError),
		string(ResultMiddlewareError),
	}
}

type (
	// ProviderSpec defines the specification for an AI provider.
	ProviderSpec struct {
		Name         string            `json:"name"`
		ProviderType string            `json:"providerType"`
		BaseURL      string            `json:"baseURL"`
		APIKey       string            `json:"apiKey"`
		Headers      map[string]string `json:"headers,omitempty"`
		// Optional parameters for specific providers, such as Azure.
		Endpoint     string `json:"endpoint,omitempty"`     // It is used for Azure OpenAI.
		DeploymentID string `json:"deploymentID,omitempty"` // It is used for Azure OpenAI.
		APIVersion   string `json:"apiVersion,omitempty"`   // It is used for Azure OpenAI.
	}

	Context struct {
		Ctx      *context.Context
		Provider *ProviderSpec

		// Req is the original request from the user. It's body is in ReqBody.
		Req                   *httpprot.Request
		ReqBody               []byte
		ReqInfo               *protocol.GeneralRequest
		OpenAIReq             map[string]any
		RespType              ResponseType
		ClaudeMessagesRequest *ClaudeMessagesRequest

		// ParseMetricFn is a function that parses the response body to a metric.
		// If it is sent, it will be called to parse the response body to a metric.
		// Otherwise, default ParseMetricFn will be used.
		ParseMetricFn func(fc *FinishContext) *metricshub.Metric

		resp      *Response
		callBacks []func(fc *FinishContext)

		stop   bool
		result string
	}

	Response struct {
		StatusCode int
		// ContentLength records the length of the associated content. The
		// value -1 indicates that the length is unknown. Unless Request.Method
		// is "HEAD", values >= 0 indicate that the given number of bytes may
		// be read from Body.
		ContentLength int64
		Header        http.Header
		// BodyReader is the response body reader. Only one of BodyReader or BodyBytes should be set.
		BodyReader io.Reader
		// BodyBytes is the response body bytes. Only one of BodyReader or BodyBytes should be set.
		BodyBytes []byte
	}

	FinishContext struct {
		StatusCode int
		Header     http.Header
		// RespBody is the final response body that sent to the user.
		RespBody []byte
		Duration int64
	}
)

func New(ctx *context.Context, provider *ProviderSpec) (*Context, error) {
	req := ctx.GetInputRequest().(*httpprot.Request)
	// request body
	body, err := io.ReadAll(req.GetPayload())
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	c := &Context{
		Ctx:       ctx,
		Provider:  provider,
		Req:       req,
		ReqBody:   body,
		OpenAIReq: map[string]any{},
		ReqInfo:   &protocol.GeneralRequest{},
		RespType:  ResponseType(""),
	}

	path := req.URL().Path
	if strings.HasSuffix(path, string(ResponseTypeMessage)) {
		c.RespType = ResponseTypeMessage
	} else if strings.HasSuffix(path, string(ResponseTypeChatCompletions)) {
		c.RespType = ResponseTypeChatCompletions
	} else if strings.HasSuffix(path, string(ResponseTypeCompletions)) {
		c.RespType = ResponseTypeCompletions
	} else if strings.HasSuffix(path, string(ResponseTypeModels)) {
		c.RespType = ResponseTypeModels
	} else {
		return nil, fmt.Errorf("unsupported request path: %s", path)
	}

	if c.RespType == ResponseTypeModels {
		return c, nil
	}

	err = c.adaptReqInOpenAIFormat()
	if err != nil {
		return nil, err
	}

	// parse OpenAI request
	openAIReq := make(map[string]interface{})
	err = json.Unmarshal(c.ReqBody, &openAIReq)
	if err != nil {
		return nil, err
	}

	model, stream, streamOptions, err := func() (model string, stream bool, options protocol.StreamOptions, err error) {
		defer func() {
			var ok bool
			if r := recover(); r != nil {
				if err, ok = r.(error); !ok {
					err = fmt.Errorf("failed to parse OpenAI request: %v", r)
				}
			}
		}()

		model = openAIReq["model"].(string)
		stream, _ = openAIReq["stream"].(bool)
		var ok bool
		options, ok = openAIReq["stream_options"].(protocol.StreamOptions)
		if ok {
			if options.IncludeUsage == nil {
				// Default to true if not specified
				options.IncludeUsage = new(bool)
				*options.IncludeUsage = true
			}
		}
		return
	}()
	if err != nil {
		return nil, fmt.Errorf("invalid OpenAI request body: %w", err)
	}

	c.OpenAIReq = openAIReq
	c.ReqInfo = &protocol.GeneralRequest{
		Model:         model,
		Stream:        stream,
		StreamOptions: streamOptions,
	}

	return c, nil
}

func (c *Context) adaptReqInOpenAIFormat() error {
	if c.ReqBody == nil {
		return fmt.Errorf("request body is missing")
	}

	// Only transform Anthropic message requests to OpenAI format.
	if c.RespType != ResponseTypeMessage {
		return nil
	}

	// Get the Anthropic x-api-key header and set it as Authorization Bearer.
	if apiKey := c.Req.Header().Get("x-api-key").(string); apiKey != "" {
		if c.Req.Header().Get("Authorization").(string) == "" {
			c.Req.Header().Set("Authorization", "Bearer "+apiKey)
		}
	}

	var req ClaudeMessagesRequest
	err := json.Unmarshal(c.ReqBody, &req)
	if err != nil {
		return fmt.Errorf("failed to unmarshal request body: %w", err)
	}

	c.ClaudeMessagesRequest = &req
	openAIReq, err := ConvertClaudeToOpenAI(&req, GlobalModelManager, GlobalAnthropicConfig)
	if err != nil {
		return fmt.Errorf("failed to convert Anthropic message to OpenAI request: %w", err)
	}

	c.ReqBody, err = json.Marshal(openAIReq)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	return nil
}

func (c *Context) adaptRespInOpenAIFormat() {
	// Only adapt OpenAI responses back to Anthropic format for Anthropic message requests
	if c.RespType != ResponseTypeMessage || c.resp == nil {
		return
	}

	if c.ReqInfo.Stream {
		// For streaming responses, convert OpenAI SSE stream to Anthropic format
		c.adaptStreamingRespToAnthropic()
	} else {
		// For non-streaming responses, convert OpenAI chat completion to Anthropic message
		c.adaptNonStreamingRespToAnthropic()
	}
}

// adaptStreamingRespToAnthropic converts OpenAI streaming response to Anthropic streaming format
func (c *Context) adaptStreamingRespToAnthropic() {
	if c.resp.BodyReader == nil {
		return
	}

	// Create a pipe to transform the streaming data
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		defer func() {
			if r := recover(); r != nil {
				pw.CloseWithError(fmt.Errorf("streaming conversion panic: %v", r))
			}
		}()

		scanner := bufio.NewScanner(c.resp.BodyReader)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}

			// Handle SSE format if present: "data: {...}"
			var jsonData string
			if strings.HasPrefix(line, "data: ") {
				jsonData = strings.TrimPrefix(line, "data: ")

				// Handle end of stream
				if jsonData == "[DONE]" {
					fmt.Fprintf(pw, "data: [DONE]\n\n")
					break
				}
			} else {
				// Handle raw JSON format (OpenAI streaming)
				jsonData = line
			}

			// Try to parse as OpenAI streaming chunk
			var openaiChunk map[string]interface{}
			if err := json.Unmarshal([]byte(jsonData), &openaiChunk); err != nil {
				// If parsing fails, pass through as-is (maintain original format)
				if strings.HasPrefix(line, "data: ") {
					fmt.Fprintf(pw, "%s\n\n", line)
				} else {
					fmt.Fprintf(pw, "data: %s\n\n", line)
				}
				continue
			}

			// Convert OpenAI chunk to Anthropic format
			anthropicEvent := c.convertOpenAIChunkToAnthropic(openaiChunk)
			if anthropicEvent != nil {
				if eventBytes, err := json.Marshal(anthropicEvent); err == nil {
					fmt.Fprintf(pw, "data: %s\n\n", string(eventBytes))
				}
			}
		}

		if err := scanner.Err(); err != nil {
			pw.CloseWithError(err)
		}
	}()

	// Replace the body reader with our converted stream
	// Ensure only BodyReader is set (not BodyBytes)
	c.resp.BodyReader = pr
	c.resp.BodyBytes = nil
}

// adaptNonStreamingRespToAnthropic converts OpenAI non-streaming response to Anthropic format
func (c *Context) adaptNonStreamingRespToAnthropic() {
	var bodyBytes []byte
	var err error

	// Read the response body
	if c.resp.BodyBytes != nil {
		bodyBytes = c.resp.BodyBytes
	} else if c.resp.BodyReader != nil {
		bodyBytes, err = io.ReadAll(c.resp.BodyReader)
		if err != nil {
			return
		}
	} else {
		return
	}

	var openaiResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &openaiResp); err != nil {
		logger.Errorf("failed to parse OpenAI response: %v", err)
		return
	}

	// Convert to Anthropic Message format using existing function
	anthropicMsg, err := ConvertOpenAIToClaudeResponse(openaiResp, c.ClaudeMessagesRequest)
	if err != nil {
		logger.Errorf("failed to convert OpenAI response to Anthropic format: %v", err)
		return
	}

	// Marshal back to JSON
	convertedBytes, err := json.Marshal(anthropicMsg)
	if err != nil {
		return
	}

	// Update response body with converted data
	// Ensure only BodyBytes is set (not BodyReader)
	c.resp.BodyBytes = convertedBytes
	c.resp.BodyReader = nil
	c.resp.ContentLength = int64(len(convertedBytes))
	// NOTICE: Precise header Content-Length is critical for claude client.
	c.resp.Header.Set("Content-Length", fmt.Sprintf("%d", c.resp.ContentLength))
}

// convertOpenAIChunkToAnthropic converts an OpenAI streaming chunk to Anthropic event format
func (c *Context) convertOpenAIChunkToAnthropic(chunk map[string]interface{}) map[string]interface{} {
	// Extract basic fields
	id, _ := chunk["id"].(string)
	model, _ := chunk["model"].(string)
	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Check if this is the first chunk (has role)
	if role, exists := delta["role"]; exists && role == "assistant" {
		// Convert to message_start event
		return map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":            id,
				"type":          "message",
				"role":          "assistant",
				"model":         model,
				"content":       []interface{}{},
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]interface{}{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		}
	}

	// Check if this has content (text delta)
	if content, exists := delta["content"]; exists && content != "" {
		return map[string]interface{}{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]interface{}{
				"type": "text_delta",
				"text": content,
			},
		}
	}

	// Check if this has finish_reason (message delta)
	if finishReason, exists := choice["finish_reason"]; exists && finishReason != nil {
		var anthropicStopReason string
		switch finishReason {
		case "stop":
			anthropicStopReason = "end_turn"
		case "length":
			anthropicStopReason = "max_tokens"
		case "tool_calls":
			anthropicStopReason = "tool_use"
		default:
			anthropicStopReason = "end_turn"
		}

		event := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": anthropicStopReason,
			},
		}

		// Include usage if present
		if usage, exists := chunk["usage"]; exists {
			if usageMap, ok := usage.(map[string]interface{}); ok {
				event["delta"].(map[string]interface{})["usage"] = map[string]interface{}{
					"output_tokens": usageMap["completion_tokens"],
				}
			}
		}

		return event
	}

	// Handle empty delta (final chunk) - this happens when delta is empty but no finish_reason yet
	if len(delta) == 0 {
		// This might be an intermediate chunk, skip it
		return nil
	}

	return nil
}

// GetResponse returns the response of the context.
func (c *Context) GetResponse() *Response {
	return c.resp
}

// SetResponse sets the response of the context.
// If you need to close the response body, you should add a callback
// function to the context using AddCallBack method.
func (c *Context) SetResponse(resp *Response) {
	c.resp = resp
	c.adaptRespInOpenAIFormat()
}

// AddCallBack adds a callback function to the context.
// The callback will be called when the reponse is sent to the user.
func (c *Context) AddCallBack(cb func(fc *FinishContext)) {
	c.callBacks = append(c.callBacks, cb)
}

// CallBacks returns all callback functions registered in the context.
func (c *Context) Callbacks() []func(fc *FinishContext) {
	return c.callBacks
}

// Stop stops the context execution and sets the result to be returned.
func (c *Context) Stop(result ResultError) {
	c.stop = true
	c.result = string(result)
}

// IsStopped checks if the context execution is stopped.
func (c *Context) IsStopped() bool {
	return c.stop
}

// Result returns the result of the context execution.
func (c *Context) Result() ResultError {
	return ResultError(c.result)
}
