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

	"github.com/buraksezer/consistent"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/spaolacci/murmur3"
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

// StickySessionSpec is the spec for sticky session.
type StickySessionSpec struct {
	Mode string `json:"mode" jsonschema:"required,enum=CookieConsistentHash"`
	// AppCookieName need to be omitempty when we support other sticky mode.
	AppCookieName string `json:"appCookieName" jsonschema:"required"`
}

// LoadBalanceSpec is the spec to create a load balancer.
type LoadBalanceSpec struct {
	Policy        string             `json:"policy" jsonschema:"omitempty,enum=,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
	HeaderHashKey string             `json:"headerHashKey" jsonschema:"omitempty"`
	StickySession *StickySessionSpec `json:"stickySession" jsonschema:"omitempty"`
}

// NewLoadBalancer creates a load balancer for servers according to spec.
func NewLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer {
	switch spec.Policy {
	case LoadBalancePolicyRoundRobin, "":
		return newRoundRobinLoadBalancer(spec, servers)
	case LoadBalancePolicyRandom:
		return newRandomLoadBalancer(spec, servers)
	case LoadBalancePolicyWeightedRandom:
		return newWeightedRandomLoadBalancer(spec, servers)
	case LoadBalancePolicyIPHash:
		return newIPHashLoadBalancer(spec, servers)
	case LoadBalancePolicyHeaderHash:
		return newHeaderHashLoadBalancer(spec, servers)
	default:
		logger.Errorf("unsupported load balancing policy: %s", spec.Policy)
		return newRoundRobinLoadBalancer(spec, servers)
	}
}

// hashMember is member used for hash
type hashMember struct {
	server *Server
}

// String implements consistent.Member interface
func (m hashMember) String() string {
	return m.server.ID()
}

// hasher is used for hash
type hasher struct{}

// Sum64 implement hash function using murmur3
func (h hasher) Sum64(data []byte) uint64 {
	return murmur3.Sum64(data)
}

// BaseLoadBalancer implement the common part of load balancer.
type BaseLoadBalancer struct {
	spec           *LoadBalanceSpec
	Servers        []*Server
	consistentHash *consistent.Consistent
}

func (blb *BaseLoadBalancer) init(spec *LoadBalanceSpec, servers []*Server) {
	blb.spec = spec
	blb.Servers = servers

	if spec.StickySession == nil || len(servers) == 0 {
		return
	}

	// For now, we only support HeaderConsistentHash & CookieConsistentHash
	members := make([]consistent.Member, len(servers))
	for i, s := range servers {
		members[i] = hashMember{server: s}
	}

	cfg := consistent.Config{
		PartitionCount:    1024,
		ReplicationFactor: 50,
		Load:              1.25,
		Hasher:            hasher{},
	}
	blb.consistentHash = consistent.New(members, cfg)
}

// ChooseServer chooses the sticky server if enable
func (blb *BaseLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if blb.spec.StickySession == nil {
		return nil
	}

	cookie, err := req.Cookie(blb.spec.StickySession.AppCookieName)
	if err != nil {
		return nil
	}

	m := blb.consistentHash.LocateKey([]byte(cookie.Value))
	if m != nil {
		return m.(hashMember).server
	}

	return nil
}

// randomLoadBalancer does load balancing in a random manner.
type randomLoadBalancer struct {
	BaseLoadBalancer
}

func newRandomLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *randomLoadBalancer {
	lb := &randomLoadBalancer{}
	lb.init(spec, servers)
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *randomLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	return lb.Servers[rand.Intn(len(lb.Servers))]
}

// roundRobinLoadBalancer does load balancing in a round robin manner.
type roundRobinLoadBalancer struct {
	BaseLoadBalancer
	counter uint64
}

func newRoundRobinLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *roundRobinLoadBalancer {
	lb := &roundRobinLoadBalancer{}
	lb.init(spec, servers)
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *roundRobinLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	counter := atomic.AddUint64(&lb.counter, 1) - 1
	return lb.Servers[int(counter)%len(lb.Servers)]
}

// WeightedRandomLoadBalancer does load balancing in a weighted random manner.
type WeightedRandomLoadBalancer struct {
	BaseLoadBalancer
	totalWeight int
}

func newWeightedRandomLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *WeightedRandomLoadBalancer {
	lb := &WeightedRandomLoadBalancer{}
	lb.init(spec, servers)
	for _, server := range lb.Servers {
		lb.totalWeight += server.Weight
	}
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *WeightedRandomLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
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

func newIPHashLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *ipHashLoadBalancer {
	lb := &ipHashLoadBalancer{}
	lb.init(spec, servers)
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *ipHashLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
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

func newHeaderHashLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *headerHashLoadBalancer {
	lb := &headerHashLoadBalancer{}
	lb.init(spec, servers)
	lb.key = spec.HeaderHashKey
	return lb
}

// ChooseServer implements the LoadBalancer interface.
func (lb *headerHashLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}

	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	v := req.HTTPHeader().Get(lb.key)
	hash := fnv.New32()
	hash.Write([]byte(v))
	return lb.Servers[hash.Sum32()%uint32(len(lb.Servers))]
}
