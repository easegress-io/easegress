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
	"sync"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// SlotSize is the size of slots
	SlotSize = 1024
)

// BaseServerPool defines a server pool.
type BaseServerPool struct {
	name         string
	done         chan struct{}
	wg           sync.WaitGroup
	filter       RequestMatcher
	loadBalancer atomic.Value
	slots        []string
}

// BaseServerPoolSpec is the spec for a base server pool.
type BaseServerPoolSpec struct {
	Filter          *RequestMatcherSpec `json:"filter" jsonschema:"omitempty"`
	ServerTags      []string            `json:"serverTags" jsonschema:"omitempty,uniqueItems=true"`
	Servers         []*Server           `json:"servers" jsonschema:"omitempty"`
	ServiceRegistry string              `json:"serviceRegistry" jsonschema:"omitempty"`
	ServiceName     string              `json:"serviceName" jsonschema:"omitempty"`
	LoadBalance     *LoadBalanceSpec    `json:"loadBalance" jsonschema:"omitempty"`
}

// Validate validates ServerPoolSpec.
func (sps *BaseServerPoolSpec) Validate() error {
	if sps.ServiceName == "" && len(sps.Servers) == 0 {
		return fmt.Errorf("both serviceName and servers are empty")
	}

	serversGotWeight := 0
	for _, server := range sps.Servers {
		if server.Weight > 0 {
			serversGotWeight++
		}
	}
	if serversGotWeight > 0 && serversGotWeight < len(sps.Servers) {
		msgFmt := "not all servers have weight(%d/%d)"
		return fmt.Errorf(msgFmt, serversGotWeight, len(sps.Servers))
	}

	return nil
}

// Init initialize the base server pool according to the spec.
func (bsp *BaseServerPool) Init(super *supervisor.Supervisor, name string, spec *BaseServerPoolSpec) {
	bsp.name = name
	bsp.done = make(chan struct{})

	if spec.Filter != nil {
		bsp.filter = NewRequestMatcher(spec.Filter)
	}

	if spec.ServiceRegistry == "" || spec.ServiceName == "" {
		bsp.createLoadBalancer(spec.LoadBalance, spec.Servers)
		return
	}

	// watch service registry
	entity := super.MustGetSystemController(serviceregistry.Kind)
	registry := entity.Instance().(*serviceregistry.ServiceRegistry)

	instances, err := registry.ListServiceInstances(spec.ServiceRegistry, spec.ServiceName)
	if err != nil {
		msgFmt := "first try to use service %s/%s failed(will try again): %v"
		logger.Warnf(msgFmt, spec.ServiceRegistry, spec.ServiceName, err)
		bsp.createLoadBalancer(spec.LoadBalance, spec.Servers)
	}

	bsp.useService(spec, instances)

	watcher := registry.NewServiceWatcher(spec.ServiceRegistry, spec.ServiceName)
	bsp.wg.Add(1)
	go func() {
		for {
			select {
			case <-bsp.done:
				watcher.Stop()
				bsp.wg.Done()
				return
			case event := <-watcher.Watch():
				bsp.useService(spec, event.Instances)
			}
		}
	}()
}

// LoadBalancer returns the load balancer of the server pool.
func (bsp *BaseServerPool) LoadBalancer() LoadBalancer {
	return bsp.loadBalancer.Load().(LoadBalancer)
}

// allocateSlots allocates slots for servers
func (bsp *BaseServerPool) allocateSlots(servers []*Server) {
	// calculate size
	if bsp.slots == nil {
		bsp.slots = make([]string, SlotSize)
	}
	slotL, svrL := len(bsp.slots), len(servers)
	slotses, sizes := make(map[string][]int, svrL), make(map[string]int, svrL)
	for i, svr := range servers {
		id := svr.ID()
		slotses[id] = make([]int, 0, slotL/svrL+1)
		divisor, remainder := slotL/svrL, slotL%svrL
		if remainder >= (i + 1) {
			divisor++
		}
		sizes[id] = divisor
	}

	// remain and free slots
	for pos, id := range bsp.slots {
		slots := slotses[id]
		if slots != nil && len(slots) < sizes[id] {
			// remain consistent slot
			slotses[id] = append(slots, pos)
		} else {
			// free slot for lost and redundant server
			bsp.slots[pos] = ""
		}
	}

	// allocate slots to lacks
	pos := 0
	for _, svr := range servers {
		id := svr.ID()
		slots := slotses[id]
		for i := len(slots); i < sizes[id]; i++ {
			for ; ; pos++ {
				if pos >= slotL {
					break
				}
				if bsp.slots[pos] == "" {
					bsp.slots[pos] = id
					slots = append(slots, pos)
					pos++
					break
				}
			}
		}
		svrSlots := make(map[int]bool, len(slots))
		for _, slot := range slots {
			svrSlots[slot] = true
		}
		svr.slots = svrSlots
	}
}

func (bsp *BaseServerPool) createLoadBalancer(spec *LoadBalanceSpec, servers []*Server) {
	for _, server := range servers {
		server.checkAddrPattern()
	}

	if spec == nil {
		spec = &LoadBalanceSpec{}
	}

	bsp.allocateSlots(servers)
	lb := NewLoadBalancer(spec, servers)
	bsp.loadBalancer.Store(lb)
}

func (bsp *BaseServerPool) useService(spec *BaseServerPoolSpec, instances map[string]*serviceregistry.ServiceInstanceSpec) {
	servers := make([]*Server, 0)

	for _, instance := range instances {
		// default to true in case of sp.spec.ServerTags is empty
		match := true

		for _, tag := range spec.ServerTags {
			if match = stringtool.StrInSlice(tag, instance.Tags); match {
				break
			}
		}

		if match {
			servers = append(servers, &Server{
				URL:    instance.URL(),
				Tags:   instance.Tags,
				Weight: instance.Weight,
			})
		}
	}

	if len(servers) == 0 {
		msgFmt := "%s/%s: no service instance satisfy tags: %v"
		logger.Warnf(msgFmt, spec.ServiceRegistry, spec.ServiceName, spec.ServerTags)
		servers = spec.Servers
	}

	bsp.createLoadBalancer(spec.LoadBalance, servers)
}

func (bsp *BaseServerPool) close() {
	close(bsp.done)
	bsp.wg.Wait()
}
