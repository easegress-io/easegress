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

package layer4rawserver

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/hashtool"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// PolicyRoundRobin is the policy of round-robin.
	PolicyRoundRobin = "roundRobin"
	// PolicyRandom is the policy of random.
	PolicyRandom = "random"
	// PolicyWeightedRandom is the policy of weighted random.
	PolicyWeightedRandom = "weightedRandom"
	// PolicyIPHash is the policy of ip hash.
	PolicyIPHash = "ipHash"
)

type (
	servers struct {
		poolSpec *PoolSpec
		super    *supervisor.Supervisor

		mutex           sync.Mutex
		serviceRegistry *serviceregistry.ServiceRegistry
		serviceWatcher  serviceregistry.ServiceWatcher
		static          *staticServers

		done chan struct{}
	}

	staticServers struct {
		count      uint64
		weightsSum int
		servers    []*Server
		lb         LoadBalance
	}

	// Server is proxy server.
	Server struct {
		Addr   string   `yaml:"url" jsonschema:"required,format=hostport"`
		Tags   []string `yaml:"tags" jsonschema:"omitempty,uniqueItems=true"`
		Weight int      `yaml:"weight" jsonschema:"omitempty,minimum=0,maximum=100"`
	}

	// LoadBalance is load balance for multiple servers.
	LoadBalance struct {
		Policy        string `yaml:"policy" jsonschema:"required,enum=roundRobin,enum=random,enum=weightedRandom,enum=ipHash"`
		HeaderHashKey string `yaml:"headerHashKey" jsonschema:"omitempty"`
	}
)

func newServers(super *supervisor.Supervisor, poolSpec *PoolSpec) *servers {
	s := &servers{
		poolSpec: poolSpec,
		super:    super,
		done:     make(chan struct{}),
	}

	s.useStaticServers()
	if poolSpec.ServiceRegistry == "" || poolSpec.ServiceName == "" {
		return s
	}

	s.serviceRegistry = s.super.MustGetSystemController(serviceregistry.Kind).
		Instance().(*serviceregistry.ServiceRegistry)
	s.tryUseService()
	s.serviceWatcher = s.serviceRegistry.NewServiceWatcher(s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)

	go s.watchService()
	return s
}

func (s *Server) String() string {
	return fmt.Sprintf("%s,%v,%d", s.Addr, s.Tags, s.Weight)
}

func (s *servers) watchService() {
	for {
		select {
		case <-s.done:
			return
		case event := <-s.serviceWatcher.Watch():
			s.handleEvent(event)
		}
	}
}

func (s *servers) handleEvent(event *serviceregistry.ServiceEvent) {
	s.useService(event.Instances)
}

func (s *servers) tryUseService() {
	serviceInstanceSpecs, err := s.serviceRegistry.ListServiceInstances(s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)

	if err != nil {
		logger.Errorf("get service %s/%s failed: %v",
			s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName, err)
		s.useStaticServers()
		return
	}
	s.useService(serviceInstanceSpecs)
}

func (s *servers) useService(serviceInstanceSpecs map[string]*serviceregistry.ServiceInstanceSpec) {
	var servers []*Server
	for _, instance := range serviceInstanceSpecs {
		servers = append(servers, &Server{
			Addr:   fmt.Sprintf("%s:%d", instance.Address, instance.Port),
			Tags:   instance.Tags,
			Weight: instance.Weight,
		})
	}
	if len(servers) == 0 {
		logger.Errorf("%s/%s: empty service instance",
			s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)
		s.useStaticServers()
		return
	}

	dynamicServers := newStaticServers(servers, s.poolSpec.ServersTags, s.poolSpec.LoadBalance)
	if dynamicServers.len() == 0 {
		logger.Errorf("%s/%s: no service instance satisfy tags: %v",
			s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName, s.poolSpec.ServersTags)
		s.useStaticServers()
	}

	logger.Infof("use dynamic service: %s/%s", s.poolSpec.ServiceRegistry, s.poolSpec.ServiceName)

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.static = dynamicServers
}

func (s *servers) useStaticServers() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.static = newStaticServers(s.poolSpec.Servers, s.poolSpec.ServersTags, s.poolSpec.LoadBalance)
}

func (s *servers) snapshot() *staticServers {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.static
}

func (s *servers) len() int {
	static := s.snapshot()
	return static.len()
}

func (s *servers) next(cliAddr string) (*Server, error) {
	static := s.snapshot()
	if static.len() == 0 {
		return nil, fmt.Errorf("no server available")
	}
	return static.next(cliAddr), nil
}

func (s *servers) close() {
	close(s.done)

	if s.serviceWatcher != nil {
		s.serviceWatcher.Stop()
	}
}

func newStaticServers(servers []*Server, tags []string, lb *LoadBalance) *staticServers {
	if servers == nil {
		servers = make([]*Server, 0)
	}

	ss := &staticServers{}
	if lb == nil {
		ss.lb.Policy = PolicyRoundRobin
	} else {
		ss.lb = *lb
	}

	defer ss.prepare()

	if len(tags) == 0 {
		ss.servers = servers
		return ss
	}

	chosenServers := make([]*Server, 0)
	for _, server := range servers {
		for _, tag := range tags {
			if stringtool.StrInSlice(tag, server.Tags) {
				chosenServers = append(chosenServers, server)
				break
			}
		}
	}
	ss.servers = chosenServers
	return ss
}

func (ss *staticServers) prepare() {
	for _, server := range ss.servers {
		ss.weightsSum += server.Weight
	}
}

func (ss *staticServers) len() int {
	return len(ss.servers)
}

func (ss *staticServers) next(cliAddr string) *Server {
	switch ss.lb.Policy {
	case PolicyRoundRobin:
		return ss.roundRobin()
	case PolicyRandom:
		return ss.random()
	case PolicyWeightedRandom:
		return ss.weightedRandom()
	case PolicyIPHash:
		return ss.ipHash(cliAddr)
	}
	logger.Errorf("BUG: unknown load balance policy: %s", ss.lb.Policy)
	return ss.roundRobin()
}

func (ss *staticServers) roundRobin() *Server {
	count := atomic.AddUint64(&ss.count, 1)
	// NOTE: startEventLoop from 0.
	count--
	return ss.servers[int(count)%len(ss.servers)]
}

func (ss *staticServers) random() *Server {
	return ss.servers[rand.Intn(len(ss.servers))]
}

func (ss *staticServers) weightedRandom() *Server {
	randomWeight := rand.Intn(ss.weightsSum)
	for _, server := range ss.servers {
		randomWeight -= server.Weight
		if randomWeight < 0 {
			return server
		}
	}

	logger.Errorf("BUG: weighted random can't pick a server: sum(%d) servers(%+v)",
		ss.weightsSum, ss.servers)

	return ss.random()
}

func (ss *staticServers) ipHash(cliAddr string) *Server {
	sum32 := int(hashtool.Hash32(cliAddr))
	return ss.servers[sum32%len(ss.servers)]
}
