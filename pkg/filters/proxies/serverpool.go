/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package proxies provides the common interface and implementation of proxies.
package proxies

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/serviceregistry"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

// ServerPoolImpl is the interface for server pool.
type ServerPoolImpl interface {
	CreateLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer
}

// ServerPoolBase defines a base server pool.
type ServerPoolBase struct {
	spImpl       ServerPoolImpl
	Name         string
	done         chan struct{}
	wg           sync.WaitGroup
	loadBalancer atomic.Value
}

// ServerPoolBaseSpec is the spec for a base server pool.
type ServerPoolBaseSpec struct {
	ServerTags      []string         `json:"serverTags,omitempty" jsonschema:"uniqueItems=true"`
	Servers         []*Server        `json:"servers,omitempty"`
	SetUpstreamHost bool             `json:"setUpstreamHost,omitempty"`
	ServiceRegistry string           `json:"serviceRegistry,omitempty"`
	ServiceName     string           `json:"serviceName,omitempty"`
	LoadBalance     *LoadBalanceSpec `json:"loadBalance,omitempty"`
}

// Validate validates ServerPoolSpec.
func (sps *ServerPoolBaseSpec) Validate() error {
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

	if sps.ServiceName != "" && sps.LoadBalance != nil && sps.LoadBalance.HealthCheck != nil {
		return fmt.Errorf("can not open health check for service discovery")
	}

	return nil
}

// Init initialize the base server pool according to the spec.
func (spb *ServerPoolBase) Init(spImpl ServerPoolImpl, super *supervisor.Supervisor, name string, spec *ServerPoolBaseSpec) {
	spb.spImpl = spImpl
	spb.Name = name
	spb.done = make(chan struct{})

	if spec.ServiceRegistry == "" || spec.ServiceName == "" {
		spb.createLoadBalancer(spec.LoadBalance, spec.Servers)
		return
	}

	// watch service registry
	entity := super.MustGetSystemController(serviceregistry.Kind)
	registry := entity.Instance().(*serviceregistry.ServiceRegistry)

	instances, err := registry.ListServiceInstances(spec.ServiceRegistry, spec.ServiceName)
	if err != nil {
		msgFmt := "first try to use service %s/%s failed(will try again): %v"
		logger.Warnf(msgFmt, spec.ServiceRegistry, spec.ServiceName, err)
		spb.createLoadBalancer(spec.LoadBalance, spec.Servers)
	}

	spb.useService(spec, instances)

	watcher := registry.NewServiceWatcher(spec.ServiceRegistry, spec.ServiceName)
	spb.wg.Add(1)
	go func() {
		for {
			select {
			case <-spb.done:
				watcher.Stop()
				spb.wg.Done()
				return
			case event := <-watcher.Watch():
				spb.useService(spec, event.Instances)
			}
		}
	}()
}

// LoadBalancer returns the load balancer of the server pool.
func (spb *ServerPoolBase) LoadBalancer() LoadBalancer {
	if v := spb.loadBalancer.Load(); v != nil {
		return v.(LoadBalancer)
	}
	return nil
}

func (spb *ServerPoolBase) createLoadBalancer(spec *LoadBalanceSpec, servers []*Server) {
	for _, server := range servers {
		server.CheckAddrPattern()
	}

	if spec == nil {
		spec = &LoadBalanceSpec{}
	}

	lb := spb.spImpl.CreateLoadBalancer(spec, servers)
	if old := spb.loadBalancer.Swap(lb); old != nil {
		old.(LoadBalancer).Close()
	}
}

func (spb *ServerPoolBase) useService(spec *ServerPoolBaseSpec, instances map[string]*serviceregistry.ServiceInstanceSpec) {
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

	spb.createLoadBalancer(spec.LoadBalance, servers)
}

// Done returns the done channel, which indicates the closing of the server pool.
func (spb *ServerPoolBase) Done() <-chan struct{} {
	return spb.done
}

// Close closes the server pool.
func (spb *ServerPoolBase) Close() {
	close(spb.done)
	spb.wg.Wait()
	if lb := spb.LoadBalancer(); lb != nil {
		lb.Close()
	}
}
