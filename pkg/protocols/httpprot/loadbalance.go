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
	"math/rand"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/util/hashtool"
)

const (
	// PolicyIPHash is the policy of ip hash.
	PolicyIPHash = "ipHash"
	// PolicyHeaderHash is the policy of header hash.
	PolicyHeaderHash = "headerHash"
)

type LoadBalancer struct {
	general protocols.LoadBalancer

	spec    *LoadBalancerSpec
	servers []protocols.Server
}

type LoadBalancerSpec struct {
	Policy        string `yaml:"policy" jsonschema:"required,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
	HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
}

func (s LoadBalancerSpec) validate() error {
	if s.Policy == PolicyHeaderHash && len(s.HeaderHashKey) == 0 {
		return fmt.Errorf("headerHash needs to specify headerHashKey")
	}

	return nil
}

var _ protocols.LoadBalancer = (*LoadBalancer)(nil)

func NewLoadBalancer(spec interface{}, servers []protocols.Server) (protocols.LoadBalancer, error) {
	s := spec.(*LoadBalancerSpec)
	if err := s.validate(); err != nil {
		return nil, err
	}
	lb := &LoadBalancer{
		spec:    s,
		servers: servers,
	}
	p := s.Policy
	if p == protocols.PolicyRandom || p == protocols.PolicyWeightedRandom || p == protocols.PolicyRoundRobin {
		glb, err := protocols.NewLoadBalancer(spec, servers)
		if err != nil {
			return nil, err
		}
		lb.general = glb
		return lb, nil
	}
	return lb, nil
}

func (lb *LoadBalancer) ChooseServer(req protocols.Request) protocols.Server {
	r := req.(*Request)
	switch lb.spec.Policy {
	case protocols.PolicyRoundRobin, protocols.PolicyRandom, protocols.PolicyWeightedRandom:
		return lb.general.ChooseServer(req)
	case PolicyIPHash:
		return lb.chooseIPHash(r)
	case PolicyHeaderHash:
		return lb.chooseHeaderHash(r)
	default:
		logger.Errorf("unsupported load balancing policy: %s", lb.spec.Policy)
		return lb.servers[rand.Intn(len(lb.servers))]
	}
}

func (lb *LoadBalancer) chooseIPHash(req *Request) protocols.Server {
	sum32 := int(hashtool.Hash32(req.RealIP()))
	return lb.servers[sum32%len(lb.servers)]
}

func (lb *LoadBalancer) chooseHeaderHash(req *Request) protocols.Server {
	value := req.HTTPHeader().Get(lb.spec.HeaderHashKey)
	sum32 := int(hashtool.Hash32(value))
	return lb.servers[sum32%len(lb.servers)]
}
