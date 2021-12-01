/*
 * Copyright (c) 2017, MegaEase
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

package spec

import "github.com/megaease/easegress/pkg/util/stringtool"

type (
	// ServiceSelector is to select service instances
	// according to service names and labels.
	ServiceSelector struct {
		MatchServices       []string          `yaml:"matchServices" jsonschema:"required,uniqueItems=true"`
		MatchInstanceLabels map[string]string `yaml:"matchInstanceLabels" jsonschema:"required"`
	}
)

// NoopServiceSelector selects none of services or instances.
var NoopServiceSelector = &ServiceSelector{}

// MatchInstance returns if selecting the service instance.
func (s *ServiceSelector) MatchInstance(serviceName string, instancelabels map[string]string) bool {
	if !stringtool.StrInSlice(serviceName, s.MatchServices) {
		return false
	}

	for k, v := range s.MatchInstanceLabels {
		if instancelabels[k] != v {
			return false
		}
	}

	return true
}

// MatchService returns if selecting the service.
func (s *ServiceSelector) MatchService(serviceName string) bool {
	return stringtool.StrInSlice(serviceName, s.MatchServices)
}
