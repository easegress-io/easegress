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

package proxy

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

const (
	// LoadBalancePolicyRoundRobin is the load balance policy of round robin.
	LoadBalancePolicyRoundRobin = "roundRobin"
	// LoadBalancePolicyRandom is the load balance policy of random.
	LoadBalancePolicyRandom = "random"
	// LoadBalancePolicyWeightedRandom is the load balance policy of weighted random.
	LoadBalancePolicyWeightedRandom = "weightedRandom"
	// LoadBalancePolicyIPHash is the load balance policy of IP hash.
	LoadBalancePolicyIPHash = "ipHash"
	// LoadBalancePolicyHeaderHash is the load balance policy of HTTP header hash.
	LoadBalancePolicyHeaderHash = "headerHash"
)

// LoadBalancer is the interface of an HTTP load balancer.
type LoadBalancer interface {
	ChooseServer(req *httpprot.Request) *Server
}

// LoadBalanceSpec is the spec to create a load balancer.
type LoadBalanceSpec struct {
	Policy        string `yaml:"policy" jsonschema:"omitempty,enum=,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
	HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
}

// NewLoadBalancer creates a load balancer for servers according to spec.
func NewLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer {
	switch spec.Policy {
	case LoadBalancePolicyRoundRobin, "":
		return newRoundRobinLoadBalancer(servers)
	case LoadBalancePolicyRandom:
		return newRandomLoadBalancer(servers)
	case LoadBalancePolicyWeightedRandom:
		return newWeightedRandomLoadBalancer(servers)
	case LoadBalancePolicyIPHash:
		return newIPHashLoadBalancer(servers)
	case LoadBalancePolicyHeaderHash:
		return newHeaderHashLoadBalancer(servers, spec.HeaderHashKey)
	default:
		logger.Errorf("unsupported load balancing policy: %s", spec.Policy)
		return newRoundRobinLoadBalancer(servers)
	}
}

// BaseLoadBalancer implement the common part of load balancer.
type BaseLoadBalancer struct {
	Servers []*Server
}

// randomLoadBalancer does load balancing in a random manner.
type randomLoadBalancer struct {
	BaseLoadBalancer
}

func newRandomLoadBalancer(servers []*Server) *randomLoadBalancer {
	return &randomLoadBalancer{
		BaseLoadBalancer: BaseLoadBalancer{
			Servers: servers,
		},
	}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *randomLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}
	return lb.Servers[rand.Intn(len(lb.Servers))]
}

// roundRobinLoadBalancer does load balancing in a round robin manner.
type roundRobinLoadBalancer struct {
	BaseLoadBalancer
	counter uint64
}

func newRoundRobinLoadBalancer(servers []*Server) *roundRobinLoadBalancer {
	return &roundRobinLoadBalancer{
		BaseLoadBalancer: BaseLoadBalancer{
			Servers: servers,
		},
	}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *roundRobinLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}
	counter := atomic.AddUint64(&lb.counter, 1) - 1
	return lb.Servers[int(counter)%len(lb.Servers)]
}

// WeightedRandomLoadBalancer does load balancing in a weighted random manner.
type WeightedRandomLoadBalancer struct {
	BaseLoadBalancer
	totalWeight int
}

func newWeightedRandomLoadBalancer(servers []*Server) *WeightedRandomLoadBalancer {
	lb := &WeightedRandomLoadBalancer{
		BaseLoadBalancer: BaseLoadBalancer{
			Servers: servers,
		},
	}
	for _, server := range servers {
		lb.totalWeight += server.Weight
	}
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *WeightedRandomLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}

	randomWeight := rand.Intn(lb.totalWeight)
	for _, server := range lb.Servers {
		randomWeight -= server.Weight
		if randomWeight < 0 {
			return server
		}
	}

	panic(fmt.Errorf("BUG: should not run to here, total weight=%d", lb.totalWeight))
}

// ipHashLoadBalancer does load balancing based on IP hash.
type ipHashLoadBalancer struct {
	BaseLoadBalancer
}

func newIPHashLoadBalancer(servers []*Server) *ipHashLoadBalancer {
	return &ipHashLoadBalancer{
		BaseLoadBalancer: BaseLoadBalancer{
			Servers: servers,
		},
	}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *ipHashLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}
	ip := req.RealIP()
	hash := fnv.New32()
	hash.Write([]byte(ip))
	return lb.Servers[hash.Sum32()%uint32(len(lb.Servers))]
}

// headerHashLoadBalancer does load balancing based on header hash.
type headerHashLoadBalancer struct {
	BaseLoadBalancer
	key string
}

func newHeaderHashLoadBalancer(servers []*Server, key string) *headerHashLoadBalancer {
	return &headerHashLoadBalancer{
		BaseLoadBalancer: BaseLoadBalancer{
			Servers: servers,
		},
		key: key,
	}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *headerHashLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}
	v := req.HTTPHeader().Get(lb.key)
	hash := fnv.New32()
	hash.Write([]byte(v))
	return lb.Servers[hash.Sum32()%uint32(len(lb.Servers))]
}
