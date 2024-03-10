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

// Package eserviceregistry provides EtcdServiceRegistry.
package eserviceregistry

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/serviceregistry"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// Category is the category of EtcdServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of EtcdServiceRegistry.
	Kind = "EtcdServiceRegistry"

	requestTimeout = 5 * time.Second
)

var aliases = []string{"etcd"}

func init() {
	supervisor.Register(&EtcdServiceRegistry{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  aliases,
	})
}

type (
	// EtcdServiceRegistry is Object EtcdServiceRegistry.
	EtcdServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		instances       map[string]*serviceregistry.ServiceInstanceSpec
		notify          chan *serviceregistry.RegistryEvent

		clientMutex sync.RWMutex
		client      *clientv3.Client

		statusMutex  sync.Mutex
		instancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the EtcdServiceRegistry.
	Spec struct {
		Endpoints    []string `json:"endpoints" jsonschema:"required,uniqueItems=true"`
		Prefix       string   `json:"prefix" jsonschema:"required,pattern=^/"`
		CacheTimeout string   `json:"cacheTimeout" jsonschema:"required,format=duration"`
	}

	// Status is the status of EtcdServiceRegistry.
	Status struct {
		Health              string         `json:"health"`
		ServiceInstancesNum map[string]int `json:"instancesNum"`
	}
)

// Category returns the category of EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of EtcdServiceRegistry.
func (e *EtcdServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		Prefix:       "/services/",
		CacheTimeout: "10s",
	}
}

// Init initializes EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Init(superSpec *supervisor.Spec) {
	e.superSpec, e.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	e.reload()
}

// Inherit inherits previous generation of EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	e.Init(superSpec)
}

func (e *EtcdServiceRegistry) reload() {
	e.serviceRegistry = e.superSpec.Super().MustGetSystemController(serviceregistry.Kind).
		Instance().(*serviceregistry.ServiceRegistry)
	e.firstDone = false
	e.notify = make(chan *serviceregistry.RegistryEvent, 10)

	e.instancesNum = map[string]int{}
	e.done = make(chan struct{})

	_, err := e.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v", e.superSpec.Name(), err)
	}

	e.serviceRegistry.RegisterRegistry(e)

	go e.run()
}

func (e *EtcdServiceRegistry) getClient() (*clientv3.Client, error) {
	e.clientMutex.RLock()
	if e.client != nil {
		client := e.client
		e.clientMutex.RUnlock()
		return client, nil
	}
	e.clientMutex.RUnlock()

	return e.buildClient()
}

func (e *EtcdServiceRegistry) buildClient() (*clientv3.Client, error) {
	e.clientMutex.Lock()
	defer e.clientMutex.Unlock()

	// DCL
	if e.client != nil {
		return e.client, nil
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:            e.spec.Endpoints,
		AutoSyncInterval:     1 * time.Minute,
		DialTimeout:          10 * time.Second,
		DialKeepAliveTime:    1 * time.Minute,
		DialKeepAliveTimeout: 1 * time.Minute,
		LogConfig: logger.EtcdClientLoggerConfig(e.superSpec.Super().Options(),
			"object_"+e.superSpec.Name()),
	})
	if err != nil {
		return nil, err
	}

	e.client = client

	return client, nil
}

func (e *EtcdServiceRegistry) closeClient() {
	e.clientMutex.Lock()
	defer e.clientMutex.Unlock()

	if e.client == nil {
		return
	}
	err := e.client.Close()
	if err != nil {
		logger.Errorf("%s close etcd client failed: %v", e.superSpec.Name(), err)
	}
	e.client = nil
}

func (e *EtcdServiceRegistry) run() {
	defer e.closeClient()

	cacheTimeout, err := time.ParseDuration(e.spec.CacheTimeout)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			e.spec.CacheTimeout, err)
		return
	}

	e.update()

	for {
		select {
		case <-e.done:
			return
		case <-time.After(cacheTimeout):
			e.update()
		}
	}
}

func (e *EtcdServiceRegistry) update() {
	instances, err := e.ListAllServiceInstances()
	if err != nil {
		logger.Errorf("list all service instances failed: %v", err)
		return
	}

	instancesNum := make(map[string]int)
	for _, instance := range instances {
		instancesNum[instance.ServiceName]++
	}

	var event *serviceregistry.RegistryEvent
	if !e.firstDone {
		e.firstDone = true
		event = &serviceregistry.RegistryEvent{
			SourceRegistryName: e.Name(),
			UseReplace:         true,
			Replace:            instances,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(e.Name(), e.instances, instances)
	}

	if event.Empty() {
		return
	}

	e.notify <- event
	e.instances = instances

	e.statusMutex.Lock()
	e.instancesNum = instancesNum
	e.statusMutex.Unlock()
}

// Status returns status of EtcdServiceRegister.
func (e *EtcdServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := e.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	e.statusMutex.Lock()
	instancesNum := e.instancesNum
	e.statusMutex.Unlock()

	s.ServiceInstancesNum = instancesNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Close() {
	e.serviceRegistry.DeregisterRegistry(e.Name())

	close(e.done)
}

// Name returns name.
func (e *EtcdServiceRegistry) Name() string {
	return e.superSpec.Name()
}

// Notify returns notify channel.
func (e *EtcdServiceRegistry) Notify() <-chan *serviceregistry.RegistryEvent {
	return e.notify
}

// ApplyServiceInstances applies service instances to the registry.
func (e *EtcdServiceRegistry) ApplyServiceInstances(instances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := e.getClient()
	if err != nil {
		return fmt.Errorf("%s get etcd client failed: %v",
			e.superSpec.Name(), err)
	}

	ops := []clientv3.Op{}
	for _, instance := range instances {
		err := instance.Validate()
		if err != nil {
			return fmt.Errorf("%+v is invalid: %v", instance, err)
		}

		buff, err := codectool.MarshalJSON(instance)
		if err != nil {
			return fmt.Errorf("marshal %+v to json failed: %v", instance, err)
		}

		key := e.serviceInstanceEtcdKey(instance)
		ops = append(ops, clientv3.OpPut(key, string(buff)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	_, err = client.Txn(ctx).Then(ops...).Commit()
	return err
}

// DeleteServiceInstances applies service instances to the registry.
func (e *EtcdServiceRegistry) DeleteServiceInstances(instances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := e.getClient()
	if err != nil {
		return fmt.Errorf("%s get etcd client failed: %v",
			e.superSpec.Name(), err)
	}

	ops := []clientv3.Op{}
	for _, instance := range instances {
		key := e.serviceInstanceEtcdKey(instance)
		ops = append(ops, clientv3.OpDelete(key))
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	_, err = client.Txn(ctx).Then(ops...).Commit()
	return err
}

// GetServiceInstance get service instance from the registry.
func (e *EtcdServiceRegistry) GetServiceInstance(serviceName, instanceID string) (*serviceregistry.ServiceInstanceSpec, error) {
	resp, err := e.getWithTimeout(requestTimeout, e.serviceInstanceEtcdKeyFromRaw(serviceName, instanceID))
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("%s/%s not found", serviceName, instanceID)
	}

	instance := &serviceregistry.ServiceInstanceSpec{}
	err = codectool.Unmarshal(resp.Kvs[0].Value, instance)
	if err != nil {
		return nil, fmt.Errorf("unmarshal %s to json failed: %v", resp.Kvs[0].Value, err)
	}

	err = instance.Validate()
	if err != nil {
		return nil, fmt.Errorf("%+v is invalid: %v", instance, err)
	}

	return instance, nil
}

func (e *EtcdServiceRegistry) getWithTimeout(timeout time.Duration, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	client, err := e.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get etcd client failed: %v",
			e.superSpec.Name(), err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	return client.Get(ctx, key, opts...)
}

// ListServiceInstances list service instances of one service from the registry.
func (e *EtcdServiceRegistry) ListServiceInstances(serviceName string) (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	resp, err := e.getWithTimeout(requestTimeout, e.serviceEtcdPrefix(serviceName), clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, kv := range resp.Kvs {
		instance := &serviceregistry.ServiceInstanceSpec{}
		err = codectool.Unmarshal(kv.Value, instance)
		if err != nil {
			return nil, fmt.Errorf("unmarshal %s to json failed: %v", kv.Value, err)
		}

		err = instance.Validate()
		if err != nil {
			return nil, fmt.Errorf("%+v is invalid: %v", instance, err)
		}

		instances[instance.Key()] = instance
	}

	return instances, nil
}

// ListAllServiceInstances list all service instances from the registry.
func (e *EtcdServiceRegistry) ListAllServiceInstances() (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	resp, err := e.getWithTimeout(requestTimeout, e.spec.Prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, kv := range resp.Kvs {
		instance := &serviceregistry.ServiceInstanceSpec{}
		err = codectool.Unmarshal(kv.Value, instance)
		if err != nil {
			return nil, fmt.Errorf("unmarshal %s to codectool failed: %v", kv.Value, err)
		}

		err = instance.Validate()
		if err != nil {
			return nil, fmt.Errorf("%+v is invalid: %v", instance, err)
		}

		instances[instance.Key()] = instance
	}

	return instances, nil
}

func (e *EtcdServiceRegistry) serviceEtcdPrefix(serviceName string) string {
	return path.Join(e.spec.Prefix, serviceName) + "/"
}

func (e *EtcdServiceRegistry) serviceInstanceEtcdKey(instance *serviceregistry.ServiceInstanceSpec) string {
	return e.serviceInstanceEtcdKeyFromRaw(instance.ServiceName, instance.InstanceID)
}

func (e *EtcdServiceRegistry) serviceInstanceEtcdKeyFromRaw(serviceName, instanceID string) string {
	return path.Join(e.spec.Prefix, serviceName, instanceID)
}
