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

// Package rawconfigtrafficcontroller implements the RawConfigTrafficController.
package rawconfigtrafficcontroller

import (
	"fmt"
	"strings"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
	"github.com/megaease/easegress/v2/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the category of RawConfigTrafficController.
	Category = supervisor.CategorySystemController

	// Kind is the kind of RawConfigTrafficController.
	Kind = "RawConfigTrafficController"

	// DefaultNamespace is the namespace of RawConfigTrafficController
	DefaultNamespace = api.DefaultNamespace
)

type (
	// RawConfigTrafficController is a system controller to manage
	// TrafficGate, Pipeline and their relationship.
	RawConfigTrafficController struct {
		superSpec *supervisor.Spec
		spec      *Spec

		watcher   *supervisor.ObjectEntityWatcher
		namespace string
		done      chan struct{}
	}

	// Spec describes RawConfigTrafficController.
	Spec struct{}
)

func init() {
	supervisor.Register(&RawConfigTrafficController{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"rawconfigtrafficcontroller", "rctc"},
	})
}

// Category returns the category of RawConfigTrafficController.
func (rctc *RawConfigTrafficController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of RawConfigTrafficController.
func (rctc *RawConfigTrafficController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of RawConfigTrafficController.
func (rctc *RawConfigTrafficController) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes RawConfigTrafficController.
func (rctc *RawConfigTrafficController) Init(superSpec *supervisor.Spec) {
	rctc.superSpec, rctc.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	rctc.reload(nil)
}

// Inherit inherits previous generation of RawConfigTrafficController.
func (rctc *RawConfigTrafficController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	rctc.superSpec, rctc.spec = superSpec, superSpec.ObjectSpec().(*Spec)

	// Close will clean all the using resources.
	prev := previousGeneration.(*RawConfigTrafficController)
	watcher := prev.watcher
	close(prev.done)

	rctc.reload(watcher)
}

// GetPipeline gets Pipeline within the default namespace
func (rctc *RawConfigTrafficController) GetPipeline(name string) (context.Handler, bool) {
	tc := rctc.getTrafficController()

	p, exist := tc.GetPipeline(DefaultNamespace, name)
	if !exist {
		return nil, false
	}
	handler := p.Instance().(context.Handler)
	return handler, true
}

func (rctc *RawConfigTrafficController) reload(watcher *supervisor.ObjectEntityWatcher) {
	rctc.namespace = DefaultNamespace

	rctc.watcher = watcher
	if rctc.watcher == nil {
		rctc.watcher = rctc.superSpec.Super().ObjectRegistry().NewWatcher(rctc.superSpec.Name(),
			supervisor.FilterCategory(
				supervisor.CategoryTrafficGate,
				supervisor.CategoryPipeline))
	}

	rctc.done = make(chan struct{})

	go rctc.run()
}

func (rctc *RawConfigTrafficController) getTrafficController() *trafficcontroller.TrafficController {
	entity, exists := rctc.superSpec.Super().GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}

	return tc
}

func (rctc *RawConfigTrafficController) run() {
	for {
		select {
		case <-rctc.done:
			return
		case event := <-rctc.watcher.Watch():
			rctc.handleEvent(event)
		}
	}
}

func (rctc *RawConfigTrafficController) handleEvent(event *supervisor.ObjectEntityWatcherEvent) {
	tc := rctc.getTrafficController()

	for name, entity := range event.Delete {
		var err error

		kind := entity.Spec().Kind()
		if kind == pipeline.Kind {
			err = tc.DeletePipeline(DefaultNamespace, name)
		} else if _, ok := supervisor.TrafficObjectKinds[kind]; ok {
			err = tc.DeleteTrafficGate(DefaultNamespace, name)
		} else {
			logger.Errorf("BUG: unexpected kind %T", kind)
		}

		if err != nil {
			logger.Errorf("delete %s %s/%s failed: %v", kind, DefaultNamespace, name, err)
		}
	}

	for _, entity := range event.Create {
		var err error

		kind := entity.Spec().Kind()
		if kind == pipeline.Kind {
			_, err = tc.CreatePipeline(DefaultNamespace, entity)
		} else if _, ok := supervisor.TrafficObjectKinds[kind]; ok {
			_, err = tc.CreateTrafficGate(DefaultNamespace, entity)
		} else {
			logger.Errorf("BUG: unexpected kind %T", kind)
		}

		if err != nil {
			logger.Errorf("create %s %s/%s failed: %v", kind, DefaultNamespace, entity.Spec().Name(), err)
		}
	}

	for _, entity := range event.Update {
		var err error

		kind := entity.Instance().Kind()
		if kind == pipeline.Kind {
			_, err = tc.UpdatePipeline(DefaultNamespace, entity)
		} else if _, ok := supervisor.TrafficObjectKinds[kind]; ok {
			_, err = tc.UpdateTrafficGate(DefaultNamespace, entity)
		} else {
			logger.Errorf("BUG: unexpected kind %T", kind)
		}

		if err != nil {
			logger.Errorf("update %s %s/%s failed: %v", kind, DefaultNamespace, entity.Spec().Name(), err)
		}
	}
}

// Status returns the status of RawConfigTrafficController.
// StatusInSameNamespace:
//   - Namespace: default -> DefaultNamespace
//   - TrafficGates: map[objectName]objectStatus
//   - Pipelines: map[objectName]objectStatus
func (rctc *RawConfigTrafficController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: struct{}{},
	}
}

// Close closes RawConfigTrafficController.
func (rctc *RawConfigTrafficController) Close() {
	close(rctc.done)
	rctc.superSpec.Super().ObjectRegistry().CloseWatcher(rctc.superSpec.Name())
	tc := rctc.getTrafficController()
	tc.Clean(rctc.namespace)
}
