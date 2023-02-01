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

package proxies

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
)

// LoadBalancer is the interface of a load balancer.
type LoadBalancer interface {
	ChooseServer(req protocols.Request) *Server
	ReturnServer(server *Server, req protocols.Request, resp protocols.Response)
	Close()
}

// LoadBalanceSpec is the spec to create a load balancer.
type LoadBalanceSpec struct {
	Policy        string             `json:"policy" jsonschema:"omitempty,enum=,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash,enum=headerHash"`
	HeaderHashKey string             `json:"headerHashKey" jsonschema:"omitempty"`
	StickySession *StickySessionSpec `json:"stickySession" jsonschema:"omitempty"`
	HealthCheck   *HealthCheckSpec   `json:"healthCheck" jsonschema:"omitempty"`
}

// LoadBalancePolicy is the interface of a load balance policy.
type LoadBalancePolicy interface {
	ChooseServer(req protocols.Request, sg *ServerGroup) *Server
	Close()
}

// GeneralLoadBalancer implements a general purpose load balancer.
type GeneralLoadBalancer struct {
	spec           *LoadBalanceSpec
	servers        []*Server
	healthyServers atomic.Pointer[ServerGroup]

	done chan struct{}

	lbp LoadBalancePolicy
	ss  SessionSticker
	hc  HealthChecker
}

// NewGeneralLoadBalancer creates a new GeneralLoadBalancer.
func NewGeneralLoadBalancer(spec *LoadBalanceSpec, servers []*Server) *GeneralLoadBalancer {
	lb := &GeneralLoadBalancer{
		spec:    spec,
		servers: servers,
	}
	lb.healthyServers.Store(newServerGroup(servers))
	return lb
}

// Init initializes the load balancer.
func (glb *GeneralLoadBalancer) Init(
	fnNewSessionSticker func(*StickySessionSpec) SessionSticker,
	fnNewHealthChecker func(*HealthCheckSpec) HealthChecker,
	aa any,
) {
	//	glb.lbp = lbp

	if glb.spec.StickySession != nil {
		ss := fnNewSessionSticker(glb.spec.StickySession)
		ss.UpdateServers(glb.servers)
		glb.ss = ss
	}

	if glb.spec.HealthCheck == nil {
		return
	}

	if glb.spec.HealthCheck.Fails <= 0 {
		glb.spec.HealthCheck.Fails = 1
	}

	if glb.spec.HealthCheck.Passes <= 0 {
		glb.spec.HealthCheck.Passes = 1
	}

	glb.hc = fnNewHealthChecker(glb.spec.HealthCheck)

	interval, _ := time.ParseDuration(glb.spec.HealthCheck.Interval)
	if interval <= 0 {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	glb.done = make(chan struct{})
	go func() {
		for {
			select {
			case <-glb.done:
				ticker.Stop()
				return
			case <-ticker.C:
				glb.checkServers()
			}
		}
	}()
}

func (glb *GeneralLoadBalancer) checkServers() {
	changed := false

	servers := make([]*Server, 0, len(glb.servers))
	for _, svr := range glb.servers {
		succ := glb.hc.Check(svr)
		if succ {
			if svr.HealthCounter < 0 {
				svr.HealthCounter = 0
			}
			svr.HealthCounter++
			if svr.Unhealth && svr.HealthCounter >= glb.spec.HealthCheck.Passes {
				logger.Warnf("server:%v becomes healthy.", svr.ID())
				svr.Unhealth = false
				changed = true
			}
		} else {
			if svr.HealthCounter > 0 {
				svr.HealthCounter = 0
			}
			svr.HealthCounter--
			if svr.Healthy() && svr.HealthCounter <= -glb.spec.HealthCheck.Fails {
				logger.Warnf("server:%v becomes healthy.", svr.ID())
				svr.Unhealth = true
				changed = true
			}
		}

		if svr.Healthy() {
			servers = append(servers, svr)
		}
	}

	if !changed {
		return
	}

	glb.healthyServers.Store(newServerGroup(servers))
	if glb.ss != nil {
		glb.ss.UpdateServers(servers)
	}
}

// ChooseServer chooses a server according to the load balancing spec.
func (glb *GeneralLoadBalancer) ChooseServer(req protocols.Request) *Server {
	sg := glb.healthyServers.Load()
	if len(sg.Servers) == 0 {
		return nil
	}

	if glb.ss != nil {
		if svr := glb.ss.GetServer(req, sg); svr != nil {
			return svr
		}
	}

	return glb.lbp.ChooseServer(req, sg)
}

// ReturnServer returns a server to the load balancer.
func (glb *GeneralLoadBalancer) ReturnServer(server *Server, req protocols.Request, resp protocols.Response) {
	if glb.ss != nil {
		glb.ss.ReturnServer(server, req, resp)
	}
}

// Close closes the load balancer
func (glb *GeneralLoadBalancer) Close() {
	if glb.hc != nil {
		glb.hc.Close()
	}
	if glb.ss != nil {
		glb.ss.Close()
	}
	glb.lbp.Close()
}

// RandomLoadBalancePolicy is a load balance policy that chooses a server randomly.
type RandomLoadBalancePolicy struct {
}

// ChooseServer chooses a server randomly.
func (lbp *RandomLoadBalancePolicy) ChooseServer(req protocols.Request, sg *ServerGroup) *Server {
	return sg.Servers[rand.Intn(len(sg.Servers))]
}

// RoundRobinLoadBalancePolicy is a load balance policy that chooses a server by round robin.
type RoundRobinLoadBalancePolicy struct {
	counter uint64
}

// ChooseServer chooses a server by round robin.
func (lbp *RoundRobinLoadBalancePolicy) ChooseServer(req protocols.Request, sg *ServerGroup) *Server {
	counter := atomic.AddUint64(&lbp.counter, 1) - 1
	return sg.Servers[int(counter)%len(sg.Servers)]
}

// WeightedRandomLoadBalancePolicy is a load balance policy that chooses a server randomly by weight.
type WeightedRandomLoadBalancePolicy struct {
}

// ChooseServer chooses a server randomly by weight.
func (lbp *WeightedRandomLoadBalancePolicy) ChooseServer(req protocols.Request, sg *ServerGroup) *Server {
	w := rand.Intn(sg.TotalWeight)
	for _, svr := range sg.Servers {
		w -= svr.Weight
		if w < 0 {
			return svr
		}
	}

	panic(fmt.Errorf("BUG: should not run to here, total weight=%d", sg.TotalWeight))
}

// IPHashLoadBalancePolicy is a load balance policy that chooses a server by ip hash.
type IPHashLoadBalancePolicy struct {
}

// ChooseServer chooses a server by ip hash.
func (lbp *IPHashLoadBalancePolicy) ChooseServer(req protocols.Request, sg *ServerGroup) *Server {
	type realIPer interface {
		RealIP() string
	}

	ri, ok := req.(realIPer)
	if !ok {
		panic("IPHashLoadBalancePolicy only support request with RealIP()")
	}

	ip := ri.RealIP()
	hash := fnv.New32()
	hash.Write([]byte(ip))
	return sg.Servers[hash.Sum32()%uint32(len(sg.Servers))]
}

// HeaderHashLoadBalancePolicy is a load balance policy that chooses a server by header hash.
type HeaderHashLoadBalancePolicy struct {
	spec *LoadBalanceSpec
}

// ChooseServer chooses a server by header hash.
func (lbp *HeaderHashLoadBalancePolicy) ChooseServer(req protocols.Request, sg *ServerGroup) *Server {
	v, ok := req.Header().Get(lbp.spec.HeaderHashKey).(string)
	if !ok {
		panic("HeaderHashLoadBalancePolicy only support headers with string values")
	}

	hash := fnv.New32()
	hash.Write([]byte(v))
	return sg.Servers[hash.Sum32()%uint32(len(sg.Servers))]
}
