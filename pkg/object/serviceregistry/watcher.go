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

	"github.com/google/uuid"
)

type (
	// ServiceEvent is the event of service.
	// Only one of Apply and Delete should be filled.
	ServiceEvent struct {
		// Apply creates or updates the service.
		Apply *ServiceSpec

		// Delete has optional service instances.
		Delete *ServiceSpec
	}

	// RegistryEvent is the event of service registry.
	// Only one of Init, Delete and Apply should be filled.
	RegistryEvent struct {
		RegistryName string

		// Replace replaces all services of the registry.
		Replace map[string]*ServiceSpec

		// Apply creates or updates services of the registry.
		Apply map[string]*ServiceSpec

		// Delete deletes services of the registry.
		// The spec of element has optional service instances.
		Delete map[string]*ServiceSpec
	}

	// ServiceWatcher is the watcher of service.
	ServiceWatcher interface {
		ID() string

		RegistryName() string
		ServiceName() string

		// Exists returns if the service exists.
		Exists() bool

		Watch() <-chan *ServiceEvent

		Stop()
	}

	// RegistryWatcher is the watcher of service registry.
	RegistryWatcher interface {
		ID() string

		RegistryName() string

		// Exists returns if the registry exists.
		Exists() bool

		// The channel will be closed if the registry is closed.
		Watch() <-chan *RegistryEvent

		Stop()
	}

	serviceWatcher struct {
		id           string
		registryName string
		serviceName  string
		eventChan    chan *ServiceEvent

		existsFn func() bool
		stopFn   func()
	}

	registryWatcher struct {
		id           string
		registryName string
		eventChan    chan *RegistryEvent

		existsFn func() bool
		stopFn   func()
	}
)

// NewRegistryWatcher creates a registry watcher.
func (sr *ServiceRegistry) NewRegistryWatcher(registryName string) RegistryWatcher {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	id := sr.generateID()

	watcher := &registryWatcher{
		id:           id,
		registryName: registryName,
		eventChan:    make(chan *RegistryEvent, 10),
		existsFn:     sr.registryExistsFn(registryName),
		stopFn:       sr.registryWacherStopFn(registryName, id),
	}

	_, exists := sr.registryBuckets[registryName]
	if !exists {
		sr.registryBuckets[registryName] = newRegisterBucket()
	}

	sr.registryBuckets[registryName].registryWatchers[id] = watcher

	return watcher
}

// NewServiceWatcher creates a service watcher.
func (sr *ServiceRegistry) NewServiceWatcher(registryName, serviceName string) ServiceWatcher {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	id := sr.generateID()

	watcher := &serviceWatcher{
		id:           id,
		registryName: registryName,
		serviceName:  serviceName,
		eventChan:    make(chan *ServiceEvent, 10),
		existsFn:     sr.serviceExistsFn(registryName, serviceName),
		stopFn:       sr.serviceWacherStopFn(registryName, serviceName, id),
	}

	_, exists := sr.registryBuckets[registryName]
	if !exists {
		sr.registryBuckets[registryName] = newRegisterBucket()
	}

	_, exists = sr.registryBuckets[registryName].serviceBuckets[serviceName]
	if !exists {
		sr.registryBuckets[registryName].serviceBuckets[serviceName] = newServiceBucket()
	}
	sr.registryBuckets[registryName].serviceBuckets[serviceName].serviceWatchers[id] = watcher

	return watcher
}

func (sr *ServiceRegistry) registryExistsFn(registryName string) func() bool {
	return func() bool {
		sr.mutex.Lock()
		defer sr.mutex.Unlock()

		bucket, exists := sr.registryBuckets[registryName]

		return exists && bucket.registered
	}
}

func (sr *ServiceRegistry) registryWacherStopFn(registryName, watcherID string) func() {
	return func() {
		sr.mutex.Lock()
		defer sr.mutex.Unlock()

		bucket, exists := sr.registryBuckets[registryName]
		if !exists {
			return
		}

		delete(bucket.registryWatchers, watcherID)

		if bucket.needClean() {
			delete(sr.registryBuckets, registryName)
		}
	}
}

func (sr *ServiceRegistry) serviceExistsFn(registryName, serviceName string) func() bool {
	return func() bool {
		_, err := sr.GetService(registryName, serviceName)
		return err == nil
	}
}

func (sr *ServiceRegistry) serviceWacherStopFn(registryName, serviceName, watcherID string) func() {
	return func() {
		sr.mutex.Lock()
		defer sr.mutex.Unlock()

		bucket, exists := sr.registryBuckets[registryName]
		if !exists {
			return
		}

		delete(bucket.serviceBuckets, watcherID)

		if bucket.needClean() {
			delete(sr.registryBuckets, registryName)
		}
	}
}

func (sr *ServiceRegistry) generateID() string {
	return uuid.NewString()
}

func (sr *ServiceRegistry) serviceWatchersKey(registryName, serviceName string) string {
	return fmt.Sprintf("%s/%s", registryName, serviceName)
}

func (w *serviceWatcher) ID() string {
	return w.id
}

func (w *serviceWatcher) RegistryName() string {
	return w.registryName
}

func (w *serviceWatcher) ServiceName() string {
	return w.serviceName
}

func (w *serviceWatcher) Watch() <-chan *ServiceEvent {
	return w.eventChan
}

func (w *serviceWatcher) EventChan() chan<- *ServiceEvent {
	return w.eventChan
}

func (w *serviceWatcher) Exists() bool {
	return w.existsFn()
}

func (w *serviceWatcher) Stop() {
	w.stopFn()
}

// ---

func (w *registryWatcher) ID() string {
	return w.id
}

func (w *registryWatcher) RegistryName() string {
	return w.registryName
}

func (w *registryWatcher) Watch() <-chan *RegistryEvent {
	return w.eventChan
}

func (w *registryWatcher) EventChan() chan<- *RegistryEvent {
	return w.eventChan
}

func (w *registryWatcher) Exists() bool {
	return w.existsFn()
}

func (w *registryWatcher) Stop() {
	w.stopFn()
}

// --- TODO: Use deepcopy-gen to generate code.

// DeepCopy deep copies ServiceEvent.
func (e *ServiceEvent) DeepCopy() *ServiceEvent {
	copy := &ServiceEvent{}

	if e.Apply != nil {
		copy.Apply = e.Apply.DeepCopy()
	}

	if e.Delete != nil {
		copy.Delete = e.Delete.DeepCopy()
	}

	return copy
}

// DeepCopy deep copies RegistryEvent.
func (e *RegistryEvent) DeepCopy() *RegistryEvent {
	copy := &RegistryEvent{
		RegistryName: e.RegistryName,
	}

	if e.Replace != nil {
		copy.Replace = make(map[string]*ServiceSpec)
		for k, v := range e.Replace {
			copy.Replace[k] = v.DeepCopy()
		}
	}

	if e.Apply != nil {
		copy.Apply = make(map[string]*ServiceSpec)
		for k, v := range e.Apply {
			copy.Apply[k] = v.DeepCopy()
		}
	}

	if e.Delete != nil {
		copy.Delete = make(map[string]*ServiceSpec)
		for k, v := range e.Delete {
			copy.Delete[k] = v.DeepCopy()
		}
	}

	return copy
}
