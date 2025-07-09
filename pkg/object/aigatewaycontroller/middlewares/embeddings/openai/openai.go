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

package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings/embedtypes"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
)

const openaiEmbedPath = "/v1/embeddings"

// TODO: Add logic for this package
type (
	openaiEmbeddingHanlder struct {
		spec *embedtypes.EmbeddingSpec
	}
)

func New(spec *embedtypes.EmbeddingSpec) embedtypes.EmbeddingHandler {
	handler := &openaiEmbeddingHanlder{
		spec: spec,
	}
	return handler
}

func (h *openaiEmbeddingHanlder) EmbedDocuments(text string) ([]float32, error) {
	// prepare the request body
	embedReq := &protocol.EmbedRequest{
		Model:          h.spec.Model,
		Input:          text,
		EncodingFormat: "float",
	}
	reqBody, err := json.Marshal(embedReq)
	if err != nil {
		return nil, err
	}
	u, err := url.JoinPath(h.spec.BaseURL, openaiEmbedPath)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.spec.APIKey)
	for k, v := range h.spec.Headers {
		req.Header.Set(k, v)
	}

	// parse the response body
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
	var embedResp protocol.EmbeddingResponse
	if err := json.Unmarshal(data, &embedResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if len(embedResp.Data) == 0 || len(embedResp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai embedding response is empty")
	}
	return embedResp.Data[0].Embedding, nil
}

func (h *openaiEmbeddingHanlder) EmbedQuery(text string) ([]float32, error) {
	// TODO
	return nil, nil
}
