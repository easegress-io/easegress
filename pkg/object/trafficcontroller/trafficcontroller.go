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

		// Pointer aims to safely transform it to next generation.
		mutex      *sync.Mutex
		namespaces map[string]*Namespace
	}

	// Namespace is the namespace
	Namespace struct {
		namespace string
		// The scenario here satisfies the first common case:
		// When the entry for a given key is only ever written once but read many times.
		// Reference: https://golang.org/pkg/sync/#Map
		// types of both: map[string]*supervisor.ObjectEntity
		httpservers   sync.Map
		httppipelines sync.Map
	}

	// WalkFunc is the type of the function called for
	// walking http server and http pipeline.
	WalkFunc = supervisor.WalkFunc

	// Spec describes TrafficController.
	Spec struct {
	}

	// Status is the status of namespaces
	Status struct {
		Namespaces []string `yaml:"namespaces"`
	}

	// HTTPServerStatus is the HTTP server status
	HTTPServerStatus struct {
		Spec   map[string]interface{} `yaml:"spec"`
		Status *httpserver.Status     `yaml:"status"`
	}

	// HTTPPipelineStatus is the HTTP pipeline status
	HTTPPipelineStatus struct {
		Spec   map[string]interface{} `yaml:"spec"`
		Status *httppipeline.Status   `yaml:"status"`
	}

	// StatusInSameNamespace is the universal status in one space.
	// TrafficController won't use it.
	StatusInSameNamespace struct {
		Namespace     string                         `yaml:"namespace"`
		HTTPServers   map[string]*HTTPServerStatus   `yaml:"httpServers"`
		HTTPPipelines map[string]*HTTPPipelineStatus `yaml:"httpPipelines"`
	}
)

func init() {
	supervisor.Register(&TrafficController{})
}

func newNamespace(namespace string) *Namespace {
	return &Namespace{
		namespace: namespace,
	}
}

// GetHandler gets handler within the namespace
func (ns *Namespace) GetHandler(name string) (protocol.HTTPHandler, bool) {
	entity, exists := ns.httppipelines.Load(name)
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
func (tc *TrafficController) Init(superSpec *supervisor.Spec) {
	tc.superSpec, tc.spec, tc.super = superSpec, superSpec.ObjectSpec().(*Spec), superSpec.Super()

	tc.mutex = &sync.Mutex{}
	tc.namespaces = make(map[string]*Namespace)

	tc.reload(nil)
}

// Inherit inherits previous generation of TrafficController.
func (tc *TrafficController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	tc.superSpec, tc.super = superSpec, superSpec.Super()
	tc.reload(previousGeneration.(*TrafficController))
}

func (tc *TrafficController) reload(previousGeneration *TrafficController) {
	if previousGeneration != nil {
		tc.mutex, tc.namespaces = previousGeneration.mutex, previousGeneration.namespaces
	}
}

// CreateHTTPServerForSpec creates HTTP server with a spec
func (tc *TrafficController) CreateHTTPServerForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.CreateHTTPServer(namespace, entity)
}

// CreateHTTPServer creates HTTP server
func (tc *TrafficController) CreateHTTPServer(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	entity.InitWithRecovery(space)
	space.httpservers.Store(name, entity)

	logger.Infof("create http server %s/%s", namespace, name)

	return entity, nil
}

// UpdateHTTPServerForSpec updates HTTP server with a Spec
func (tc *TrafficController) UpdateHTTPServerForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.UpdateHTTPServer(namespace, entity)
}

// UpdateHTTPServer updates HTTP server
func (tc *TrafficController) UpdateHTTPServer(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
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

// ApplyHTTPServerForSpec applies HTTP servers with a Spec
func (tc *TrafficController) ApplyHTTPServerForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.ApplyHTTPServer(namespace, entity)
}

// ApplyHTTPServer applies HTTP Server
func (tc *TrafficController) ApplyHTTPServer(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.httpservers.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		space.httpservers.Store(name, entity)

		logger.Infof("create http server %s/%s", namespace, name)
	} else {
		prev := previousEntity.(*supervisor.ObjectEntity)
		if prev.Spec().Equals(entity.Spec()) {
			logger.Infof("http server %s/%s nothing change", namespace, name)
			return prev, nil
		}

		entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
		space.httpservers.Store(name, entity)

		logger.Infof("update http server %s/%s", namespace, name)
	}

	return entity, nil
}

// DeleteHTTPServer deletes a HTTP server
func (tc *TrafficController) DeleteHTTPServer(namespace, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
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

// GetHTTPServer gets HTTP servers by its namespace and name
func (tc *TrafficController) GetHTTPServer(namespace, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return nil, false
	}

	entity, exists := space.httpservers.Load(name)
	if !exists {
		return nil, false
	}

	return entity.(*supervisor.ObjectEntity), exists
}

// ListHTTPServers lists the HTTP servers
func (tc *TrafficController) ListHTTPServers(namespace string) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
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

// WalkHTTPServers walks HTTP servers
func (tc *TrafficController) WalkHTTPServers(namespace string, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkHTTPServers recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return
	}

	space.httpservers.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

// WalkHTTPPipelines walks the HTTP pipelines
func (tc *TrafficController) WalkHTTPPipelines(namespace string, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkHTTPPipelines recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return
	}

	space.httppipelines.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

// CreateHTTPPipelineForSpec creates a HTTP pipeline by a spec
func (tc *TrafficController) CreateHTTPPipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.CreateHTTPPipeline(namespace, entity)
}

// CreateHTTPPipeline creates a HTTP pipeline
func (tc *TrafficController) CreateHTTPPipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	entity.InitWithRecovery(space)
	space.httppipelines.Store(name, entity)

	logger.Infof("create http pipeline %s/%s", namespace, name)

	return entity, nil
}

// UpdateHTTPPipelineForSpec updates the HTTP pipeline with a Spec
func (tc *TrafficController) UpdateHTTPPipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.UpdateHTTPPipeline(namespace, entity)
}

// UpdateHTTPPipeline updates the HTTP pipeline
func (tc *TrafficController) UpdateHTTPPipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
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

// ApplyHTTPPipelineForSpec applies the HTTP pipeline with a Spec
func (tc *TrafficController) ApplyHTTPPipelineForSpec(namespace string, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.ApplyHTTPPipeline(namespace, entity)
}

// ApplyHTTPPipeline applies the HTTP pipeline
func (tc *TrafficController) ApplyHTTPPipeline(namespace string, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}

	name := entity.Spec().Name()

	previousEntity, exists := space.httppipelines.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		space.httppipelines.Store(name, entity)

		logger.Infof("create http pipeline %s/%s", namespace, name)
	} else {
		prev := previousEntity.(*supervisor.ObjectEntity)
		if prev.Spec().Equals(entity.Spec()) {
			logger.Infof("http pipeline %s/%s nothing change", namespace, name)
			return prev, nil
		}

		entity.InheritWithRecovery(prev, space)
		space.httppipelines.Store(name, entity)

		logger.Infof("update http pipeline %s/%s", namespace, name)
	}

	return entity, nil
}

// DeleteHTTPPipeline deletes the HTTP pipeline by its namespace and name
func (tc *TrafficController) DeleteHTTPPipeline(namespace, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
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

// GetHTTPPipeline returns the pipeline by its namespace and name.
func (tc *TrafficController) GetHTTPPipeline(namespace, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
	if !exists {
		return nil, false
	}

	entity, exists := space.httppipelines.Load(name)
	if !exists {
		return nil, false
	}

	return entity.(*supervisor.ObjectEntity), exists
}

// ListHTTPPipelines lists the HTTP pipelines
func (tc *TrafficController) ListHTTPPipelines(namespace string) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exists := tc.namespaces[namespace]
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

// Clean all http servers and http pipelines of one namespace.
func (tc *TrafficController) Clean(namespace string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, exist := tc.namespaces[namespace]
	if !exist {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	space.httpservers.Range(func(k, v interface{}) bool {
		v.(*supervisor.ObjectEntity).CloseWithRecovery()
		logger.Infof("delete http server %s/%s", namespace, k)
		space.httpservers.Delete(k)
		return true
	})

	space.httppipelines.Range(func(k, v interface{}) bool {
		v.(*supervisor.ObjectEntity).CloseWithRecovery()
		logger.Infof("delete http pipeline %s/%s", namespace, k)
		space.httppipelines.Delete(k)
		return true
	})

	tc._cleanSpace(namespace)

	return nil
}

// _cleanSpace must be called after deleting HTTPServer or HTTPPipeline.
// It's caller's duty to keep concurrent safety.
func (tc *TrafficController) _cleanSpace(namespace string) {
	space, exists := tc.namespaces[namespace]
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
		delete(tc.namespaces, namespace)
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

	for namespace := range tc.namespaces {
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

	for name, space := range tc.namespaces {
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

		delete(tc.namespaces, name)
		logger.Infof("delete namespace %s", name)
	}
}
