package registry

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"

	"github.com/ArthurHlt/go-eureka-client/eureka"
	consul "github.com/hashicorp/consul/api"
	"gopkg.in/yaml.v2"
)

const (
	defaultRegistryExpireSecond = 2 * 60 // default registry routine expire time, 2 minutes

	// RegistryTypeEureka indicates a Eureka registry center
	RegistryTypeEureka = "eureka"
	// RegistryTypeConsul indicates a Consul registry center
	RegistryTypeConsul = "consul"

	// SerivceStatusUp indicates this service instance can accept ingress traffic
	SerivceStatusUp = "UP"

	// SerivceStatusOutOfSerivce indicates this service can't accept ingress traffic
	SerivceStatusOutOfSerivce = "OUT_OF_SERVICE"

	defaultLeasesSeconds = 3600 * 24 * 365 // default one year leases
)

type (

	// ServiceInstance one registry info of serivce
	ServiceInstance struct {
		// Provide by registry client
		ServiceName string `yaml:"serviceName" jsonschema:"required"`
		InstanceID  string `yaml:"instanceID" jsonschema:"required"`
		IP          string `yaml:"IP" jsonschema:"required"`
		Port        uint32 `yaml:"port" jsonschema:"required"`
		Tenant      string `yaml:"tenat" jsonschema:"required"`

		// Set by heartbeat timer event or API
		Status       string `yaml:"status" jsonschema:"omitempty"`
		Leases       int64  `yaml:"timestamp" jsonschema:"omitempty"`
		RegistryTime int64  `yaml:"registryTime" jsonschema:"omitempty"`
	}

	// RegistryCenterServer handle all registry about logic
	RegistryCenterServer struct {
		// Currently we supports Eureka/Consul
		RegistryType string
		Registried   bool

		store         storage.Storage
		notifyIngress chan IngressMsg
	}
)

// NewRegistryCenterServer creates a initialized registry center server
func NewRegistryCenterServer(registryType string, store storage.Storage, notifyIngress chan IngressMsg) *RegistryCenterServer {
	return &RegistryCenterServer{
		RegistryType:  registryType,
		store:         store,
		notifyIngress: notifyIngress,
	}
}

// RegistryServiceInstance accepts Java Process's registry request in Eureka/Consul Format
// cause Eureka/Consul registry format are both with HTTP and POST body, we can combine them
// into one routine.
// Todo: Consider to split this routine for other None RESTful POST body
//       format in future.
func (rcs *RegistryCenterServer) RegistryServiceInstance(ins *ServiceInstance, service *MeshServiceSpec, sidecar *SidecarSpec) string {
	// valid the input
	if rcs.Registried == true {
		// already registried
		return ""
	}

	insPort := ins.Port // the original Java processing listening port
	ins.Port = uint32(sidecar.IngressPort)
	ins.Tenant = service.RegisterTenant

	// registry this instance asynchronously
	go rcs.registry(ins, insPort)

	return ins.InstanceID
}

func (rcs *RegistryCenterServer) registry(ins *ServiceInstance, insPort uint32) {
	var (
		err      error
		tryTimes int = 0
	)
	msg := IngressMsg{
		storeMsg: storeOpMsg{
			op: opTypeCreate,
		},
		instancePort: insPort,
		serviceName:  ins.ServiceName,
	}

	// notify ingress server by chan
	rcs.notifyIngress <- msg

	// level triggered, loop unitl it success
	for {
		tryTimes++

		// check the ingress pipeline and http server object exists
		/*if _, exists := rcs.super.GetRunningObject(genIngressHTTPSvrObjectName(ins.ServiceName), supervisor.CategoryPipeline); exists != true {
			continue
		} else if _, exists := rcs.super.GetRunningObject(genIngressHTTPSvrObjectName(ins.InstanceID), supervisor.CategoryTrafficGate); exists != true {
			continue
		}*/

		// set this instance status up
		ins.Status = SerivceStatusUp
		ins.Leases = time.Now().Unix() + defaultLeasesSeconds
		ins.RegistryTime = time.Now().Unix()

		if err = rcs.registryIntoStore(ins); err != nil {
			logger.Errorf("service %s try to create ingress failed, err %v, times %d", ins.ServiceName, err, tryTimes)
			continue
		}

		rcs.Registried = true
		logger.Debugf("service %s , instanceID %s, regitry succ, try time %d", ins.ServiceName, ins.InstanceID, tryTimes)
		break
	}

	return
}

// decodeByConsulFormat accepts Java Process's registry request in Consul Format
// then transfer it into eashMesh's format.
func (rcs *RegistryCenterServer) decodeByConsulFormat(body []byte) (*ServiceInstance, error) {
	var (
		err error
		reg *consul.AgentServiceRegistration
		ins *ServiceInstance
	)

	dec := json.NewDecoder(bytes.NewReader(body))
	if err = dec.Decode(reg); err != nil {
		return nil, err
	}

	ins.IP = reg.Address
	ins.Port = uint32(reg.Port)
	ins.ServiceName = reg.Name
	if ins.InstanceID, err = common.UUID(); err != nil {
		logger.Errorf("BUG generate uuid failed, %v", err)
	}

	return nil, err
}

// decodeByEurekaFormat accepts Java Process's registry request in Consul Format
// then transfer it into eashMesh's format.
func (rcs *RegistryCenterServer) decodeByEurekaFormat(body []byte) (*ServiceInstance, error) {
	var (
		err       error
		eurekaIns *eureka.InstanceInfo
		ins       *ServiceInstance
	)

	if err = xml.Unmarshal(body, eurekaIns); err != nil {
		return ins, err
	}

	ins.IP = eurekaIns.IpAddr
	ins.Port = uint32(eurekaIns.Port.Port)
	if ins.InstanceID, err = common.UUID(); err != nil {
		logger.Errorf("BUG generate uuid failed, %v", err)
	}

	return nil, err
}

func (rcs *RegistryCenterServer) decodeBody(reqBody []byte) (*ServiceInstance, error) {
	var (
		ins *ServiceInstance
		err error
	)
	if rcs.RegistryType == RegistryTypeEureka {
		if ins, err = rcs.decodeByEurekaFormat(reqBody); err != nil {
			return nil, err
		}
	} else if rcs.RegistryType == RegistryTypeConsul {
		if ins, err = rcs.decodeByConsulFormat(reqBody); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unkonw registry request, %v ", string(reqBody))
	}

	return ins, err

}

// registryIntoStore writes instance record into store.
func (rcs *RegistryCenterServer) registryIntoStore(ins *ServiceInstance) error {
	var err error
	buff, err := yaml.Marshal(ins)
	if err != nil {
		logger.Errorf("marshal registry instance %#v to yaml failed: %v", ins, err)
		return err
	}

	lockID := fmt.Sprint(meshServiceInstanceEtcdLockPrefix, ins.InstanceID)

	lockReleaseFunc := func() {
		if err = rcs.store.ReleaseLock(lockID); err != nil {
			logger.Errorf("release lock ID %s failed err %v", lockID, err)
			err = nil
		}
	}

	if err = rcs.store.AcquireLock(lockID, defaultRegistryExpireSecond); err != nil {
		logger.Errorf("require lock %s failed %v")
		return err
	}

	defer lockReleaseFunc()

	name := fmt.Sprintf(meshServiceInstancePrefix, ins.ServiceName, ins.InstanceID)
	if err = rcs.store.Set(name, string(buff)); err != nil {
		return err
	}

	return err
}
