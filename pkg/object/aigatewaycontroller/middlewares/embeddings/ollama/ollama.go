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

package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings/embedtypes"
)

const ollamaEmbedPath = "/api/embed"

type (
	ollamaEmbeddingHanlder struct {
		spec *embedtypes.EmbeddingSpec
	}

	Duration struct {
		time.Duration
	}

	// EmbedRequest is the reqeust of ollamd embedding api.
	EmbedRequest struct {
		Model string `json:"model"`

		// Input is the input to embed, can be a string or an array of strings.
		Input any `json:"input"`
	}

	// EmbedResponse is the response of ollamd embedding api.
	EmbedResponse struct {
		Model      string      `json:"model"`
		Embeddings [][]float32 `json:"embeddings"`

		TotalDuration   time.Duration `json:"total_duration,omitempty"`
		LoadDuration    time.Duration `json:"load_duration,omitempty"`
		PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	}
)

func New(spec *embedtypes.EmbeddingSpec) embedtypes.EmbeddingHandler {
	handler := &ollamaEmbeddingHanlder{
		spec: spec,
	}
	return handler
}

func (h *ollamaEmbeddingHanlder) EmbedDocuments(text string) ([]float32, error) {
	embedReq := &EmbedRequest{
		Model: h.spec.Model,
		Input: text,
	}
	reqBody, err := json.Marshal(embedReq)
	if err != nil {
		return nil, err
	}
	u, err := url.JoinPath(h.spec.BaseURL, ollamaEmbedPath)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embedding request failed with status code %d, %s", resp.StatusCode, string(data))
	}
	var embedResp EmbedResponse
	if err := json.Unmarshal(data, &embedResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(embedResp.Embeddings) == 0 || len(embedResp.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama embedding response is empty")
	}
	return embedResp.Embeddings[0], nil
}

func (h *ollamaEmbeddingHanlder) EmbedQuery(text string) ([]float32, error) {
	return h.EmbedDocuments(text)
}
