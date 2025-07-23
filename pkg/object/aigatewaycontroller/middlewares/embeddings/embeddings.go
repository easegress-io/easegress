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

package embeddings

import (
	"fmt"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings/embedtypes"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings/ollama"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares/embeddings/openai"
)

type EmbeddingSpec = embedtypes.EmbeddingSpec
type EmbeddingHandler = embedtypes.EmbeddingHandler

// registryMap maps embedding provider types to their respective handler constructors.
var registryMap = map[string]func(*EmbeddingSpec) EmbeddingHandler{
	"ollama": ollama.New,
	"openai": openai.New,
}

func New(spec *EmbeddingSpec) EmbeddingHandler {
	return registryMap[spec.ProviderType](spec)
}

func ValidateSpec(spec *EmbeddingSpec) error {
	if spec == nil {
		return fmt.Errorf("embedding spec cannot be nil")
	}
	if _, exists := registryMap[spec.ProviderType]; !exists {
		return fmt.Errorf("unknown embedding provider type: %s", spec.ProviderType)
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
