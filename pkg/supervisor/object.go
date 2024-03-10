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

package supervisor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/cluster"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

type (
	// ObjectEntity is the object entity.
	ObjectEntity struct {
		super *Supervisor

		generation uint64
		instance   Object
		spec       *Spec
	}

	// ObjectEntityWatcher is the watcher for object entity
	ObjectEntityWatcher struct {
		mutex     sync.Mutex
		filter    ObjectEntityWatcherFilter
		entities  map[string]*ObjectEntity
		eventChan chan *ObjectEntityWatcherEvent
	}

	// ObjectEntityWatcherEvent is the event for watcher
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
		super *Supervisor

		configSyncer          cluster.Syncer
		configSyncChan        <-chan map[string]string
		configPrefix          string
		configLocalPath       string
		backupConfigLocalPath string

		mutex    sync.Mutex
		entities map[string]*ObjectEntity
		watchers map[string]*ObjectEntityWatcher

		done chan struct{}
	}
)

const (
	defaultDumpInterval   = 1 * time.Hour
	configFilePath        = "running_objects.json"
	backupdConfigFilePath = "running_objects.bak.json"
)

// FilterCategory returns a bool function to check if the object entity is filtered by category or not
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

func newObjectRegistry(super *Supervisor, initObjs map[string]string, interval string) *ObjectRegistry {
	cls := super.Cluster()
	prefix := cls.Layout().ConfigObjectPrefix()
	objs, err := cls.GetPrefix(prefix)
	if err != nil {
		panic(fmt.Errorf("get existing objects failed: %v", err))
	}
	for k, v := range initObjs {
		key := cls.Layout().ConfigObjectKey(k)
		if _, ok := objs[key]; ok {
			logger.Infof("initial object %s already exist in config, skip", k)
			continue
		}
		if err = cls.Put(key, v); err != nil {
			panic(fmt.Errorf("add initial object %s to config failed: %v", k, err))
		}
	}
	dumpInterval := defaultDumpInterval
	if len(interval) != 0 {
		if confInterval, err := time.ParseDuration(interval); err == nil {
			dumpInterval = confInterval
		} else {
			logger.Errorf("parse objects dump interval [%s] as time.Duration error", confInterval)
		}
	}
	syncer, err := cls.Syncer(dumpInterval)
	if err != nil {
		panic(fmt.Errorf("get syncer failed: %v", err))
	}

	syncChan, err := syncer.SyncPrefix(prefix)
	if err != nil {
		panic(fmt.Errorf("sync prefix %s failed: %v", prefix, err))
	}

	or := &ObjectRegistry{
		super:                 super,
		configSyncer:          syncer,
		configSyncChan:        syncChan,
		configPrefix:          prefix,
		configLocalPath:       filepath.Join(super.Options().AbsHomeDir, configFilePath),
		backupConfigLocalPath: filepath.Join(super.Options().AbsHomeDir, backupdConfigFilePath),
		entities:              make(map[string]*ObjectEntity),
		watchers:              map[string]*ObjectEntityWatcher{},
		done:                  make(chan struct{}),
	}

	go or.run()

	return or
}

func (or *ObjectRegistry) run() {
	for {
		select {
		case <-or.done:
			return
		case kv := <-or.configSyncChan:
			config := make(map[string]string)
			for k, v := range kv {
				k = strings.TrimPrefix(k, or.configPrefix)
				config[k] = v
			}
			or.applyConfig(config)
			or.storeConfigInLocal(config)
		}
	}
}

func (or *ObjectRegistry) applyConfig(config map[string]string) {
	or.mutex.Lock()
	defer or.mutex.Unlock()

	var (
		deleted = make(map[string]*ObjectEntity)
		created = make(map[string]*ObjectEntity)
		updated = make(map[string]*ObjectEntity)
	)

	for name, entity := range or.entities {
		if _, exists := config[name]; !exists {
			delete(or.entities, name)
			deleted[name] = entity
		}
	}

	for name, jsonConfig := range config {
		entity, err := or.super.NewObjectEntityFromConfig(jsonConfig)
		if err != nil {
			logger.Errorf("BUG: %s: %v", name, err)
			continue
		}

		prevEntity, exists := or.entities[name]
		if exists && prevEntity.Spec().Equals(entity.Spec()) {
			continue
		}

		if prevEntity != nil {
			updated[name] = entity
		} else {
			created[name] = entity
		}
		or.entities[name] = entity
	}

	for _, watcher := range or.watchers {
		func() {
			watcher.mutex.Lock()
			defer watcher.mutex.Unlock()

			event := newObjectEntityWatcherEvent()

			for name, entity := range deleted {
				if watcher.filter(entity) {
					event.Delete[name] = entity
					delete(watcher.entities, name)
				}
			}
			for name, entity := range created {
				if watcher.filter(entity) {
					event.Create[name] = entity
					watcher.entities[name] = entity
				}
			}
			for name, entity := range updated {
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

// NewWatcher creates a watcher
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

// CloseWatcher closes and releases a watcher.
func (or *ObjectRegistry) CloseWatcher(name string) {
	or.mutex.Lock()
	defer or.mutex.Unlock()

	delete(or.watchers, name)
}

func (or *ObjectRegistry) storeConfigInLocal(config map[string]string) {
	buff := bytes.NewBuffer(nil)

	configBuff, err := codectool.MarshalJSON(config)
	if err != nil {
		logger.Errorf("marshal %s to json failed: %v", buff, err)
		return
	}
	buff.Write(configBuff)

	if _, err := os.Stat(or.configLocalPath); err == nil {
		err = os.Rename(or.configLocalPath, or.backupConfigLocalPath)
		if err != nil {
			logger.Errorf("rename %s to %s failed: %v", or.configLocalPath, or.backupConfigLocalPath, err)
		}
	}

	err = os.WriteFile(or.configLocalPath, buff.Bytes(), 0o644)
	if err != nil {
		logger.Errorf("write %s failed: %v", or.configLocalPath, err)
		return
	}
}

func (or *ObjectRegistry) close() {
	or.configSyncer.Close()
	close(or.done)
}

// Watch returns then channel to notify the event.
func (w *ObjectEntityWatcher) Watch() <-chan *ObjectEntityWatcherEvent {
	return w.eventChan
}

// Entities returns the snapshot of the object entities of the watcher.
func (w *ObjectEntityWatcher) Entities() map[string]*ObjectEntity {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	entities := make(map[string]*ObjectEntity)
	for name, entity := range w.entities {
		entities[name] = entity
	}

	return entities
}

// NewObjectEntityFromConfig creates an object entity from configuration
func (s *Supervisor) NewObjectEntityFromConfig(jsonConfig string) (*ObjectEntity, error) {
	spec, err := s.NewSpec(jsonConfig)
	if err != nil {
		return nil, fmt.Errorf("create spec failed: %s: %v", jsonConfig, err)
	}

	return s.NewObjectEntityFromSpec(spec)
}

// NewObjectEntityFromSpec creates an object entity from a spec
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
func (e *ObjectEntity) InitWithRecovery(muxMapper context.MuxMapper) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from Init, err: %v, stack trace:\n%s\n",
				e.spec.Name(), err, debug.Stack())
		}
	}()

	switch instance := e.Instance().(type) {
	case Controller:
		instance.Init(e.Spec())
	case TrafficObject:
		instance.Init(e.Spec(), muxMapper)
	default:
		panic(fmt.Errorf("BUG: unsupported object type %T", instance))
	}

	e.generation = 1
}

// InheritWithRecovery inherits the object with built-in recovery.
func (e *ObjectEntity) InheritWithRecovery(previousEntity *ObjectEntity, muxMapper context.MuxMapper) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from Inherit, err: %v, stack trace:\n%s\n",
				e.spec.Name(), err, debug.Stack())
		}
	}()

	switch instance := e.Instance().(type) {
	case Controller:
		instance.Inherit(e.Spec(), previousEntity.Instance())
	case TrafficObject:
		instance.Inherit(e.Spec(), previousEntity.Instance(), muxMapper)
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
