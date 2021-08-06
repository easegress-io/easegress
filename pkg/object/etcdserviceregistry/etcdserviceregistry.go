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

package eserviceregistry

import (
	"context"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of EtcdServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of EtcdServiceRegistry.
	Kind = "EtcdServiceRegistry"
)

func init() {
	supervisor.Register(&EtcdServiceRegistry{})
}

type (
	// EtcdServiceRegistry is Object EtcdServiceRegistry.
	EtcdServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		serviceSpecs    map[string]*serviceregistry.ServiceSpec
		notify          chan *serviceregistry.RegistryEvent

		clientMutex sync.RWMutex
		client      *clientv3.Client

		statusMutex         sync.Mutex
		serviceInstancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the EtcdServiceRegistry.
	Spec struct {
		Endpoints    []string `yaml:"endpoints" jsonschema:"required,uniqueItems=true"`
		Prefix       string   `yaml:"prefix" jsonschema:"required,pattern=^/"`
		CacheTimeout string   `yaml:"cacheTimeout" jsonschema:"required,format=duration"`
	}

	// Status is the status of EtcdServiceRegistry.
	Status struct {
		Health              string         `yaml:"health"`
		ServiceInstancesNum map[string]int `yaml:"serviceInstancesNum"`
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
		CacheTimeout: "60s",
	}
}

// Init initilizes EtcdServiceRegistry.
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
	e.serviceRegistry.RegisterRegistry(e)
	e.notify = make(chan *serviceregistry.RegistryEvent, 10)

	e.serviceInstancesNum = map[string]int{}
	e.done = make(chan struct{})

	_, err := e.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v", e.superSpec.Name(), err)
	}

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
	client, err := e.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v",
			e.superSpec.Name(), err)
		return
	}
	resp, err := client.Get(context.Background(), e.spec.Prefix, clientv3.WithPrefix())
	if err != nil {
		logger.Errorf("%s pull services failed: %v",
			e.superSpec.Name(), err)
		return
	}

	serviceSpecs := make(map[string]*serviceregistry.ServiceSpec)
	serviceInstancesNum := map[string]int{}
	for _, kv := range resp.Kvs {
		serviceInstanceSpec := &serviceregistry.ServiceInstanceSpec{}
		err := yaml.Unmarshal(kv.Value, serviceInstanceSpec)
		if err != nil {
			logger.Errorf("%s: unmarshal %s to yaml failed: %v",
				kv.Key, kv.Value, err)
			continue
		}
		if err := serviceInstanceSpec.Validate(); err != nil {
			logger.Errorf("%s is invalid: %v", kv.Value, err)
			continue
		}

		serviceName := serviceInstanceSpec.ServiceName

		serviceSpec, exists := serviceSpecs[serviceName]
		if !exists {
			serviceSpecs[serviceName] = &serviceregistry.ServiceSpec{
				RegistryName: e.Name(),
				ServiceName:  serviceName,
				Instances:    []*serviceregistry.ServiceInstanceSpec{serviceInstanceSpec},
			}
		} else {
			serviceSpec.Instances = append(serviceSpec.Instances, serviceInstanceSpec)
		}

		serviceInstancesNum[serviceName]++
	}

	var event *serviceregistry.RegistryEvent
	if !e.firstDone {
		e.firstDone = true
		event = &serviceregistry.RegistryEvent{
			RegistryName: e.Name(),
			Replace:      serviceSpecs,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(e.Name(), e.serviceSpecs, serviceSpecs)
	}

	e.notify <- event
	e.serviceSpecs = serviceSpecs

	e.statusMutex.Lock()
	e.serviceInstancesNum = serviceInstancesNum
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
	serviceInstancesNum := e.serviceInstancesNum
	e.statusMutex.Unlock()

	s.ServiceInstancesNum = serviceInstancesNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Close() {
	e.serviceRegistry.DeregisterRegistry(e.Name())

	e.closeClient()
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

// ApplyServices applies service specs to etcd registry.
func (e *EtcdServiceRegistry) ApplyServices(serviceSpec []*serviceregistry.ServiceSpec) error {
	// TODO
	return nil
}

// GetService applies service specs to etcd registry.
func (e *EtcdServiceRegistry) GetService(serviceName string) (*serviceregistry.ServiceSpec, error) {
	// TODO
	return nil, nil
}

// ListServices lists service specs from etcd registry.
func (e *EtcdServiceRegistry) ListServices() ([]*serviceregistry.ServiceSpec, error) {
	// TODO
	return nil, nil
}
