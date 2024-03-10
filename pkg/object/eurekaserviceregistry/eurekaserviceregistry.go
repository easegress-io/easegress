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

// Package eurekaserviceregistry provides EurekaServiceRegistry.
package eurekaserviceregistry

import (
	"fmt"
	"sync"
	"time"

	eurekaapi "github.com/ArthurHlt/go-eureka-client/eureka"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/serviceregistry"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the category of EurekaServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of EurekaServiceRegistry.
	Kind = "EurekaServiceRegistry"

	// MetaKeyRegistryName is the key of service metadata.
	MetaKeyRegistryName = "RegistryName"

	name = "eurekaserviceregistry"
)

var aliases = []string{"eureka"}

func init() {
	supervisor.Register(&EurekaServiceRegistry{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     name,
		Aliases:  aliases,
	})
}

type (
	// EurekaServiceRegistry is Object EurekaServiceRegistry.
	EurekaServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		instances       map[string]*serviceregistry.ServiceInstanceSpec
		notify          chan *serviceregistry.RegistryEvent

		clientMutex sync.RWMutex
		client      *eurekaapi.Client

		statusMutex  sync.Mutex
		instancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the EurekaServiceRegistry.
	Spec struct {
		Endpoints    []string `json:"endpoints" jsonschema:"required,uniqueItems=true"`
		SyncInterval string   `json:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of EurekaServiceRegistry.
	Status struct {
		Health              string         `json:"health"`
		ServiceInstancesNum map[string]int `json:"instancesNum"`
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

// Init initializes EurekaServiceRegistry.
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
	e.notify = make(chan *serviceregistry.RegistryEvent, 10)
	e.firstDone = false

	e.instancesNum = make(map[string]int)
	e.done = make(chan struct{})

	_, err := e.getClient()
	if err != nil {
		logger.Errorf("%s get eureka client failed: %v", e.superSpec.Name(), err)
	}

	e.serviceRegistry.RegisterRegistry(e)

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
	defer e.closeClient()

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
	instances, err := e.ListAllServiceInstances()
	if err != nil {
		logger.Errorf("list all service instances failed: %v", err)
		return
	}

	instancesNum := make(map[string]int)
	for _, instance := range instances {
		instancesNum[instance.ServiceName]++
	}

	var event *serviceregistry.RegistryEvent
	if !e.firstDone {
		e.firstDone = true
		event = &serviceregistry.RegistryEvent{
			SourceRegistryName: e.Name(),
			UseReplace:         true,
			Replace:            instances,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(e.Name(), e.instances, instances)
	}

	if event.Empty() {
		return
	}

	e.notify <- event
	e.instances = instances

	e.statusMutex.Lock()
	e.instancesNum = instancesNum
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
	instancesNum := e.instancesNum
	e.statusMutex.Unlock()

	s.ServiceInstancesNum = instancesNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes EurekaServiceRegistry.
func (e *EurekaServiceRegistry) Close() {
	e.serviceRegistry.DeregisterRegistry(e.Name())

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

// ApplyServiceInstances applies service instances to the registry.
func (e *EurekaServiceRegistry) ApplyServiceInstances(instances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := e.getClient()
	if err != nil {
		return fmt.Errorf("%s get eureka client failed: %v",
			e.superSpec.Name(), err)
	}

	for _, instance := range instances {
		err := instance.Validate()
		if err != nil {
			return fmt.Errorf("%+v is invalid: %v", instance, err)
		}
	}

	for _, instance := range instances {
		info := e.serviceInstanceToInstanceInfo(instance)
		err = client.RegisterInstance(info.App, info)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteServiceInstances applies service instances to the registry.
func (e *EurekaServiceRegistry) DeleteServiceInstances(instances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := e.getClient()
	if err != nil {
		return fmt.Errorf("%s get eureka client failed: %v",
			e.superSpec.Name(), err)
	}

	for _, instance := range instances {
		err := client.UnregisterInstance(instance.ServiceName, instance.InstanceID)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetServiceInstance get service instance from the registry.
func (e *EurekaServiceRegistry) GetServiceInstance(serviceName, instanceID string) (*serviceregistry.ServiceInstanceSpec, error) {
	instances, err := e.ListServiceInstances(serviceName)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		if instance.InstanceID == instanceID {
			return instance, nil
		}
	}

	return nil, fmt.Errorf("%s/%s not found", serviceName, instanceID)
}

// ListServiceInstances list service instances of one service from the registry.
func (e *EurekaServiceRegistry) ListServiceInstances(serviceName string) (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	client, err := e.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get eureka client failed: %v",
			e.superSpec.Name(), err)
	}

	app, err := client.GetApplication(serviceName)
	if err != nil {
		return nil, err
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, info := range app.Instances {
		for _, serviceInstance := range e.instanceInfoToServiceInstances(&info) {
			err := serviceInstance.Validate()
			if err != nil {
				return nil, fmt.Errorf("%+v is invalid: %v", serviceInstance, err)
			}
			instances[serviceInstance.Key()] = serviceInstance
		}
	}

	return instances, nil
}

// ListAllServiceInstances list all service instances from the registry.
func (e *EurekaServiceRegistry) ListAllServiceInstances() (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	client, err := e.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get eureka client failed: %v",
			e.superSpec.Name(), err)
	}

	apps, err := client.GetApplications()
	if err != nil {
		return nil, err
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, app := range apps.Applications {
		for _, info := range app.Instances {
			for _, serviceInstance := range e.instanceInfoToServiceInstances(&info) {
				err := serviceInstance.Validate()
				if err != nil {
					return nil, fmt.Errorf("%+v is invalid: %v", serviceInstance, err)
				}
				instances[serviceInstance.Key()] = serviceInstance
			}
		}
	}

	return instances, nil
}

func (e *EurekaServiceRegistry) instanceInfoToServiceInstances(info *eurekaapi.InstanceInfo) []*serviceregistry.ServiceInstanceSpec {
	var instances []*serviceregistry.ServiceInstanceSpec

	registryName := e.Name()
	if info.Metadata != nil && info.Metadata.Map != nil &&
		info.Metadata.Map[MetaKeyRegistryName] != "" {
		registryName = info.Metadata.Map[MetaKeyRegistryName]
	}

	address := info.IpAddr
	if address == "" {
		address = info.HostName
	}

	baseServiceInstanceSpec := serviceregistry.ServiceInstanceSpec{
		RegistryName: registryName,
		ServiceName:  info.App,
		InstanceID:   info.InstanceID,
		Address:      address,
	}

	if info.Port != nil && info.Port.Enabled {
		plain := baseServiceInstanceSpec
		plain.Port = uint16(info.Port.Port)
		plain.Scheme = "http"
		instances = append(instances, &plain)
	}

	if info.SecurePort != nil && info.SecurePort.Enabled {
		secure := baseServiceInstanceSpec
		secure.Port = uint16(info.SecurePort.Port)
		secure.Scheme = "https"
		instances = append(instances, &secure)
	}

	return instances
}

func (e *EurekaServiceRegistry) serviceInstanceToInstanceInfo(serviceInstance *serviceregistry.ServiceInstanceSpec) *eurekaapi.InstanceInfo {
	info := &eurekaapi.InstanceInfo{
		Metadata: &eurekaapi.MetaData{
			Map: map[string]string{
				MetaKeyRegistryName: serviceInstance.RegistryName,
			},
		},
		App:        serviceInstance.ServiceName,
		InstanceID: serviceInstance.InstanceID,
		IpAddr:     serviceInstance.Address,
	}

	switch serviceInstance.Scheme {
	case "", "http":
		info.Port = &eurekaapi.Port{
			Port:    int(serviceInstance.Port),
			Enabled: true,
		}
	case "https":
		info.SecurePort = &eurekaapi.Port{
			Port:    int(serviceInstance.Port),
			Enabled: true,
		}
	}

	return info
}
