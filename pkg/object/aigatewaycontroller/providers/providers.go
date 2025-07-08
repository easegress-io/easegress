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

func NewProvider(spec *aicontext.ProviderSpec) Provider {
	if providerType, exists := ProviderTypeRegistry[spec.ProviderType]; exists {
		provider := reflect.New(providerType).Interface().(Provider)
		provider.init(spec)
		return provider
	}
	return nil
}

func ValidateSpec(spec *aicontext.ProviderSpec) error {
	if spec == nil {
		return fmt.Errorf("provider spec cannot be nil")
	}
	if providerType, exist := ProviderTypeRegistry[spec.ProviderType]; exist {
		provider := reflect.New(providerType).Interface().(Provider)
		return provider.validate(spec)
	}
	return fmt.Errorf("unknown provider type: %s", spec.ProviderType)
}
