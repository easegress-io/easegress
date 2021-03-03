package meshcontroller

import (
	"fmt"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"gopkg.in/yaml.v2"
)

type (
	// MeshServiceServer handle mesh service about logic, its resilience/sidecar/... configs
	// apply into EG's pipeline , httpserver, and update these object continually
	MeshServiceServer struct {
		store        MeshStorage
		AliveSeconds int64

		// store keys to watch
		watchIngressSpecNames map[string](chan storeOpMsg)
		watchEgressSpecNames  map[string](chan storeOpMsg)

		ingressNotifyChan chan IngressMsg
		mux               sync.RWMutex
	}
)

// NewMeshServiceServer retusn a initialized MeshServiceServer
func NewMeshServiceServer(store MeshStorage, aliveSeconds int64, ingressNotifyChan chan IngressMsg) *MeshServiceServer {
	return &MeshServiceServer{
		store:                 store,
		AliveSeconds:          aliveSeconds,
		watchIngressSpecNames: make(map[string](chan storeOpMsg)),
		watchEgressSpecNames:  make(map[string](chan storeOpMsg)),
		ingressNotifyChan:     ingressNotifyChan,
		mux:                   sync.RWMutex{},
	}
}

// CheckSpecs checks specified specs for one services
func (mss *MeshServiceServer) CheckSpecs() error {
	mss.checkIngressSpecs()
	mss.checkEgressSpecs()
	return nil
}

// addWatchIngressSpecNames
func (mss *MeshServiceServer) addWatchIngressSpecNames(serviceName string) error {
	var err error
	httpServerName := genIngreePipelineSpecName(serviceName)
	pipelineName := genIngreePipelineSpecName(serviceName)

	mss.mux.Lock()
	defer mss.mux.Unlock()

	// add watch ingress HTTPServer spec
	if _, ok := mss.watchIngressSpecNames[httpServerName]; !ok {
		if mss.watchIngressSpecNames[httpServerName], err = mss.store.WatchKey(httpServerName); err != nil {
			logger.Errorf("BUG failed to watch ingress http server spec name: %s, err: %v", httpServerName, err)
			return err
		}
	}

	// add watch ingress HTTPPipeline spec
	if _, ok := mss.watchIngressSpecNames[pipelineName]; !ok {
		if mss.watchIngressSpecNames[pipelineName], err = mss.store.WatchKey(pipelineName); err != nil {
			logger.Errorf("BUG failed to watch ingress pipeline spec name : %s, err :%v", pipelineName)
			return err
		}
	}

	return nil
}

func (mss *MeshServiceServer) checkIngressSpecs() {
	mss.mux.RLock()
	defer mss.mux.RUnlock()

	// iterate all wanted ingress related keys
	for k, v := range mss.watchIngressSpecNames {
		if mss.ingressNotifyChan == nil {
			logger.Errorf("BUG, using notify ingress without init it, should not be called in master role")
			continue
		}

		select {
		case msg := <-v:
			logger.Debugf("ingress key :%s has operation %v ", k, msg)
			// notify ingress
			mss.ingressNotifyChan <- IngressMsg{storeMsg: msg}
		default:
			// for not blocking read
		}
	}

	return
}

func (mss *MeshServiceServer) checkEgressSpecs() {

}

func (mss *MeshServiceServer) setServiceInstanceHeartbeat(serviceName, ID string, heartbeat *HeartbeatSpec) error {
	key := fmt.Sprintf(meshServiceInstanceHeartbeatPrefix, serviceName, ID)
	if newHeartbeat, err := yaml.Marshal(heartbeat); err != nil {
		logger.Errorf("BUG, service :%s marahsal yaml failed, heartbeat : %v , err : %v", serviceName, heartbeat, err)
		return err
	} else {
		if err = mss.store.Set(key, string(newHeartbeat)); err != nil {
			logger.Errorf("service :%s , set store failed, key %s, err :%v", serviceName, key, err)
			return err
		}
	}

	return nil
}

func (mss *MeshServiceServer) getServiceInstanceHeartbeat(serviceName, ID string) (*HeartbeatSpec, error) {
	var (
		err           error
		heartbeatYAML string
		heartbeat     HeartbeatSpec
	)
	key := fmt.Sprintf(meshServiceInstanceHeartbeatPrefix, serviceName, ID)
	if heartbeatYAML, err = mss.store.Get(key); err != nil {
		if err == ErrKeyNoExist {
			//
		} else {
			logger.Errorf("get service : %s's heartbeat %s from store failed, err :%v", serviceName, key, err)
		}
		return &heartbeat, err
	} else {
		if err = yaml.Unmarshal([]byte(heartbeatYAML), &heartbeat); err != nil {
			logger.Errorf("BUG, serivce : %s ummarshal yaml failed, spec :%s, err : %v", serviceName, heartbeatYAML, err)
			return &heartbeat, err
		}
	}

	return &heartbeat, err
}

// CheckLocalInstaceHeartbeat communicate with Java process and check its health.
func (mss *MeshServiceServer) CheckLocalInstaceHeartbeat(serviceName, ID string) error {
	var (
		alive bool
	)

	//[TODO] call Java process agent with JMX, check it alive

	if alive == true {
		// update store heart beat record
		heartbeat, err := mss.getServiceInstanceHeartbeat(serviceName, ID)
		if err != nil && err != ErrKeyNoExist {
			return err
		}

		heartbeat.LastActiveTime = time.Now().Unix()

		return mss.setServiceInstanceHeartbeat(serviceName, ID, heartbeat)

	} else {
		// do nothing, master will notice this irregular
		// and cause the update of Egress's Pipelines which are relied
		// on this instance
	}

	return nil
}

// WatchSerivceInstancesHeartbeat watchs all service instances heart beat in mesh.
func (mss *MeshServiceServer) WatchSerivceInstancesHeartbeat() error {
	// Get all tenants
	tenantSpecs, err := mss.store.GetWithPrefix(meshTenantListPrefix)
	if err != nil && err != ErrKeyNoExist {
		logger.Errorf("get all tenant failed, err :%v", err)
		return err
	}

	var services []string
	for _, v := range tenantSpecs {
		var tenant TenantSpec
		if err = yaml.Unmarshal([]byte(v.val), &tenant); err != nil {
			logger.Errorf("BUG, unmarshal tenat : %s, spe : %s failed, err : %v", v.key, v.val, err)
			continue
		} else {
			// Get all serivces from one tenant
			services = append(services, tenant.ServicesList...)
		}
	}

	var allIns []*ServiceInstance
	// find  all serivce instance
	for _, v := range services {
		insList, err := mss.GetSerivceInstances(v)
		if err != nil && err != ErrKeyNoExist {
			logger.Errorf("get serivce :%s, instance list failed, err :%v", v, err)
			continue
		}

		allIns = append(allIns, insList...)
	}

	// read heartbeat record, if more than 60 seconds(configurable) after it has been pulled up,
	//  then set the instance to OUT_OF_SERVICE
	currTimeStamp := time.Now().Unix()
	for _, v := range allIns {
		heartbeat, err := mss.getServiceInstanceHeartbeat(v.ServiceName, v.InstanceID)

		if err != nil || currTimeStamp-heartbeat.LastActiveTime > mss.AliveSeconds {
			err = mss.UpdateServiceInstanceStatus(v.ServiceName, v.InstanceID, SerivceStatusOutOfSerivce)
			if err != nil {
				logger.Errorf("all service instance alive check failed, serivce :%s, ID :%s, err :%v", v.ServiceName, v.InstanceID, err)
			}
		}
	}

	return nil
}

// CreateDefaultSpecs generate a mesh service's default specs, including
// resilience, observability, loadBalance, and sidecar spec.
// also, it will create a default ingress/egress HTTPPipeline spec and HTTPservice spec
// and wrtie them into storage at last.
func (mss *MeshServiceServer) CreateDefaultSpecs(serviceName, tenant string) error {
	var (
		err           error
		resilenceSpec string
	)
	// create default basic specs when

	// generate default resilience spec,
	mss.store.Set(fmt.Sprint(meshServiceResiliencePrefix, serviceName), resilenceSpec)

	return err

}

// GetServiceSpec gets meshserivce spec from store.
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

// GetTenantSpec gets tenant basic info and its service name list.
func (mss *MeshServiceServer) GetTenantSpec(tenant string) (string, error) {
	var (
		err        error
		tenantSpec string
	)

	if tenantSpec, err = mss.store.Get(fmt.Sprint(meshTenantPrefix, tenant)); err != nil {
		logger.Errorf("get tenant: %s spec failed, %v", tenant, err)
	}

	return tenantSpec, err
}

// GetSerivceInstances get one service Instances from storage by provided ID.
func (mss *MeshServiceServer) GetSerivceInstance(serviceName, ID string) (*ServiceInstance, error) {
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

// GetSerivceInstances get whole service Instances from store.
func (mss *MeshServiceServer) GetSerivceInstances(serviceName string) ([]*ServiceInstance, error) {
	var (
		err      error
		insList  []*ServiceInstance
		insYAMLs []record
	)

	insYAMLs, err = mss.store.GetWithPrefix(fmt.Sprintf(meshServiceInstanceListPrefix, serviceName))

	if err != nil && err != ErrKeyNoExist {
		return insList, err
	}

	for _, v := range insYAMLs {
		var ins *ServiceInstance
		if err = yaml.Unmarshal([]byte(v.val), ins); err != nil {
			logger.Errorf("BUG unmarsh service :%s, record key :%s , val %s failed, err %v", serviceName, v.key, v.val, err)
			continue
		}
		insList = append(insList, ins)
	}

	return insList, nil

}

// DeleteSerivceInstance deletes one service registry instance.
func (mss *MeshServiceServer) DeleteSerivceInstance(serviceName, ID string) error {
	return mss.store.Delete(fmt.Sprintf(meshServiceInstancePrefix, serviceName, ID))
}

// UpdateServiceInstance  updates one instance's status field.
func (mss *MeshServiceServer) UpdateServiceInstanceLeases(serviceName, ID string, leases int64) error {
	updateLeases := func(ins *ServiceInstance) {
		if ins.Leases != leases {
			ins.Leases = leases
		}
	}

	err := mss.updateSerivceInstanceWithLock(serviceName, ID, updateLeases)
	return err
}

// UpdateServiceInstanceStatus  updates one instance's status field.
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
