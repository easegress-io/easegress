package master

import (
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"
	"gopkg.in/yaml.v2"
)

type (
	// MeshServiceServer handle mesh service about logic, its resilience/sidecar/... configs
	// apply into EG's pipeline , httpserver, and update these object continually
	MeshServiceServer struct {
		AliveSeconds int64
		store        storage.Storage

		mux sync.RWMutex
	}
)

// NewMeshServiceServer retusn a initialized MeshServiceServer
func NewMeshServiceServer(store storage.Storage, aliveSeconds int64) *MeshServiceServer {
	return &MeshServiceServer{
		store:        store,
		mux:          sync.RWMutex{},
		AliveSeconds: aliveSeconds,
	}
}

func (mss *MeshServiceServer) setServiceInstanceHeartbeat(serviceName, ID string, heartbeat *spec.Heartbeat) error {
	key := layout.GenServiceHeartbeatKey(serviceName, ID)
	if newHeartbeat, err := yaml.Marshal(heartbeat); err != nil {
		logger.Errorf("BUG, service :%s marahsal yaml failed, heartbeat : %v , err : %v", serviceName, heartbeat, err)
		return err
	} else {
		if err = mss.store.Put(key, string(newHeartbeat)); err != nil {
			logger.Errorf("service :%s , set store failed, key %s, err :%v", serviceName, key, err)
			return err
		}
	}

	return nil
}

func (mss *MeshServiceServer) getServiceInstanceHeartbeat(serviceName, ID string) (*spec.Heartbeat, error) {
	var (
		err           error
		heartbeatYAML *string
		heartbeat     spec.Heartbeat
	)
	key := layout.GenServiceHeartbeatKey(serviceName, ID)
	if heartbeatYAML, err = mss.store.Get(key); err != nil {
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
func (mss *MeshServiceServer) WatchSerivceInstancesHeartbeat() error {
	// Get all tenants

	tenantSpecs, err := mss.store.GetPrefix(layout.GetTenantPrefix())
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
	// find  all serivce instance
	for _, v := range services {
		insList, err := mss.GetSerivceInstances(v)
		if err != nil {
			logger.Errorf("get serivce :%s, instance list failed, err :%v", v, err)
			continue
		}

		for _, v := range insList {
			allIns = append(allIns, v)
		}
	}

	// read heartbeat record, if more than 60 seconds(configurable) after it has been pulled up,
	//  then set the instance to OUT_OF_SERVICE
	currTimeStamp := time.Now().Unix()
	for _, v := range allIns {
		heartbeat, err := mss.getServiceInstanceHeartbeat(v.ServiceName, v.InstanceID)

		if err != nil || currTimeStamp-heartbeat.LastActiveTime > mss.AliveSeconds {
			err = mss.UpdateServiceInstanceStatus(v.ServiceName, v.InstanceID, registrycenter.SerivceStatusOutOfSerivce)
			if err != nil {
				logger.Errorf("all service instance alive check failed, serivce :%s, ID :%s, err :%v", v.ServiceName, v.InstanceID, err)
			}
		}
	}

	return nil
}

// CreateDefaultSpecs generate a mesh service's default specs, including
// resilience, observability, loadBalance, and sidecar spec.
func (mss *MeshServiceServer) CreateDefaultSpecs(serviceName, tenant string) error {
	var (
		err error
		//resilenceSpec string
	)
	// create default basic specs when

	// generate default resilience spec,

	return err

}

// GetServiceSpec gets meshserivce spec from store.
func (mss *MeshServiceServer) GetServiceSpec(serviceName string) (*spec.Service, error) {
	var (
		err     error
		service *spec.Service
		spec    *string
	)
	if spec, err = mss.store.Get(layout.GenServerKey(serviceName)); err != nil {
		return service, err
	}

	err = yaml.Unmarshal([]byte(*spec), service)
	return service, err

}

// GetSidecarSepc gets meshserivce sidecar spec from etcd
func (mss *MeshServiceServer) GetSidecarSepc(serviceName string) (*spec.Sidecar, error) {
	return nil, nil
}

// GetTenantSpec gets tenant basic info and its service name list.
func (mss *MeshServiceServer) GetTenant(tenantName string) (*spec.Tenant, error) {
	var (
		err    error
		tenant *spec.Tenant
		spec   *string
	)

	if spec, err = mss.store.Get(layout.GenTenantKey(tenantName)); err != nil {
		logger.Errorf("get tenant: %s spec failed, %v", tenantName, err)
	}
	err = yaml.Unmarshal([]byte(*spec), tenant)

	return tenant, err
}

// GetSerivceInstances get whole service Instances from store.
func (mss *MeshServiceServer) GetSerivceInstances(serviceName string) (map[string]*spec.ServiceInstance, error) {
	insList := make(map[string]*spec.ServiceInstance)

	insYAMLs, err := mss.store.GetPrefix(layout.GenServiceInstancePrefix(serviceName))
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
func (mss *MeshServiceServer) DeleteSerivceInstance(serviceName, ID string) error {
	return mss.store.Delete(layout.GenServiceInstanceKey(serviceName, ID))
}

// UpdateServiceInstanceLeases updates one instance's status field.
func (mss *MeshServiceServer) UpdateServiceInstanceLeases(serviceName, ID string, leases int64) error {
	updateLeases := func(ins *spec.ServiceInstance) {
		if ins.Leases != leases {
			ins.Leases = leases
		}
	}
	err := mss.updateSerivceInstance(serviceName, ID, updateLeases)
	return err
}

// UpdateServiceInstanceStatus updates one instance's status field.
func (mss *MeshServiceServer) UpdateServiceInstanceStatus(serviceName, ID, status string) error {
	updateStatus := func(ins *spec.ServiceInstance) {
		if ins.Status != status {
			ins.Status = status
		}
	}
	err := mss.updateSerivceInstance(serviceName, ID, updateStatus)
	return err
}

func (mss *MeshServiceServer) updateSerivceInstance(serviceName, ID string, fn func(ins *spec.ServiceInstance)) error {
	var err error

	return err
}
