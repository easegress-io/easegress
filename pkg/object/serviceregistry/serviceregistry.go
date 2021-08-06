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

package serviceregistry

import (
	"fmt"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of ServiceRegistry.
	Category = supervisor.CategorySystemController

	// Kind is the kind of ServiceRegistry.
	Kind = "ServiceRegistry"
)

func init() {
	supervisor.Register(&ServiceRegistry{})
}

type (
	// ServiceRegistry is a system controller to be center registry
	// between external registries and internal handlers and watchers.
	// To be specific: it dispatches event from every registry to every watcher,
	// and wraps the operations from handlers to every registry.
	ServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		mutex sync.RWMutex

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
		// TODO: Support updating for system controller.
		// Please notice some components may reference of old system controller
		// after reloading, this should be fixed.
		SyncInterval string `yaml:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of ServiceRegistry.
	Status struct{}

	// Registry stands for the specific service registry.
	Registry interface {
		Name() string

		// Operation from the registry.
		Notify() <-chan *RegistryEvent

		// Operations to the registry.
		ApplyServices(serviceSpec []*ServiceSpec) error
		GetService(serviceName string) (*ServiceSpec, error)
		ListServices() ([]*ServiceSpec, error)
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

	go sr.watchRegistry(bucket)

	return nil
}

func (sr *ServiceRegistry) watchRegistry(bucket *registryBucket) {
	for {
		select {
		case <-bucket.done:
			return
		case event := <-bucket.registry.Notify():
			event.RegistryName = bucket.registry.Name()

			sr.handleRegistryEvent(event)
		}
	}
}

func (sr *ServiceRegistry) handleRegistryEvent(event *RegistryEvent) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	bucket, exists := sr.registryBuckets[event.RegistryName]
	if !exists {
		logger.Errorf("BUG: registry bucket %s not found", event.RegistryName)
		return
	}

	for _, watcher := range bucket.registryWatchers {
		watcher.(*registryWatcher).EventChan() <- event.DeepCopy()
	}

	for serviceName, serviceBucket := range bucket.serviceBuckets {
		replace, replaceExists := event.Replace[serviceName]
		apply, applyExists := event.Apply[serviceName]
		del, delExists := event.Delete[serviceName]

		if replaceExists {
			for _, watcher := range serviceBucket.serviceWatchers {
				watcher.(*serviceWatcher).EventChan() <- &ServiceEvent{
					Apply: replace.DeepCopy(),
				}
			}
			continue
		}

		if applyExists {
			for _, watcher := range serviceBucket.serviceWatchers {
				watcher.(*serviceWatcher).EventChan() <- &ServiceEvent{
					Apply: apply.DeepCopy(),
				}
			}
			continue
		}

		if delExists {
			for _, watcher := range serviceBucket.serviceWatchers {
				watcher.(*serviceWatcher).EventChan() <- &ServiceEvent{
					Delete: del.DeepCopy(),
				}
			}
			continue
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

	bucket.registered = false
	close(bucket.done)

	if bucket.needClean() {
		delete(sr.registryBuckets, registryName)
	}

	return nil
}

// ApplyServices applies the services to the registry with change RegistryName of all specs.
func (sr *ServiceRegistry) ApplyServices(registryName string, serviceSpecs []*ServiceSpec) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return fmt.Errorf("%s not found", registryName)
	}

	for _, spec := range serviceSpecs {
		spec.RegistryName = registryName
	}

	return bucket.registry.ApplyServices(serviceSpecs)
}

// GetService gets the service of the registry.
func (sr *ServiceRegistry) GetService(registryName, serviceName string) (*ServiceSpec, error) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return nil, fmt.Errorf("%s not found", registryName)
	}

	return bucket.registry.GetService(serviceName)
}

// ListServices lists all services of the registry.
func (sr *ServiceRegistry) ListServices(registryName string) ([]*ServiceSpec, error) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	bucket, exists := sr.registryBuckets[registryName]
	if !exists || !bucket.registered {
		return nil, fmt.Errorf("%s not found", registryName)
	}

	return bucket.registry.ListServices()
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

// Init initilizes ServiceRegistry.
func (sr *ServiceRegistry) Init(superSpec *supervisor.Spec) {
	sr.superSpec, sr.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	sr.reload()
}

// Inherit inherits previous generation of ServiceRegistry.
func (sr *ServiceRegistry) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	sr.Init(superSpec)
}

func (sr *ServiceRegistry) reload() {
	sr.registryBuckets = make(map[string]*registryBucket)
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
