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
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		connMutex sync.RWMutex
		conn      *zookeeper.Conn

		statusMutex sync.Mutex
		serversNum  map[string]int

		done chan struct{}
	}

	// Spec describes the ZookeeperServiceRegistry.
	Spec struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		ConnTimeout  string   `yaml:"conntimeout" jsonschema:"required,format=duration"`
		ZKServices   []string `yaml:"zkservices" jsonschema:"required,uniqueItems=true"`
		Prefix       string   `yaml:"prefix" jsonschema:"required,pattern=^/"`
		SyncInterval string   `yaml:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of ZookeeperServiceRegistry.
	Status struct {
		Health     string         `yaml:"health"`
		ServersNum map[string]int `yaml:"serversNum"`
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
func (zk *ZookeeperServiceRegistry) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	zk.superSpec, zk.spec, zk.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	zk.reload()
}

// Inherit inherits previous generation of ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Inherit(superSpec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	previousGeneration.Close()
	zk.Init(superSpec, super)
}

func (zk *ZookeeperServiceRegistry) reload() {
	zk.serversNum = make(map[string]int)
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

	if exist == false {
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

	servers := []*serviceregistry.Server{}
	serversNum := map[string]int{}
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

		server := new(serviceregistry.Server)
		// Note: zookeeper allows store custom format into one path, so we choose to store
		//       serviceregistry.Server JSON format directly.
		err = json.Unmarshal(data, server)
		if err != nil {
			logger.Errorf("BUG %s unmarshal fullpath %s failed %v", zk.superSpec.Name(), fullPath, err)
			return
		}
		logger.Debugf("zk %s fullpath %s server is  %v", zk.superSpec.Name(), fullPath, server)
		serversNum[fullPath]++
		servers = append(servers, server)
	}

	serviceregistry.Global.ReplaceServers(zk.superSpec.Name(), servers)

	zk.statusMutex.Lock()
	zk.serversNum = serversNum
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
	s.ServersNum = zk.serversNum
	zk.statusMutex.Unlock()

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes ZookeeperServiceRegistry.
func (zk *ZookeeperServiceRegistry) Close() {
	zk.closeConn()
	close(zk.done)

	serviceregistry.Global.CloseRegistry(zk.superSpec.Name())
}
