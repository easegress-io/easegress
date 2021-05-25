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

	"github.com/megaease/easegateway/pkg/cluster"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/option"
	"github.com/megaease/easegateway/pkg/storage"
)

// Global is the global supervisor.
var Global *Supervisor

// InitGlobalSupervisor initializes global supervisor.
// FIXME: Prune Global stuff after sending super to filters.
func InitGlobalSupervisor(super *Supervisor) {
	Global = super
}

const (
	maxStatusesRecordCount = 10
)

type (
	// Supervisor manages all objects.
	Supervisor struct {
		options *option.Options
		cls     cluster.Cluster

		storage           *storage.Storage
		runningCategories map[ObjectCategory]*RunningCategory
		firstHandle       bool
		firstHandleDone   chan struct{}
		done              chan struct{}
	}

	// RunningCategory is the bucket to gather running objects in the same category.
	RunningCategory struct {
		mutex          sync.RWMutex
		category       ObjectCategory
		runningObjects map[string]*RunningObject
	}

	// RunningObject is the running object.
	RunningObject struct {
		object Object
		spec   *Spec
	}
)

func newRunningObjectFromConfig(config string) (*RunningObject, error) {
	spec, err := NewSpec(config)
	if err != nil {
		return nil, fmt.Errorf("create spec failed: %s: %v", config, err)
	}

	return newRunningObjectFromSpec(spec)
}

func newRunningObjectFromSpec(spec *Spec) (*RunningObject, error) {
	registerObject, exists := objectRegistry[spec.Kind()]
	if !exists {
		return nil, fmt.Errorf("unsupported kind: %s", spec.Kind())
	}

	obj := reflect.New(reflect.TypeOf(registerObject).Elem()).Interface()
	object, ok := obj.(Object)
	if !ok {
		return nil, fmt.Errorf("%T is not an object", obj)
	}

	return &RunningObject{
		spec:   spec,
		object: object,
	}, nil
}

// Instance returns the instance of the object.
func (ro *RunningObject) Instance() Object {
	return ro.object
}

// Spec returns the spec of the object.
func (ro *RunningObject) Spec() *Spec {
	return ro.spec
}

func (ro *RunningObject) initWithRecovery(super *Supervisor) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from init, err: %v, stack trace:\n%s\n",
				ro.spec.Name(), err, debug.Stack())
		}
	}()
	ro.Instance().Init(ro.Spec(), super)
}

func (ro *RunningObject) inheritWithRecovery(previousGeneration Object, super *Supervisor) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from update, err: %v, stack trace:\n%s\n",
				ro.spec.Name(), err, debug.Stack())
		}
	}()
	ro.Instance().Inherit(ro.Spec(), previousGeneration, super)
}

func (ro *RunningObject) closeWithRecovery() {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: recover from close, err: %v, stack trace:\n%s\n",
				ro.spec.Name(), err, debug.Stack())
		}
	}()

	ro.object.Close()
}

// MustNew creates a Supervisor.
func MustNew(opt *option.Options, cls cluster.Cluster) *Supervisor {
	s := &Supervisor{
		options: opt,
		cls:     cls,

		storage:           storage.New(opt, cls),
		runningCategories: make(map[ObjectCategory]*RunningCategory),
		firstHandle:       true,
		firstHandleDone:   make(chan struct{}),
		done:              make(chan struct{}),
	}

	for _, category := range objectOrderedCategories {
		s.runningCategories[category] = &RunningCategory{
			category:       category,
			runningObjects: make(map[string]*RunningObject),
		}
	}

	s.initSystemControllers()

	go s.run()

	return s
}

// Options returns the options applied to supervisor.
func (s *Supervisor) Options() *option.Options {
	return s.options
}

// Cluster return the cluster applied to supervisor.
func (s *Supervisor) Cluster() cluster.Cluster {
	return s.cls
}

func (s *Supervisor) initSystemControllers() {
	for kind, rootObject := range objectRegistry {
		if rootObject.Category() != CategorySystemController {
			continue
		}

		meta := &MetaSpec{
			// NOTE: Use kind to be the name since the system controller is unique.
			Name: kind,
			Kind: kind,
		}
		spec := newSpecInternal(meta, rootObject.DefaultSpec())
		ro, err := newRunningObjectFromSpec(spec)
		if err != nil {
			panic(err)
		}

		ro.initWithRecovery(s)
		s.runningCategories[CategorySystemController].runningObjects[kind] = ro

		logger.Infof("create system controller %s", spec.Name())
	}
}

func (s *Supervisor) run() {
	for {
		select {
		case <-s.done:
			s.close()
			return
		case config := <-s.storage.WatchConfig():
			s.applyConfig(config)
		}
	}
}

func (s *Supervisor) applyConfig(config map[string]string) {
	if s.firstHandle {
		defer func() {
			s.firstHandle = false
			close(s.firstHandleDone)
		}()
	}

	// Create, update, delete from high to low priority.
	for _, category := range objectOrderedCategories {
		// NOTE: System controller can't be manipulated after initialized.
		if category != CategorySystemController {
			s.applyConfigInCategory(config, category)
		}
	}
}

func (s *Supervisor) applyConfigInCategory(config map[string]string, category ObjectCategory) {
	rc := s.runningCategories[category]

	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	// Delete running object.
	for name, ro := range rc.runningObjects {
		if _, exists := config[name]; !exists {
			ro.closeWithRecovery()
			delete(rc.runningObjects, name)
			logger.Infof("delete %s", name)
		}
	}

	// Create or update running object.
	for name, yamlConfig := range config {
		var prevInstance Object
		prev, exists := rc.runningObjects[name]
		if exists {
			// No need to update if the config not changed.
			if yamlConfig == prev.spec.YAMLConfig() {
				continue
			}
			prevInstance = prev.Instance()
		}

		ro, err := newRunningObjectFromConfig(yamlConfig)
		if err != nil {
			logger.Errorf("BUG: %s: %v", name, err)
			continue
		}
		if ro.object.Category() != category {
			continue
		}
		if name != ro.Spec().Name() {
			logger.Errorf("BUG: key and spec got different names: %s vs %s",
				name, ro.Spec().Name())
			continue
		}

		if prevInstance == nil {
			ro.initWithRecovery(s)
			logger.Infof("create %s", name)
		} else {
			ro.inheritWithRecovery(prevInstance, s)
			logger.Infof("update %s", name)
		}

		rc.runningObjects[name] = ro
	}
}

// WalkFunc is the type of the function called for
// each running object visited by WalkRunningObjects.
type WalkFunc func(runningObject *RunningObject) bool

// WalkRunningObjects walks every running object until walkFn returns false.
func (s *Supervisor) WalkRunningObjects(walkFn WalkFunc, category ObjectCategory) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkRunningObjects recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}

	}()

	for _, rc := range s.runningCategories {
		if category != CategoryAll && category != rc.category {
			continue
		}

		func() {
			rc.mutex.RLock()
			defer rc.mutex.RUnlock()

			for _, runningObject := range rc.runningObjects {
				toContinue := walkFn(runningObject)
				if !toContinue {
					break
				}
			}
		}()
	}
}

// GetRunningObject returns the running object with the existing flag.
// If the category is empty string, GetRunningObject will try to find
// the running object in every category.
func (s *Supervisor) GetRunningObject(name string, category ObjectCategory) (ro *RunningObject, exists bool) {
	searchFromCategory := func(rc *RunningCategory) {
		rc.mutex.RLock()
		defer rc.mutex.RUnlock()

		ro, exists = rc.runningObjects[name]
	}

	switch category {
	case CategoryAll:
		for _, rc := range s.runningCategories {
			searchFromCategory(rc)
			if exists {
				return
			}
		}
	default:
		rc, rcExists := s.runningCategories[category]
		if !rcExists {
			logger.Errorf("BUG: category %s not found", category)
			return nil, false
		}
		searchFromCategory(rc)
	}

	return
}

// FirstHandleDone returns the firstHandleDone channel,
// which will be closed after creating all objects at first time.
func (s *Supervisor) FirstHandleDone() chan struct{} {
	return s.firstHandleDone
}

// Close closes Supervisor.
func (s *Supervisor) Close(wg *sync.WaitGroup) {
	defer wg.Done()
	s.done <- struct{}{}
	<-s.done
}

func (s *Supervisor) close() {
	s.storage.Close()

	// Close from low to high priority.
	for i := len(objectOrderedCategories) - 1; i >= 0; i-- {
		rc := s.runningCategories[objectOrderedCategories[i]]
		func() {
			rc.mutex.Lock()
			defer rc.mutex.Unlock()

			for name, ro := range rc.runningObjects {
				ro.closeWithRecovery()
				delete(rc.runningObjects, name)
				logger.Infof("delete %s", ro.spec.Name())
			}
		}()
	}

	close(s.done)
}
