package meshcontroller

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"

	"github.com/ArthurHlt/go-eureka-client/eureka"
	consul "github.com/hashicorp/consul/api"
	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"gopkg.in/yaml.v2"
)

const (
	defaultRegistryExpireSecond = 2 * 60 // default registry routine expire time, 2 minutes

	// RegistryTypeEureka indicates a Eureka registry center
	RegistryTypeEureka = "eureka"
	// RegistryTypeConsul indicates a Consul registry center
	RegistryTypeConsul = "consul"
)

type (
	// RegistryCenter provids registry implement constract for different protocols,e.g. Consul, Eureka
	RegistryCenter interface {
		// Registry accepts RESTful POST/PUT body and transform them into
		Registry(body []byte) error
	}

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

		store      MeshStorage
		ingressSvc *IngressServer
	}
)

// RegistryServiceInstance accepts Java Process's registry request in Eureka/Consul Format
// cause Eureka/Consul registry format are both with HTTP and POST body, we can combine them
// into one routine.
// Todo: Consider to split this routien for other None RESTful POST body
//       format in future.
func (rcs *RegistryCenterServer) RegistryServiceInstance(reqBody []byte) error {
	var err error

	// valid the input
	if rcs.Registried == true {
		// already registried
		return nil
	}

	var ins *ServiceInstance
	if ins, err = rcs.decodeByEurekaFormat(reqBody); err == nil {
		// registry from eureke
	} else if ins, err = rcs.decodeByConsulFormat(reqBody); err == nil {
		// registry from consul
	} else {
		return fmt.Errorf("unkonw registry request, %v ", string(reqBody))
	}

	var service *MeshServiceSpec
	var sidecar *SidecarSpec
	// Get mesh service and sidecar spec

	// Modify to accept traffic from sidecar
	insPort := ins.Port
	ins.Port = uint32(sidecar.IngressPort)
	ins.Tenant = service.RegisterTenant

	go rcs.registry(ins, insPort)

	return err
}

func (rcs *RegistryCenterServer) registry(ins *ServiceInstance, insPort uint32) {
	var (
		err      error
		tryTimes int = 0
	)

	// level triggered, loop unitl it success
	for {
		tryTimes++

		if err = rcs.ingressSvc.createIngress(ins.ServiceName, ins.InstanceID, insPort); err != nil {
			logger.Errorf("service %s try to create ingress failed, err %v, times %d", ins.ServiceName, err, tryTimes)
			continue
		}

		if err = rcs.registryIntoEtcd(ins); err != nil {
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
// then transfer it into eashMesh's format
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

	return nil, err
}

// decodeByEurekaFormat accepts Java Process's registry request in Consul Format
// then transfer it into eashMesh's format
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
		logger.Errorf("[BUG] generate uuid failed, %v", err)
	}

	return nil, err
}

// registryIntoEtcd writes instance record into etcd
func (rcs *RegistryCenterServer) registryIntoEtcd(ins *ServiceInstance) error {
	var err error
	buff, err := yaml.Marshal(ins)
	if err != nil {
		logger.Errorf("marshal registry instance %#v to yaml failed: %v", ins, err)
		return err
	}

	lockID := fmt.Sprint(meshServiceInstanceEtcdLockPrefix, ins.InstanceID)
	if err = rcs.store.AcquireLock(lockID, defaultRegistryExpireSecond); err != nil {
		logger.Errorf("require lock %s failed %v")
		return err
	}
	name := fmt.Sprintf(meshServiceInstancePrefix, ins.ServiceName, ins.InstanceID)
	if err = rcs.store.Set(name, string(buff)); err != nil {
		return err
	}

	if err = rcs.store.ReleaseLock(lockID); err != nil {
		logger.Errorf("release lock ID %s failed err %v", lockID, err)
		// reset err to nil to ignore the lock relese problem, we have done the desired resgistry task
		err = nil
	}

	return err
}
