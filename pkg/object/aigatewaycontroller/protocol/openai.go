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

package protocol

import (
	"net/http"
)

// Error reference from https://platform.openai.com/docs/api-reference/responses-streaming/error
type Error struct {
	Message        string  `json:"message"`
	Type           string  `json:"type"`
	SequenceNumber int     `json:"sequence_number,omitempty"`
	Param          *string `json:"param"`
	Code           *string `json:"code"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

func NewError(code int, message string) ErrorResponse {
	var etype string
	switch code {
	case http.StatusBadRequest:
		etype = "invalid_request_error"
	case http.StatusNotFound:
		etype = "not_found_error"
	default:
		etype = "api_error"
	}
	return ErrorResponse{Error{Type: etype, Message: message}}
}

type GeneralRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type GeneralResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
}

type ChatCompletion struct {
	GeneralResponse
	Usage Usage `json:"usage,omitempty"`
}

type ChatCompletionChunk struct {
	GeneralResponse
	Usage *Usage `json:"usage,omitempty"`
}

type Completion struct {
	GeneralResponse
	Usage Usage `json:"usage,omitempty"`
}

type CompletionChunk struct {
	GeneralResponse
	Usage *Usage `json:"usage,omitempty"`
}

// ================================== Embedding Response Structure ==================================

// Embedding represents the response structure for OpenAI embeddings.
// see more details from https://platform.openai.com/docs/guides/embeddings
type Embedding struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type EmbeddingResponse struct {
	Object string         `json:"object"`
	Data   []Embedding    `json:"data"`
	Model  string         `json:"model"`
	Usage  EmbeddingUsage `json:"usage"`
}

// ================================== Image Response Structure ==================================

// ImageObject represents the structure of an image object in the OpenAI API response.
// see more details from https://platform.openai.com/docs/api-reference/images/object
type ImageObject struct {
	B64Json       string `json:"b64_json"`
	URL           string `json:"url"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type ImageUsageInputTokensDetails struct {
	ImageTokens int `json:"image_tokens"`
	TextTokens  int `json:"text_tokens"`
}

type ImageUsage struct {
	InputTokens        int                          `json:"input_tokens"`
	OutputTokens       int                          `json:"output_tokens"`
	TotalTokens        int                          `json:"total_tokens"`
	InputTokensDetails ImageUsageInputTokensDetails `json:"input_tokens_details,omitempty"`
}

type ImageResponse struct {
	Background   string        `json:"background,omitempty"`
	Created      int64         `json:"created"`
	Data         []ImageObject `json:"data"`
	OutputFormat string        `json:"output_format,omitempty"`
	Quality      string        `json:"quality,omitempty"`
	Size         string        `json:"size,omitempty"`
	Usage        ImageUsage    `json:"usage,omitempty"`
}
