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

package master

import (
	"sort"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	syncInterval = 10 * time.Second
)

type (
	registrySyncer struct {
		superSpec *supervisor.Spec
		spec      *spec.Admin

		externalInstances map[string]*serviceregistry.ServiceInstanceSpec
		service           *service.Service
		informer          informer.Informer
		serviceRegistry   *serviceregistry.ServiceRegistry
		registryWatcher   serviceregistry.RegistryWatcher

		done chan struct{}
	}
)

func newRegistrySyncer(superSpec *supervisor.Spec) *registrySyncer {
	spec := superSpec.ObjectSpec().(*spec.Admin)

	rs := &registrySyncer{
		superSpec: superSpec,
		spec:      spec,

		done: make(chan struct{}),
	}

	if spec.ExternalServiceRegistry == "" {
		return rs
	}

	store := storage.New(superSpec.Name(), superSpec.Super().Cluster())
	rs.service = service.New(superSpec)
	rs.informer = informer.NewInformer(store, "")
	rs.informer.OnAllServiceInstanceSpecs(rs.serviceInstanceSpecsFunc)

	rs.serviceRegistry = superSpec.Super().MustGetSystemController(serviceregistry.Kind).Instance().(*serviceregistry.ServiceRegistry)
	rs.registryWatcher = rs.serviceRegistry.NewRegistryWatcher(spec.ExternalServiceRegistry)

	go rs.run()

	return rs
}

func (rs *registrySyncer) run() {
	for {
		select {
		case <-rs.done:
			return
		case event := <-rs.registryWatcher.Watch():
			if !rs.needSync() {
				continue
			}

			rs.handleEvent(event)
			rs.updateServiceSpec()
		}
	}
}

func (rs *registrySyncer) handleEvent(event *serviceregistry.RegistryEvent) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("recover from %v", r)
		}
	}()

	if event.UseReplace {
		event.Replace = rs.filterExternalInstances(event.Replace, rs.externalRegistryName())

		oldInstances := rs.service.ListAllServiceInstanceSpecs()
		for _, oldInstance := range oldInstances {
			if oldInstance.RegistryName != rs.externalRegistryName() {
				continue
			}

			instance := &serviceregistry.ServiceInstanceSpec{
				RegistryName: oldInstance.RegistryName,
				ServiceName:  oldInstance.ServiceName,
				InstanceID:   oldInstance.InstanceID,
			}
			_, exists := event.Replace[instance.Key()]
			if !exists {
				rs.service.DeleteServiceInstanceSpec(oldInstance.ServiceName, oldInstance.InstanceID)
			}
		}

		for _, instance := range event.Replace {
			rs.service.PutServiceInstanceSpec(rs.externalToMeshInstance(instance))
		}

		return
	}

	event.Delete = rs.filterExternalInstances(event.Delete, rs.externalRegistryName())
	event.Apply = rs.filterExternalInstances(event.Apply, rs.externalRegistryName())

	for _, instance := range event.Delete {
		rs.service.DeleteServiceInstanceSpec(instance.ServiceName, instance.InstanceID)
	}
	for _, instance := range event.Apply {
		rs.service.PutServiceInstanceSpec(rs.externalToMeshInstance(instance))
	}
}

func (rs *registrySyncer) updateServiceSpec() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("recover from %v", r)
		}
	}()

	allInstances := rs.service.ListAllServiceInstanceSpecs()
	externalServices := make(map[string]struct{})
	for _, instance := range allInstances {
		if instance.RegistryName != rs.externalRegistryName() {
			continue
		}
		externalServices[instance.ServiceName] = struct{}{}
	}

	allServices := rs.service.ListServiceSpecs()
	for _, service := range allServices {
		if service.RegistryName != rs.externalRegistryName() {
			continue
		}

		_, exists := externalServices[service.Name]
		if !exists {
			logger.Infof("delete external service spec %s", service.Name)
			rs.service.DeleteServiceSpec(service.Name)
		}
	}

	if len(externalServices) == 0 {
		rs.service.DeleteTenantSpec(rs.externalRegistryName())
		return
	}

	servicesSlice := make([]string, 0)
	for service := range externalServices {
		servicesSlice = append(servicesSlice, service)
	}
	sort.Strings(servicesSlice)
	rs.service.PutTenantSpec(&spec.Tenant{
		Name:        rs.externalRegistryName(),
		Services:    servicesSlice,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Description: "Generated by registry syncer",
	})

	for service := range externalServices {
		serviceSpec := &spec.Service{
			RegistryName:   rs.externalRegistryName(),
			Name:           service,
			RegisterTenant: rs.externalRegistryName(),
			Sidecar: &spec.Sidecar{
				DiscoveryType:   rs.spec.RegistryType,
				Address:         "127.0.0.1",
				IngressPort:     13001,
				IngressProtocol: "http",
				EgressPort:      13002,
				EgressProtocol:  "http",
			},
		}
		rs.service.PutServiceSpec(serviceSpec)
	}
}

func (rs *registrySyncer) serviceInstanceSpecsFunc(meshInstances map[string]*spec.ServiceInstanceSpec) bool {
	if !rs.needSync() {
		return true
	}

	oldInstances, err := rs.serviceRegistry.ListAllServiceInstances(rs.spec.ExternalServiceRegistry)
	if err != nil {
		logger.Errorf("list all service instances of %s: %v", rs.spec.ExternalServiceRegistry, err)
		return true
	}

	oldInstances = rs.filterExternalInstances(oldInstances, rs.meshRegistryName())

	meshInstances = rs.filterMeshInstances(meshInstances, rs.meshRegistryName())
	newInstances := rs.meshToExternalInstances(meshInstances)

	event := serviceregistry.NewRegistryEventFromDiff(rs.meshRegistryName(), oldInstances, newInstances)

	if len(event.Apply) != 0 {
		err := rs.serviceRegistry.ApplyServiceInstances(rs.externalRegistryName(), event.Apply)
		if err != nil {
			logger.Errorf("apply service instances failed: %v", err)
			return true
		}
	}
	if len(event.Delete) != 0 {
		err := rs.serviceRegistry.DeleteServiceInstances(rs.externalRegistryName(), event.Delete)
		if err != nil {
			logger.Errorf("delete service instances failed: %v", err)
		}
	}

	return true
}

func (rs *registrySyncer) needSync() bool {
	// NOTE: Only need one member in the cluster to do sync.
	return rs.superSpec.Super().Cluster().IsLeader()
}

func (rs *registrySyncer) meshRegistryName() string {
	return rs.superSpec.Name()
}

func (rs *registrySyncer) externalRegistryName() string {
	return rs.spec.ExternalServiceRegistry
}

func (rs *registrySyncer) filterExternalInstances(instances map[string]*serviceregistry.ServiceInstanceSpec, registryNames ...string) map[string]*serviceregistry.ServiceInstanceSpec {
	result := make(map[string]*serviceregistry.ServiceInstanceSpec)

	for _, instance := range instances {
		if stringtool.StrInSlice(instance.RegistryName, registryNames) {
			result[instance.Key()] = instance
		}
	}

	return result
}

func (rs *registrySyncer) filterMeshInstances(instances map[string]*spec.ServiceInstanceSpec, registryNames ...string) map[string]*spec.ServiceInstanceSpec {
	result := make(map[string]*spec.ServiceInstanceSpec)

	for _, instance := range instances {
		if instance.Status != spec.ServiceStatusUp {
			continue
		}
		if stringtool.StrInSlice(instance.RegistryName, registryNames) {
			result[instance.Key()] = instance
		}
	}

	return result
}

func (rs *registrySyncer) meshToExternalInstances(instances map[string]*spec.ServiceInstanceSpec) map[string]*serviceregistry.ServiceInstanceSpec {
	result := make(map[string]*serviceregistry.ServiceInstanceSpec)

	for _, instance := range instances {
		externalInstance := &serviceregistry.ServiceInstanceSpec{
			RegistryName: instance.RegistryName,
			ServiceName:  instance.ServiceName,
			InstanceID:   instance.InstanceID,

			Address: instance.IP,
			Port:    uint16(instance.Port),
		}
		result[externalInstance.Key()] = externalInstance
	}

	return result
}

func (rs *registrySyncer) externalToMeshInstance(instance *serviceregistry.ServiceInstanceSpec) *spec.ServiceInstanceSpec {
	return &spec.ServiceInstanceSpec{
		RegistryName: instance.RegistryName,
		ServiceName:  instance.ServiceName,
		InstanceID:   instance.InstanceID,
		IP:           instance.Address,
		Port:         uint32(instance.Port),
		Status:       spec.ServiceStatusUp,
	}
}

func (rs *registrySyncer) close() {
	close(rs.done)

	if rs.informer != nil {
		rs.informer.Close()
	}
}
