package registrycenter

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/storage"

	"github.com/ArthurHlt/go-eureka-client/eureka"
	consul "github.com/hashicorp/consul/api"
	"gopkg.in/yaml.v2"
)

const (
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
	// Server handle all registry about logic
	Server struct {
		// Currently we supports Eureka/Consul
		RegistryType string
		registried   bool
		serviceName  string

		store storage.Storage
		// notifyIngress chan IngressMsg
	}
)

// ErrAlreadyRegistried indicateds this instances has already been registried
var ErrAlreadyRegistried = fmt.Errorf("serivce already registrired")

// NewRegistryCenterServer creates a initialized registry center server
func NewRegistryCenterServer(registryType string, serviceName string, store storage.Storage) *Server {
	return &Server{
		RegistryType: registryType,
		store:        store,
		serviceName:  serviceName,
	}
}

// Registried returns whether service registry task done
func (rcs *Server) Registried() bool {
	return rcs.registried
}

// RegistryServiceInstance changes instance port and tenatn and stores
// them, it will asynchronously check ingress ready or not
func (rcs *Server) RegistryServiceInstance(ins *spec.ServiceInstance, service *spec.Service, fn func() bool) (string, error) {
	// valid the input
	if rcs.registried == true {
		// already registried
		return "", ErrAlreadyRegistried
	}

	// change the original Java processing listening port
	// to siecar ingress port
	ins.Port = uint32(service.Sidecar.IngressPort)
	ins.Tenant = service.RegisterTenant

	// registry this instance asynchronously
	go rcs.registry(ins, fn)

	return ins.InstanceID, nil
}

// registry stores serviceInstance record after Ingress successfully create
// its pipeline and HTTPServer
func (rcs *Server) registry(ins *spec.ServiceInstance, fn func() bool) {
	var (
		err      error
		tryTimes int = 0
	)

	// level triggered, loop unitl it success
	for {
		tryTimes++
		// check the ingress pipeline and http server object exists
		if fn() == false {
			continue
		}
		// set this instance status up
		ins.Status = SerivceStatusUp
		ins.Leases = time.Now().Unix() + defaultLeasesSeconds
		ins.RegistryTime = time.Now().Unix()

		if err = rcs.registryIntoStore(ins); err != nil {
			logger.Errorf("service %s try to create ingress failed, err %v, times %d", ins.ServiceName, err, tryTimes)
			continue
		}

		rcs.registried = true
		logger.Debugf("service %s , instanceID %s, regitry succ, try time %d", ins.ServiceName, ins.InstanceID, tryTimes)
		break
	}

	return
}

// decodeByConsulFormat accepts Java Process's registry request in Consul Format
// then transfer it into eashMesh's format.
func (rcs *Server) decodeByConsulFormat(body []byte) (*spec.ServiceInstance, error) {
	var (
		err error
		reg *consul.AgentServiceRegistration
		ins *spec.ServiceInstance
	)

	dec := json.NewDecoder(bytes.NewReader(body))
	if err = dec.Decode(reg); err != nil {
		return nil, err
	}

	ins.IP = reg.Address
	ins.Port = uint32(reg.Port)
	ins.ServiceName = rcs.serviceName
	if ins.InstanceID, err = common.UUID(); err != nil {
		logger.Errorf("BUG generate uuid failed, %v", err)
	}

	return nil, err
}

// decodeByEurekaFormat accepts Java Process's registry request in Consul Format
// then transfer it into eashMesh's format.
func (rcs *Server) decodeByEurekaFormat(body []byte) (*spec.ServiceInstance, error) {
	var (
		err       error
		eurekaIns *eureka.InstanceInfo
		ins       *spec.ServiceInstance
	)

	if err = xml.Unmarshal(body, eurekaIns); err != nil {
		return ins, err
	}

	ins.IP = eurekaIns.IpAddr
	ins.Port = uint32(eurekaIns.Port.Port)
	ins.ServiceName = rcs.serviceName
	if ins.InstanceID, err = common.UUID(); err != nil {
		logger.Errorf("BUG generate uuid failed, %v", err)
	}

	return nil, err
}

// DecodeRegistryBody decodes Eureka/Consul registry request body according to the
// registry type in config
func (rcs *Server) DecodeRegistryBody(reqBody []byte) (*spec.ServiceInstance, error) {
	var (
		ins *spec.ServiceInstance
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
func (rcs *Server) registryIntoStore(ins *spec.ServiceInstance) error {
	var err error
	buff, err := yaml.Marshal(ins)
	if err != nil {
		logger.Errorf("marshal registry instance %#v to yaml failed: %v", ins, err)
		return err
	}

	logger.Errorf("buff is %s", string(buff))

	name := layout.GenServiceInstanceKey(rcs.serviceName, ins.InstanceID)
	if err = rcs.store.Put(name, string(buff)); err != nil {
		return err
	}
	return err
}
