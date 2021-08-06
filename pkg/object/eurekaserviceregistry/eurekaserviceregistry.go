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

package eurekaserviceregistry

import (
	"sync"
	"time"

	eurekaapi "github.com/ArthurHlt/go-eureka-client/eureka"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of EurekaServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of EurekaServiceRegistry.
	Kind = "EurekaServiceRegistry"
)

func init() {
	supervisor.Register(&EurekaServiceRegistry{})
}

type (
	// EurekaServiceRegistry is Object EurekaServiceRegistry.
	EurekaServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		serviceSpecs    map[string]*serviceregistry.ServiceSpec
		notify          chan *serviceregistry.RegistryEvent

		clientMutex sync.RWMutex
		client      *eurekaapi.Client

		statusMutex         sync.Mutex
		serviceInstancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the EurekaServiceRegistry.
	Spec struct {
		Endpoints    []string `yaml:"endpoints" jsonschema:"required,uniqueItems=true"`
		SyncInterval string   `yaml:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of EurekaServiceRegistry.
	Status struct {
		Health              string         `yaml:"health"`
		ServiceInstancesNum map[string]int `yaml:"serviceInstancesNum"`
	}
)

// Category returns the category of EurekaServiceRegistry.
func (e *EurekaServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of EurekaServiceRegistry.
func (e *EurekaServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of EurekaServiceRegistry.
func (e *EurekaServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		Endpoints:    []string{"http://127.0.0.1:8761/eureka"},
		SyncInterval: "10s",
	}
}

// Init initilizes EurekaServiceRegistry.
func (e *EurekaServiceRegistry) Init(superSpec *supervisor.Spec) {
	e.superSpec, e.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	e.reload()
}

// Inherit inherits previous generation of EurekaServiceRegistry.
func (e *EurekaServiceRegistry) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	e.Init(superSpec)
}

func (e *EurekaServiceRegistry) reload() {
	e.serviceRegistry = e.superSpec.Super().MustGetSystemController(serviceregistry.Kind).
		Instance().(*serviceregistry.ServiceRegistry)
	e.serviceRegistry.RegisterRegistry(e)
	e.notify = make(chan *serviceregistry.RegistryEvent, 10)

	e.serviceInstancesNum = make(map[string]int)
	e.done = make(chan struct{})

	_, err := e.getClient()
	if err != nil {
		logger.Errorf("%s get eureka client failed: %v", e.superSpec.Name(), err)
	}

	go e.run()
}

func (e *EurekaServiceRegistry) getClient() (*eurekaapi.Client, error) {
	e.clientMutex.RLock()
	if e.client != nil {
		client := e.client
		e.clientMutex.RUnlock()
		return client, nil
	}
	e.clientMutex.RUnlock()

	return e.buildClient()
}

func (e *EurekaServiceRegistry) buildClient() (*eurekaapi.Client, error) {
	e.clientMutex.Lock()
	defer e.clientMutex.Unlock()

	// DCL
	if e.client != nil {
		return e.client, nil
	}

	client := eurekaapi.NewClient(e.spec.Endpoints)

	e.client = client

	return client, nil
}

func (e *EurekaServiceRegistry) closeClient() {
	e.clientMutex.Lock()
	defer e.clientMutex.Unlock()

	if e.client == nil {
		return
	}

	e.client = nil
}

func (e *EurekaServiceRegistry) run() {
	syncInterval, err := time.ParseDuration(e.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			e.spec.SyncInterval, err)
		return
	}

	e.update()

	for {
		select {
		case <-e.done:
			return
		case <-time.After(syncInterval):
			e.update()
		}
	}
}

func (e *EurekaServiceRegistry) update() {
	client, err := e.getClient()
	if err != nil {
		logger.Errorf("%s get eureka client failed: %v",
			e.superSpec.Name(), err)
		return
	}

	apps, err := client.GetApplications()
	if err != nil {
		logger.Errorf("%s get services failed: %v",
			e.superSpec.Name(), err)
		return
	}

	serviceSpecs := make(map[string]*serviceregistry.ServiceSpec)
	serviceInstancesNum := map[string]int{}
	for _, app := range apps.Applications {
		for _, instance := range app.Instances {
			var instanceSpecs []*serviceregistry.ServiceInstanceSpec

			baseServiceInstanceSpec := serviceregistry.ServiceInstanceSpec{
				ServiceName: app.Name,
				Hostname:    instance.HostName,
				HostIP:      instance.IpAddr,
				Port:        uint16(instance.Port.Port),
			}

			if instance.Port != nil && instance.Port.Enabled {
				plain := baseServiceInstanceSpec
				instanceSpecs = append(instanceSpecs, &plain)
				serviceInstancesNum[app.Name]++
			}

			if instance.SecurePort != nil && instance.SecurePort.Enabled {
				secure := baseServiceInstanceSpec
				secure.Scheme = "https"
				instanceSpecs = append(instanceSpecs, &secure)

				serviceInstancesNum[app.Name]++
			}

			serviceName := app.Name

			serviceSpec, exists := serviceSpecs[serviceName]
			if !exists {
				serviceSpecs[serviceName] = &serviceregistry.ServiceSpec{
					RegistryName: e.Name(),
					ServiceName:  serviceName,
				}
				serviceSpecs[serviceName].Instances = append(serviceSpecs[serviceName].Instances, instanceSpecs...)
			} else {
				serviceSpec.Instances = append(serviceSpec.Instances, instanceSpecs...)
			}
		}
	}

	var event *serviceregistry.RegistryEvent
	if !e.firstDone {
		e.firstDone = true
		event = &serviceregistry.RegistryEvent{
			RegistryName: e.Name(),
			Replace:      serviceSpecs,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(e.Name(), e.serviceSpecs, serviceSpecs)
	}

	e.notify <- event
	e.serviceSpecs = serviceSpecs

	e.statusMutex.Lock()
	e.serviceInstancesNum = serviceInstancesNum
	e.statusMutex.Unlock()
}

// Status returns status of EurekaServiceRegister.
func (e *EurekaServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := e.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	e.statusMutex.Lock()
	serviceInstancesNum := e.serviceInstancesNum
	e.statusMutex.Unlock()

	s.ServiceInstancesNum = serviceInstancesNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes EurekaServiceRegistry.
func (e *EurekaServiceRegistry) Close() {
	e.serviceRegistry.DeregisterRegistry(e.Name())

	e.closeClient()
	close(e.done)
}

// Name returns name.
func (e *EurekaServiceRegistry) Name() string {
	return e.superSpec.Name()
}

// Notify returns notify channel.
func (e *EurekaServiceRegistry) Notify() <-chan *serviceregistry.RegistryEvent {
	return e.notify
}

// ApplyServices applies service specs to eureka registry.
func (e *EurekaServiceRegistry) ApplyServices(serviceSpec []*serviceregistry.ServiceSpec) error {
	// TODO
	return nil
}

// GetService applies service specs to eureka registry.
func (e *EurekaServiceRegistry) GetService(serviceName string) (*serviceregistry.ServiceSpec, error) {
	// TODO
	return nil, nil
}

// ListServices lists service specs from eureka registry.
func (e *EurekaServiceRegistry) ListServices() ([]*serviceregistry.ServiceSpec, error) {
	// TODO
	return nil, nil
}
