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
	CoHereProvider struct {
		BaseProvider
	}
)

var _ Provider = (*CoHereProvider)(nil)

// NewCoHereProvider initializes an NewCoHereProvider with the given ProviderSpec.
func NewCoHereProvider(spec *ProviderSpec) *CoHereProvider {
	if spec != nil && spec.BaseURL == "" {
		spec.BaseURL = "https://api.cohere.ai/compatibility"
	}
	return &CoHereProvider{
		BaseProvider: BaseProvider{
			providerSpec: spec,
		},
	}
}

func (p *CoHereProvider) Type() string {
	return "cohere"
}
