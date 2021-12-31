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

	processFn func(string, context.Protocol, *supervisor.ObjectEntity) (*supervisor.ObjectEntity, error)
)

func init() {
	supervisor.Register(&TrafficController{})
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

func (tc *TrafficController) getNamespaceAndMap(namespace string, fieldName namespaceField, protocol context.Protocol, create bool) (*Namespace, *sync.Map, error) {
	space, err := tc.getNamespace(namespace, create)
	if err != nil {
		return nil, nil, err
	}

	field := space.getField(fieldName)
	res, exists := field[protocol]

	if !exists {
		if !create {
			return nil, nil, fmt.Errorf("%s for protocol %s in namespace %s not found", fieldName, protocol, namespace)
		}
		res = &sync.Map{}
		field[protocol] = res
	}
	return space, res, nil
}

func (tc *TrafficController) processForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec, processFn processFn) (
	*supervisor.ObjectEntity, error) {

	entity, err := tc.super.NewObjectEntityFromSpec(superSpec)
	if err != nil {
		return nil, err
	}
	return processFn(namespace, protocolType, entity)
}

func (tc *TrafficController) processCreate(namespace string, fieldName namespaceField, protocolType context.Protocol,
	entity *supervisor.ObjectEntity) (*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, objMap, _ := tc.getNamespaceAndMap(namespace, fieldName, protocolType, true)

	name := entity.Spec().Name()
	entity.InitWithRecovery(space)
	objMap.Store(name, entity)

	logger.Infof("create %v %s/%s/%s", fieldName, namespace, protocolType, name)
	return entity, nil
}

func (tc *TrafficController) processUpdate(namespace string, fieldName namespaceField, protocolType context.Protocol,
	entity *supervisor.ObjectEntity) (*supervisor.ObjectEntity, error) {

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, objMap, err := tc.getNamespaceAndMap(namespace, fieldName, protocolType, false)
	if err != nil {
		return nil, err
	}

	name := entity.Spec().Name()
	previousEntity, exists := objMap.Load(name)
	if !exists {
		return nil, fmt.Errorf("%s %s/%s/%s not found", fieldName, namespace, protocolType, name)
	}
	entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
	objMap.Store(name, entity)

	logger.Infof("update %s %s/%s/%s", fieldName, namespace, protocolType, name)
	return entity, nil
}

func (tc *TrafficController) processApply(namespace string, fieldName namespaceField, protocolType context.Protocol,
	entity *supervisor.ObjectEntity) (*supervisor.ObjectEntity, error) {

	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	space, objMap, _ := tc.getNamespaceAndMap(namespace, fieldName, protocolType, true)

	name := entity.Spec().Name()
	previousEntity, exists := objMap.Load(name)
	if !exists {
		entity.InitWithRecovery(space)
		objMap.Store(name, entity)

		logger.Infof("create %s %s/%s/%s", fieldName, namespace, protocolType, name)
	} else {
		prev := previousEntity.(*supervisor.ObjectEntity)
		if prev.Spec().Equals(entity.Spec()) {
			logger.Infof("%s %s/%s/%s nothing change", fieldName, namespace, protocolType, name)
			return prev, nil
		}
		entity.InheritWithRecovery(previousEntity.(*supervisor.ObjectEntity), space)
		objMap.Store(name, entity)

		logger.Infof("update %s %s/%s/%s", fieldName, namespace, protocolType, name)
	}

	return entity, nil
}

func (tc *TrafficController) processDelete(namespace string, fieldName namespaceField, protocolType context.Protocol, name string) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, objMap, err := tc.getNamespaceAndMap(namespace, fieldName, protocolType, false)
	if err != nil {
		return err
	}

	entity, exists := objMap.LoadAndDelete(name)
	if !exists {
		return fmt.Errorf("%s %s/%s/%s not found", fieldName, namespace, protocolType, name)
	}

	entity.(*supervisor.ObjectEntity).CloseWithRecovery()
	logger.Infof("delete %s %s/%s/%s", fieldName, namespace, protocolType, name)

	tc._cleanSpace(namespace)

	return nil
}

func (tc *TrafficController) processGet(namespace string, fieldName namespaceField, protocolType context.Protocol, name string) (*supervisor.ObjectEntity, bool) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, objMap, err := tc.getNamespaceAndMap(namespace, fieldName, protocolType, false)
	if err != nil {
		return nil, false
	}

	entity, exists := objMap.Load(name)
	if !exists {
		return nil, false
	}

	return entity.(*supervisor.ObjectEntity), exists
}

func (tc *TrafficController) processList(namespace string, fieldName namespaceField, protocolType context.Protocol) []*supervisor.ObjectEntity {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, objMap, err := tc.getNamespaceAndMap(namespace, fieldName, protocolType, false)
	if err != nil {
		return nil
	}

	entities := []*supervisor.ObjectEntity{}
	objMap.Range(func(k, v interface{}) bool {
		entities = append(entities, v.(*supervisor.ObjectEntity))
		return true
	})

	return entities
}

func (tc *TrafficController) processWalk(namespace string, fieldName namespaceField, protocolType context.Protocol, walkFn WalkFunc) {
	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("walkServers recover from err: %v, stack trace:\n%s\n",
				err, debug.Stack())
		}
	}()

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, objMap, err := tc.getNamespaceAndMap(namespace, fieldName, protocolType, false)
	if err != nil {
		return
	}

	objMap.Range(func(k, v interface{}) bool {
		return walkFn(v.(*supervisor.ObjectEntity))
	})
}

// CreateServerForSpec creates server with a spec
func (tc *TrafficController) CreateServerForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	return tc.processForSpec(namespace, protocolType, superSpec, tc.CreateServer)
}

// CreateServer creates server
func (tc *TrafficController) CreateServer(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	return tc.processCreate(namespace, serverField, protocolType, entity)
}

// UpdateServerForSpec updates server with a Spec
func (tc *TrafficController) UpdateServerForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	return tc.processForSpec(namespace, protocolType, superSpec, tc.UpdateServer)
}

// UpdateServer updates server
func (tc *TrafficController) UpdateServer(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	return tc.processUpdate(namespace, serverField, protocolType, entity)
}

// ApplyServerForSpec applies servers with a Spec
func (tc *TrafficController) ApplyServerForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	return tc.processForSpec(namespace, protocolType, superSpec, tc.ApplyServer)
}

// ApplyServer applies Server
func (tc *TrafficController) ApplyServer(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	return tc.processApply(namespace, serverField, protocolType, entity)
}

// DeleteServer deletes a server
func (tc *TrafficController) DeleteServer(namespace string, protocolType context.Protocol, name string) error {
	return tc.processDelete(namespace, serverField, protocolType, name)
}

// GetServer gets servers by its namespace and name
func (tc *TrafficController) GetServer(namespace string, protocolType context.Protocol, name string) (*supervisor.ObjectEntity, bool) {
	return tc.processGet(namespace, serverField, protocolType, name)
}

// ListServers lists the servers
func (tc *TrafficController) ListServers(namespace string, protocolType context.Protocol) []*supervisor.ObjectEntity {
	return tc.processList(namespace, serverField, protocolType)
}

// WalkServers walks servers
func (tc *TrafficController) WalkServers(namespace string, protocolType context.Protocol, walkFn WalkFunc) {
	tc.processWalk(namespace, serverField, protocolType, walkFn)
}

// WalkPipelines walks the pipelines
func (tc *TrafficController) WalkPipelines(namespace string, protocolType context.Protocol, walkFn WalkFunc) {
	tc.processWalk(namespace, pipelineField, protocolType, walkFn)
}

// CreatePipelineForSpec creates a pipeline by a spec
func (tc *TrafficController) CreatePipelineForSpec(namespace string, protocol context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	return tc.processForSpec(namespace, protocol, superSpec, tc.CreatePipeline)
}

// CreatePipeline creates a pipeline
func (tc *TrafficController) CreatePipeline(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	return tc.processCreate(namespace, pipelineField, protocolType, entity)
}

// UpdatePipelineForSpec updates the pipeline with a Spec
func (tc *TrafficController) UpdatePipelineForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	return tc.processForSpec(namespace, protocolType, superSpec, tc.UpdatePipeline)
}

// UpdatePipeline updates the pipeline
func (tc *TrafficController) UpdatePipeline(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	return tc.processUpdate(namespace, pipelineField, protocolType, entity)
}

// ApplyPipelineForSpec applies the pipeline with a Spec
func (tc *TrafficController) ApplyPipelineForSpec(namespace string, protocolType context.Protocol, superSpec *supervisor.Spec) (
	*supervisor.ObjectEntity, error) {

	return tc.processForSpec(namespace, protocolType, superSpec, tc.ApplyPipeline)
}

// ApplyPipeline applies the pipeline
func (tc *TrafficController) ApplyPipeline(namespace string, protocolType context.Protocol, entity *supervisor.ObjectEntity) (
	*supervisor.ObjectEntity, error) {

	return tc.processApply(namespace, pipelineField, protocolType, entity)
}

// DeletePipeline deletes the pipeline by its namespace and name
func (tc *TrafficController) DeletePipeline(namespace string, protocolType context.Protocol, name string) error {
	return tc.processDelete(namespace, pipelineField, protocolType, name)
}

// GetPipeline returns the pipeline by its namespace and name.
func (tc *TrafficController) GetPipeline(namespace string, protocolType context.Protocol, name string) (*supervisor.ObjectEntity, bool) {
	return tc.processGet(namespace, pipelineField, protocolType, name)
}

// ListPipelines lists the pipelines for given protocols
func (tc *TrafficController) ListPipelines(namespace string, protocolType context.Protocol) []*supervisor.ObjectEntity {
	return tc.processList(namespace, pipelineField, protocolType)
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
