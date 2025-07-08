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

package middlewares

import "fmt"

type (
	// EmbeddingHandler defines the interface for embedding handlers in the AI Gateway Controller.
	EmbeddingHandler interface {
		Embed(text string) ([]float32, error)

		init(spec *EmbeddingSpec)
	}

	// EmbeddingSpec defines the specification for embedding providers.
	EmbeddingSpec struct {
		ProviderType string            `json:"providerType"`
		BaseURL      string            `json:"baseURL"`
		APIKey       string            `json:"apiKey"`
		Headers      map[string]string `json:"headers,omitempty"`
		Model        string            `json:"model"`
	}
)

// newEmbeddingHandler creates a new EmbeddingHandler based on the provided EmbeddingSpec.
// It supports "ollama" and "openai" providers.
// validate checks the EmbeddingSpec for required fields and supported provider types.
func newEmbeddingHandler(spec *EmbeddingSpec) EmbeddingHandler {
	if spec.ProviderType == "ollama" {
		handler := &ollamaEmbeddingHanlder{}
		handler.init(spec)
		return handler
	}
	handler := &openAIEmbeddingHandler{}
	handler.init(spec)
	return handler
}

func (spec *EmbeddingSpec) validate() error {
	if spec.ProviderType != "ollama" && spec.ProviderType != "openai" {
		return fmt.Errorf("unsupported embedding provider type: %s, only support ollama and openai for now", spec.ProviderType)
	}
	if spec.BaseURL == "" {
		return fmt.Errorf("baseURL is required for embedding provider")
	}
	if spec.ProviderType == "openai" && spec.APIKey == "" {
		return fmt.Errorf("apiKey is required for openai embedding provider")
	}
	if spec.Model == "" {
		return fmt.Errorf("model is required for embedding provider")
	}
	return nil
}

type (
	ollamaEmbeddingHanlder struct{}
)

var _ EmbeddingHandler = (*ollamaEmbeddingHanlder)(nil)

func (h *ollamaEmbeddingHanlder) init(spec *EmbeddingSpec) {
	// TODO
}

func (h *ollamaEmbeddingHanlder) Embed(text string) ([]float32, error) {
	// TODO
	return nil, nil
}

type (
	openAIEmbeddingHandler struct{}
)

var _ EmbeddingHandler = (*openAIEmbeddingHandler)(nil)

func (h *openAIEmbeddingHandler) init(spec *EmbeddingSpec) {
	// TODO
}

func (h *openAIEmbeddingHandler) Embed(text string) ([]float32, error) {
	// TODO
	return nil, nil
}
