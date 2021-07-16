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

package service

import (
	"fmt"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/api"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/supervisor"
)

type (
	// Service is the business layer between mesh and store.
	// It is not concurrently safe, the users need to do it by themselves.
	Service struct {
		superSpec *supervisor.Spec
		spec      *spec.Admin

		store storage.Storage
	}
)

func New(superSpec *supervisor.Spec) *Service {
	s := &Service{
		superSpec: superSpec,
		spec:      superSpec.ObjectSpec().(*spec.Admin),
		store:     storage.New(superSpec.Name(), superSpec.Super().Cluster()),
	}

	return s
}

// Lock locks all store, it will do cluster panic if failed.
func (s *Service) Lock() {
	err := s.store.Lock()
	if err != nil {
		api.ClusterPanic(err)
	}
}

// Unlock unlocks all store, it will do cluster panic if failed.
func (s *Service) Unlock() {
	err := s.store.Unlock()
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *Service) PutServiceSpec(serviceSpec *spec.Service) {
	buff, err := yaml.Marshal(serviceSpec)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", serviceSpec, err))
	}

	err = s.store.Put(layout.ServiceSpecKey(serviceSpec.Name), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *Service) GetServiceSpec(serviceName string) *spec.Service {
	serviceSpec, _ := s.GetServiceSpecWithInfo(serviceName)
	return serviceSpec
}

func (s *Service) GetServiceSpecWithInfo(serviceName string) (*spec.Service, *mvccpb.KeyValue) {
	kv, err := s.store.GetRaw(layout.ServiceSpecKey(serviceName))
	if err != nil {
		api.ClusterPanic(err)
	}

	if kv == nil {
		return nil, nil
	}

	serviceSpec := &spec.Service{}
	err = yaml.Unmarshal([]byte(kv.Value), serviceSpec)
	if err != nil {
		panic(fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", string(kv.Value), err))
	}

	return serviceSpec, kv
}

func (s *Service) GetGlobalCanaryHeaders() *spec.GlobalCanaryHeaders {
	globalCanaryHeaders, _ := s.GetGlobalCanaryHeadersWithInfo()
	return globalCanaryHeaders
}

func (s *Service) GetGlobalCanaryHeadersWithInfo() (*spec.GlobalCanaryHeaders, *mvccpb.KeyValue) {
	kv, err := s.store.GetRaw(layout.GlobalCanaryHeaders())
	if err != nil {
		api.ClusterPanic(err)
	}

	if kv == nil {
		return nil, nil
	}

	globalCanaryHeaders := &spec.GlobalCanaryHeaders{}
	err = yaml.Unmarshal([]byte(kv.Value), globalCanaryHeaders)
	if err != nil {
		panic(fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", string(kv.Value), err))
	}

	return globalCanaryHeaders, kv
}

func (s *Service) PutGlobalCanaryHeaders(globalCanaryHeaders *spec.GlobalCanaryHeaders) {
	buff, err := yaml.Marshal(globalCanaryHeaders)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", globalCanaryHeaders, err))
	}

	err = s.store.Put(layout.GlobalCanaryHeaders(), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *Service) DeleteServiceSpec(serviceName string) {
	err := s.store.Delete(layout.ServiceSpecKey(serviceName))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *Service) ListServiceSpecs() []*spec.Service {
	services := []*spec.Service{}
	kvs, err := s.store.GetPrefix(layout.ServiceSpecPrefix())
	if err != nil {
		api.ClusterPanic(err)
	}

	for _, v := range kvs {
		serviceSpec := &spec.Service{}
		err := yaml.Unmarshal([]byte(v), serviceSpec)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		services = append(services, serviceSpec)
	}

	return services
}

func (s *Service) GetTenantSpec(tenantName string) *spec.Tenant {
	tenant, _ := s.GetTenantSpecWithInfo(tenantName)
	return tenant
}

func (s *Service) GetTenantSpecWithInfo(tenantName string) (*spec.Tenant, *mvccpb.KeyValue) {
	kvs, err := s.store.GetRaw(layout.TenantSpecKey(tenantName))
	if err != nil {
		api.ClusterPanic(err)
	}

	if kvs == nil {
		return nil, nil
	}

	tenant := &spec.Tenant{}
	err = yaml.Unmarshal(kvs.Value, tenant)
	if err != nil {
		panic(fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", string(kvs.Value), err))
	}

	return tenant, kvs
}

func (s *Service) PutTenantSpec(tenantSpec *spec.Tenant) {
	buff, err := yaml.Marshal(tenantSpec)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", tenantSpec, err))
	}

	err = s.store.Put(layout.TenantSpecKey(tenantSpec.Name), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *Service) ListAllServiceInstanceStatuses() []*spec.ServiceInstanceStatus {
	return s.listServiceInstanceStatuses(true, "")
}

func (s *Service) ListServiceInstanceStatuses(serviceName string) []*spec.ServiceInstanceStatus {
	return s.listServiceInstanceStatuses(false, serviceName)
}

func (s *Service) listServiceInstanceStatuses(all bool, serviceName string) []*spec.ServiceInstanceStatus {
	statuses := []*spec.ServiceInstanceStatus{}
	var prefix string
	if all {
		prefix = layout.AllServiceInstanceStatusPrefix()
	} else {
		prefix = layout.ServiceInstanceSpecPrefix(serviceName)
	}

	kvs, err := s.store.GetPrefix(prefix)
	if err != nil {
		api.ClusterPanic(err)
	}

	for _, v := range kvs {
		status := &spec.ServiceInstanceStatus{}
		if err = yaml.Unmarshal([]byte(v), status); err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}

		statuses = append(statuses, status)
	}

	return statuses
}

func (s *Service) ListAllServiceInstanceSpecs() []*spec.ServiceInstanceSpec {
	return s.listServiceInstanceSpecs(true, "")
}

func (s *Service) ListServiceInstanceSpecs(serviceName string) []*spec.ServiceInstanceSpec {
	return s.listServiceInstanceSpecs(false, serviceName)
}

func (s *Service) listServiceInstanceSpecs(all bool, serviceName string) []*spec.ServiceInstanceSpec {
	specs := []*spec.ServiceInstanceSpec{}
	var prefix string
	if all {
		prefix = layout.AllServiceInstanceSpecPrefix()
	} else {
		prefix = layout.ServiceInstanceSpecPrefix(serviceName)
	}

	kvs, err := s.store.GetPrefix(prefix)
	if err != nil {
		api.ClusterPanic(err)
	}

	for _, v := range kvs {
		_spec := &spec.ServiceInstanceSpec{}
		if err = yaml.Unmarshal([]byte(v), _spec); err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}

		specs = append(specs, _spec)
	}

	return specs
}

func (s *Service) GetServiceInstanceSpec(serviceName, instanceID string) *spec.ServiceInstanceSpec {
	value, err := s.store.Get(layout.ServiceInstanceSpecKey(serviceName, instanceID))
	if err != nil {
		api.ClusterPanic(err)
	}

	if value == nil {
		return nil
	}

	instanceSpec := &spec.ServiceInstanceSpec{}
	err = yaml.Unmarshal([]byte(*value), instanceSpec)
	if err != nil {
		panic(fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", *value, err))
	}

	return instanceSpec
}

func (s *Service) PutServiceInstanceSpec(_spec *spec.ServiceInstanceSpec) {
	buff, err := yaml.Marshal(_spec)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", _spec, err))
	}

	err = s.store.Put(layout.ServiceInstanceSpecKey(_spec.ServiceName, _spec.InstanceID), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *Service) ListTenantSpecs() []*spec.Tenant {
	tenants := []*spec.Tenant{}
	kvs, err := s.store.GetPrefix(layout.TenantPrefix())
	if err != nil {
		api.ClusterPanic(err)
	}

	for _, v := range kvs {
		tenantSpec := &spec.Tenant{}
		err := yaml.Unmarshal([]byte(v), tenantSpec)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		tenants = append(tenants, tenantSpec)
	}

	return tenants
}

func (s *Service) DeleteTenantSpec(tenantName string) {
	err := s.store.Delete(layout.TenantSpecKey(tenantName))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *Service) GetIngressSpec(ingressName string) *spec.Ingress {
	ingress, _ := s.GetIngressSpecWithInfo(ingressName)
	return ingress
}

func (s *Service) GetIngressSpecWithInfo(ingressName string) (*spec.Ingress, *mvccpb.KeyValue) {
	kvs, err := s.store.GetRaw(layout.IngressSpecKey(ingressName))
	if err != nil {
		api.ClusterPanic(err)
	}

	if kvs == nil {
		return nil, nil
	}

	ingress := &spec.Ingress{}
	err = yaml.Unmarshal(kvs.Value, ingress)
	if err != nil {
		panic(fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", string(kvs.Value), err))
	}

	return ingress, kvs
}

func (s *Service) PutIngressSpec(ingressSpec *spec.Ingress) {
	buff, err := yaml.Marshal(ingressSpec)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", ingressSpec, err))
	}

	err = s.store.Put(layout.IngressSpecKey(ingressSpec.Name), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *Service) ListIngressSpecs() []*spec.Ingress {
	ingresses := []*spec.Ingress{}
	kvs, err := s.store.GetPrefix(layout.IngressPrefix())
	if err != nil {
		api.ClusterPanic(err)
	}

	for _, v := range kvs {
		ingressSpec := &spec.Ingress{}
		err := yaml.Unmarshal([]byte(v), ingressSpec)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		ingresses = append(ingresses, ingressSpec)
	}

	return ingresses
}

func (s *Service) DeleteIngressSpec(ingressName string) {
	err := s.store.Delete(layout.IngressSpecKey(ingressName))
	if err != nil {
		api.ClusterPanic(err)
	}
}
