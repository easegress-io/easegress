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

package rawconfigtrafficcontroller

import (
	"fmt"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of RawConfigTrafficController.
	Category = supervisor.CategorySystemController

	// Kind is the kind of RawConfigTrafficController.
	Kind = "RawConfigTrafficController"

	// DefaultNamespace is the namespace of RawConfigTrafficController
	DefaultNamespace = "default"
)

type (
	// RawConfigTrafficController is a system controller to manage
	// TrafficGate, Pipeline and their relationship.
	RawConfigTrafficController struct {
		superSpec *supervisor.Spec
		spec      *Spec

		watcher   *supervisor.ObjectEntityWatcher
		tc        *trafficcontroller.TrafficController
		namespace string
		done      chan struct{}
	}

	// Spec describes RawConfigTrafficController.
	Spec struct{}

	// Status is the status of RawConfigTrafficController.
	Status = trafficcontroller.StatusInSameNamespace
)

func init() {
	supervisor.Register(&RawConfigTrafficController{})
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
	rctc.reload()
}

// Inherit inherits previous generation of RawConfigTrafficController.
func (rctc *RawConfigTrafficController) Inherit(spec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	rctc.Init(spec)
}

// GetHTTPPipeline gets Pipeline within the default namespace
func (rctc *RawConfigTrafficController) GetHTTPPipeline(name string) (protocol.HTTPHandler, bool) {
	p, exist := rctc.tc.GetHTTPPipeline(DefaultNamespace, name)
	if !exist {
		return nil, false
	}
	handler := p.Instance().(protocol.HTTPHandler)
	return handler, true
}

func (rctc *RawConfigTrafficController) reload() {
	entity, exists := rctc.superSpec.Super().GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}
	rctc.tc = tc
	rctc.namespace = DefaultNamespace

	rctc.watcher = rctc.superSpec.Super().ObjectRegistry().NewWatcher(rctc.superSpec.Name(),
		supervisor.FilterCategory(
			supervisor.CategoryTrafficGate,
			supervisor.CategoryPipeline))
	rctc.done = make(chan struct{})

	go rctc.run()
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
	for name, entity := range event.Delete {
		var err error

		kind := entity.Spec().Kind()
		switch kind {
		case httpserver.Kind:
			err = rctc.tc.DeleteHTTPServer(DefaultNamespace, name)
		case httppipeline.Kind:
			err = rctc.tc.DeleteHTTPPipeline(DefaultNamespace, name)
		default:
			logger.Errorf("BUG: unexpected kind %T", kind)
		}

		if err != nil {
			logger.Errorf("delete %s %s/%s failed: %v", kind, DefaultNamespace, name, err)
		}
	}

	for _, entity := range event.Create {
		var err error

		kind := entity.Spec().Kind()
		switch kind {
		case httpserver.Kind:
			_, err = rctc.tc.CreateHTTPServer(DefaultNamespace, entity)
		case httppipeline.Kind:
			_, err = rctc.tc.CreateHTTPPipeline(DefaultNamespace, entity)
		default:
			logger.Errorf("BUG: unexpected kind %T", kind)
		}

		if err != nil {
			logger.Errorf("create %s %s/%s failed: %v", kind, DefaultNamespace, entity.Spec().Name(), err)
		}
	}

	for _, entity := range event.Update {
		var err error

		kind := entity.Instance().Kind()
		switch kind {
		case httpserver.Kind:
			_, err = rctc.tc.UpdateHTTPServer(DefaultNamespace, entity)
		case httppipeline.Kind:
			_, err = rctc.tc.UpdateHTTPPipeline(DefaultNamespace, entity)
		default:
			logger.Errorf("BUG: unexpected kind %T", kind)
		}

		if err != nil {
			logger.Errorf("update %s %s/%s failed: %v", kind, DefaultNamespace, entity.Spec().Name(), err)
		}
	}
}

// Status returns the status of RawConfigTrafficController.
func (rctc *RawConfigTrafficController) Status() *supervisor.Status {
	status := &Status{
		Namespace:     rctc.namespace,
		HTTPServers:   make(map[string]*trafficcontroller.HTTPServerStatus),
		HTTPPipelines: make(map[string]*trafficcontroller.HTTPPipelineStatus),
	}

	rctc.tc.WalkHTTPServers(rctc.namespace, func(entity *supervisor.ObjectEntity) bool {
		status.HTTPServers[entity.Spec().Name()] = &trafficcontroller.HTTPServerStatus{
			Spec:   entity.Spec().RawSpec(),
			Status: entity.Instance().Status().ObjectStatus.(*httpserver.Status),
		}
		return true
	})

	rctc.tc.WalkHTTPPipelines(rctc.namespace, func(entity *supervisor.ObjectEntity) bool {
		status.HTTPPipelines[entity.Spec().Name()] = &trafficcontroller.HTTPPipelineStatus{
			Spec:   entity.Spec().RawSpec(),
			Status: entity.Instance().Status().ObjectStatus.(*httppipeline.Status),
		}
		return true
	})

	return &supervisor.Status{
		ObjectStatus: status,
	}
}

// Close closes RawConfigTrafficController.
func (rctc *RawConfigTrafficController) Close() {
	close(rctc.done)
	rctc.superSpec.Super().ObjectRegistry().CloseWatcher(rctc.superSpec.Name())
	rctc.tc.Clean(rctc.namespace)
}
