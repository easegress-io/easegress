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

package trafficcontroller

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of TrafficController.
	Category = supervisor.CategorySystemController

	// Kind is the kind of TrafficController.
	Kind = "TrafficController"
)

type (
	// TrafficController is a system controller to manage
	// TrafficGate, Pipeline and their relationship.
	TrafficController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		mutex  sync.Mutex
		spaces map[string]*Space
	}

	Space struct {
		namespace string
		// The scenario here satisfies the first common case:
		// When the entry for a given key is only ever written once but read many times.
		// Referecen: https://golang.org/pkg/sync/#Map
		// types of both: map[string]*supervisor.ObjectEntity
		httpservers   sync.Map
		httppipelines sync.Map
	}

	// WalkHTTPServerFunc is the type of the function called for
	// walking http server and http pipeline.
	WalkFunc = supervisor.WalkFunc

	// Spec describes TrafficController.
	Spec struct {
	}

	Status struct {
		Namespaces []string `yaml:"namespaces"`
	}

	// StatusOneSpace is the universal status in one space.
	// TrafficController won't use it.
	StatusOneSpace struct {
		Namespace     string                          `yaml:"namespace"`
		HTTPServers   map[string]*httpserver.Status   `yaml:"httpServers"`
		HTTPPipelines map[string]*httppipeline.Status `yaml:"httpPipelines"`
	}
)

func init() {
	supervisor.Register(&TrafficController{})
}

func newSpace(namespace string) *Space {
	return &Space{
		namespace: namespace,
	}
}

// Space gets handler within the namspace of it.
func (s *Space) GetHandler(name string) (protocol.HTTPHandler, bool) {
	entity, exists := s.httppipelines.Load(name)
	if !exists {
		return nil, false
	}

	handler := entity.(*supervisor.ObjectEntity).Instance().(protocol.HTTPHandler)
	return handler, true
}

// Category returns the category of TrafficController.
func (tc *TrafficController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of TrafficController.
func (tc *TrafficController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of TrafficController.
func (tc *TrafficController) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes TrafficController.
func (tc *TrafficController) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	tc.superSpec, tc.spec, tc.super = superSpec, superSpec.ObjectSpec().(*Spec), super

	tc.spaces = make(map[string]*Space)

	tc.reload(nil)
}

// Inherit inherits previous generation of TrafficController.
func (tc *TrafficController) Inherit(spec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	tc.reload(previousGeneration.(*TrafficController))
}

func (tc *TrafficController) reload(previousGeneration *TrafficController) {
	if previousGeneration != nil {
		tc.mutex, tc.spaces = previousGeneration.mutex, previousGeneration.spaces
	}
}

func (tc *TrafficController) CreateHTTPServerForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.CreateHTTPServer(namespace, entity)
}

func (tc *TrafficController) CreateHTTPServer(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		space = newSpace(namespace)
		tc.spaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	entity.InitWithRecovery(space)
	space.httpservers.Store(name, entity)

	logger.Infof("create http server %s/%s", namespace, name)

	return entity, nil
}

func (tc *TrafficController) UpdateHTTPServerForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.UpdateHTTPServer(namespace, entity)
}

func (tc *TrafficController) UpdateHTTPServer(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.httpservers.Load(name)
	if !exists {
		return nil, fmt.Errorf("http server %s/%s not found", namespace, name)
	}

	entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
	space.httpservers.Store(name, entity)

	logger.Infof("update http server %s/%s", namespace, name)

	return entity, nil
}

func (tc *TrafficController) ApplyHTTPServerForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.ApplyHTTPServer(namespace, entity)
}

func (tc *TrafficController) ApplyHTTPServer(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		space = newSpace(namespace)
		tc.spaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.httpservers.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		space.httpservers.Store(name, entity)

		logger.Infof("create http server %s/%s", namespace, name)
	} else {
		entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
		space.httpservers.Store(name, entity)

		logger.Infof("update http server %s/%s", namespace, name)
	}

	return entity, nil
}

func (tc *TrafficController) DeleteHTTPServer(namespace, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	entity, exists := space.httpservers.LoadAndDelete(name)
	if !exists {
		return fmt.Errorf("http server %s/%s not found", namespace, name)
	}

	entity.(*supervisor.ObjectEntity).CloseWithRecovery()
	logger.Infof("delete http server %s/%s", namespace, name)

	tc._cleanSpace(namespace)

	return nil
}

func (tc *TrafficController) GetHTTPServer(namespace, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return nil, false
	}

	entity, exists := space.httpservers.Load(name)

	return entity.(*supervisor.ObjectEntity), exists
}

func (tc *TrafficController) ListHTTPServers(namespace string) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return nil
	}

	entities := []*supervisor.ObjectEntity{}
	space.httpservers.Range(func(k, v interface{}) bool {
		entities = append(entities, v.(*supervisor.ObjectEntity))
		return true
	})

	return entities
}

func (tc *TrafficController) WalkHTTPServers(namespace string, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkHTTPServers recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return
	}

	space.httpservers.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

func (tc *TrafficController) WalkHTTPPipelines(namespace string, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkHTTPPipelines recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return
	}

	space.httppipelines.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

func (tc *TrafficController) CreateHTTPPipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.CreateHTTPPipeline(namespace, entity)
}

func (tc *TrafficController) CreateHTTPPipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		space = newSpace(namespace)
		tc.spaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	entity.InitWithRecovery(space)
	space.httppipelines.Store(name, entity)

	logger.Infof("create http pipeline %s/%s", namespace, name)

	return entity, nil
}

func (tc *TrafficController) UpdateHTTPPipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.UpdateHTTPPipeline(namespace, entity)
}

func (tc *TrafficController) UpdateHTTPPipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return nil, fmt.Errorf("namespace %s not found", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.httppipelines.Load(name)
	if !exists {
		return nil, fmt.Errorf("http pipeline %s/%s not found", namespace, name)
	}

	entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
	space.httppipelines.Store(name, entity)

	logger.Infof("update http pipeline %s/%s", namespace, name)

	return entity, nil
}

func (tc *TrafficController) ApplyHTTPPipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.ApplyHTTPPipeline(namespace, entity)
}

func (tc *TrafficController) ApplyHTTPPipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		space = newSpace(namespace)
		tc.spaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.httppipelines.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		space.httppipelines.Store(name, entity)

		logger.Infof("create http pipeline %s/%s", namespace, name)
	} else {
		entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
		space.httppipelines.Store(name, entity)

		logger.Infof("update http pipeline %s/%s", namespace, name)
	}

	return entity, nil
}

func (tc *TrafficController) DeleteHTTPPipeline(namespace, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	entity, exists := space.httppipelines.LoadAndDelete(name)
	if !exists {
		return fmt.Errorf("http pipeline %s/%s not found", namespace, name)
	}

	entity.(*supervisor.ObjectEntity).CloseWithRecovery()
	logger.Infof("delete http pipeline %s/%s", namespace, name)

	tc._cleanSpace(namespace)

	return nil
}

func (tc *TrafficController) GetHTTPPipeline(namespace, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return nil, false
	}

	entity, exists := space.httppipelines.Load(name)

	return entity.(*supervisor.ObjectEntity), exists
}

func (tc *TrafficController) ListHTTPPipelines(namespace string) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.spaces[namespace]
	if !exists {
		return nil
	}

	entities := []*supervisor.ObjectEntity{}
	space.httppipelines.Range(func(k, v interface{}) bool {
		entities = append(entities, v.(*supervisor.ObjectEntity))
		return true
	})

	return entities
}

// _cleanSpace must be called after deleting HTTPServer or HTTPPipeline.
// It's caller's duty to keep concurrent safety.
func (tc *TrafficController) _cleanSpace(namespace string) {
	space, exists := tc.spaces[namespace]
	if !exists {
		logger.Errorf("BUG: namespace %s not found", namespace)
	}

	serverLen, pipelineLen := 0, 0
	space.httpservers.Range(func(k, v interface{}) bool {
		serverLen++
		return false
	})
	space.httppipelines.Range(func(k, v interface{}) bool {
		pipelineLen++
		return false
	})
	if serverLen+pipelineLen == 0 {
		delete(tc.spaces, namespace)
		logger.Infof("delete namespace %s", namespace)
	}
}

// Status returns the status of TrafficController.
func (tc *TrafficController) Status() *supervisor.Status {
	// NOTE: TrafficController won't report any namespaced statuses.
	// Higher controllers should report their own namespaced status.

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	namespaces := []string{}

	for namespace := range tc.spaces {
		namespaces = append(namespaces, namespace)
	}

	return &supervisor.Status{
		ObjectStatus: &Status{
			Namespaces: namespaces,
		},
	}
}

// Close closes TrafficController.
func (tc *TrafficController) Close() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	for name, space := range tc.spaces {
		space.httpservers.Range(func(k, v interface{}) bool {
			entity := v.(*supervisor.ObjectEntity)
			entity.CloseWithRecovery()
			logger.Infof("delete http server %s/%s", space.namespace, k)
			return true
		})

		space.httppipelines.Range(func(k, v interface{}) bool {
			entity := v.(*supervisor.ObjectEntity)
			entity.CloseWithRecovery()
			logger.Infof("delete http pipeline %s/%s", space.namespace, k)
			return true
		})

		delete(tc.spaces, name)
		logger.Infof("delete namespace %s", name)
	}
}
