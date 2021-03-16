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

	// SerivceStatusOutOfSerivce indicates this service instance can't accept ingress traffic
	SerivceStatusOutOfSerivce = "OUT_OF_SERVICE"
)

type (
	// Server handle all registry about logic
	Server struct {
		// Currently we supports Eureka/Consul
		RegistryType string
		registried   bool
		serviceName  string
		instanceID   string
		tenant       string
		done         chan struct{}

		store storage.Storage
	}

	// ReadyFunc is a function to check Ingress/Egress ready to work
	ReadyFunc func() bool
)

// NewRegistryCenterServer creates a initialized registry center server.
func NewRegistryCenterServer(registryType string, serviceName string, store storage.Storage) *Server {
	return &Server{
		RegistryType: registryType,
		store:        store,
		serviceName:  serviceName,
		registried:   false,
		done:         make(chan struct{}),
	}
}

// Registried checks whether service registry or not.
func (rcs *Server) Registried() bool {
	return rcs.registried
}

// Close closes the registry center.
func (rcs *Server) Close() {
	close(rcs.done)
}

// Registry changes instance port and adds tenant.
// It will asynchronously check ingress/egress ready or not.
func (rcs *Server) Registry(ins *spec.ServiceInstanceSpec, service *spec.Service,
	ingressReady ReadyFunc, egressReady ReadyFunc) (string, error) {
	if rcs.registried == true {
		return "", spec.ErrAlreadyRegistried
	}

	ins.Port = uint32(service.Sidecar.IngressPort)

	go rcs.registry(ins, service, ingressReady, egressReady)

	return ins.InstanceID, nil
}

func (rcs *Server) registry(ins *spec.ServiceInstanceSpec, service *spec.Service,
	ingressReady ReadyFunc, egressReady ReadyFunc) {
	var (
		err      error
		tryTimes uint64 = 0
	)

	for {
		select {
		case <-rcs.done:
			return
		default:
			// level triggered, loop unitl it success
			tryTimes++
			if ingressReady() == false || egressReady() == false {
				continue
			}

			ins.Status = SerivceStatusUp
			ins.RegistryTime = time.Now().Format(time.RFC3339)

			if err = rcs.put(ins); err != nil {
				logger.Errorf("create service:%s ingress failed, err:%v, try times:%d", ins.ServiceName, err, tryTimes)
				continue
			}

			rcs.registried = true
			rcs.instanceID = ins.InstanceID
			rcs.tenant = service.RegisterTenant
			logger.Infof("registry succ, service:%s, instanceID:%s, regitry succ, try times:%d", ins.ServiceName, ins.InstanceID, tryTimes)
			return
		}
	}
}

func (rcs *Server) decodeByConsulFormat(body []byte) (*spec.ServiceInstanceSpec, error) {
	var (
		err error
		reg *consul.AgentServiceRegistration
		ins *spec.ServiceInstanceSpec
	)

	dec := json.NewDecoder(bytes.NewReader(body))
	if err = dec.Decode(reg); err != nil {
		return nil, err
	}

	ins.IP = reg.Address
	ins.Port = uint32(reg.Port)
	ins.ServiceName = rcs.serviceName
	if ins.InstanceID, err = common.UUID(); err != nil {
		logger.Errorf("BUG: generate uuid failed, %v", err)
	}

	return nil, err
}

func (rcs *Server) decodeByEurekaFormat(body []byte) (*spec.ServiceInstanceSpec, error) {
	var (
		err       error
		eurekaIns *eureka.InstanceInfo
		ins       *spec.ServiceInstanceSpec
	)

	if err = xml.Unmarshal(body, eurekaIns); err != nil {
		logger.Errorf("decode eureka body:%s, failed, err:%v", string(body), err)
		return ins, err
	}

	ins.IP = eurekaIns.IpAddr
	ins.Port = uint32(eurekaIns.Port.Port)
	ins.ServiceName = rcs.serviceName
	if ins.InstanceID, err = common.UUID(); err != nil {
		logger.Errorf("BUG: generate uuid failed, err:%v", err)
	}

	return nil, err
}

// DecodeRegistryBody decodes Eureka/Consul registry request body according to the
// registry type in config.
func (rcs *Server) DecodeRegistryBody(reqBody []byte) (*spec.ServiceInstanceSpec, error) {
	var (
		ins *spec.ServiceInstanceSpec
		err error
	)
	switch rcs.RegistryType {
	case RegistryTypeEureka:
		ins, err = rcs.decodeByEurekaFormat(reqBody)
	case RegistryTypeConsul:
		ins, err = rcs.decodeByConsulFormat(reqBody)
	default:
		return nil, fmt.Errorf("BUG: can't recognize registry type:%s, req body:%s", rcs.RegistryType, (reqBody))
	}

	return ins, err
}

func (rcs *Server) put(ins *spec.ServiceInstanceSpec) error {
	var err error
	buff, err := yaml.Marshal(ins)
	if err != nil {
		logger.Errorf("BUG: marshal registry instance:%#v to yaml failed, err:%v", ins, err)
		return err
	}

	name := layout.ServiceInstanceSpecKey(rcs.serviceName, ins.InstanceID)
	if err = rcs.store.Put(name, string(buff)); err != nil {
		logger.Errorf("put service:%s failed, err:%v", ins.ServiceName, err)
		return err
	}
	return err
}
