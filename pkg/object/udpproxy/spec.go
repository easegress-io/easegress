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
	"fmt"

	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (

	// Spec describes the udp server
	Spec struct {
		Name string `yaml:"name" json:"name" jsonschema:"required"`
		Port uint16 `yaml:"port" json:"port" jsonschema:"required"`

		ContinuousMode        bool   `yaml:"continuousMode" jsonschema:"omitempty"`
		DownstreamIdleTimeout uint32 `yaml:"downstreamIdleTimeout" jsonschema:"omitempty,minimum=1"`
		UpstreamIdleTimeout   uint32 `yaml:"upstreamIdleTimeout" jsonschema:"omitempty,minimum=1"`

		Pool     *PoolSpec      `yaml:"pool" jsonschema:"required"`
		IPFilter *ipfilter.Spec `yaml:"ipFilters,omitempty" jsonschema:"omitempty"`
	}

	// PoolSpec describes a pool of servers.
	PoolSpec struct {
		ServiceRegistry string       `yaml:"serviceRegistry" jsonschema:"omitempty"`
		ServiceName     string       `yaml:"serviceName" jsonschema:"omitempty"`
		Servers         []*Server    `yaml:"servers" jsonschema:"omitempty"`
		ServersTags     []string     `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
		LoadBalance     *LoadBalance `yaml:"loadBalance" jsonschema:"required"`
	}
)

// Validate validates Layer4 Server.
func (spec *Spec) Validate() error {
	if poolErr := spec.Pool.Validate(); poolErr != nil {
		return poolErr
	}

	return nil
}

// Validate validates poolSpec.
func (s PoolSpec) Validate() error {
	if s.ServiceName == "" && len(s.Servers) == 0 {
		return fmt.Errorf("both serviceName and servers are empty")
	}

	serversGotWeight := 0
	for _, server := range s.Servers {
		if server.Weight > 0 {
			serversGotWeight++
		}
	}
	if serversGotWeight > 0 && serversGotWeight < len(s.Servers) {
		return fmt.Errorf("not all servers have weight(%d/%d)",
			serversGotWeight, len(s.Servers))
	}

	if s.ServiceName == "" {
		servers := newStaticServers(s.Servers, s.ServersTags, s.LoadBalance)
		if servers.len() == 0 {
			return fmt.Errorf("serversTags picks none of servers")
		}
	}
	return nil
}
