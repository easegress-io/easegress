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

	"github.com/megaease/easegress/pkg/context"
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
		servers   map[context.Protocol]*sync.Map
		pipelines map[context.Protocol]*sync.Map
	}

	// WalkFunc is the type of the function called for
	// walking server and pipeline.
	WalkFunc = supervisor.WalkFunc

	// Spec describes TrafficController.
	Spec struct {
	}

	// Status is the status of namespaces
	Status struct {
		Specs []*StatusInSameNamespace `yaml:"specs"`
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
	ns := &Namespace{
		namespace: namespace,
		servers:   make(map[context.Protocol]*sync.Map),
		pipelines: make(map[context.Protocol]*sync.Map),
	}
	for _, p := range context.Protocols {
		ns.servers[p] = &sync.Map{}
		ns.pipelines[p] = &sync.Map{}
	}
	return ns
}

// GetHandler gets handler within the namespace
func (ns *Namespace) GetHandler(name string, protocolType context.Protocol) (protocol.Handler, bool) {
	pipelines, exists := ns.pipelines[protocolType]
	if !exists {
		return nil, false
	}

	entity, exists := pipelines.Load(name)
	if !exists {
		return nil, false
	}

	handler := entity.(*supervisor.ObjectEntity).Instance().(protocol.Handler)
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

// getNamespace will return namespace for given namespace.
func (tc *TrafficController) getNamespace(namespace string, create bool) (*Namespace, error) {
	space, exists := tc.namespaces[namespace]
	if !exists {
		if !create {
			return nil, fmt.Errorf("namespace %s not found", namespace)
		}
		space = newNamespace(namespace)
		tc.namespaces[namespace] = space
		logger.Infof("create namespace %s", namespace)
	}
	return space, nil
}

// getNamespaceAndServers will return servers map for given namespace and protocol.
// when create set to true, getNamespaceAndServers will create corresponding object if not found.
func (tc *TrafficController) getNamespaceAndServers(namespace string, protocol context.Protocol, create bool) (*Namespace, *sync.Map, error) {
	space, err := tc.getNamespace(namespace, create)
	if err != nil {
		return nil, nil, err
	}

	servers, exists := space.servers[protocol]
	if !exists {
		if !create {
			return nil, nil, fmt.Errorf("servers for protocol %s in namespace %s not found", protocol, namespace)
		}
		servers = &sync.Map{}
		space.servers[protocol] = servers
	}
	return space, servers, nil
}

// getNamespaceAndPipelines will return servers map for given namespace and protocol.
// when create set to true, getNamespaceAndPipelines will create corresponding object if not found.
func (tc *TrafficController) getNamespaceAndPipelines(namespace string, protocol context.Protocol, create bool) (*Namespace, *sync.Map, error) {
	space, err := tc.getNamespace(namespace, create)
	if err != nil {
		return nil, nil, err
	}

	pipelines, exists := space.pipelines[protocol]
	if !exists {
		if !create {
			return nil, nil, fmt.Errorf("pipelines for protocol %s in namespace %s not found", protocol, namespace)
		}
		pipelines = &sync.Map{}
		space.pipelines[protocol] = pipelines
	}
	return space, pipelines, nil
}

// CreateServerForSpec creates server with a spec
func (tc *TrafficController) CreateServerForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.CreateServer(namespace, protocolType, entity)
}

// CreateServer creates server
func (tc *TrafficController) CreateServer(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, servers, _ := tc.getNamespaceAndServers(namespace, protocolType, true)

	name := entity.Spec().Name()
	entity.InitWithRecovery(space)
	servers.Store(name, entity)

	logger.Infof("create server %s/%s/%s", namespace, protocolType, name)
	return entity, nil
}

// UpdateServerForSpec updates server with a Spec
func (tc *TrafficController) UpdateServerForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.UpdateServer(namespace, protocolType, entity)
}

// UpdateServer updates server
func (tc *TrafficController) UpdateServer(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, servers, err := tc.getNamespaceAndServers(namespace, protocolType, false)
	if err != nil {
		return nil, err
	}

	name := entity.Spec().Name()
	previousEntity, exists := servers.Load(name)
	if !exists {
		return nil, fmt.Errorf("server %s/%s/%s not found", namespace, protocolType, name)
	}
	entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
	servers.Store(name, entity)

	logger.Infof("update server %s/%s/%s", namespace, protocolType, name)
	return entity, nil
}

// ApplyServerForSpec applies servers with a Spec
func (tc *TrafficController) ApplyServerForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.ApplyServer(namespace, protocolType, entity)
}

// ApplyServer applies Server
func (tc *TrafficController) ApplyServer(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, servers, _ := tc.getNamespaceAndServers(namespace, protocolType, true)

	name := entity.Spec().Name()
	previousEntity, exists := servers.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		servers.Store(name, entity)

		logger.Infof("create server %s/%s/%s", namespace, protocolType, name)
	} else {
		prev := previousEntity.(*supervisor.ObjectEntity)
		if prev.Spec().Equals(entity.Spec()) {
			logger.Infof("server %s/%s/%s nothing change", namespace, protocolType, name)
			return prev, nil
		}
		entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
		servers.Store(name, entity)

		logger.Infof("update server %s/%s/%s", namespace, protocolType, name)
	}

	return entity, nil
}

// DeleteServer deletes a server
func (tc *TrafficController) DeleteServer(namespace string, protocolType context.Protocol, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, servers, err := tc.getNamespaceAndServers(namespace, protocolType, false)
	if err != nil {
		return err
	}

	entity, exists := servers.LoadAndDelete(name)
	if !exists {
		return fmt.Errorf("server %s/%s/%s not found", namespace, protocolType, name)
	}

	entity.(*supervisor.ObjectEntity).CloseWithRecovery()
	logger.Infof("delete server %s/%s/%s", namespace, protocolType, name)

	tc._cleanSpace(namespace)

	return nil
}

// GetServer gets servers by its namespace and name
func (tc *TrafficController) GetServer(namespace string, protocolType context.Protocol, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, servers, err := tc.getNamespaceAndServers(namespace, protocolType, false)
	if err != nil {
		return nil, false
	}

	entity, exists := servers.Load(name)
	if !exists {
		return nil, false
	}

	return entity.(*supervisor.ObjectEntity), exists
}

// ListServers lists the servers
func (tc *TrafficController) ListServers(namespace string, protocolType context.Protocol) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, servers, err := tc.getNamespaceAndServers(namespace, protocolType, false)
	if err != nil {
		return nil
	}

	entities := []*supervisor.ObjectEntity{}
	servers.Range(func(k, v interface{}) bool {
		entities = append(entities, v.(*supervisor.ObjectEntity))
		return true
	})

	return entities
}

// WalkServers walks servers
func (tc *TrafficController) WalkServers(namespace string, protocolType context.Protocol, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkServers recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, servers, err := tc.getNamespaceAndServers(namespace, protocolType, false)
	if err != nil {
		return
	}

	servers.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

// WalkPipelines walks the pipelines
func (tc *TrafficController) WalkPipelines(namespace string, protocolType context.Protocol, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkPipelines recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, pipelines, err := tc.getNamespaceAndPipelines(namespace, protocolType, false)
	if err != nil {
		return
	}

	pipelines.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

// CreatePipelineForSpec creates a pipeline by a spec
func (tc *TrafficController) CreatePipelineForSpec(namespace string, protocol context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.CreatePipeline(namespace, protocol, entity)
}

// CreatePipeline creates a pipeline
func (tc *TrafficController) CreatePipeline(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, pipelines, _ := tc.getNamespaceAndPipelines(namespace, protocolType, true)

	name := entity.Spec().Name()
	entity.InitWithRecovery(space)
	pipelines.Store(name, entity)

	logger.Infof("create pipeline %s/%s/%s", namespace, protocolType, name)
	return entity, nil
}

// UpdatePipelineForSpec updates the pipeline with a Spec
func (tc *TrafficController) UpdatePipelineForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.UpdatePipeline(namespace, protocolType, entity)
}

// UpdatePipeline updates the pipeline
func (tc *TrafficController) UpdatePipeline(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, pipelines, err := tc.getNamespaceAndPipelines(namespace, protocolType, false)
	if err != nil {
		return nil, err
	}

	name := entity.Spec().Name()

	previousEntity, exists := pipelines.Load(name)
	if !exists {
		return nil, fmt.Errorf("pipeline %s/%s/%s not found", namespace, protocolType, name)
	}

	entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
	pipelines.Store(name, entity)

	logger.Infof("update pipeline %s/%s/%s", namespace, protocolType, name)
	return entity, nil
}

// ApplyPipelineForSpec applies the pipeline with a Spec
func (tc *TrafficController) ApplyPipelineForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return tc.ApplyPipeline(namespace, protocolType, entity)
}

// ApplyPipeline applies the pipeline
func (tc *TrafficController) ApplyPipeline(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, pipelines, _ := tc.getNamespaceAndPipelines(namespace, protocolType, true)

	name := entity.Spec().Name()

	previousEntity, exists := pipelines.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		pipelines.Store(name, entity)

		logger.Infof("create pipeline %s/%s/%s", namespace, protocolType, name)
	} else {
		prev := previousEntity.(*supervisor.ObjectEntity)
		if prev.Spec().Equals(entity.Spec()) {
			logger.Infof("pipeline %s/%s/%s nothing change", namespace, protocolType, name)
			return prev, nil
		}

		entity.InheritWithRecovery(prev, space)
		pipelines.Store(name, entity)

		logger.Infof("update pipeline %s/%s/%s", namespace, protocolType, name)
	}

	return entity, nil
}

// DeletePipeline deletes the pipeline by its namespace and name
func (tc *TrafficController) DeletePipeline(namespace string, protocolType context.Protocol, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, pipelines, err := tc.getNamespaceAndPipelines(namespace, protocolType, false)
	if err != nil {
		return err
	}

	entity, exists := pipelines.LoadAndDelete(name)
	if !exists {
		return fmt.Errorf("pipeline %s/%s/%s not found", namespace, protocolType, name)
	}

	entity.(*supervisor.ObjectEntity).CloseWithRecovery()
	logger.Infof("delete pipeline %s/%s/%s", namespace, protocolType, name)

	tc._cleanSpace(namespace)

	return nil
}

// GetPipeline returns the pipeline by its namespace and name.
func (tc *TrafficController) GetPipeline(namespace string, protocolType context.Protocol, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, pipelines, err := tc.getNamespaceAndPipelines(namespace, protocolType, false)
	if err != nil {
		return nil, false
	}

	entity, exists := pipelines.Load(name)
	if !exists {
		return nil, false
	}

	return entity.(*supervisor.ObjectEntity), exists
}

// ListPipelines lists the pipelines for given protocols
func (tc *TrafficController) ListPipelines(namespace string, protocolType context.Protocol) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, pipelines, err := tc.getNamespaceAndPipelines(namespace, protocolType, false)
	if err != nil {
		return nil
	}

	entities := []*supervisor.ObjectEntity{}
	pipelines.Range(func(k, v interface{}) bool {
		entities = append(entities, v.(*supervisor.ObjectEntity))
		return true
	})

	return entities
}

// Clean all servers and pipelines of one namespace.
func (tc *TrafficController) Clean(namespace string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, err := tc.getNamespace(namespace, false)
	if err != nil {
		return err
	}

	for protocolType, servers := range space.servers {
		servers.Range(func(k, v interface{}) bool {
			v.(*supervisor.ObjectEntity).CloseWithRecovery()
			logger.Infof("delete server %s/%s/%s", namespace, protocolType, k)
			servers.Delete(k)
			return true
		})
	}
	for protocolType, pipelines := range space.pipelines {
		pipelines.Range(func(k, v interface{}) bool {
			v.(*supervisor.ObjectEntity).CloseWithRecovery()
			logger.Infof("delete pipeline %s/%s/%s", namespace, protocolType, k)
			pipelines.Delete(k)
			return true
		})
	}

	tc._cleanSpace(namespace)

	return nil
}

// _cleanSpace must be called after deleting server or pipeline.
// It's caller's duty to keep concurrent safety.
func (tc *TrafficController) _cleanSpace(namespace string) {
	space, err := tc.getNamespace(namespace, false)
	if err != nil {
		logger.Errorf("BUG: namespace %s not found", namespace)
		return
	}

	serverLen, pipelineLen := 0, 0
	for _, servers := range space.servers {
		servers.Range(func(k, v interface{}) bool {
			serverLen++
			return false
		})
	}
	for _, pipelines := range space.pipelines {
		pipelines.Range(func(k, v interface{}) bool {
			pipelineLen++
			return false
		})
	}
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

	statuses := []*StatusInSameNamespace{}

	for namespace, namespaceSpec := range tc.namespaces {
		httpServers := make(map[string]*HTTPServerStatus)
		servers, exists := namespaceSpec.servers[context.HTTP]
		if exists {
			servers.Range(func(key, value interface{}) bool {
				k := key.(string)
				v := value.(*supervisor.ObjectEntity)

				httpServers[k] = &HTTPServerStatus{
					Spec:   v.Spec().RawSpec(),
					Status: v.Instance().Status().ObjectStatus.(*httpserver.Status),
				}
				return true
			})
		}

		httpPipelines := make(map[string]*HTTPPipelineStatus)
		pipelines, exists := namespaceSpec.pipelines[context.HTTP]
		if exists {
			pipelines.Range(func(key, value interface{}) bool {
				k := key.(string)
				v := value.(*supervisor.ObjectEntity)

				httpPipelines[k] = &HTTPPipelineStatus{
					Spec:   v.Spec().RawSpec(),
					Status: v.Instance().Status().ObjectStatus.(*httppipeline.Status),
				}
				return true
			})
		}

		statuses = append(statuses, &StatusInSameNamespace{
			Namespace:     namespace,
			HTTPServers:   httpServers,
			HTTPPipelines: httpPipelines,
		})
	}

	return &supervisor.Status{
		ObjectStatus: &Status{
			Specs: statuses,
		},
	}
}

// Close closes TrafficController.
func (tc *TrafficController) Close() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	for name, space := range tc.namespaces {
		for protocolType, servers := range space.servers {
			servers.Range(func(k, v interface{}) bool {
				entity := v.(*supervisor.ObjectEntity)
				entity.CloseWithRecovery()
				logger.Infof("delete server %s/%s/%s", space.namespace, protocolType, k)
				return true
			})
		}
		for protocolType, pipelines := range space.pipelines {
			pipelines.Range(func(k, v interface{}) bool {
				entity := v.(*supervisor.ObjectEntity)
				entity.CloseWithRecovery()
				logger.Infof("delete pipeline %s/%s/%s", space.namespace, protocolType, k)
				return true
			})
		}
		delete(tc.namespaces, name)
		logger.Infof("delete namespace %s", name)
	}
}
