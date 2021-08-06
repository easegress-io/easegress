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

package consulserviceregistry

import (
	"sync"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of ConsulServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of ConsulServiceRegistry.
	Kind = "ConsulServiceRegistry"
)

func init() {
	supervisor.Register(&ConsulServiceRegistry{})
}

type (
	// ConsulServiceRegistry is Object ConsulServiceRegistry.
	ConsulServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		serviceSpecs    map[string]*serviceregistry.ServiceSpec
		notify          chan *serviceregistry.RegistryEvent

		clientMutex sync.RWMutex
		client      *api.Client

		statusMutex         sync.Mutex
		serviceInstancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the ConsulServiceRegistry.
	Spec struct {
		Address      string   `yaml:"address" jsonschema:"required"`
		Scheme       string   `yaml:"scheme" jsonschema:"required,enum=http,enum=https"`
		Datacenter   string   `yaml:"datacenter" jsonschema:"omitempty"`
		Token        string   `yaml:"token" jsonschema:"omitempty"`
		Namespace    string   `yaml:"namespace" jsonschema:"omitempty"`
		SyncInterval string   `yaml:"syncInterval" jsonschema:"required,format=duration"`
		ServiceTags  []string `yaml:"serviceTags" jsonschema:"omitempty"`
	}

	// Status is the status of ConsulServiceRegistry.
	Status struct {
		Health              string         `yaml:"health"`
		ServiceInstancesNum map[string]int `yaml:"serviceInstancesNum"`
	}
)

// Category returns the category of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		Address:      "127.0.0.1:8500",
		Scheme:       "http",
		SyncInterval: "10s",
	}
}

// Init initilizes ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Init(superSpec *supervisor.Spec) {
	c.superSpec, c.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	c.reload()
}

// Inherit inherits previous generation of ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	c.Init(superSpec)
}

func (c *ConsulServiceRegistry) reload() {
	c.serviceRegistry = c.superSpec.Super().MustGetSystemController(serviceregistry.Kind).
		Instance().(*serviceregistry.ServiceRegistry)
	c.serviceRegistry.RegisterRegistry(c)
	c.notify = make(chan *serviceregistry.RegistryEvent, 10)

	c.serviceInstancesNum = map[string]int{}
	c.done = make(chan struct{})

	_, err := c.getClient()
	if err != nil {
		logger.Errorf("%s get consul client failed: %v", c.superSpec.Name(), err)
	}

	go c.run()
}

func (c *ConsulServiceRegistry) getClient() (*api.Client, error) {
	c.clientMutex.RLock()
	if c.client != nil {
		client := c.client
		c.clientMutex.RUnlock()
		return client, nil
	}
	c.clientMutex.RUnlock()

	return c.buildClient()
}

func (c *ConsulServiceRegistry) buildClient() (*api.Client, error) {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	// DCL
	if c.client != nil {
		return c.client, nil
	}

	config := api.DefaultConfig()
	config.Address = c.spec.Address
	if config.Scheme != "" {
		config.Scheme = c.spec.Scheme
	}
	if config.Datacenter != "" {
		config.Datacenter = c.spec.Datacenter
	}
	if config.Token != "" {
		config.Token = c.spec.Token
	}
	if config.Namespace != "" {
		config.Namespace = c.spec.Namespace
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	c.client = client

	return client, nil
}

func (c *ConsulServiceRegistry) closeClient() {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	if c.client == nil {
		return
	}

	c.client = nil
}

func (c *ConsulServiceRegistry) run() {
	syncInterval, err := time.ParseDuration(c.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			c.spec.SyncInterval, err)
		return
	}

	c.update()

	for {
		select {
		case <-c.done:
			return
		case <-time.After(syncInterval):
			c.update()
		}
	}
}

func (c *ConsulServiceRegistry) update() {
	client, err := c.getClient()
	if err != nil {
		logger.Errorf("%s get consul client failed: %v",
			c.superSpec.Name(), err)
		return
	}

	q := &api.QueryOptions{
		Namespace:  c.spec.Namespace,
		Datacenter: c.spec.Datacenter,
	}
	catalog := client.Catalog()

	resp, _, err := catalog.Services(q)
	if err != nil {
		logger.Errorf("%s pull catalog services failed: %v",
			c.superSpec.Name(), err)
		return
	}

	serviceSpecs := make(map[string]*serviceregistry.ServiceSpec)
	serviceInstancesNum := map[string]int{}
	for serviceName := range resp {
		services, _, err := catalog.ServiceMultipleTags(serviceName,
			c.spec.ServiceTags, q)
		if err != nil {
			logger.Errorf("%s pull catalog service %s failed: %v",
				c.superSpec.Name(), serviceName, err)
			continue
		}
		for _, service := range services {
			serviceInstanceSpec := &serviceregistry.ServiceInstanceSpec{
				ServiceName: serviceName,
			}
			serviceInstanceSpec.HostIP = service.ServiceAddress
			if serviceInstanceSpec.HostIP == "" {
				serviceInstanceSpec.HostIP = service.Address
			}
			serviceInstanceSpec.Port = uint16(service.ServicePort)
			serviceInstanceSpec.Tags = service.ServiceTags

			if err := serviceInstanceSpec.Validate(); err != nil {
				logger.Errorf("invalid server: %v", err)
				continue
			}

			serviceSpec, exists := serviceSpecs[serviceName]
			if !exists {
				serviceSpecs[serviceName] = &serviceregistry.ServiceSpec{
					RegistryName: c.Name(),
					ServiceName:  serviceName,
					Instances:    []*serviceregistry.ServiceInstanceSpec{serviceInstanceSpec},
				}
			} else {
				serviceSpec.Instances = append(serviceSpec.Instances, serviceInstanceSpec)
			}

			serviceInstancesNum[serviceName]++
		}
	}

	var event *serviceregistry.RegistryEvent
	if !c.firstDone {
		c.firstDone = true
		event = &serviceregistry.RegistryEvent{
			RegistryName: c.Name(),
			Replace:      serviceSpecs,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(c.Name(), c.serviceSpecs, serviceSpecs)
	}

	c.notify <- event
	c.serviceSpecs = serviceSpecs

	c.statusMutex.Lock()
	c.serviceInstancesNum = serviceInstancesNum
	c.statusMutex.Unlock()
}

// Status returns status of ConsulServiceRegister.
func (c *ConsulServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := c.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	c.statusMutex.Lock()
	serversNum := c.serviceInstancesNum
	c.statusMutex.Unlock()

	s.ServiceInstancesNum = serversNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes ConsulServiceRegistry.
func (c *ConsulServiceRegistry) Close() {
	c.serviceRegistry.DeregisterRegistry(c.Name())

	c.closeClient()
	close(c.done)
}

// Name returns name.
func (c *ConsulServiceRegistry) Name() string {
	return c.superSpec.Name()
}

// Notify returns notify channel.
func (c *ConsulServiceRegistry) Notify() <-chan *serviceregistry.RegistryEvent {
	return c.notify
}

// ApplyServices applies service specs to consul registry.
func (c *ConsulServiceRegistry) ApplyServices(serviceSpec []*serviceregistry.ServiceSpec) error {
	// TODO
	return nil
}

// GetService applies service specs to consul registry.
func (c *ConsulServiceRegistry) GetService(serviceName string) (*serviceregistry.ServiceSpec, error) {
	// TODO
	return nil, nil
}

// ListServices lists service specs from consul registry.
func (c *ConsulServiceRegistry) ListServices() ([]*serviceregistry.ServiceSpec, error) {
	// TODO
	return nil, nil
}
