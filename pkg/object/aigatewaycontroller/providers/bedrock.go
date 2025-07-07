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

import (
	"reflect"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
)

type (
	BedrockProvider struct {
		BaseProvider
	}
)

var _ Provider = (*BedrockProvider)(nil)

// Register the BedrockProvider type in the ProviderTypeRegistry.
func init() {
	ProviderTypeRegistry[BedrockProviderType] = reflect.TypeOf(BedrockProvider{})
}

func (p *BedrockProvider) SetSpec(spec *aicontext.ProviderSpec) {
	p.BaseProvider.SetSpec(spec)
}

func (p *BedrockProvider) Type() string {
	return BedrockProviderType
}
