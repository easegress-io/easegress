package registrycenter

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/service"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"

	"github.com/ArthurHlt/go-eureka-client/eureka"
	consul "github.com/hashicorp/consul/api"
)

const (
	// RegistryTypeEureka indicates a Eureka registry center
	RegistryTypeEureka = "eureka"
	// RegistryTypeConsul indicates a Consul registry center
	RegistryTypeConsul = "consul"
)

type (
	// Server handle all registry about logic
	Server struct {
		// Currently we supports Eureka/Consul
		RegistryType string
		registered   bool
		serviceName  string
		instanceID   string
		IP           string
		port         int
		tenant       string
		done         chan struct{}
		mutex        sync.RWMutex

		service *service.Service
	}

	// ReadyFunc is a function to check Ingress/Egress ready to work
	ReadyFunc func() bool
)

// NewRegistryCenterServer creates a initialized registry center server.
func NewRegistryCenterServer(registryType string, serviceName string, IP string, port int, instanceID string,
	service *service.Service) *Server {
	return &Server{
		RegistryType: registryType,
		serviceName:  serviceName,
		service:      service,
		registered:   false,
		mutex:        sync.RWMutex{},
		port:         port,
		IP:           IP,
		instanceID:   instanceID,

		done: make(chan struct{}),
	}
}

// Registered checks whether service registry or not.
func (rcs *Server) Registered() bool {
	rcs.mutex.RLock()
	defer rcs.mutex.RUnlock()
	return rcs.registered
}

// Close closes the registry center.
func (rcs *Server) Close() {
	close(rcs.done)
}

// Register registers itself into mesh
func (rcs *Server) Register(serviceSpec *spec.Service, ingressReady ReadyFunc, egressReady ReadyFunc) {
	rcs.tenant = serviceSpec.RegisterTenant
	if rcs.Registered() == true {
		return
	}

	ins := &spec.ServiceInstanceSpec{
		ServiceName: rcs.serviceName,
		InstanceID:  rcs.instanceID,
		IP:          rcs.IP,
		Port:        uint32(serviceSpec.Sidecar.IngressPort),
	}

	go rcs.register(ins, ingressReady, egressReady)

	return
}

func (rcs *Server) register(ins *spec.ServiceInstanceSpec, ingressReady ReadyFunc, egressReady ReadyFunc) {
	var tryTimes int = 0

	for {
		select {
		case <-rcs.done:
			return
		default:
			rcs.mutex.Lock()
			if rcs.registered == true {
				rcs.mutex.Unlock()
				return
			}
			// wrapper for the recover
			routine := func() {
				defer func() {
					if err := recover(); err != nil {
						logger.Errorf("registry center recover from: %v, stack trace:\n%s\n",
							err, debug.Stack())
					}
				}()
				// level triggered, loop unitl it success
				tryTimes++
				if ingressReady() == false || egressReady() == false {
					logger.Infof("ingress or egress not ready")
					return
				}

				// alreading been registered
				if ins := rcs.service.GetServiceInstanceSpec(rcs.serviceName, rcs.instanceID); ins != nil {
					rcs.registered = true
					return
				}

				ins.Status = spec.SerivceStatusUp
				ins.RegistryTime = time.Now().Format(time.RFC3339)
				rcs.registered = true
				rcs.service.PutServiceInstanceSpec(ins)
				logger.Infof("registry succ, service:%s, instanceID:%s, regitry succ, try times:%d", ins.ServiceName, ins.InstanceID, tryTimes)
			}

			routine()
			rcs.mutex.Unlock()
		}
	}
}

func (rcs *Server) decodeByConsulFormat(body []byte) error {
	var (
		err error
		reg consul.AgentServiceRegistration
	)

	dec := json.NewDecoder(bytes.NewReader(body))
	if err = dec.Decode(&reg); err != nil {
		return err
	}

	logger.Infof("decode consul body succ, body:%s", string(body))
	return err
}

func (rcs *Server) decodeByEurekaFormat(contentType string, body []byte) error {
	var (
		err       error
		eurekaIns eureka.InstanceInfo
	)

	switch contentType {
	case "application/json":
		dec := json.NewDecoder(bytes.NewReader(body))
		if err = dec.Decode(&eurekaIns); err != nil {
			logger.Errorf("decode eureka contentType:%s body:%s, failed, err:%v", contentType, string(body), err)
			return err
		}
	default:
		if err = xml.Unmarshal([]byte(body), &eurekaIns); err != nil {
			logger.Errorf("decode eureka contentType:%s body:%s, failed, err:%v", contentType, string(body), err)
			return err
		}
	}
	logger.Infof("decode eureka body succ, contentType:%s body:%s", contentType, string(body))

	return err
}

// DecodeRegistryBody decodes Eureka/Consul registry request body according to the
// registry type in config.
func (rcs *Server) DecodeRegistryBody(contentType string, reqBody []byte) error {
	var err error

	switch rcs.RegistryType {
	case RegistryTypeEureka:
		err = rcs.decodeByEurekaFormat(contentType, reqBody)
	case RegistryTypeConsul:
		err = rcs.decodeByConsulFormat(reqBody)
	default:
		return fmt.Errorf("BUG: can't recognize registry type:%s, req body:%s", rcs.RegistryType, (reqBody))
	}

	return err
}
