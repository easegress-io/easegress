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

package zookeeperserviceregistry

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	zookeeper "github.com/go-zookeeper/zk"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
)

const (
	// Category is the category of ZookeeperServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of ZookeeperServiceRegistry.
	Kind = "ZookeeperServiceRegistry"
)

func init() {
	supervisor.Register(&ZookeeperServiceRegistry{})
}

type (
	// ZookeeperServiceRegistry is Object ZookeeperServiceRegistry.
	ZookeeperServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		serviceSpecs    map[string]*serviceregistry.ServiceSpec
		notify          chan *serviceregistry.RegistryEvent

		connMutex sync.RWMutex
		conn      *zookeeper.Conn

		statusMutex         sync.Mutex
		serviceInstancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the ZookeeperServiceRegistry.
	Spec struct {
		ConnTimeout  string   `yaml:"conntimeout" jsonschema:"required,format=duration"`
		ZKServices   []string `yaml:"zkservices" jsonschema:"required,uniqueItems=true"`
		Prefix       string   `yaml:"prefix" jsonschema:"required,pattern=^/"`
		SyncInterval string   `yaml:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of ZookeeperServiceRegistry.
	Status struct {
		Health              string         `yaml:"health"`
		ServiceInstancesNum map[string]int `yaml:"serviceInstancesNum"`
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

// Init initilizes ZookeeperServiceRegistry.
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
	zk.serviceRegistry.RegisterRegistry(zk)
	zk.notify = make(chan *serviceregistry.RegistryEvent, 10)

	zk.serviceInstancesNum = make(map[string]int)
	zk.done = make(chan struct{})

	_, err := zk.getConn()
	if err != nil {
		logger.Errorf("%s get zookeeper conn failed: %v", zk.superSpec.Name(), err)
	}

	go zk.run()
}

func (zk *ZookeeperServiceRegistry) getConn() (*zookeeper.Conn, error) {
	zk.connMutex.RLock()
	if zk.conn != nil {
		conn := zk.conn
		zk.connMutex.RUnlock()
		return conn, nil
	}
	zk.connMutex.RUnlock()

	return zk.buildConn()
}

func (zk *ZookeeperServiceRegistry) buildConn() (*zookeeper.Conn, error) {
	zk.connMutex.Lock()
	defer zk.connMutex.Unlock()

	// DCL
	if zk.conn != nil {
		return zk.conn, nil
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

	zk.conn = conn

	return conn, nil
}

func (zk *ZookeeperServiceRegistry) closeConn() {
	zk.connMutex.Lock()
	defer zk.connMutex.Unlock()

	if zk.conn == nil {
		return
	}

	zk.conn.Close()
}

func (zk *ZookeeperServiceRegistry) run() {
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
	conn, err := zk.getConn()
	if err != nil {
		logger.Errorf("%s get zookeeper conn failed: %v",
			zk.superSpec.Name(), err)
		return
	}

	childs, _, err := conn.Children(zk.spec.Prefix)
	if err != nil {
		logger.Errorf("%s get path: %s children failed: %v", zk.superSpec.Name(), zk.spec.Prefix, err)
		return
	}

	serviceSpecs := make(map[string]*serviceregistry.ServiceSpec)
	serviceInstancesNum := map[string]int{}
	for _, child := range childs {
		fullPath := zk.spec.Prefix + "/" + child
		data, _, err := conn.Get(fullPath)
		if err != nil {
			if err == zookeeper.ErrNoNode {
				continue
			}

			logger.Errorf("%s get child path %s failed: %v", zk.superSpec.Name(), fullPath, err)
			return
		}

		serviceInstanceSpec := &serviceregistry.ServiceInstanceSpec{}
		// Note: zookeeper allows store custom format into one path, so we choose to store
		//       serviceregistry.Server JSON format directly.
		err = json.Unmarshal(data, serviceInstanceSpec)
		if err != nil {
			logger.Errorf("%s unmarshal fullpath %s to json failed: %v", zk.superSpec.Name(), fullPath, err)
			return
		}

		if err := serviceInstanceSpec.Validate(); err != nil {
			logger.Errorf("%s is invalid: %v", data, err)
			continue
		}

		serviceName := serviceInstanceSpec.ServiceName

		serviceSpec, exists := serviceSpecs[serviceName]
		if !exists {
			serviceSpecs[serviceName] = &serviceregistry.ServiceSpec{
				RegistryName: zk.Name(),
				ServiceName:  serviceName,
				Instances:    []*serviceregistry.ServiceInstanceSpec{serviceInstanceSpec},
			}
		} else {
			serviceSpec.Instances = append(serviceSpec.Instances, serviceInstanceSpec)
		}

		serviceInstancesNum[fullPath]++
	}

	var event *serviceregistry.RegistryEvent
	if !zk.firstDone {
		zk.firstDone = true
		event = &serviceregistry.RegistryEvent{
			RegistryName: zk.Name(),
			Replace:      serviceSpecs,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(zk.Name(), zk.serviceSpecs, serviceSpecs)
	}

	zk.notify <- event
	zk.serviceSpecs = serviceSpecs

	zk.statusMutex.Lock()
	zk.serviceInstancesNum = serviceInstancesNum
	zk.statusMutex.Unlock()
}

// Status returns status of EurekaServiceRegister.
func (zk *ZookeeperServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := zk.getConn()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	zk.statusMutex.Lock()
	s.ServiceInstancesNum = zk.serviceInstancesNum
	zk.statusMutex.Unlock()

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Close() {
	zk.serviceRegistry.DeregisterRegistry(zk.Name())

	zk.closeConn()
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

// ApplyServices applies service specs to zookeeper registry.
func (zk *ZookeeperServiceRegistry) ApplyServices(serviceSpec []*serviceregistry.ServiceSpec) error {
	// TODO
	return nil
}

// GetService applies service specs to zookeeper registry.
func (zk *ZookeeperServiceRegistry) GetService(serviceName string) (*serviceregistry.ServiceSpec, error) {
	// TODO
	return nil, nil
}

// ListServices lists service specs from zookeeper registry.
func (zk *ZookeeperServiceRegistry) ListServices() ([]*serviceregistry.ServiceSpec, error) {
	// TODO
	return nil, nil
}
