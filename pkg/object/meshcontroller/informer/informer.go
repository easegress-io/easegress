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

package informer

import (
	"fmt"
	"sync"

	yamljsontool "github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/cluster"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
)

const (
	// TODO: Support EventCreate.

	// EventUpdate is the update inform event.
	EventUpdate = "Update"
	// EventDelete is the delete inform event.
	EventDelete = "Delete"

	// AllParts is the path of the whole structure.
	AllParts GJSONPath = ""

	// ServiceObservability  is the path of service observability.
	ServiceObservability GJSONPath = "observability"

	// ServiceResilience is the path of service resilience.
	ServiceResilience GJSONPath = "resilience"

	// ServiceCanary is the path of service canary.
	ServiceCanary GJSONPath = "canary"

	// ServiceLoadBalance is the path of service loadbalance.
	ServiceLoadBalance GJSONPath = "loadBalance"

	// ServiceCircuitBreaker is the path of service resilience's circuritBreaker part.
	ServiceCircuitBreaker GJSONPath = "resilience.circuitBreaker"
)

type (
	// Event is the type of inform event.
	Event struct {
		EventType string
		RawKV     *mvccpb.KeyValue
	}

	// GJSONPath is the type of inform path, in GJSON syntax.
	GJSONPath string

	specHandleFunc  func(event Event, value string) bool
	specsHandleFunc func(map[string]string) bool

	// The returning boolean flag of all callback functions means
	// if the stuff continues to be watched.

	// ServiceSpecFunc is the callback function type for service spec.
	ServiceSpecFunc func(event Event, serviceSpec *spec.Service) bool

	// ServiceSpecFunc is the callback function type for service specs.
	ServiceSpecsFunc func(value map[string]*spec.Service) bool

	// ServiceInstanceSpecFunc is the callback function type for service instance spec.
	ServicesInstanceSpecFunc func(event Event, instanceSpec *spec.ServiceInstanceSpec) bool

	// ServiceInstanceSpecFunc is the callback function type for service instance specs.
	ServiceInstanceSpecsFunc func(value map[string]*spec.ServiceInstanceSpec) bool

	// ServiceInstanceStatusFunc is the callback function type for service instance status.
	ServiceInstanceStatusFunc func(event Event, value *spec.ServiceInstanceStatus) bool

	// ServiceInstanceStatusesFunc is the callback function type for service instance statuses.
	ServiceInstanceStatusesFunc func(value map[string]*spec.ServiceInstanceStatus) bool

	// TenantSpecFunc is the callback function type for tenant spec.
	TenantSpecFunc func(event Event, value *spec.Tenant) bool

	// TenantSpecsFunc is the callback function type for tenant specs.
	TenantSpecsFunc func(value map[string]*spec.Tenant) bool

	// IngressSpecFunc is the callback function type for service spec.
	IngressSpecFunc func(event Event, ingressSpec *spec.Ingress) bool

	// IngressSpecFunc is the callback function type for service specs.
	IngressSpecsFunc func(value map[string]*spec.Ingress) bool

	// Informer is the interface for informing two type of storage changed for every Mesh spec structure.
	//  1. Based on comparison between old and new part of entry.
	//  2. Based on comparison on entries with the same prefix.
	Informer interface {
		OnPartOfServiceSpec(serviceName string, gjsonPath GJSONPath, fn ServiceSpecFunc) error
		OnServiceSpecs(servicePrefix string, fn ServiceSpecsFunc) error

		OnPartOfInstanceSpec(serviceName, instanceID string, gjsonPath GJSONPath, fn ServicesInstanceSpecFunc) error
		OnServiceInstanceSpecs(serviceName string, fn ServiceInstanceSpecsFunc) error

		OnPartOfServiceInstanceStatus(serviceName, instanceID string, gjsonPath GJSONPath, fn ServiceInstanceStatusFunc) error
		OnServiceInstanceStatuses(serviceName string, fn ServiceInstanceStatusesFunc) error

		OnPartOfTenantSpec(tenantName string, gjsonPath GJSONPath, fn TenantSpecFunc) error
		OnTenantSpecs(tenantPrefix string, fn TenantSpecsFunc) error

		OnPartOfIngressSpec(serviceName string, gjsonPath GJSONPath, fn IngressSpecFunc) error
		OnIngressSpecs(fn IngressSpecsFunc) error

		StopWatchServiceSpec(serviceName string, gjsonPath GJSONPath)
		StopWatchServiceInstanceSpec(serviceName string)

		Close()
	}

	// meshInformer is the informer for mesh usage
	meshInformer struct {
		mutex   sync.Mutex
		store   storage.Storage
		syncers map[string]*cluster.Syncer

		closed bool
		done   chan struct{}
	}
)

var (
	// ErrAlreadyWatched is the error when watch the same entry again.
	ErrAlreadyWatched = fmt.Errorf("already watched")

	// ErrClosed is the error when watching a closed informer.
	ErrClosed = fmt.Errorf("informer already been closed")

	// ErrNotFound is the error when watching an entry which is not found.
	ErrNotFound = fmt.Errorf("not found")
)

// NewInformer creates an informer.
func NewInformer(store storage.Storage) Informer {
	inf := &meshInformer{
		store:   store,
		syncers: make(map[string]*cluster.Syncer),
		mutex:   sync.Mutex{},
		done:    make(chan struct{}),
	}

	return inf
}

func (inf *meshInformer) stopSyncOneKey(key string) {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if syncer, exists := inf.syncers[key]; exists {
		syncer.Close()
		delete(inf.syncers, key)
	}
}

func serviceSpecSyncerKey(serviceName string, gjsonPath GJSONPath) string {
	return fmt.Sprintf("service-spec-%s-%s", serviceName, gjsonPath)
}

// OnPartOfServiceSpec watches one service's spec by given gjsonPath.
func (inf *meshInformer) OnPartOfServiceSpec(serviceName string, gjsonPath GJSONPath, fn ServiceSpecFunc) error {
	storeKey := layout.ServiceSpecKey(serviceName)
	syncerKey := serviceSpecSyncerKey(serviceName, gjsonPath)

	specFunc := func(event Event, value string) bool {
		serviceSpec := &spec.Service{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), serviceSpec); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, serviceSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, gjsonPath, specFunc)
}

func (inf *meshInformer) StopWatchServiceSpec(serviceName string, gjsonPath GJSONPath) {
	syncerKey := serviceSpecSyncerKey(serviceName, gjsonPath)
	inf.stopSyncOneKey(syncerKey)
}

// OnPartOfInstanceSpec watches one service's instance spec by given gjsonPath.
func (inf *meshInformer) OnPartOfInstanceSpec(serviceName, instanceID string, gjsonPath GJSONPath, fn ServicesInstanceSpecFunc) error {
	storeKey := layout.ServiceInstanceSpecKey(serviceName, instanceID)
	syncerKey := fmt.Sprintf("service-instance-spec-%s-%s-%s", serviceName, instanceID, gjsonPath)

	specFunc := func(event Event, value string) bool {
		instanceSpec := &spec.ServiceInstanceSpec{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), instanceSpec); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, instanceSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, gjsonPath, specFunc)
}

// OnPartOfServiceInstanceStatus watches one service instance status spec by given gjsonPath.
func (inf *meshInformer) OnPartOfServiceInstanceStatus(serviceName, instanceID string, gjsonPath GJSONPath, fn ServiceInstanceStatusFunc) error {
	storeKey := layout.ServiceInstanceStatusKey(serviceName, instanceID)
	syncerKey := fmt.Sprintf("service-instance-status-%s-%s-%s", serviceName, instanceID, gjsonPath)

	specFunc := func(event Event, value string) bool {
		instanceStatus := &spec.ServiceInstanceStatus{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), instanceStatus); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, instanceStatus)
	}

	return inf.onSpecPart(storeKey, syncerKey, gjsonPath, specFunc)
}

// OnPartOfTenantSpec watches one tenant status spec by given gjsonPath.
func (inf *meshInformer) OnPartOfTenantSpec(tenant string, gjsonPath GJSONPath, fn TenantSpecFunc) error {
	storeKey := layout.TenantSpecKey(tenant)
	syncerKey := fmt.Sprintf("tenant-%s", tenant)

	specFunc := func(event Event, value string) bool {
		tenantSpec := &spec.Tenant{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), tenantSpec); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, tenantSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, gjsonPath, specFunc)
}

// OnPartOfIngressSpec watches one ingress status spec by given gjsonPath.
func (inf *meshInformer) OnPartOfIngressSpec(ingress string, gjsonPath GJSONPath, fn IngressSpecFunc) error {
	storeKey := layout.IngressSpecKey(ingress)
	syncerKey := fmt.Sprintf("ingress-%s", ingress)

	specFunc := func(event Event, value string) bool {
		ingressSpec := &spec.Ingress{}
		if event.EventType != EventDelete {
			if err := yaml.Unmarshal([]byte(value), ingressSpec); err != nil {
				if err != nil {
					logger.Errorf("BUG: unmarshal %s to yaml failed: %v", value, err)
					return true
				}
			}
		}
		return fn(event, ingressSpec)
	}

	return inf.onSpecPart(storeKey, syncerKey, gjsonPath, specFunc)
}

// OnServiceSpecs watches service specs with the prefix.
func (inf *meshInformer) OnServiceSpecs(servicePrefix string, fn ServiceSpecsFunc) error {
	syncerKey := fmt.Sprintf("prefix-service-%s", servicePrefix)

	specsFunc := func(kvs map[string]string) bool {
		services := make(map[string]*spec.Service)
		for k, v := range kvs {
			service := &spec.Service{}
			if err := yaml.Unmarshal([]byte(v), service); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			services[k] = service
		}

		return fn(services)
	}

	return inf.onSpecs(servicePrefix, syncerKey, specsFunc)
}

func serviceInstanceSpecSyncerKey(serviceName string) string {
	return fmt.Sprintf("prefix-service-instance-spec-%s", serviceName)
}

// OnServiceInstanceSpecs watches one service all instance specs.
func (inf *meshInformer) OnServiceInstanceSpecs(serviceName string, fn ServiceInstanceSpecsFunc) error {
	instancePrefix := layout.ServiceInstanceSpecPrefix(serviceName)
	syncerKey := serviceInstanceSpecSyncerKey(serviceName)

	specsFunc := func(kvs map[string]string) bool {
		instanceSpecs := make(map[string]*spec.ServiceInstanceSpec)
		for k, v := range kvs {
			instanceSpec := &spec.ServiceInstanceSpec{}
			if err := yaml.Unmarshal([]byte(v), instanceSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			instanceSpecs[k] = instanceSpec
		}

		return fn(instanceSpecs)
	}

	return inf.onSpecs(instancePrefix, syncerKey, specsFunc)
}

func (inf *meshInformer) StopWatchServiceInstanceSpec(serviceName string) {
	syncerKey := serviceInstanceSpecSyncerKey(serviceName)
	inf.stopSyncOneKey(syncerKey)
}

// OnServiceInstanceStatuses watches service instance statuses with the same prefix.
func (inf *meshInformer) OnServiceInstanceStatuses(serviceName string, fn ServiceInstanceStatusesFunc) error {
	syncerKey := fmt.Sprintf("prefix-service-instance-status-%s", serviceName)
	instanceStatusPrefix := layout.ServiceInstanceStatusPrefix(serviceName)

	specsFunc := func(kvs map[string]string) bool {
		instanceStatuses := make(map[string]*spec.ServiceInstanceStatus)
		for k, v := range kvs {
			instanceStatus := &spec.ServiceInstanceStatus{}
			if err := yaml.Unmarshal([]byte(v), instanceStatus); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			instanceStatuses[k] = instanceStatus
		}

		return fn(instanceStatuses)
	}

	return inf.onSpecs(instanceStatusPrefix, syncerKey, specsFunc)
}

// OnTenantSpecs watches tenant specs with the same prefix.
func (inf *meshInformer) OnTenantSpecs(tenantPrefix string, fn TenantSpecsFunc) error {
	syncerKey := fmt.Sprintf("prefix-tenant-%s", tenantPrefix)

	specsFunc := func(kvs map[string]string) bool {
		tenants := make(map[string]*spec.Tenant)
		for k, v := range kvs {
			tenantSpec := &spec.Tenant{}
			if err := yaml.Unmarshal([]byte(v), tenantSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			tenants[k] = tenantSpec
		}

		return fn(tenants)
	}

	return inf.onSpecs(tenantPrefix, syncerKey, specsFunc)
}

// OnIngressSpecs watches ingress specs
func (inf *meshInformer) OnIngressSpecs(fn IngressSpecsFunc) error {
	storeKey := layout.IngressPrefix()
	syncerKey := "prefix-ingress"

	specsFunc := func(kvs map[string]string) bool {
		ingresss := make(map[string]*spec.Ingress)
		for k, v := range kvs {
			ingressSpec := &spec.Ingress{}
			if err := yaml.Unmarshal([]byte(v), ingressSpec); err != nil {
				logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
				continue
			}
			ingresss[k] = ingressSpec
		}

		return fn(ingresss)
	}

	return inf.onSpecs(storeKey, syncerKey, specsFunc)
}

func (inf *meshInformer) comparePart(path GJSONPath, old, new string) bool {
	if path == AllParts {
		return old == new
	}

	oldJSON, err := yamljsontool.YAMLToJSON([]byte(old))
	if err != nil {
		logger.Errorf("BUG: transform yaml %s to json failed: %v", old, err)
		return true
	}

	newJSON, err := yamljsontool.YAMLToJSON([]byte(new))
	if err != nil {
		logger.Errorf("BUG: transform yaml %s to json failed: %v", new, err)
		return true
	}

	return gjson.Get(string(oldJSON), string(path)) == gjson.Get(string(newJSON), string(path))
}

func (inf *meshInformer) onSpecPart(storeKey, syncerKey string, gjsonPath GJSONPath, fn specHandleFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if inf.closed {
		return ErrClosed
	}

	if _, ok := inf.syncers[syncerKey]; ok {
		logger.Infof("sync key: %s already", syncerKey)
		return ErrAlreadyWatched
	}

	syncer, err := inf.store.Syncer()
	if err != nil {
		return err
	}

	ch, err := syncer.SyncRaw(storeKey)
	if err != nil {
		return err
	}

	inf.syncers[syncerKey] = syncer

	go inf.sync(ch, syncerKey, gjsonPath, fn)

	return nil
}

func (inf *meshInformer) onSpecs(storePrefix, syncerKey string, fn specsHandleFunc) error {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	if inf.closed {
		return ErrClosed
	}

	if _, exists := inf.syncers[syncerKey]; exists {
		logger.Infof("sync prefix:%s already", syncerKey)
		return ErrAlreadyWatched
	}

	syncer, err := inf.store.Syncer()
	if err != nil {
		return err
	}

	ch, err := syncer.SyncPrefix(storePrefix)
	if err != nil {
		return err
	}

	inf.syncers[syncerKey] = syncer

	go inf.syncPrefix(ch, syncerKey, fn)

	return nil
}

func (inf *meshInformer) Close() {
	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	for _, syncer := range inf.syncers {
		syncer.Close()
	}

	inf.closed = true
}

func (inf *meshInformer) sync(ch <-chan *mvccpb.KeyValue, syncerKey string, path GJSONPath, fn specHandleFunc) {
	var oldKV *mvccpb.KeyValue

	for kv := range ch {
		var (
			event Event
			value string
		)

		if kv == nil {
			event.EventType = EventDelete
		} else if oldKV == nil || !inf.comparePart(path, string(oldKV.Value), string(kv.Value)) {
			event.EventType = EventUpdate
			event.RawKV = kv
			value = string(kv.Value)
		}

		oldKV = kv

		if !fn(event, value) {
			inf.stopSyncOneKey(syncerKey)
		}
	}
}

func (inf *meshInformer) syncPrefix(ch <-chan map[string]string, syncerKey string, fn specsHandleFunc) {
	for kvs := range ch {
		if !fn(kvs) {
			inf.stopSyncOneKey(syncerKey)
		}
	}
}
