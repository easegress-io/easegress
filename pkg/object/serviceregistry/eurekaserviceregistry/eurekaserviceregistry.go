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

package eurekaserviceregistry

import (
	"fmt"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/serviceregistry"
	"github.com/megaease/easegateway/pkg/supervisor"

	eurekaapi "github.com/ArthurHlt/go-eureka-client/eureka"
)

const (
	// Category is the category of EurekaServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of EurekaServiceRegistry.
	Kind = "EurekaServiceRegistry"
)

func init() {
	supervisor.Register(&EurekaServiceRegistry{})
}

type (
	// EurekaServiceRegistry is Object EurekaServiceRegistry.
	EurekaServiceRegistry struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		clientMutex sync.RWMutex
		client      *eurekaapi.Client

		statusMutex sync.Mutex
		serversNum  map[string]int

		done chan struct{}
	}

	// Spec describes the EurekaServiceRegistry.
	Spec struct {
		Endpoints    []string `yaml:"endpoints" jsonschema:"required,uniqueItems=true"`
		SyncInterval string   `yaml:"syncInterval" jsonschema:"required,format=duration"`
	}

	// Status is the status of EurekaServiceRegistry.
	Status struct {
		Health     string         `yaml:"health"`
		ServersNum map[string]int `yaml:"serversNum"`
	}
)

// Category returns the category of EurekaServiceRegistry.
func (eureka *EurekaServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of EurekaServiceRegistry.
func (eureka *EurekaServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of EurekaServiceRegistry.
func (eureka *EurekaServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		Endpoints:    []string{"http://127.0.0.1:8761/eureka"},
		SyncInterval: "10s",
	}
}

// Init initilizes EurekaServiceRegistry.
func (eureka *EurekaServiceRegistry) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	eureka.superSpec, eureka.spec, eureka.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	eureka.reload()
}

// Inherit inherits previous generation of EurekaServiceRegistry.
func (eureka *EurekaServiceRegistry) Inherit(superSpec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	previousGeneration.Close()
	eureka.Init(superSpec, super)
}

func (eureka *EurekaServiceRegistry) reload() {
	eureka.serversNum = make(map[string]int)
	eureka.done = make(chan struct{})

	_, err := eureka.getClient()
	if err != nil {
		logger.Errorf("%s get eureka client failed: %v", eureka.superSpec.Name(), err)
	}

	go eureka.run()
}

func (eureka *EurekaServiceRegistry) getClient() (*eurekaapi.Client, error) {
	eureka.clientMutex.RLock()
	if eureka.client != nil {
		client := eureka.client
		eureka.clientMutex.RUnlock()
		return client, nil
	}
	eureka.clientMutex.RUnlock()

	return eureka.buildClient()
}

func (eureka *EurekaServiceRegistry) buildClient() (*eurekaapi.Client, error) {
	eureka.clientMutex.Lock()
	defer eureka.clientMutex.Unlock()

	// DCL
	if eureka.client != nil {
		return eureka.client, nil
	}

	client := eurekaapi.NewClient(eureka.spec.Endpoints)

	eureka.client = client

	return client, nil
}

func (eureka *EurekaServiceRegistry) closeClient() {
	eureka.clientMutex.Lock()
	defer eureka.clientMutex.Unlock()

	if eureka.client == nil {
		return
	}

	eureka.client = nil
}

func (eureka *EurekaServiceRegistry) run() {
	syncInterval, err := time.ParseDuration(eureka.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			eureka.spec.SyncInterval, err)
		return
	}

	eureka.update()

	for {
		select {
		case <-eureka.done:
			return
		case <-time.After(syncInterval):
			eureka.update()
		}
	}
}

func (eureka *EurekaServiceRegistry) update() {
	client, err := eureka.getClient()
	if err != nil {
		logger.Errorf("%s get eureka client failed: %v",
			eureka.superSpec.Name(), err)
		return
	}

	apps, err := client.GetApplications()
	if err != nil {
		logger.Errorf("%s get services failed: %v",
			eureka.superSpec.Name(), err)
		return
	}

	servers := []*serviceregistry.Server{}
	serversNum := map[string]int{}
	for _, app := range apps.Applications {
		for _, instance := range app.Instances {
			baseServer := serviceregistry.Server{
				ServiceName: app.Name,
				Hostname:    instance.HostName,
				HostIP:      instance.IpAddr,
				Port:        uint16(instance.Port.Port),
			}
			if instance.Port != nil && instance.Port.Enabled {
				server := baseServer

				fmt.Printf("server: %+v\n", server)
				servers = append(servers, &server)
				serversNum[app.Name]++
			}

			if instance.SecurePort != nil && instance.SecurePort.Enabled {
				server := baseServer
				server.Scheme = "https"
				fmt.Printf("server: %+v\n", server)
				servers = append(servers, &server)
				serversNum[app.Name]++
			}
		}
	}

	serviceregistry.Global.ReplaceServers(eureka.superSpec.Name(), servers)

	eureka.statusMutex.Lock()
	eureka.serversNum = serversNum
	eureka.statusMutex.Unlock()
}

// Status returns status of EurekaServiceRegister.
func (eureka *EurekaServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := eureka.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	eureka.statusMutex.Lock()
	serversNum := eureka.serversNum
	eureka.statusMutex.Unlock()

	s.ServersNum = serversNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes EurekaServiceRegistry.
func (eureka *EurekaServiceRegistry) Close() {
	eureka.closeClient()
	close(eureka.done)

	serviceregistry.Global.CloseRegistry(eureka.superSpec.Name())
}
