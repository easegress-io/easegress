package master

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/api"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegateway/pkg/supervisor"

	"gopkg.in/yaml.v2"
)

type (
	masterService struct {
		mutex sync.RWMutex

		superSpec           *supervisor.Spec
		spec                *spec.Admin
		maxHeartbeatTimeout time.Duration

		store storage.Storage

		failedServiceInstances []string
	}
)

func newMasterService(superSpec *supervisor.Spec, store storage.Storage) *masterService {
	s := &masterService{
		superSpec: superSpec,
		spec:      superSpec.ObjectSpec().(*spec.Admin),
		store:     store,
	}

	heartbeat, err := time.ParseDuration(s.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat interval %s to duration failed: %v",
			s.spec.HeartbeatInterval, err)
	}

	s.maxHeartbeatTimeout = heartbeat * 2

	return s
}

func (s *masterService) checkInstancesHeartbeat() {
	statuses := s.listServiceInstanceStatuses()
	specs := s.listServiceInstanceSpecs()

	failedInstances := []*spec.ServiceInstanceSpec{}
	now := time.Now()
	for instanceKey, status := range statuses {
		_spec, existed := specs[instanceKey]
		if !existed {
			logger.Errorf("BUG: %s got no spec", instanceKey)
			continue
		}

		lastHeartbeatTime, err := time.Parse(time.RFC3339, status.LastHeartbeatTime)
		if err != nil {
			logger.Errorf("BUG: parse %s last heartbeat time failed: %v", instanceKey, err)
			continue
		}

		gap := now.Sub(lastHeartbeatTime)
		if gap > s.maxHeartbeatTimeout {
			failedInstances = append(failedInstances, _spec)
		}
	}

	s.handleFailedInstances(failedInstances)
}

func (s *masterService) handleFailedInstances(failedInstances []*spec.ServiceInstanceSpec) {
	for _, _spec := range failedInstances {
		_spec.Status = registrycenter.SerivceStatusOutOfSerivce

		buff, err := yaml.Marshal(_spec)
		if err != nil {
			logger.Errorf("BUG: marshal %#v to yaml failed: %v", _spec, err)
			continue
		}

		key := layout.ServiceInstanceSpecKey(_spec.ServiceName, _spec.InstanceID)
		err = s.store.Put(key, string(buff))
		if err != nil {
			api.ClusterPanic(err)
		}
	}

}

func (s *masterService) status() *Status {
	s.mutex.RLock()
	defer s.mutex.Unlock()

	return &Status{}
}

func (s *masterService) putServiceSpec(serviceSpec *spec.Service) {
	buff, err := yaml.Marshal(serviceSpec)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", serviceSpec, err))
	}

	err = s.store.Put(layout.ServiceSpecKey(serviceSpec.Name), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *masterService) getServiceSpec(serviceName string) *spec.Service {
	value, err := s.store.Get(layout.ServiceSpecKey(serviceName))
	if err != nil {
		api.ClusterPanic(err)
	}

	if value == nil {
		return nil
	}

	serviceSpec := &spec.Service{}
	err = yaml.Unmarshal([]byte(*value), serviceSpec)
	if err != nil {
		panic(fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", *value, err))
	}

	return serviceSpec
}

func (s *masterService) deleteServiceSpec(serviceName string) {
	err := s.store.Delete(layout.ServiceSpecKey(serviceName))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *masterService) listServiceSpecs() []*spec.Service {
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

func (s *masterService) getTenantSpec(tenantName string) *spec.Tenant {
	value, err := s.store.Get(layout.TenantSpecKey(tenantName))
	if err != nil {
		api.ClusterPanic(err)
	}

	if value == nil {
		return nil
	}

	tenant := &spec.Tenant{}
	err = yaml.Unmarshal([]byte(*value), tenant)
	if err != nil {
		panic(fmt.Errorf("BUG: unmarshal %s to yaml failed: %v", *value, err))
	}

	return tenant
}

func (s *masterService) putTenantSpec(tenantSpec *spec.Tenant) {
	buff, err := yaml.Marshal(tenantSpec)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", tenantSpec, err))
	}

	err = s.store.Put(layout.ServiceSpecKey(tenantSpec.Name), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *masterService) listServiceInstanceStatuses() map[string]*spec.ServiceInstanceStatus {
	statuses := make(map[string]*spec.ServiceInstanceStatus)
	prefix := layout.AllServiceInstanceStatusPrefix()

	kvs, err := s.store.GetPrefix(prefix)
	if err != nil {
		api.ClusterPanic(err)
	}

	for k, v := range kvs {
		instance := &spec.ServiceInstanceStatus{}
		if err = yaml.Unmarshal([]byte(v), instance); err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}

		// NOTE: The format of instanceKey is `serviceName/instanceID`.
		instanceKey := strings.TrimPrefix(k, prefix)
		statuses[instanceKey] = instance
	}

	return statuses
}

func (s *masterService) listServiceInstanceSpecs() map[string]*spec.ServiceInstanceSpec {
	specs := make(map[string]*spec.ServiceInstanceSpec)
	prefix := layout.AllServiceInstanceSpecPrefix()

	kvs, err := s.store.GetPrefix(prefix)
	if err != nil {
		api.ClusterPanic(err)
	}

	for k, v := range kvs {
		_spec := &spec.ServiceInstanceSpec{}
		if err = yaml.Unmarshal([]byte(v), _spec); err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}

		// NOTE: The format of instanceKey is `serviceName/instanceID`.
		instanceKey := strings.TrimPrefix(k, prefix)
		specs[instanceKey] = _spec
	}

	return specs
}

func (s *masterService) getServiceInstanceSpec(serviceName, instanceID string) *spec.ServiceInstanceSpec {
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

func (s *masterService) putServiceInstanceSpec(_spec *spec.ServiceInstanceSpec) {
	buff, err := yaml.Marshal(_spec)
	if err != nil {
		panic(fmt.Errorf("BUG: marshal %#v to yaml failed: %v", _spec, err))
	}

	err = s.store.Put(layout.ServiceInstanceSpecKey(_spec.ServiceName, _spec.InstanceID), string(buff))
	if err != nil {
		api.ClusterPanic(err)
	}
}

func (s *masterService) listTenantSpecs() []*spec.Tenant {
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

func (s *masterService) deleteTenantSpec(tenantName string) {
	err := s.store.Delete(layout.TenantSpecKey(tenantName))
	if err != nil {
		api.ClusterPanic(err)
	}
}
