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

package embedtypes

type (
	// EmbeddingHandler defines the interface for embedding handlers in the AI Gateway Controller.
	EmbeddingHandler interface {
		EmbedDocuments(text string) ([]float32, error)
		EmbedQuery(text string) ([]float32, error)
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
