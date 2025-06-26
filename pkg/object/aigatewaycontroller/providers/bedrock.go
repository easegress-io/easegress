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

package providers

type (
	BedrockProvider struct {
		BaseProvider
	}
)

var _ Provider = (*BedrockProvider)(nil)

// NewOllamaProvider initializes an OllamaProvider with the given ProviderSpec.
func NewBedrockProvider(spec *ProviderSpec) *BedrockProvider {
	return &BedrockProvider{
		BaseProvider: BaseProvider{
			providerSpec: spec,
		},
	}
}

func (p *BedrockProvider) Type() string {
	return "bedrock"
}
