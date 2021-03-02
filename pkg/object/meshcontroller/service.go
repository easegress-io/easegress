package meshcontroller

import (
	"fmt"
	"sync"

	"github.com/megaease/easegateway/pkg/logger"
	"gopkg.in/yaml.v2"
)

// MeshServiceServer handle mesh service about logic, its resilience/sidecar/... configs
// apply
type MeshServiceServer struct {
	store MeshStorage

	// store keys to watch
	watchIngressSpecNames map[string]string

	watchEgressSpecNames map[string]string

	mux sync.Mutex
}

// NewDefaultMeshServiceServer retusn a initialized MeshServiceServer
func NewDefaultMeshServiceServer(store MeshStorage) *MeshServiceServer {
	return &MeshServiceServer{
		store:                 store,
		watchIngressSpecNames: make(map[string]string),
		watchEgressSpecNames:  make(map[string]string),
	}
}

// CheckSpecs checks specified specs for one services
func (mss *MeshServiceServer) CheckSpecs() error {
	mss.checkIngressSpecs()
	mss.checkEgressSpecs()
	return nil
}

func (mss *MeshServiceServer) addWatchIngressSpecName(name string) {
	mss.mux.Lock()
	defer mss.mux.Unlock()

	mss.watchIngressSpecNames[name] = ""
}

func (mss *MeshServiceServer) checkIngressSpecs() {
	for k, v := range mss.watchIngressSpecNames {
		spec, err := mss.store.Get(k)
		if err != nil {
			if err != ErrKeyNoExist {
				logger.Errorf("chcek ingress specs failed, name %s name, err %v")
			} else {

			}
		}

		// has diff
		if spec != v {

		}
	}

}

func (mss *MeshServiceServer) checkEgressSpecs() {

}

// CheckLocalInstaceHearbeat communicate with Java process and check its health
func (mss *MeshServiceServer) CheckLocalInstaceHearbeat(serviceName string) error {
	var (
		err   error
		alive bool
	)

	//[TODO] call Java process agent with JMX, check it alive
	// er

	if alive == true {
		// update store heart beat record
	}

	return err
}

// WatchSerivceInstancesHeartbeat watchs all service instances heart beat in mesh
func (mss *MeshServiceServer) WatchSerivceInstancesHeartbeat() error {
	// Get all serivces
	// find one serivce instance

	// read heartbeat, if more than 30s (configurable), then set the instance to OUT_OF_SERVICE
	return nil
}

// CreateDefaultSpecs generate a mesh service's default specs, including
// resilience, observability, loadBalance, and sidecar spec.
// also, it will create a default ingress HTTPPipeline spec and HTTPservice spec
// and wrtie them into storage at last.
func (mss *MeshServiceServer) CreateDefaultSpecs(serviceName, tenant string) error {
	var (
		err           error
		resilenceSpec string
	)
	// create default basic specs when

	// generate default resilience spec,

	//
	mss.store.Set(fmt.Sprint(meshServiceResiliencePrefix, serviceName), resilenceSpec)

	return err

}

// GetServiceSpec gets meshserivce spec from etcd
func (mss *MeshServiceServer) GetServiceSpec(serviceName string) (*MeshServiceSpec, error) {
	var (
		err     error
		service *MeshServiceSpec
		spec    string
	)
	if spec, err = mss.store.Get(fmt.Sprint(meshServicePrefix, serviceName)); err != nil {
		return service, err
	}

	err = yaml.Unmarshal([]byte(spec), service)
	return service, err

}

// GetSidecarSepc gets meshserivce sidecar spec from etcd
func (mss *MeshServiceServer) GetSidecarSepc(serviceName string) (*SidecarSpec, error) {
	var (
		err     error
		sidecar *SidecarSpec
		spec    string
	)
	if spec, err = mss.store.Get(fmt.Sprint(meshServiceSidecarPrefix, serviceName)); err != nil {
		return sidecar, err
	}

	err = yaml.Unmarshal([]byte(spec), sidecar)
	return sidecar, err
}

// GetTenantSpec gets tenant basic info and its service name list
func (mss *MeshServiceServer) GetTenantSpec(tenant string) (string, error) {
	var (
		err        error
		tenantSpec string
	)

	if tenantSpec, err = mss.store.Get(fmt.Sprint(meshTenantServiceListPrefix, tenant)); err != nil {
		logger.Errorf("get tenant: %s spec failed, %v", tenant, err)
	}

	return tenantSpec, err
}

// GetSerivceInstances get one service Instances from storage by provided ID
func (mss *MeshServiceServer) GetSerivceInstance(serviceName, ID string) (*ServiceInstance, error) {
	var (
		err     error
		ins     *ServiceInstance
		insYAML string
	)

	insYAML, err = mss.store.Get(fmt.Sprintf(meshServiceInstancePrefix, serviceName))
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal([]byte(insYAML), ins)

	return ins, err

}

// GetSerivceInstances get whole service Instances from storage
func (mss *MeshServiceServer) GetSerivceInstances(serviceName string) ([]*ServiceInstance, error) {
	var (
		err      error
		insList  []*ServiceInstance
		insYAMLs []record
	)

	insYAMLs, err = mss.store.GetWithPrefix(fmt.Sprintf(meshServiceInstanceListPrefix, serviceName))

	if err != nil {
		return insList, err
	}

	for _, v := range insYAMLs {
		var ins *ServiceInstance
		if err = yaml.Unmarshal([]byte(v.val), ins); err != nil {
			logger.Errorf("[BUG] unmarsh service :%s, record key :%s , val %s failed, err %v", serviceName, v.key, v.val, err)
			continue
		}
		insList = append(insList, ins)
	}

	return insList, nil

}

// DeleteSerivceInstance deletes one service registry instance
func (mss *MeshServiceServer) DeleteSerivceInstance(serviceName, ID string) error {
	return mss.store.Delete(fmt.Sprintf(meshServiceInstancePrefix, serviceName, ID))
}

// UpdateServiceInstance  updates one instance's status field
func (mss *MeshServiceServer) UpdateServiceInstanceLeases(serviceName, ID string, leases int64) error {
	updateLeases := func(ins *ServiceInstance) {
		if ins.Leases != leases {
			ins.Leases = leases
		}
	}

	err := mss.updateSerivceInstanceWithLock(serviceName, ID, updateLeases)
	return err
}

// UpdateServiceInstanceStatus  updates one instance's status field
func (mss *MeshServiceServer) UpdateServiceInstanceStatus(serviceName, ID, status string) error {
	updateStatus := func(ins *ServiceInstance) {
		if ins.Status != status {
			ins.Status = status
		}

	}

	err := mss.updateSerivceInstanceWithLock(serviceName, ID, updateStatus)
	return err
}

func (mss *MeshServiceServer) updateSerivceInstanceWithLock(serviceName, ID string, updateFunc func(old *ServiceInstance)) error {
	var (
		err error
		ins *ServiceInstance
	)

	lockID := fmt.Sprint(meshServiceInstanceEtcdLockPrefix, ID)
	lockReleaseFunc := func() {
		if err = mss.store.ReleaseLock(lockID); err != nil {
			logger.Errorf("release lock ID %s failed err %v", lockID, err)
			err = nil
		}
	}

	if err = mss.store.AcquireLock(lockID, defaultRegistryExpireSecond); err != nil {
		logger.Errorf("require lock %s failed %v")
		return err
	}

	defer lockReleaseFunc()

	if ins, err = mss.GetSerivceInstance(serviceName, ID); err != nil {

		return fmt.Errorf("get serivce :%s , instacne %s failed, err : %v", serviceName, ID, err)
	}

	updateFunc(ins)
	var insYAML []byte
	insYAML, err = yaml.Marshal(ins)
	err = mss.store.Set(fmt.Sprintf(meshServiceInstancePrefix, serviceName, ID), string(insYAML))
	return err
}
