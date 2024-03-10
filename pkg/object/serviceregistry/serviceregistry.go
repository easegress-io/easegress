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

// Package serviceregistry provides the service registry.
package serviceregistry

import (
	"fmt"
	"strings"
	"sync"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the category of ServiceRegistry.
	Category = supervisor.CategorySystemController

	// Kind is the kind of ServiceRegistry.
	Kind = "ServiceRegistry"
)

func init() {
	supervisor.Register(&ServiceRegistry{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"sr", "serviceregistry"},
	})
}

type (
	// ServiceRegistry is a system controller to be center registry
	// between external registries and internal handlers and watchers.
	// To be specific: it dispatches event from every registry to every watcher,
	// and wraps the operations from handlers to every registry.
	ServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		mutex *sync.Mutex

		// The key is registry name.
		registryBuckets map[string]*registryBucket

		done chan struct{}
	}

	registryBucket struct {
		// These fields will be changed in register and deregister.
		registered bool
		registry   Registry
		done       chan struct{}

		// Key is the registry name.
		registryWatchers map[string]RegistryWatcher

		// Key is the service name.
		serviceBuckets map[string]*serviceBucket
	}

	serviceBucket struct {
		serviceWatchers map[string]ServiceWatcher
	}

	// Spec describes ServiceRegistry.
	Spec struct {
		SyncInterval string `json:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of ServiceRegistry.
	Status struct{}

	// Registry stands for the specific service registry.
	Registry interface {
		Name() string

		// Operation from the registry.
		Notify() <-chan *RegistryEvent

		// Operations to the registry.
		// The key of maps is key of service instance.
		ApplyServiceInstances(serviceInstances map[string]*ServiceInstanceSpec) error
		DeleteServiceInstances(serviceInstances map[string]*ServiceInstanceSpec) error
		// GetServiceInstance must return error if not found.
		GetServiceInstance(serviceName, instanceID string) (*ServiceInstanceSpec, error)
		// ListServiceInstances could return zero elements of instances with nil error.
		ListServiceInstances(serviceName string) (map[string]*ServiceInstanceSpec, error)
		// ListServiceInstances could return zero elements of instances with nil error.
		ListAllServiceInstances() (map[string]*ServiceInstanceSpec, error)
	}
)

// newRegisterBucket creates a registrybucket, the registry could be nil.
func newRegisterBucket() *registryBucket {
	return &registryBucket{
		done:             make(chan struct{}),
		registryWatchers: make(map[string]RegistryWatcher),
		serviceBuckets:   make(map[string]*serviceBucket),
	}
}

func (b *registryBucket) needClean() bool {
	return !b.registered && len(b.registryWatchers) == 0 && len(b.serviceBuckets) == 0
}

func newServiceBucket() *serviceBucket {
	return &serviceBucket{
		serviceWatchers: make(map[string]ServiceWatcher),
	}
}

func (b *serviceBucket) needClean() bool {
	return len(b.serviceWatchers) == 0
}

// RegisterRegistry registers the registry and watch it.
func (sr *ServiceRegistry) RegisterRegistry(registry Registry) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	bucket, exists := sr.registryBuckets[registry.Name()]
	if exists {
		if bucket.registered {
			return fmt.Errorf("registry %s already registered", registry.Name())
		}
	} else {
		bucket = newRegisterBucket()
		sr.registryBuckets[registry.Name()] = bucket
	}

	bucket.registered, bucket.registry = true, registry

	// NOTE: There will be data race warning, if it calls Notify within watchRegistry,
	// even it won't cause bug. So we move it out here.
	go sr.watchRegistry(registry.Notify(), bucket)

	return nil
}

func (sr *ServiceRegistry) watchRegistry(notify <-chan *RegistryEvent, bucket *registryBucket) {
	for {
		select {
		case <-bucket.done:
			return
		case event := <-notify:
			// Defensive programming for driver not to judge it.
			if event.Empty() {
				continue
			}

			err := event.Validate()
			if err != nil {
				logger.Errorf("registry event from %v is invalid: %v",
					bucket.registry.Name(), err)
				continue
			}

			// Defensive programming for driver not to fulfill the field.
			event.SourceRegistryName = bucket.registry.Name()

			sr.handleRegistryEvent(event)
		}
	}
}

func (sr *ServiceRegistry) handleRegistryEvent(event *RegistryEvent) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	sr._handleRegistryEvent(event)
}

func (sr *ServiceRegistry) _handleRegistryEvent(event *RegistryEvent) {
	bucket, exists := sr.registryBuckets[event.SourceRegistryName]
	if !exists || !bucket.registered {
		logger.Errorf("BUG: registry bucket %s not found", event.SourceRegistryName)
		return
	}

	for _, watcher := range bucket.registryWatchers {
		watcher.(*registryWatcher).EventChan() <- event.DeepCopy()
	}

	// Quickly return for performance.
	if len(bucket.serviceBuckets) == 0 {
		return
	}

	applyOrDeleteServices := make(map[string]struct{})
	for _, instance := range event.Apply {
		applyOrDeleteServices[instance.ServiceName] = struct{}{}
	}
	for _, instance := range event.Delete {
		applyOrDeleteServices[instance.ServiceName] = struct{}{}
	}

	for serviceName, serviceBucket := range bucket.serviceBuckets {
		_, applyOrDelete := applyOrDeleteServices[serviceName]
		if !event.UseReplace && !applyOrDelete {
			continue
		}

		instances, err := bucket.registry.ListServiceInstances(serviceName)
		if err != nil {
			logger.Errorf("list service instances of %s/%s failed: %v",
				event.SourceRegistryName, serviceName, err)
			continue
		}

		for _, watcher := range serviceBucket.serviceWatchers {
			watcher.(*serviceWatcher).EventChan() <- &ServiceEvent{
				SourceRegistryName: event.SourceRegistryName,
				Instances:          instances,
			}
		}
	}
}

// DeregisterRegistry remove the registry.
func (sr *ServiceRegistry) DeregisterRegistry(registryName string) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return fmt.Errorf("%s not found", registryName)
	}

	cleanEvent := &RegistryEvent{
		SourceRegistryName: registryName,
		UseReplace:         true,
	}

	sr._handleRegistryEvent(cleanEvent)

	close(bucket.done)
	bucket.registered, bucket.registry, bucket.done = false, nil, make(chan struct{})

	if bucket.needClean() {
		delete(sr.registryBuckets, registryName)
	}

	return nil
}

// ApplyServiceInstances applies the services to the registry with change RegistryName of all specs.
func (sr *ServiceRegistry) ApplyServiceInstances(registryName string, serviceInstances map[string]*ServiceInstanceSpec) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	return sr._applyServiceInstances(registryName, serviceInstances)
}

func (sr *ServiceRegistry) _applyServiceInstances(registryName string, serviceInstances map[string]*ServiceInstanceSpec) error {
	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return fmt.Errorf("%s not found", registryName)
	}

	return bucket.registry.ApplyServiceInstances(serviceInstances)
}

// GetServiceInstance get service instance.
func (sr *ServiceRegistry) GetServiceInstance(registryName, serviceName, instanceID string) (*ServiceInstanceSpec, error) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	return sr._getServiceInstance(registryName, serviceName, instanceID)
}

func (sr *ServiceRegistry) _getServiceInstance(registryName, serviceName, instanceID string) (*ServiceInstanceSpec, error) {
	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return nil, fmt.Errorf("%s not found", registryName)
	}

	return bucket.registry.GetServiceInstance(serviceName, instanceID)
}

// ListServiceInstances service instances of one service.
func (sr *ServiceRegistry) ListServiceInstances(registryName, serviceName string) (map[string]*ServiceInstanceSpec, error) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	return sr._listServiceInstances(registryName, serviceName)
}

func (sr *ServiceRegistry) _listServiceInstances(registryName, serviceName string) (map[string]*ServiceInstanceSpec, error) {
	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return nil, fmt.Errorf("%s not found", registryName)
	}

	return bucket.registry.ListServiceInstances(serviceName)
}

// DeleteServiceInstances service instances of one service.
func (sr *ServiceRegistry) DeleteServiceInstances(registryName string, serviceInstances map[string]*ServiceInstanceSpec) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	return sr._deleteServiceInstances(registryName, serviceInstances)
}

func (sr *ServiceRegistry) _deleteServiceInstances(registryName string, serviceInstances map[string]*ServiceInstanceSpec) error {
	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return fmt.Errorf("%s not found", registryName)
	}

	return bucket.registry.DeleteServiceInstances(serviceInstances)
}

// ListAllServiceInstances service instances of all services.
func (sr *ServiceRegistry) ListAllServiceInstances(registryName string) (map[string]*ServiceInstanceSpec, error) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	return sr._listAllServiceInstances(registryName)
}

func (sr *ServiceRegistry) _listAllServiceInstances(registryName string) (map[string]*ServiceInstanceSpec, error) {
	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return nil, fmt.Errorf("%s not found", registryName)
	}

	return bucket.registry.ListAllServiceInstances()
}

// Category returns the category of ServiceRegistry.
func (sr *ServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of ServiceRegistry.
func (sr *ServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of ServiceRegistry.
func (sr *ServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		SyncInterval: "10s",
	}
}

// Init initializes ServiceRegistry.
func (sr *ServiceRegistry) Init(superSpec *supervisor.Spec) {
	sr.superSpec, sr.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	sr.reload(nil)
}

// Inherit inherits previous generation of ServiceRegistry.
func (sr *ServiceRegistry) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	sr.superSpec, sr.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	sr.reload(previousGeneration.(*ServiceRegistry))
	previousGeneration.Close()
}

func (sr *ServiceRegistry) reload(previousGeneration *ServiceRegistry) {
	if previousGeneration != nil {
		sr.registryBuckets = previousGeneration.registryBuckets
		sr.mutex = previousGeneration.mutex
	} else {
		sr.registryBuckets = make(map[string]*registryBucket)
		sr.mutex = &sync.Mutex{}
	}

	sr.done = make(chan struct{})
}

// Status returns status of ServiceRegistry.
func (sr *ServiceRegistry) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: &Status{},
	}
}

// Close closes ServiceRegistry.
func (sr *ServiceRegistry) Close() {
	close(sr.done)
}
