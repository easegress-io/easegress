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
)

const (
	// PolicyRoundRobin is the policy of round-robin.
	PolicyRoundRobin = "roundRobin"
	// PolicyRandom is the policy of random.
	PolicyRandom = "random"
	// PolicyWeightedRandom is the policy of weighted random.
	PolicyWeightedRandom = "weightedRandom"
)

type LoadBalancerSpec struct {
	Policy string `yaml:"policy" jsonschema:"required"`
}

var _ LoadBalancer = (*RandomLoadBalancer)(nil)
var _ LoadBalancer = (*RoundRobinLoadBalancer)(nil)
var _ LoadBalancer = (*WeightedRandomLoadBalancer)(nil)

func NewLoadBalancer(spec interface{}, servers []Server) (LoadBalancer, error) {
	sepc := spec.(*LoadBalancerSpec)
	switch sepc.Policy {
	case PolicyRoundRobin:
		return newRoundRobinLoadBalancer(servers), nil
	case PolicyRandom:
		return newRandomLoadBalancer(servers), nil
	case PolicyWeightedRandom:
		return newWeightedRandomLoadBalancer(servers), nil
	default:
		return nil, fmt.Errorf("unsupported load balancing policy: %s", sepc.Policy)
	}
}

type RandomLoadBalancer struct {
	servers []Server
}

func newRandomLoadBalancer(servers []Server) *RandomLoadBalancer {
	return &RandomLoadBalancer{servers: servers}
}

func (rlb *RandomLoadBalancer) ChooseServer(req Request) Server {
	return rlb.servers[rand.Intn(len(rlb.servers))]
}

type RoundRobinLoadBalancer struct {
	servers []Server
	count   uint64
}

func newRoundRobinLoadBalancer(servers []Server) *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{servers: servers}
}

func (rrlb *RoundRobinLoadBalancer) ChooseServer(req Request) Server {
	count := atomic.AddUint64(&rrlb.count, 1)
	count--
	return rrlb.servers[int(count)%len(rrlb.servers)]
}

type WeightedRandomLoadBalancer struct {
	servers    []Server
	weightsSum int
}

func newWeightedRandomLoadBalancer(servers []Server) *WeightedRandomLoadBalancer {
	wrlb := &WeightedRandomLoadBalancer{servers: servers}
	for _, server := range servers {
		wrlb.weightsSum += server.Weight()
	}
	return wrlb
}

func (wrlb *WeightedRandomLoadBalancer) ChooseServer(req Request) Server {
	randomWeight := rand.Intn(wrlb.weightsSum)
	for _, server := range wrlb.servers {
		randomWeight -= server.Weight()
		if randomWeight < 0 {
			return server
		}
	}
	return wrlb.servers[rand.Intn(len(wrlb.servers))]
}
