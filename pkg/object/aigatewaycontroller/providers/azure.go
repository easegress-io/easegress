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
	"fmt"
	"reflect"

	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
)

type (
	AzureProvider struct {
		BaseProvider
	}
)

var _ Provider = (*AzureProvider)(nil)

// Register the AzureProvider type in the ProviderTypeRegistry.
func init() {
	ProviderTypeRegistry[AzureProviderType] = reflect.TypeOf(&AzureProvider{})
}

func (p *AzureProvider) SetSpec(spec *aicontext.ProviderSpec) {
	if spec != nil && spec.BaseURL == "" {
		spec.BaseURL = fmt.Sprintf("https://%s/openai/deployments/%s", spec.Endpoint, spec.DeploymentID)
	}
	p.BaseProvider.SetSpec(spec)
}

func (p *AzureProvider) Type() string {
	return AzureProviderType
}
