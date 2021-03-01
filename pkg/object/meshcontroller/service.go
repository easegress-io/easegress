package meshcontroller

import (
	"fmt"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"gopkg.in/yaml.v2"
)

// MeshServiceServer handle mesh service about logic, its resilience/sidecar/... config  apply
type MeshServiceServer struct {
	store MeshStorage
}

// WatchLocalInstaceHearbeat communicate with Java process and check its health
func (mss *MeshServiceServer) WatchLocalInstaceHearbeat(inveral time.Duration) error {

	for {
		// TODO
	}

	return nil
}

// WatchSerivceInstancesHeartbeat watchs all service instances heart beat in mesh
func (mss *MeshServiceServer) WatchSerivceInstancesHeartbeat() error {

	// Get all serivces
	// find one serivce instance

	// read heartbeat, if more than 30s (configurable), then set the instance to OUT_OF_SERVICE
	return nil
}

// CreateDefaultSpecs generate a mesh service's default specs, including
//   resilience, observability, loadBalance, and sidecar spec.
//   also, it will create a default
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

// GetSerivceInstances get whole service Instances from ETCD
func (mss *MeshServiceServer) GetSerivceInstances(serviceName, ID string) (*ServiceInstance, error) {
	var (
		err     error
		ins     *ServiceInstance
		insYAML string
	)

	insYAML, err = mss.store.Get(fmt.Sprintf(meshServiceInstancePrefix, serviceName, ID))

	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal([]byte(insYAML), ins)

	return ins, err

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

	if ins, err = mss.GetSerivceInstances(serviceName, ID); err != nil {

		return fmt.Errorf("get serivce :%s , instacne %s failed, err : %v", serviceName, ID, err)
	}

	updateFunc(ins)
	var insYAML []byte
	insYAML, err = yaml.Marshal(ins)
	err = mss.store.Set(fmt.Sprintf(meshServiceInstancePrefix, serviceName, ID), string(insYAML))
	return err
}
