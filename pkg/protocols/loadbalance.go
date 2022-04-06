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

	"gopkg.in/yaml.v2"
)

const (
	// PolicyRoundRobin is the policy of round-robin.
	PolicyRoundRobin = "roundRobin"
	// PolicyRandom is the policy of random.
	PolicyRandom = "random"
	// PolicyWeightedRandom is the policy of weighted random.
	PolicyWeightedRandom = "weightedRandom"
)

// LoadBalancerSpec is the spec of a load balancer.
type LoadBalancerSpec struct {
	Policy string `yaml:"policy" jsonschema:"required"`
}

var _ LoadBalancer = (*RandomLoadBalancer)(nil)
var _ LoadBalancer = (*RoundRobinLoadBalancer)(nil)
var _ LoadBalancer = (*WeightedRandomLoadBalancer)(nil)

// NewLoadBalancer creates a new load balancer for servers from spec.
func NewLoadBalancer(spec interface{}, servers []Server) (LoadBalancer, error) {
	lbs, ok := spec.(*LoadBalancerSpec)
	if !ok {
		data, err := yaml.Marshal(spec)
		if err != nil {
			return nil, err
		}
		lbs := &LoadBalancerSpec{}
		if err = yaml.Unmarshal(data, lbs); err != nil {
			return nil, err
		}
	}

	switch lbs.Policy {
	case PolicyRoundRobin:
		return newRoundRobinLoadBalancer(servers), nil
	case PolicyRandom:
		return newRandomLoadBalancer(servers), nil
	case PolicyWeightedRandom:
		return newWeightedRandomLoadBalancer(servers), nil
	default:
		return nil, fmt.Errorf("unsupported load balancing policy: %s", lbs.Policy)
	}
}

// RandomLoadBalancer does load balancing in a random manner.
type RandomLoadBalancer struct {
	servers []Server
}

func newRandomLoadBalancer(servers []Server) *RandomLoadBalancer {
	return &RandomLoadBalancer{servers: servers}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *RandomLoadBalancer) ChooseServer(req Request) Server {
	return lb.servers[rand.Intn(len(lb.servers))]
}

// RoundRobinLoadBalancer does load balancing in a round robin manner.
type RoundRobinLoadBalancer struct {
	servers []Server
	count   uint64
}

func newRoundRobinLoadBalancer(servers []Server) *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{servers: servers}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *RoundRobinLoadBalancer) ChooseServer(req Request) Server {
	count := atomic.AddUint64(&lb.count, 1) - 1
	return lb.servers[int(count)%len(lb.servers)]
}

// WeightedRandomLoadBalancer does load balancing in a weighted random manner.
type WeightedRandomLoadBalancer struct {
	servers     []Server
	totalWeight int
}

func newWeightedRandomLoadBalancer(servers []Server) *WeightedRandomLoadBalancer {
	lb := &WeightedRandomLoadBalancer{servers: servers}
	for _, server := range servers {
		lb.totalWeight += server.Weight()
	}
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *WeightedRandomLoadBalancer) ChooseServer(req Request) Server {
	randomWeight := rand.Intn(lb.totalWeight)
	for _, server := range lb.servers {
		randomWeight -= server.Weight()
		if randomWeight < 0 {
			return server
		}
	}
	panic(fmt.Errorf("BUG: should not run to here, total weight=%d", lb.totalWeight))
}
