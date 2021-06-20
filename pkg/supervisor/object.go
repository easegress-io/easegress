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

package supervisor

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocol"
)

type (
	// ObjectEntity is the object entity.
	ObjectEntity struct {
		super *Supervisor

		generation uint64
		instance   Object
		spec       *Spec
	}

	ObjectEntityWatcher struct {
		mutex     sync.Mutex
		filter    ObjectEntityWatcherFilter
		entities  map[string]*ObjectEntity
		eventChan chan *ObjectEntityWatcherEvent
	}

	ObjectEntityWatcherEvent struct {
		Delete map[string]*ObjectEntity
		Create map[string]*ObjectEntity
		Update map[string]*ObjectEntity
	}

	// ObjectEntityWatcherFilter is the filter type,
	// it returns true when it wants to get the notification
	// about the entity's creation/update/deletion.
	ObjectEntityWatcherFilter func(entity *ObjectEntity) bool

	// ObjectRegistry records every object entity according to config
	// from storage without manage their lifecycle.
	ObjectRegistry struct {
		super        *Supervisor
		configSyncer *configSyncer

		mutex    sync.Mutex
		entities map[string]*ObjectEntity
		watchers map[string]*ObjectEntityWatcher

		done chan struct{}
	}
)

func FilterCategory(categories ...ObjectCategory) ObjectEntityWatcherFilter {
	allCategory := false
	for _, category := range categories {
		if category == CategoryAll {
			allCategory = true
		}
	}
	return func(entity *ObjectEntity) bool {
		if allCategory {
			return true
		}

		for _, category := range categories {
			if category == entity.instance.Category() {
				return true
			}
		}

		return false
	}
}

func newObjectEntityWatcherEvent() *ObjectEntityWatcherEvent {
	return &ObjectEntityWatcherEvent{
		Delete: make(map[string]*ObjectEntity),
		Create: make(map[string]*ObjectEntity),
		Update: make(map[string]*ObjectEntity),
	}
}

func newObjectRegistry(super *Supervisor) *ObjectRegistry {
	or := &ObjectRegistry{
		super:        super,
		configSyncer: newconfigSyncer(super.Options(), super.Cluster()),
		entities:     make(map[string]*ObjectEntity),
		watchers:     map[string]*ObjectEntityWatcher{},
		done:         make(chan struct{}),
	}

	go or.run()

	return or
}

func (or *ObjectRegistry) run() {
	for {
		select {
		case <-or.done:
			return
		case config := <-or.configSyncer.watchConfig():
			or.applyConfig(config)
		}
	}
}

func (or *ObjectRegistry) applyConfig(config map[string]string) {
	or.mutex.Lock()
	defer func() {
		or.mutex.Unlock()
	}()

	del, create, update := make(map[string]*ObjectEntity), make(map[string]*ObjectEntity), make(map[string]*ObjectEntity)

	for name, entity := range or.entities {
		if _, exists := config[name]; !exists {
			delete(or.entities, name)
			del[name] = entity
		}
	}

	for name, yamlConfig := range config {
		prevEntity, exists := or.entities[name]
		if exists && yamlConfig == prevEntity.Spec().yamlConfig {
			continue
		}

		entity, err := or.super.NewObjectEntityFromConfig(yamlConfig)
		if err != nil {
			logger.Errorf("BUG: %s: %v", name, err)
			continue
		}

		if prevEntity != nil {
			update[name] = entity
		} else {
			create[name] = entity
		}
		or.entities[name] = entity
	}

	for _, watcher := range or.watchers {
		func() {
			watcher.mutex.Lock()
			defer watcher.mutex.Unlock()

			event := newObjectEntityWatcherEvent()

			for name, entity := range del {
				if watcher.filter(entity) {
					event.Delete[name] = entity
					delete(watcher.entities, name)
				}
			}
			for name, entity := range create {
				if watcher.filter(entity) {
					event.Create[name] = entity
					watcher.entities[name] = entity
				}
			}
			for name, entity := range update {
				if watcher.filter(entity) {
					event.Update[name] = entity
					watcher.entities[name] = entity
				}
			}

			if len(event.Delete)+len(event.Create)+len(event.Update) > 0 {
				watcher.eventChan <- event
			}
		}()
	}
}

func (or *ObjectRegistry) NewWatcher(name string, filter ObjectEntityWatcherFilter) *ObjectEntityWatcher {
	watcher := &ObjectEntityWatcher{
		filter:    filter,
		entities:  make(map[string]*ObjectEntity),
		eventChan: make(chan *ObjectEntityWatcherEvent, 10),
	}

	or.mutex.Lock()
	defer or.mutex.Unlock()

	firstEvent := newObjectEntityWatcherEvent()
	for name, entity := range or.entities {
		if watcher.filter(entity) {
			firstEvent.Create[name] = entity
			watcher.entities[name] = entity
		}
	}
	watcher.eventChan <- firstEvent

	_, exists := or.watchers[name]
	if exists {
		logger.Errorf("BUG: watcher %s existed", name)
		return nil
	}

	or.watchers[name] = watcher

	return watcher
}

func (or *ObjectRegistry) CloseWatcher(name string) {
	or.mutex.Lock()
	defer or.mutex.Unlock()

	delete(or.watchers, name)
}

func (or *ObjectRegistry) close() {
	or.configSyncer.close()
	close(or.done)
}

// Watch returns then channel to notify the event.
func (w *ObjectEntityWatcher) Watch() <-chan *ObjectEntityWatcherEvent {
	return w.eventChan
}

// Entities returns the snapshot of the object entites of the watcher.
func (w *ObjectEntityWatcher) Entities() map[string]*ObjectEntity {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	entities := make(map[string]*ObjectEntity)
	for name, entity := range w.entities {
		entities[name] = entity
	}

	return entities
}

func (s *Supervisor) NewObjectEntityFromConfig(config string) (*ObjectEntity, error) {
	spec, err := NewSpec(config)
	if err != nil {
		return nil, fmt.Errorf("create spec failed: %s: %v", config, err)
	}

	return s.NewObjectEntityFromSpec(spec)
}

func (s *Supervisor) NewObjectEntityFromSpec(spec *Spec) (*ObjectEntity, error) {
	registerObject, exists := objectRegistry[spec.Kind()]
	if !exists {
		return nil, fmt.Errorf("unsupported kind: %s", spec.Kind())
	}

	obj := reflect.New(reflect.TypeOf(registerObject).Elem()).Interface()
	instance, ok := obj.(Object)
	if !ok {
		return nil, fmt.Errorf("%T is not an object", obj)
	}

	return &ObjectEntity{
		super:      s,
		generation: 0,
		spec:       spec,
		instance:   instance,
	}, nil
}

// Instance returns the instance of the object entity.
func (e *ObjectEntity) Instance() Object {
	return e.instance
}

// Spec returns the spec of the object entity.
func (e *ObjectEntity) Spec() *Spec {
	return e.spec
}

// Generation returns the generation of the object entity.
func (e *ObjectEntity) Generation() uint64 {
	return e.generation
}

// InitWithRecovery initializes the object with built-in recovery.
// muxMapper could be nil if the object is not TrafficGate and Pipeline.
func (e *ObjectEntity) InitWithRecovery(muxMapper protocol.MuxMapper) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from Init, err: %v, stack trace:\n%s\n",
				e.spec.Name(), err, debug.Stack())
		}
	}()

	switch instance := e.Instance().(type) {
	case Controller:
		instance.Init(e.Spec(), e.super)
	case TrafficObject:
		instance.Init(e.Spec(), e.super, muxMapper)
	default:
		panic(fmt.Errorf("BUG: unsupported object type %T", instance))
	}

	e.generation = 1
}

// InheritWithRecovery inherits the object with built-in recovery.
func (e *ObjectEntity) InheritWithRecovery(previousEntity *ObjectEntity, muxMapper protocol.MuxMapper) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from Inherit, err: %v, stack trace:\n%s\n",
				e.spec.Name(), err, debug.Stack())
		}
	}()

	switch instance := e.Instance().(type) {
	case Controller:
		instance.Inherit(e.Spec(), previousEntity.Instance(), e.super)
	case TrafficObject:
		instance.Inherit(e.Spec(), previousEntity.Instance(), e.super, muxMapper)
	default:
		panic(fmt.Errorf("BUG: unsupported object type %T", instance))
	}

	e.generation++
}

// CloseWithRecovery closes the object with built-in recovery.
func (e *ObjectEntity) CloseWithRecovery() {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from Close, err: %v, stack trace:\n%s\n",
				e.spec.Name(), err, debug.Stack())
		}
	}()

	e.instance.Close()
}
