package master

import (
	"sync"
	"time"

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

		superSpec *supervisor.Spec
		spec      *spec.Admin

		maxHeartbeatTimeout time.Duration

		store storage.Storage
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

func (s *masterService) setServiceInstanceHeartbeat(serviceName, ID string, heartbeat *spec.Heartbeat) error {
	key := layout.ServiceHeartbeatKey(serviceName, ID)

	newHeartbeat, err := yaml.Marshal(heartbeat)
	if err != nil {
		logger.Errorf("BUG, service :%s marahsal yaml failed, heartbeat : %v , err : %v", serviceName, heartbeat, err)
		return err
	}

	if err = s.store.Put(key, string(newHeartbeat)); err != nil {
		logger.Errorf("service :%s , set store failed, key %s, err :%v", serviceName, key, err)
		return err
	}

	return nil
}

func (s *masterService) getServiceInstanceHeartbeat(serviceName, ID string) (*spec.Heartbeat, error) {
	var (
		err           error
		heartbeatYAML *string
		heartbeat     spec.Heartbeat
	)
	key := layout.ServiceHeartbeatKey(serviceName, ID)
	if heartbeatYAML, err = s.store.Get(key); err != nil {
		if err != nil {
			logger.Errorf("get service : %s's heartbeat %s from store failed, err :%v", serviceName, key, err)
			return nil, err
		}
		if heartbeatYAML == nil {
			return &heartbeat, err
		}
	} else {
		if err = yaml.Unmarshal([]byte(*heartbeatYAML), &heartbeat); err != nil {
			logger.Errorf("BUG, serivce : %s ummarshal yaml failed, spec :%s, err : %v", serviceName, heartbeatYAML, err)
			return &heartbeat, err
		}
	}

	return &heartbeat, err
}

// WatchSerivceInstancesHeartbeat watchs all service instances heart beat in mesh.
func (s *masterService) WatchSerivceInstancesHeartbeat() error {
	tenantSpecs, err := s.store.GetPrefix(layout.TenantPrefix())
	if err != nil {
		logger.Errorf("get all tenant failed, err :%v", err)
		return err
	}

	var services []string
	for k, v := range tenantSpecs {
		var tenant spec.Tenant
		if err = yaml.Unmarshal([]byte(v), &tenant); err != nil {
			logger.Errorf("BUG, unmarshal tenat : %s, spe : %s failed, err : %v", k, v, err)
			continue
		} else {
			// Get all serivces from one tenant
			services = append(services, tenant.ServicesList...)
		}
	}

	var allIns []*spec.ServiceInstance
	for _, v := range services {
		insList, err := s.GetSerivceInstances(v)
		if err != nil {
			logger.Errorf("get serivce :%s, instance list failed, err :%v", v, err)
			continue
		}

		for _, v := range insList {
			allIns = append(allIns, v)
		}
	}

	// read heartbeat record, if more than 60 seconds(configurable) after it has been pulled up,
	// then set the instance to OUT_OF_SERVICE
	currTimeStamp := time.Now().Unix()
	for _, v := range allIns {
		heartbeat, err := s.getServiceInstanceHeartbeat(v.ServiceName, v.InstanceID)

		if err != nil || currTimeStamp-heartbeat.LastActiveTime > int64(s.maxHeartbeatTimeout.Seconds()) {
			err = s.UpdateServiceInstanceStatus(v.ServiceName, v.InstanceID, registrycenter.SerivceStatusOutOfSerivce)
			if err != nil {
				logger.Errorf("all service instance alive check failed, serivce :%s, ID :%s, err :%v", v.ServiceName, v.InstanceID, err)
			}
		}
	}

	return nil
}

// CreateDefaultSpecs generate a mesh service's default specs, including
// resilience, observability, loadBalance, and sidecar spec.
func (s *masterService) CreateDefaultSpecs(serviceName, tenant string) error {
	var (
		err error
		//resilenceSpec string
	)
	// create default basic specs when
	serviceSpec := spec.Service{
		Name:           serviceName,
		RegisterTenant: tenant,
		Resilience:     &spec.Resilience{},
		Canary:         &spec.Canary{},
		LoadBalance:    &spec.LoadBalance{},
		Sidecar:        &spec.Sidecar{},
		Observability:  &spec.Observability{},
	}

	// generate default resilience spec,

	specBytes, err := yaml.Marshal(serviceSpec)
	if err != nil {
		logger.Errorf("Mmarshal default service spec failed, err : %v", serviceName, err)
	}
	err = s.store.Put(layout.ServiceKey(serviceName), string(specBytes))

	return err

}

func (s *masterService) CreateService(serviceSpec *spec.Service) error {
	var (
		err error
	)
	serviceSpecBytes, err := yaml.Marshal(serviceSpec)
	if err != nil {
		logger.Errorf("Unmarshal ServiceSpec: %s,failed, err : %v", serviceSpec, err)
		return err

	}

	err = s.store.Put(layout.ServiceKey(serviceSpec.Name), string(serviceSpecBytes))
	if err != nil {
		logger.Errorf("Service %s create failed, err :%v", serviceSpec.Name, err)
		return err

	}
	return nil
}

// GetService gets meshserivce spec from store.
func (s *masterService) GetService(serviceName string) (*spec.Service, error) {
	var (
		err         error
		service     *spec.Service
		serviceSpec *string
	)
	if serviceSpec, err = s.store.Get(layout.ServiceKey(serviceName)); err != nil {
		logger.Errorf("Get %s ServiceSpec failed, err :%v", serviceName, err)
		return service, err
	}

	err = yaml.Unmarshal([]byte(*serviceSpec), service)
	if err != nil {
		logger.Errorf("BUG, unmarshal Service : %s,failed, err : %v", serviceName, err)
	}
	return service, err
}

// GetServices gets meshserivce spec from store.
func (s *masterService) GetServiceList(serviceName string) (map[string]*spec.Service, error) {

	svcList := make(map[string]*spec.Service)
	svcYAMLs, err := s.store.GetPrefix(layout.ServiceKey(serviceName))
	if err != nil {
		return svcList, err
	}

	for k, v := range svcYAMLs {
		var ins *spec.Service
		if err = yaml.Unmarshal([]byte(v), ins); err != nil {
			logger.Errorf("BUG unmarsh service :%s, record key :%s , val %s failed, err %v", serviceName, k, v, err)
			continue
		}
		svcList[k] = ins
	}

	return svcList, nil
}

func (s *masterService) UpdateServiceSpec(serviceSpec *spec.Service) error {
	var err error

	serviceSpecBytes, err := yaml.Marshal(serviceSpec)
	if err != nil {
		logger.Errorf("Unmarshal ServiceSpec: %s,failed, err : %v", serviceSpec, err)
		return err
	}

	err = s.store.Put(layout.ServiceKey(serviceSpec.Name), string(serviceSpecBytes))
	if err != nil {
		logger.Errorf("Service %s update failed, err :%v", serviceSpec.Name, err)
		return err
	}

	return nil
}

func (s *masterService) DeleteService(serviceName string) error {
	return s.store.Delete(layout.ServiceKey(serviceName))
}

// GetSidecarSepc gets meshserivce sidecar spec from etcd
func (s *masterService) GetSidecarSepc(serviceName string) (*spec.Sidecar, error) {
	return nil, nil
}

// GetTenantSpec gets tenant basic info and its service name list.
func (s *masterService) GetTenant(tenantName string) (*spec.Tenant, error) {
	var (
		err    error
		tenant *spec.Tenant
		spec   *string
	)

	if spec, err = s.store.Get(layout.TenantKey(tenantName)); err != nil {
		logger.Errorf("get tenant: %s spec failed, %v", tenantName, err)
	}
	err = yaml.Unmarshal([]byte(*spec), tenant)

	return tenant, err
}

// GetSerivceInstances get whole service Instances from store.
func (s *masterService) GetSerivceInstances(serviceName string) (map[string]*spec.ServiceInstance, error) {
	insList := make(map[string]*spec.ServiceInstance)

	insYAMLs, err := s.store.GetPrefix(layout.ServiceInstancePrefix(serviceName))
	if err != nil {
		return insList, err
	}

	for k, v := range insYAMLs {
		var ins *spec.ServiceInstance
		if err = yaml.Unmarshal([]byte(v), ins); err != nil {
			logger.Errorf("BUG unmarsh service :%s, record key :%s , val %s failed, err %v", serviceName, k, v, err)
			continue
		}
		insList[k] = ins
	}

	return insList, nil

}

// DeleteSerivceInstance deletes one service registry instance.
func (s *masterService) DeleteSerivceInstance(serviceName, ID string) error {
	return s.store.Delete(layout.ServiceInstanceKey(serviceName, ID))
}

// UpdateServiceInstanceLeases updates one instance's status field.
func (s *masterService) UpdateServiceInstanceLeases(serviceName, ID string, leases int64) error {
	updateLeases := func(ins *spec.ServiceInstance) {
		if ins.Leases != leases {
			ins.Leases = leases
		}
	}
	err := s.updateSerivceInstance(serviceName, ID, updateLeases)
	return err
}

// UpdateServiceInstanceStatus updates one instance's status field.
func (s *masterService) UpdateServiceInstanceStatus(serviceName, ID, status string) error {
	updateStatus := func(ins *spec.ServiceInstance) {
		if ins.Status != status {
			ins.Status = status
		}
	}
	err := s.updateSerivceInstance(serviceName, ID, updateStatus)
	return err
}

func (s *masterService) updateSerivceInstance(serviceName, ID string, fn func(ins *spec.ServiceInstance)) error {
	var err error

	return err
}
