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
			rs.handleEvent(event)
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
			_, existed := event.Replace[instance.Key()]
			if !existed {
				rs.service.DeleteServiceInstanceSpec(oldInstance.ServiceName, oldInstance.InstanceID)
			}
		}

		for _, instance := range event.Replace {
			rs.service.PutServiceInstanceSpec(rs.externalToMeshInstance(instance))
		}
		return
	}

	for _, instance := range event.Delete {
		rs.service.DeleteServiceInstanceSpec(instance.ServiceName, instance.InstanceID)
	}
	for _, instance := range event.Apply {
		rs.service.PutServiceInstanceSpec(rs.externalToMeshInstance(instance))
	}
}

func (rs *registrySyncer) serviceInstanceSpecsFunc(meshInstances map[string]*spec.ServiceInstanceSpec) bool {
	oldInstances, err := rs.serviceRegistry.ListAllServiceInstances(rs.spec.ExternalServiceRegistry)
	if err != nil {
		logger.Errorf("list all service instances of %s: %v", rs.spec.ExternalServiceRegistry, err)
		return true
	}
	oldInstances = rs.filterExternalInstances(oldInstances, rs.meshRegistryName())

	meshInstances = rs.filterMeshInstances(meshInstances, "", rs.meshRegistryName())
	newInstances := rs.meshToExternalInstances(meshInstances)

	event := serviceregistry.NewRegistryEventFromDiff(rs.meshRegistryName(), oldInstances, newInstances)
	if len(event.Apply) != 0 {
		rs.serviceRegistry.ApplyServiceInstances(rs.externalRegistryName(), event.Apply)
	}
	if len(event.Delete) != 0 {
		rs.serviceRegistry.DeleteServiceInstances(rs.externalRegistryName(), event.Delete)
	}

	return true
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
		if stringtool.StrInSlice(instance.RegistryName, registryNames) {
			result[instance.InstanceID] = instance
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

			HostIP: instance.IP,
			Port:   uint16(instance.Port),
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
		IP:           instance.HostIP,
		Port:         uint32(instance.Port),
	}
}

func (rs *registrySyncer) close() {
	close(rs.done)

	if rs.informer != nil {
		rs.informer.Close()
	}
}
