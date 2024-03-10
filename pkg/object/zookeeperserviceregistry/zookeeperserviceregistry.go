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

// Package zookeeperserviceregistry implements the ZookeeperServiceRegistry.
package zookeeperserviceregistry

import (
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

	zookeeper "github.com/go-zookeeper/zk"
)

const (
	// Category is the category of ZookeeperServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of ZookeeperServiceRegistry.
	Kind = "ZookeeperServiceRegistry"
)

func init() {
	supervisor.Register(&ZookeeperServiceRegistry{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"zookeeper", "zk", "zkserviceregistries"},
	})
}

type (
	// ZookeeperServiceRegistry is Object ZookeeperServiceRegistry.
	ZookeeperServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		instances       map[string]*serviceregistry.ServiceInstanceSpec
		notify          chan *serviceregistry.RegistryEvent

		clientMutex sync.RWMutex
		client      *zookeeper.Conn

		statusMutex  sync.Mutex
		instancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the ZookeeperServiceRegistry.
	Spec struct {
		ConnTimeout  string   `json:"conntimeout" jsonschema:"required,format=duration"`
		ZKServices   []string `json:"zkservices" jsonschema:"required,uniqueItems=true"`
		Prefix       string   `json:"prefix" jsonschema:"required,pattern=^/"`
		SyncInterval string   `json:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of ZookeeperServiceRegistry.
	Status struct {
		Health              string         `json:"health"`
		ServiceInstancesNum map[string]int `json:"serviceInstancesNum"`
	}
)

// Category returns the category of ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		ZKServices:   []string{"127.0.0.1:2181"},
		SyncInterval: "10s",
		Prefix:       "/",
		ConnTimeout:  "6s",
	}
}

// Init initializes ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Init(superSpec *supervisor.Spec) {
	zk.superSpec, zk.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	zk.reload()
}

// Inherit inherits previous generation of ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	zk.Init(superSpec)
}

func (zk *ZookeeperServiceRegistry) reload() {
	zk.serviceRegistry = zk.superSpec.Super().MustGetSystemController(serviceregistry.Kind).
		Instance().(*serviceregistry.ServiceRegistry)
	zk.notify = make(chan *serviceregistry.RegistryEvent, 10)
	zk.firstDone = false

	zk.instancesNum = make(map[string]int)
	zk.done = make(chan struct{})

	_, err := zk.getClient()
	if err != nil {
		logger.Errorf("%s get zookeeper conn failed: %v", zk.superSpec.Name(), err)
	}

	zk.serviceRegistry.RegisterRegistry(zk)

	go zk.run()
}

func (zk *ZookeeperServiceRegistry) getClient() (*zookeeper.Conn, error) {
	zk.clientMutex.RLock()
	if zk.client != nil {
		conn := zk.client
		zk.clientMutex.RUnlock()
		return conn, nil
	}
	zk.clientMutex.RUnlock()

	return zk.buildClient()
}

func (zk *ZookeeperServiceRegistry) buildClient() (*zookeeper.Conn, error) {
	zk.clientMutex.Lock()
	defer zk.clientMutex.Unlock()

	// DCL
	if zk.client != nil {
		return zk.client, nil
	}

	conntimeout, err := time.ParseDuration(zk.spec.ConnTimeout)
	if err != nil {
		logger.Errorf("BUG: parse connection timeout duration %s failed: %v",
			zk.spec.ConnTimeout, err)
		return nil, err
	}

	conn, _, err := zookeeper.Connect(zk.spec.ZKServices, conntimeout)
	if err != nil {
		logger.Errorf("zookeeper get connection failed: %v", err)
		return nil, err
	}

	exist, _, err := conn.Exists(zk.spec.Prefix)
	if err != nil {
		logger.Errorf("zookeeper check path: %s exist failed: %v", zk.spec.Prefix, err)
		return nil, err
	}

	if !exist {
		logger.Errorf("zookeeper path: %s no exist", zk.spec.Prefix)
		return nil, fmt.Errorf("path [%s] no exist", zk.spec.Prefix)
	}

	zk.client = conn

	return conn, nil
}

func (zk *ZookeeperServiceRegistry) closeClient() {
	zk.clientMutex.Lock()
	defer zk.clientMutex.Unlock()

	if zk.client == nil {
		return
	}

	zk.client.Close()
}

func (zk *ZookeeperServiceRegistry) run() {
	defer zk.closeClient()

	syncInterval, err := time.ParseDuration(zk.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			zk.spec.SyncInterval, err)
		return
	}

	zk.update()

	for {
		select {
		case <-zk.done:
			return
		case <-time.After(syncInterval):
			zk.update()
		}
	}
}

func (zk *ZookeeperServiceRegistry) update() {
	instances, err := zk.ListAllServiceInstances()
	if err != nil {
		logger.Errorf("list all service instances failed: %v", err)
		return
	}

	instancesNum := make(map[string]int)
	for _, instance := range instances {
		instancesNum[instance.ServiceName]++
	}

	var event *serviceregistry.RegistryEvent
	if !zk.firstDone {
		zk.firstDone = true
		event = &serviceregistry.RegistryEvent{
			SourceRegistryName: zk.Name(),
			UseReplace:         true,
			Replace:            instances,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(zk.Name(), zk.instances, instances)
	}

	if event.Empty() {
		return
	}

	zk.notify <- event
	zk.instances = instances

	zk.statusMutex.Lock()
	zk.instancesNum = instancesNum
	zk.statusMutex.Unlock()
}

// Status returns status of EurekaServiceRegister.
func (zk *ZookeeperServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := zk.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	zk.statusMutex.Lock()
	s.ServiceInstancesNum = zk.instancesNum
	zk.statusMutex.Unlock()

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Close() {
	zk.serviceRegistry.DeregisterRegistry(zk.Name())

	close(zk.done)
}

// Name returns name.
func (zk *ZookeeperServiceRegistry) Name() string {
	return zk.superSpec.Name()
}

// Notify returns notify channel.
func (zk *ZookeeperServiceRegistry) Notify() <-chan *serviceregistry.RegistryEvent {
	return zk.notify
}

// ApplyServiceInstances applies service instances to the registry.
func (zk *ZookeeperServiceRegistry) ApplyServiceInstances(serviceInstances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := zk.getClient()
	if err != nil {
		return fmt.Errorf("%s get consul client failed: %v",
			zk.superSpec.Name(), err)
	}

	for _, instance := range serviceInstances {
		err := instance.Validate()
		if err != nil {
			return fmt.Errorf("%+v is invalid: %v", instance, err)
		}
	}

	for _, instance := range serviceInstances {
		buff, err := codectool.MarshalJSON(instance)
		if err != nil {
			return fmt.Errorf("marshal %+v to json failed: %v", instance, err)
		}

		path := zk.serviceInstanceZookeeperPath(instance)
		_, err = client.Set(path, buff, 0)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteServiceInstances applies service instances to the registry.
func (zk *ZookeeperServiceRegistry) DeleteServiceInstances(serviceInstances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := zk.getClient()
	if err != nil {
		return fmt.Errorf("%s get consul client failed: %v",
			zk.superSpec.Name(), err)
	}

	for _, instance := range serviceInstances {
		path := zk.serviceInstanceZookeeperPath(instance)
		err := client.Delete(path, 0)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetServiceInstance get service instance from the registry.
func (zk *ZookeeperServiceRegistry) GetServiceInstance(serviceName, instanceID string) (*serviceregistry.ServiceInstanceSpec, error) {
	serviceInstances, err := zk.ListServiceInstances(serviceName)
	if err != nil {
		return nil, err
	}

	for _, instance := range serviceInstances {
		if instance.InstanceID == instanceID {
			return instance, nil
		}
	}

	return nil, fmt.Errorf("%s/%s not found", serviceName, instanceID)
}

// ListServiceInstances list service instances of one service from the registry.
func (zk *ZookeeperServiceRegistry) ListServiceInstances(serviceName string) (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	client, err := zk.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get consul client failed: %v",
			zk.superSpec.Name(), err)
	}

	children, _, err := client.Children(zk.serviceZookeeperPrefix(serviceName))
	if err != nil {
		return nil, fmt.Errorf("%s get path: %s children failed: %v", zk.superSpec.Name(), zk.spec.Prefix, err)
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, child := range children {
		fullPath := zk.fullPathOfChild(child)
		data, _, err := client.Get(fullPath)
		if err != nil {
			return nil, fmt.Errorf("%s get child path %s failed: %v", zk.superSpec.Name(), fullPath, err)
		}

		instance := &serviceregistry.ServiceInstanceSpec{}
		err = codectool.Unmarshal(data, instance)
		if err != nil {
			return nil, fmt.Errorf("%s unmarshal fullpath %s to json failed: %v", zk.superSpec.Name(), fullPath, err)
		}

		err = instance.Validate()
		if err != nil {
			return nil, fmt.Errorf("%s is invalid: %v", data, err)
		}

		instances[instance.Key()] = instance
	}

	return instances, nil
}

// ListAllServiceInstances list all service instances from the registry.
func (zk *ZookeeperServiceRegistry) ListAllServiceInstances() (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	client, err := zk.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get consul client failed: %v",
			zk.superSpec.Name(), err)
	}

	children, _, err := client.Children(zk.spec.Prefix)
	if err != nil {
		return nil, fmt.Errorf("%s get path: %s children failed: %v", zk.superSpec.Name(), zk.spec.Prefix, err)
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, child := range children {
		fullPath := zk.fullPathOfChild(child)
		data, _, err := client.Get(fullPath)
		if err != nil {
			return nil, fmt.Errorf("%s get child path %s failed: %v", zk.superSpec.Name(), fullPath, err)
		}

		instance := &serviceregistry.ServiceInstanceSpec{}
		err = codectool.Unmarshal(data, instance)
		if err != nil {
			return nil, fmt.Errorf("%s unmarshal fullpath %s to json failed: %v", zk.superSpec.Name(), fullPath, err)
		}

		err = instance.Validate()
		if err != nil {
			return nil, fmt.Errorf("%s is invalid: %v", data, err)
		}

		instances[instance.Key()] = instance
	}

	return instances, nil
}

func (zk *ZookeeperServiceRegistry) fullPathOfChild(childPath string) string {
	return path.Join(zk.spec.Prefix, childPath)
}

func (zk *ZookeeperServiceRegistry) serviceZookeeperPrefix(serviceName string) string {
	return path.Join(zk.spec.Prefix, serviceName) + "/"
}

func (zk *ZookeeperServiceRegistry) serviceInstanceZookeeperPath(instance *serviceregistry.ServiceInstanceSpec) string {
	return zk.serviceInstanceZookeeperPathFromRaw(instance.ServiceName, instance.InstanceID)
}

func (zk *ZookeeperServiceRegistry) serviceInstanceZookeeperPathFromRaw(serviceName, instanceID string) string {
	return path.Join(zk.spec.Prefix, serviceName, instanceID)
}
