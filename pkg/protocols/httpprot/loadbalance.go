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

package httpprot

import (
	"hash/fnv"

	"github.com/megaease/easegress/pkg/protocols"
	yaml "gopkg.in/yaml.v2"
)

const (
	// PolicyIPHash is the policy of ip hash.
	PolicyIPHash = "ipHash"
	// PolicyHeaderHash is the policy of header hash.
	PolicyHeaderHash = "headerHash"
)

// LoadBalancerSpec is the spec of an HTTP load balancer.
type LoadBalancerSpec struct {
	protocols.LoadBalancerSpec `yaml:",inline"`
	HeaderHashKey              string `yaml:"headerHashKey" jsonschema:"omitempty"`
}

// NewLoadBalancer creates a new load balancer for servers from spec.
func NewLoadBalancer(spec interface{}, servers []protocols.Server) (protocols.LoadBalancer, error) {
	lbs, ok := spec.(*LoadBalancerSpec)
	if !ok {
		data, err := yaml.Marshal(spec)
		if err != nil {
			return nil, err
		}
		lbs = &LoadBalancerSpec{}
		if err = yaml.Unmarshal(data, lbs); err != nil {
			return nil, err
		}
	}

	switch lbs.Policy {
	case PolicyIPHash:
		return newIPHashLoadBalancer(servers), nil
	case PolicyHeaderHash:
		return newHeaderHashLoadBalancer(servers, lbs.HeaderHashKey), nil
	default:
		return protocols.NewLoadBalancer(lbs.LoadBalancerSpec, servers)
	}
}

// IPHashLoadBalancer does load balancing based on IP hash.
type IPHashLoadBalancer struct {
	protocols.BaseLoadBalancer
}

func newIPHashLoadBalancer(servers []protocols.Server) *IPHashLoadBalancer {
	return &IPHashLoadBalancer{
		BaseLoadBalancer: protocols.BaseLoadBalancer{
			Servers: servers,
		},
	}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *IPHashLoadBalancer) ChooseServer(req protocols.Request) protocols.Server {
	if len(lb.Servers) == 0 {
		return nil
	}
	ip := req.(*Request).RealIP()
	hash := fnv.New32()
	hash.Write([]byte(ip))
	return lb.Servers[hash.Sum32()%uint32(len(lb.Servers))]
}

// HeaderHashLoadBalancer does load balancing based on header hash.
type HeaderHashLoadBalancer struct {
	protocols.BaseLoadBalancer
	key string
}

func newHeaderHashLoadBalancer(servers []protocols.Server, key string) *HeaderHashLoadBalancer {
	return &HeaderHashLoadBalancer{
		BaseLoadBalancer: protocols.BaseLoadBalancer{
			Servers: servers,
		},
		key: key,
	}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *HeaderHashLoadBalancer) ChooseServer(req protocols.Request) protocols.Server {
	if len(lb.Servers) == 0 {
		return nil
	}
	v := req.(*Request).HTTPHeader().Get(lb.key)
	hash := fnv.New32()
	hash.Write([]byte(v))
	return lb.Servers[hash.Sum32()%uint32(len(lb.Servers))]
}
