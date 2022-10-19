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
	GetSlots() []*Server
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
func NewLoadBalancer(spec *LoadBalanceSpec, servers []*Server, old LoadBalancer) LoadBalancer {
	base := BaseLoadBalancer{
		Servers: servers,
	}
	var oldSlots []*Server
	if old != nil {
		oldSlots = old.GetSlots()
	}
	base.Slots = createSlots(oldSlots, servers)
	base.confSticky(spec)

	switch spec.Policy {
	case LoadBalancePolicyRoundRobin, "":
		return newRoundRobinLoadBalancer(base)
	case LoadBalancePolicyRandom:
		return newRandomLoadBalancer(base)
	case LoadBalancePolicyWeightedRandom:
		return newWeightedRandomLoadBalancer(base)
	case LoadBalancePolicyIPHash:
		return newIPHashLoadBalancer(base)
	case LoadBalancePolicyHeaderHash:
		return newHeaderHashLoadBalancer(base, spec.HeaderHashKey)
	default:
		logger.Errorf("unsupported load balancing policy: %s", spec.Policy)
		return newRoundRobinLoadBalancer(base)
	}
}

// confSticky configures sticky settings
func (base *BaseLoadBalancer) confSticky(spec *LoadBalanceSpec) {
	if spec.Sticky {
		base.Sticky = spec.Sticky

		if spec.StickyCookie == LoadBalanceStickyCookie {
			logger.Errorf("can't use system cookie name: %s, will be ignored", LoadBalanceStickyCookie)
			spec.StickyCookie = ""
		}
		base.StickyCookie = spec.StickyCookie

		dur, err := time.ParseDuration(spec.StickyExpire)
		if err != nil {
		        dur = 2 * time.Hour
			logger.Warnf("failed to parse duration: %s, fallback to default 2h", spec.StickyExpire)
		}
		base.StickyExpire = dur
	}
}

// BaseLoadBalancer implement the common part of load balancer.
type BaseLoadBalancer struct {
	Servers      []*Server
	Slots        []*Server
	Sticky       bool
	StickyCookie string
	StickyExpire time.Duration
}

// GetSlots gets slots
func (base *BaseLoadBalancer) GetSlots() []*Server {
	return base.Slots
}

// ChooseServer chooses the sticky server if enable
func (base *BaseLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(base.Servers) == 0 || !base.Sticky {
		return nil
	}

	if base.StickyCookie != "" {
		if cookie, err := req.Cookie(base.StickyCookie); err == nil {
			return base.chooseServerBySlot(cookie.Value)
		}
		return nil
	}

	cookie, err := req.Cookie(LoadBalanceStickyCookie)
	if err != nil {
		cookie = &http.Cookie{Name: LoadBalanceStickyCookie, Value: uuid.NewString()}
		req.AddCookie(cookie)
	}
	return base.chooseServerBySlot(cookie.Value)
}

// chooseServerBySlot choose server using consistent hash
func (base *BaseLoadBalancer) chooseServerBySlot(key string) *Server {
	hash := murmur3.New32()
	hash.Write([]byte(key))
	return base.Slots[(int)(hash.Sum32()%Total)]
}

// Manipulate customizes response for sticky session
func (base *BaseLoadBalancer) Manipulate(server *Server, req *httpprot.Request, resp *httpprot.Response) {
	if !base.Sticky || base.StickyCookie != "" {
		return
	}

	cookie, err := req.Cookie(LoadBalanceStickyCookie)
	if err != nil {
		logger.Warnf("cookie not found for: %s", LoadBalanceStickyCookie)
		return
	}
	// reset expire
	cookie.Expires = time.Now().Add(base.StickyExpire)
	resp.SetCookie(cookie)
}

// randomLoadBalancer does load balancing in a random manner.
type randomLoadBalancer struct {
	BaseLoadBalancer
}

func newRandomLoadBalancer(base BaseLoadBalancer) *randomLoadBalancer {
	return &randomLoadBalancer{
		BaseLoadBalancer: base,
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

func newRoundRobinLoadBalancer(base BaseLoadBalancer) *roundRobinLoadBalancer {
	return &roundRobinLoadBalancer{
		BaseLoadBalancer: base,
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

func newWeightedRandomLoadBalancer(base BaseLoadBalancer) *WeightedRandomLoadBalancer {
	lb := &WeightedRandomLoadBalancer{
		BaseLoadBalancer: base,
	}
	for _, server := range base.Servers {
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

func newIPHashLoadBalancer(base BaseLoadBalancer) *ipHashLoadBalancer {
	return &ipHashLoadBalancer{
		BaseLoadBalancer: base,
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

	return lb.chooseServerBySlot(req.RealIP())
}

// headerHashLoadBalancer does load balancing based on header hash.
type headerHashLoadBalancer struct {
	BaseLoadBalancer
	key string
}

func newHeaderHashLoadBalancer(base BaseLoadBalancer, key string) *headerHashLoadBalancer {
	return &headerHashLoadBalancer{
		BaseLoadBalancer: base,
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

	return lb.chooseServerBySlot(req.HTTPHeader().Get(lb.key))
}
