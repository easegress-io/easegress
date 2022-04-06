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
	"fmt"

	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/util/hashtool"
)

const (
	// PolicyIPHash is the policy of ip hash.
	PolicyIPHash = "ipHash"
	// PolicyHeaderHash is the policy of header hash.
	PolicyHeaderHash = "headerHash"
)

type LoadBalancerSpec struct {
	protocols.LoadBalancerSpec
	HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
}

func NewLoadBalancer(spec interface{}, servers []protocols.Server) (protocols.LoadBalancer, error) {
	s := spec.(*LoadBalancerSpec)
	switch s.Policy {
	case protocols.PolicyRoundRobin, protocols.PolicyRandom, protocols.PolicyWeightedRandom:
		return protocols.NewLoadBalancer(spec, servers)
	case PolicyIPHash:
		return newIPHashLoadBalancer(servers), nil
	case PolicyHeaderHash:
		return newHeaderHashLoadBalancer(servers, s.HeaderHashKey), nil
	default:
		return nil, fmt.Errorf("unsupported load balancing policy: %s", s.Policy)
	}
}

type IPHashLoadBalancer struct {
	servers []protocols.Server
}

func newIPHashLoadBalancer(servers []protocols.Server) *IPHashLoadBalancer {
	return &IPHashLoadBalancer{servers: servers}
}

func (lb *IPHashLoadBalancer) ChooseServer(req protocols.Request) protocols.Server {
	r := req.(*Request)
	sum32 := int(hashtool.Hash32(r.RealIP()))
	return lb.servers[sum32%len(lb.servers)]
}

type HeaderHashLoadBalancer struct {
	servers []protocols.Server
	key     string
}

func newHeaderHashLoadBalancer(servers []protocols.Server, key string) *HeaderHashLoadBalancer {
	return &HeaderHashLoadBalancer{servers: servers, key: key}
}

func (lb *HeaderHashLoadBalancer) ChooseServer(req protocols.Request) protocols.Server {
	r := req.(*Request)
	value := r.HTTPHeader().Get(lb.key)
	sum32 := int(hashtool.Hash32(value))
	return lb.servers[sum32%len(lb.servers)]
}
