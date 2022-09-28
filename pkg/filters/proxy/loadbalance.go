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
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
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
	// LoadBalancerStickyCookie is the cookie name used for sticky session
	LoadBalancerStickyCookie = "EASEGRESS_SESSION"
)

// LoadBalancer is the interface of an HTTP load balancer.
type LoadBalancer interface {
	ChooseServer(req *httpprot.Request) *Server
	CustomizeResponse(req *httpprot.Request, resp *httpprot.Response)
}

// LoadBalanceSpec is the spec to create a load balancer.
type LoadBalanceSpec struct {
	Policy        string `json:"policy" jsonschema:"omitempty,enum=,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
	HeaderHashKey string `json:"headerHashKey" jsonschema:"omitempty"`
	StickyEnabled bool   `json:"stickyEnabled" jsonschema:"omitempty"`
	StickyCookie  string `json:"stickyCookie" jsonschema:"omitempty"`
	StickyExpire  string `json:"stickyExpire" jsonschema:"omitempty,format=duration"`
}

// NewLoadBalancer creates a load balancer for servers according to spec.
func NewLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer {
	ld := BaseLoadBalancer{
		Servers:       servers,
		StickyEnabled: spec.StickyEnabled,
		StickyCookie:  spec.StickyCookie,
		StickyExpire:  spec.StickyExpire,
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
	Servers       []*Server
	StickyEnabled bool
	StickyExpire  string
	StickyCookie  string
}

// ChooseServer chooses the sticky server if enable
func (lb *BaseLoadBalancer) ChooseServer(req *httpprot.Request) *Server {
	if len(lb.Servers) > 0 && lb.StickyEnabled {
		var key string
		if lb.StickyCookie != "" {
			if cookie, err := req.Cookie(lb.StickyCookie); err == nil {
				key = cookie.Value
			}
		} else {
			if cookie, err := req.Cookie(LoadBalancerStickyCookie); err == nil {
				key = cookie.Value
			} else {
				key = uuid.NewString()
				req.AddCookie(&http.Cookie{Name: LoadBalancerStickyCookie, Value: key})
			}
		}
		if key != "" {
			return lb.chooseServerByHash(key)
		}
	}
	return nil
}

// chooseServerByHash choose server using consistent hash on key
func (lb *BaseLoadBalancer) chooseServerByHash(key string) *Server {
	hash := fnv.New32()
	hash.Write([]byte(key))
	return lb.Servers[hash.Sum32()%uint32(len(lb.Servers))]
}

// CustomizeResponse customizes response for sticky session
func (lb *BaseLoadBalancer) CustomizeResponse(req *httpprot.Request, resp *httpprot.Response) {
	if lb.StickyEnabled && lb.StickyCookie == "" {
		if cookie, err := req.Cookie(LoadBalancerStickyCookie); err == nil {
			// reset expire
			if d, err := time.ParseDuration(lb.StickyExpire); err == nil {
				cookie.Expires = time.Now().Add(d)
			}
			resp.SetCookie(cookie)
		}
	}
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
