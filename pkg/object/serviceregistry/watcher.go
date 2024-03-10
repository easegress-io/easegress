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

package serviceregistry

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/megaease/easegress/v2/pkg/logger"
)

type (
	// ServiceEvent is the event of service.
	// It concludes complete instances of the service.
	// NOTE: Changing inner fields needs to adapt to its methods DeepCopy, etc.
	ServiceEvent struct {
		// SourceRegistryName is the registry which caused the event,
		// the RegistryName of specs may not be the same with it.
		SourceRegistryName string
		Instances          map[string]*ServiceInstanceSpec
	}

	// RegistryEvent is the event of service registry.
	// If UseReplace is true, the event handler should use Replace field even it is empty.
	// NOTE: Changing inner fields needs to adapt to its methods Empty, DeepCopy, Validate, etc.
	RegistryEvent struct {
		// SourceRegistryName is the registry which caused the event,
		// the RegistryName of specs may not be the same with it.
		SourceRegistryName string
		UseReplace         bool

		// Replace replaces all service instances of the registry.
		Replace map[string]*ServiceInstanceSpec

		// Apply creates or updates service instances of the registry.
		Apply map[string]*ServiceInstanceSpec

		// Delete deletes service instances of the registry.
		Delete map[string]*ServiceInstanceSpec
	}

	// ServiceWatcher is the watcher of service.
	ServiceWatcher interface {
		ID() string

		RegistryName() string
		ServiceName() string

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
		stopFn:       sr.registryWatcherStopFn(registryName, id),
	}

	_, exists := sr.registryBuckets[registryName]
	if !exists {
		sr.registryBuckets[registryName] = newRegisterBucket()
	}

	sr.registryBuckets[registryName].registryWatchers[id] = watcher

	instances, err := sr._listAllServiceInstances(registryName)
	if err != nil {
		logger.Warnf("watch registry %s: list service instances failed: %v", registryName, err)
	} else {
		watcher.EventChan() <- &RegistryEvent{
			SourceRegistryName: registryName,
			UseReplace:         true,
			Replace:            instances,
		}
	}

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
		stopFn:       sr.serviceWatcherStopFn(registryName, serviceName, id),
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

	instances, err := sr._listServiceInstances(registryName, serviceName)
	if err != nil {
		logger.Warnf("watch service %s/%s: list service instances failed: %v",
			registryName, serviceName, err)
	} else {
		watcher.EventChan() <- &ServiceEvent{
			SourceRegistryName: registryName,
			Instances:          instances,
		}
	}

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

func (sr *ServiceRegistry) registryWatcherStopFn(registryName, watcherID string) func() {
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

func (sr *ServiceRegistry) serviceWatcherStopFn(registryName, serviceName, watcherID string) func() {
	return func() {
		sr.mutex.Lock()
		defer sr.mutex.Unlock()

		bucket, exists := sr.registryBuckets[registryName]
		if !exists {
			return
		}

		serviceBucket, exists := bucket.serviceBuckets[serviceName]
		if !exists {
			return
		}

		delete(serviceBucket.serviceWatchers, watcherID)

		if serviceBucket.needClean() {
			delete(bucket.serviceBuckets, serviceName)
		}

		if bucket.needClean() {
			delete(sr.registryBuckets, registryName)
		}
	}
}

func (sr *ServiceRegistry) generateID() string {
	return uuid.NewString()
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
	copy := &ServiceEvent{
		SourceRegistryName: e.SourceRegistryName,
	}

	if e.Instances != nil {
		copy.Instances = make(map[string]*ServiceInstanceSpec)
		for k, v := range e.Instances {
			copy.Instances[k] = v.DeepCopy()
		}
	}

	return copy
}

// DeepCopy deep copies RegistryEvent.
func (e *RegistryEvent) DeepCopy() *RegistryEvent {
	copy := &RegistryEvent{
		SourceRegistryName: e.SourceRegistryName,
		UseReplace:         e.UseReplace,
		Replace:            e.Replace,
	}

	if e.Replace != nil {
		copy.Replace = make(map[string]*ServiceInstanceSpec)
		for k, v := range e.Replace {
			copy.Replace[k] = v.DeepCopy()
		}
	}

	if e.Apply != nil {
		copy.Apply = make(map[string]*ServiceInstanceSpec)
		for k, v := range e.Apply {
			copy.Apply[k] = v.DeepCopy()
		}
	}

	if e.Delete != nil {
		copy.Delete = make(map[string]*ServiceInstanceSpec)
		for k, v := range e.Delete {
			copy.Delete[k] = v.DeepCopy()
		}
	}

	return copy
}

// Empty returns if the event contains nothing to handle.
func (e *RegistryEvent) Empty() bool {
	return !e.UseReplace && len(e.Replace) == 0 &&
		len(e.Delete) == 0 && len(e.Apply) == 0
}

// Validate validates RegistryEvent.
func (e *RegistryEvent) Validate() error {
	for k, v := range e.Replace {
		err := v.Validate()
		if err != nil {
			return fmt.Errorf("replace element %v is invalid: %v", k, err)
		}
	}

	for k, v := range e.Delete {
		err := v.Validate()
		if err != nil {
			return fmt.Errorf("delete element %v is invalid: %v", k, err)
		}
	}

	for k, v := range e.Apply {
		err := v.Validate()
		if err != nil {
			return fmt.Errorf("apply element %v is invalid: %v", k, err)
		}
	}

	return nil
}
