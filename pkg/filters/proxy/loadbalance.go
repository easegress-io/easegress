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
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
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
	// LoadBalanceStickyCookie is the generated cookie name used for sticky session
	LoadBalanceStickyCookie = "EASEGRESS_SESSION"
)

// LoadBalancer is the interface of an HTTP load balancer.
type LoadBalancer interface {
	ChooseServer(req *httpprot.Request) *Server
	Manipulate(server *Server, req *httpprot.Request, resp *httpprot.Response)
	Server() []*Server
}

// LoadBalanceSpec is the spec to create a load balancer.
type LoadBalanceSpec struct {
	Policy        string `json:"policy" jsonschema:"omitempty,enum=,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
	HeaderHashKey string `json:"headerHashKey" jsonschema:"omitempty"`
	Sticky        bool   `json:"sticky" jsonschema:"omitempty"`
	StickyCookie  string `json:"stickyCookie" jsonschema:"omitempty"`
	StickyExpire  string `json:"stickyExpire" jsonschema:"omitempty,format=duration"`
}

// NewLoadBalancer creates a load balancer for servers according to spec.
func NewLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer {
	d, err := time.ParseDuration(spec.StickyExpire)
	if err != nil {
		logger.Errorf("failed to parse duration: %s", spec.StickyExpire)
	}
	if spec.StickyCookie == LoadBalanceStickyCookie {
		logger.Errorf("can't use system cookie name: %s", LoadBalanceStickyCookie)
	}

	ld := BaseLoadBalancer{
		Servers:      servers,
		Sticky:       spec.Sticky,
		StickyCookie: spec.StickyCookie,
		StickyExpire: d,
	}
	switch spec.Policy {
	case LoadBalancePolicyRoundRobin, "":
		return newRoundRobinLoadBalancer(ld)
	case LoadBalancePolicyRandom:
		return newRandomLoadBalancer(ld)
	case LoadBalancePolicyWeightedRandom:
		return newWeightedRandomLoadBalancer(ld)
	case LoadBalancePolicyIPHash:
		return newIPHashLoadBalancer(ld)
	case LoadBalancePolicyHeaderHash:
		return newHeaderHashLoadBalancer(ld, spec.HeaderHashKey)
	default:
		logger.Errorf("unsupported load balancing policy: %s", spec.Policy)
		return newRoundRobinLoadBalancer(ld)
	}
}

// BaseLoadBalancer implement the common part of load balancer.
type BaseLoadBalancer struct {
	Servers      []*Server
	Sticky       bool
	StickyCookie string
	StickyExpire time.Duration
}

// Server return servers
func (lb *BaseLoadBalancer) Server() []*Server {
	return lb.Servers
}

// ChooseServer chooses the sticky server if enable
func (lb *BaseLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 || !lb.Sticky {
		return nil
	}

	if lb.StickyCookie != "" {
		if cookie, err := req.Cookie(lb.StickyCookie); err == nil {
			return lb.chooseServerByHash(cookie.Value)
		}
		return nil
	}

	cookie, err := req.Cookie(LoadBalanceStickyCookie)
	if err != nil {
		cookie = &http.Cookie{Name: LoadBalanceStickyCookie, Value: uuid.NewString()}
		req.AddCookie(cookie)
	}
	return lb.chooseServerByHash(cookie.Value)
}

// chooseServerByHash choose server using consistent hash
func (lb *BaseLoadBalancer) chooseServerByHash(key string) *Server {
	hash := murmur3.New32()
	hash.Write([]byte(key))
	slot := (int)(hash.Sum32() % Total)
	for _, svr := range lb.Servers {
		for _, s := range svr.slots {
			if s == slot {
				return svr
			}
		}
	}
	panic(fmt.Errorf("BUG: should not run to here, key=%s, slot=%d", key, slot))
}

// Manipulate customizes response for sticky session
func (lb *BaseLoadBalancer) Manipulate(server *Server, req *httpprot.Request, resp *httpprot.Response) {
	if !lb.Sticky || lb.StickyCookie != "" {
		return
	}

	cookie, err := req.Cookie(LoadBalanceStickyCookie)
	if err != nil {
		logger.Warnf("cookie not found for: %s", LoadBalanceStickyCookie)
		return
	}
	// reset expire
	cookie.Expires = time.Now().Add(lb.StickyExpire)
	resp.SetCookie(cookie)
}

// randomLoadBalancer does load balancing in a random manner.
type randomLoadBalancer struct {
	BaseLoadBalancer
}

func newRandomLoadBalancer(ld BaseLoadBalancer) *randomLoadBalancer {
	return &randomLoadBalancer{
		BaseLoadBalancer: ld,
	}
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

func newRoundRobinLoadBalancer(ld BaseLoadBalancer) *roundRobinLoadBalancer {
	return &roundRobinLoadBalancer{
		BaseLoadBalancer: ld,
	}
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

func newWeightedRandomLoadBalancer(ld BaseLoadBalancer) *WeightedRandomLoadBalancer {
	lb := &WeightedRandomLoadBalancer{
		BaseLoadBalancer: ld,
	}
	for _, server := range ld.Servers {
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

func newIPHashLoadBalancer(ld BaseLoadBalancer) *ipHashLoadBalancer {
	return &ipHashLoadBalancer{
		BaseLoadBalancer: ld,
	}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *ipHashLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}
	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	return lb.chooseServerByHash(req.RealIP())
}

// headerHashLoadBalancer does load balancing based on header hash.
type headerHashLoadBalancer struct {
	BaseLoadBalancer
	key string
}

func newHeaderHashLoadBalancer(ld BaseLoadBalancer, key string) *headerHashLoadBalancer {
	return &headerHashLoadBalancer{
		BaseLoadBalancer: ld,
		key:              key,
	}
}

// ChooseServer implements the LoadBalancer interface.
func (lb *headerHashLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) == 0 {
		return nil
	}
	if server := lb.BaseLoadBalancer.ChooseServer(req); server != nil {
		return server
	}

	return lb.chooseServerByHash(req.HTTPHeader().Get(lb.key))
}
