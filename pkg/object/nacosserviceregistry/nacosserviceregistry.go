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

// Package nacosserviceregistry provides the NacosServiceRegistry.
package nacosserviceregistry

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/serviceregistry"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/v"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

const (
	// Category is the category of NacosServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of NacosServiceRegistry.
	Kind = "NacosServiceRegistry"

	// MetaKeyRegistryName is the key of service registry name.
	MetaKeyRegistryName = "RegistryName"

	// MetaKeyInstanceID is the key of service instance ID.
	MetaKeyInstanceID = "InstanceID"
)

func init() {
	supervisor.Register(&NacosServiceRegistry{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"nacos"},
	})
}

type (
	// NacosServiceRegistry is Object NacosServiceRegistry.
	NacosServiceRegistry struct {
		superSpec *supervisor.Spec
		spec      *Spec

		serviceRegistry *serviceregistry.ServiceRegistry
		firstDone       bool
		instances       map[string]*serviceregistry.ServiceInstanceSpec
		notify          chan *serviceregistry.RegistryEvent

		clientMutex sync.RWMutex
		client      naming_client.INamingClient

		statusMutex  sync.Mutex
		instancesNum map[string]int

		done chan struct{}
	}

	// Spec describes the NacosServiceRegistry.
	Spec struct {
		Servers      []*ServerSpec `json:"servers" jsonschema:"required"`
		SyncInterval string        `json:"syncInterval" jsonschema:"required,format=duration"`
		Namespace    string        `json:"namespace,omitempty"`
		Username     string        `json:"username,omitempty"`
		Password     string        `json:"password,omitempty"`
	}

	// ServerSpec is the server config of Nacos.
	ServerSpec struct {
		Scheme      string `json:"scheme,omitempty" jsonschema:"enum=http,enum=https"`
		ContextPath string `json:"contextPath,omitempty"`
		IPAddr      string `json:"ipAddr" jsonschema:"required"`
		Port        uint16 `json:"port" jsonschema:"required"`
	}

	// Status is the status of NacosServiceRegistry.
	Status struct {
		Health              string         `json:"health"`
		ServiceInstancesNum map[string]int `json:"instancesNum"`
	}
)

var _ v.Validator = Spec{}

// Validate validates Spec itself.
func (spec Spec) Validate() error {
	if len(spec.Servers) == 0 {
		return fmt.Errorf("zero server config")
	}

	return nil
}

// Category returns the category of NacosServiceRegistry.
func (n *NacosServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of NacosServiceRegistry.
func (n *NacosServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of NacosServiceRegistry.
func (n *NacosServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		SyncInterval: "10s",
	}
}

// Init initializes NacosServiceRegistry.
func (n *NacosServiceRegistry) Init(superSpec *supervisor.Spec) {
	n.superSpec, n.spec = superSpec, superSpec.ObjectSpec().(*Spec)
	n.reload()
}

// Inherit inherits previous generation of NacosServiceRegistry.
func (n *NacosServiceRegistry) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	n.Init(superSpec)
}

func (n *NacosServiceRegistry) reload() {
	n.serviceRegistry = n.superSpec.Super().MustGetSystemController(serviceregistry.Kind).
		Instance().(*serviceregistry.ServiceRegistry)
	n.notify = make(chan *serviceregistry.RegistryEvent, 10)
	n.firstDone = false

	n.instancesNum = map[string]int{}
	n.done = make(chan struct{})

	_, err := n.getClient()
	if err != nil {
		logger.Errorf("%s get nacos client failed: %v", n.superSpec.Name(), err)
	}

	n.serviceRegistry.RegisterRegistry(n)

	go n.run()
}

func (n *NacosServiceRegistry) getClient() (naming_client.INamingClient, error) {
	n.clientMutex.RLock()
	if n.client != nil {
		client := n.client
		n.clientMutex.RUnlock()
		return client, nil
	}
	n.clientMutex.RUnlock()

	return n.buildClient()
}

func (n *NacosServiceRegistry) buildClient() (naming_client.INamingClient, error) {
	n.clientMutex.Lock()
	defer n.clientMutex.Unlock()

	// DCL
	if n.client != nil {
		return n.client, nil
	}

	logDir := filepath.Join(n.superSpec.Super().Options().HomeDir, n.superSpec.Name(), "log")
	cacheDir := filepath.Join(n.superSpec.Super().Options().HomeDir, n.superSpec.Name(), "cache")
	clientConfig := constant.ClientConfig{
		Username: n.spec.Username,
		Password: n.spec.Password,

		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogDir:              logDir,
		CacheDir:            cacheDir,
		LogLevel:            "info",
		LogRollingConfig: &constant.ClientLogRollingConfig{
			MaxAge: 3,
		},
	}

	serverConfigs := []constant.ServerConfig{}
	for _, serverSpec := range n.spec.Servers {
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr:      serverSpec.IPAddr,
			ContextPath: serverSpec.ContextPath,
			Port:        uint64(serverSpec.Port),
			Scheme:      serverSpec.Scheme,
		})
	}

	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, err
	}

	n.client = client

	return client, nil
}

func (n *NacosServiceRegistry) closeClient() {
	n.clientMutex.Lock()
	defer n.clientMutex.Unlock()

	if n.client == nil {
		return
	}

	n.client = nil
}

func (n *NacosServiceRegistry) run() {
	defer n.closeClient()

	syncInterval, err := time.ParseDuration(n.spec.SyncInterval)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			n.spec.SyncInterval, err)
		return
	}

	n.update()

	for {
		select {
		case <-n.done:
			return
		case <-time.After(syncInterval):
			n.update()
		}
	}
}

func (n *NacosServiceRegistry) update() {
	instances, err := n.ListAllServiceInstances()
	if err != nil {
		logger.Errorf("list all service instances failed: %v", err)
		return
	}

	instancesNum := make(map[string]int)
	for _, instance := range instances {
		instancesNum[instance.ServiceName]++
	}

	var event *serviceregistry.RegistryEvent
	if !n.firstDone {
		n.firstDone = true
		event = &serviceregistry.RegistryEvent{
			SourceRegistryName: n.Name(),
			UseReplace:         true,
			Replace:            instances,
		}
	} else {
		event = serviceregistry.NewRegistryEventFromDiff(n.Name(), n.instances, instances)
	}

	if event.Empty() {
		return
	}

	n.notify <- event
	n.instances = instances

	n.statusMutex.Lock()
	n.instancesNum = instancesNum
	n.statusMutex.Unlock()
}

// Status returns status of NacosServiceRegister.
func (n *NacosServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := n.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	n.statusMutex.Lock()
	serversNum := n.instancesNum
	n.statusMutex.Unlock()

	s.ServiceInstancesNum = serversNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes NacosServiceRegistry.
func (n *NacosServiceRegistry) Close() {
	n.serviceRegistry.DeregisterRegistry(n.Name())

	close(n.done)
}

// Name returns name.
func (n *NacosServiceRegistry) Name() string {
	return n.superSpec.Name()
}

// Notify returns notify channel.
func (n *NacosServiceRegistry) Notify() <-chan *serviceregistry.RegistryEvent {
	return n.notify
}

// ApplyServiceInstances applies service instances to the registry.
func (n *NacosServiceRegistry) ApplyServiceInstances(instances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := n.getClient()
	if err != nil {
		return fmt.Errorf("%s get nacos client failed: %v",
			n.superSpec.Name(), err)
	}

	for _, instance := range instances {
		err := instance.Validate()
		if err != nil {
			return fmt.Errorf("%+v is invalid: %v", instance, err)
		}
	}

	for _, instance := range instances {
		registerInstance := n.serviceInstanceToRegisterInstance(instance)
		_, err := client.RegisterInstance(*registerInstance)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteServiceInstances applies service instances to the registry.
func (n *NacosServiceRegistry) DeleteServiceInstances(instances map[string]*serviceregistry.ServiceInstanceSpec) error {
	client, err := n.getClient()
	if err != nil {
		return fmt.Errorf("%s get nacos client failed: %v",
			n.superSpec.Name(), err)
	}

	for _, instance := range instances {
		deregisterInstance := n.serviceInstanceToDeregisterInstance(instance)
		_, err := client.DeregisterInstance(*deregisterInstance)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetServiceInstance get service instance from the registry.
func (n *NacosServiceRegistry) GetServiceInstance(serviceName, instanceID string) (*serviceregistry.ServiceInstanceSpec, error) {
	instances, err := n.ListServiceInstances(serviceName)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		if instance.InstanceID == instanceID {
			return instance, nil
		}
	}

	return nil, fmt.Errorf("%s/%s not found", serviceName, instanceID)
}

// ListServiceInstances list service instances of one service from the registry.
func (n *NacosServiceRegistry) ListServiceInstances(serviceName string) (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	client, err := n.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get nacos client failed: %v",
			n.superSpec.Name(), err)
	}

	service, err := client.GetService(vo.GetServiceParam{
		ServiceName: serviceName,
	})
	if err != nil {
		return nil, err
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, nacosInstance := range service.Hosts {
		serviceInstance := n.nacosInstanceToServiceInstance(&nacosInstance)
		err := serviceInstance.Validate()
		if err != nil {
			return nil, fmt.Errorf("%+v is invalid: %v", serviceInstance, err)
		}

		instances[serviceInstance.Key()] = serviceInstance
	}

	return instances, nil
}

// ListAllServiceInstances list all service instances from the registry.
func (n *NacosServiceRegistry) ListAllServiceInstances() (map[string]*serviceregistry.ServiceInstanceSpec, error) {
	client, err := n.getClient()
	if err != nil {
		return nil, fmt.Errorf("%s get nacos client failed: %v",
			n.superSpec.Name(), err)
	}

	serviceNames := []string{}
	var pageNo uint32 = 1
	var pageSize uint32 = 1000
	for {
		services, err := client.GetAllServicesInfo(vo.GetAllServiceInfoParam{
			PageNo:   pageNo,
			PageSize: pageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("%s pull services failed: %v",
				n.superSpec.Name(), err)
		}

		if len(services.Doms) == 0 {
			break
		}

		serviceNames = append(serviceNames, services.Doms...)
		pageNo++
	}

	instances := make(map[string]*serviceregistry.ServiceInstanceSpec)
	for _, serviceName := range serviceNames {
		service, err := client.GetService(vo.GetServiceParam{
			ServiceName: serviceName,
		})
		if err != nil {
			return nil, err
		}

		for _, nacosInstance := range service.Hosts {
			serviceInstance := n.nacosInstanceToServiceInstance(&nacosInstance)
			err := serviceInstance.Validate()
			if err != nil {
				return nil, fmt.Errorf("%+v is invalid: %v", serviceInstance, err)
			}

			instances[serviceInstance.Key()] = serviceInstance
		}
	}

	return instances, nil
}

func (n *NacosServiceRegistry) serviceInstanceToRegisterInstance(instance *serviceregistry.ServiceInstanceSpec) *vo.RegisterInstanceParam {
	return &vo.RegisterInstanceParam{
		Metadata: map[string]string{
			MetaKeyRegistryName: instance.RegistryName,
			MetaKeyInstanceID:   instance.InstanceID,
		},
		ServiceName: instance.ServiceName,

		Ip:      instance.Address,
		Port:    uint64(instance.Port),
		Weight:  float64(instance.Weight),
		Enable:  true,
		Healthy: true,
	}
}

func (n *NacosServiceRegistry) serviceInstanceToDeregisterInstance(instance *serviceregistry.ServiceInstanceSpec) *vo.DeregisterInstanceParam {
	return &vo.DeregisterInstanceParam{
		ServiceName: instance.ServiceName,
		Ip:          instance.Address,
		Port:        uint64(instance.Port),
	}
}

func (n *NacosServiceRegistry) nacosInstanceToServiceInstance(nacosInstance *model.Instance) *serviceregistry.ServiceInstanceSpec {
	instanceID := nacosInstance.Metadata[MetaKeyInstanceID]
	if instanceID == "" {
		instanceID = nacosInstance.InstanceId
	}

	registryName := nacosInstance.Metadata[MetaKeyRegistryName]
	if registryName == "" {
		registryName = n.Name()
	}

	grpSvcName := strings.Split(nacosInstance.ServiceName, "@@")
	instance := &serviceregistry.ServiceInstanceSpec{
		RegistryName: registryName,
		ServiceName:  grpSvcName[len(grpSvcName)-1],
		InstanceID:   instanceID,
		Address:      nacosInstance.Ip,
		Port:         uint16(nacosInstance.Port),
		Weight:       int(nacosInstance.Weight),
	}

	return instance
}
