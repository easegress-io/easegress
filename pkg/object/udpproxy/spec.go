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

package udpproxy

import (
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/layer4backend"
)

type (

	// Spec describes the udp server
	Spec struct {
		Name string `yaml:"name" json:"name" jsonschema:"required"`
		Port uint16 `yaml:"port" json:"port" jsonschema:"required"`

		HasResponse           bool   `yaml:"hasResponse" jsonschema:"required"`
		DownstreamIdleTimeout uint32 `yaml:"downstreamIdleTimeout" jsonschema:"omitempty,minimum=1"`
		UpstreamIdleTimeout   uint32 `yaml:"upstreamIdleTimeout" jsonschema:"omitempty,minimum=1"`

		Pool     *layer4backend.PoolSpec `yaml:"pool" jsonschema:"required"`
		IPFilter *ipfilter.Spec          `yaml:"ipFilters,omitempty" jsonschema:"omitempty"`
	}
)

// Validate validates Layer4 Server.
func (spec *Spec) Validate() error {
	if poolErr := spec.Pool.Validate(); poolErr != nil {
		return poolErr
	}

	return nil
}
