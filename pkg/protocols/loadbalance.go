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

package protocols

import (
	"fmt"
	"math/rand"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/logger"
)

const (
	// PolicyRoundRobin is the policy of round-robin.
	PolicyRoundRobin = "roundRobin"
	// PolicyRandom is the policy of random.
	PolicyRandom = "random"
	// PolicyWeightedRandom is the policy of weighted random.
	PolicyWeightedRandom = "weightedRandom"
)

type GeneralLoadBalancer struct {
	spec       *GeneralLoadBalancerSpec
	servers    []Server
	count      int32
	weightsSum int
}

type GeneralLoadBalancerSpec struct {
	Policy string `yaml:"policy" jsonschema:"required,enum=roundRobin,enum=random,enum=weightedRandom"`
}

var _ LoadBalancer = (*GeneralLoadBalancer)(nil)

func NewLoadBalancer(spec interface{}, servers []Server) (LoadBalancer, error) {
	lb := &GeneralLoadBalancer{}
	lb.spec = spec.(*GeneralLoadBalancerSpec)
	lb.servers = servers

	p := lb.spec.Policy
	if p != PolicyRoundRobin && p != PolicyRandom && p != PolicyWeightedRandom {
		return nil, fmt.Errorf("unsupported load balancing policy: %s", p)
	}

	for _, s := range servers {
		lb.weightsSum += s.Weight()
	}
	return lb, nil

}

func (lb *GeneralLoadBalancer) ChooseServer(req Request) Server {
	switch lb.spec.Policy {
	case PolicyRoundRobin:
		return lb.chooseRoundRobin()
	case PolicyRandom:
		return lb.chooseRandom()
	case PolicyWeightedRandom:
		return lb.chooseWeightedRandom()
	default:
		logger.Errorf("unsupported load balancing policy: %s", lb.spec.Policy)
		return lb.chooseRoundRobin()
	}
}

func (lb *GeneralLoadBalancer) chooseRoundRobin() Server {
	count := atomic.AddInt32(&lb.count, 1)
	count--
	return lb.servers[count%int32(len(lb.servers))]
}

func (lb *GeneralLoadBalancer) chooseRandom() Server {
	return lb.servers[rand.Intn(len(lb.servers))]
}

func (lb *GeneralLoadBalancer) chooseWeightedRandom() Server {
	randomWeight := rand.Intn(lb.weightsSum)
	for _, server := range lb.servers {
		randomWeight -= server.Weight()
		if randomWeight < 0 {
			return server
		}
	}

	logger.Errorf("BUG: weighted random can't pick a server: sum(%d) servers(%+v)",
		lb.weightsSum, lb.servers)
	return lb.chooseRandom()
}
