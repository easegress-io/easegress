package meshcontroller

import (
	"fmt"
	"time"

	"github.com/kataras/iris"
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
func (mss *MeshServiceServer) GetSerivceInstances(serviceName string) error {
	var err error
	// TODO
	return err
}

// DeleteSerivceInstance deletes one service registry instance
func (mss *MeshServiceServer) DeleteSerivceInstance(serviceName, instanceID string) (*ServiceInstance, error) {
	var err error

	return nil, err

}

// UpdateServiceInstanceLeases  updates one instance's status field
func (mss *MeshServiceServer) UpdateServiceInstanceLeases(ctx iris.Context) error {

	// TODO
	return nil
}

// UpdateServiceInstanceStaus  updates one instance's status field
func (mss *MeshServiceServer) UpdateServiceInstanceStaus(ctx iris.Context) error {
	// TOOD
	return nil
}
