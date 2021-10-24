/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package registrycenter

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ArthurHlt/go-eureka-client/eureka"
	"github.com/go-chi/chi/v5"
	consul "github.com/hashicorp/consul/api"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
)

const (
	// ContentTypeXML is xml content type
	ContentTypeXML = "text/xml"
	// ContentTypeJSON is JSON content type
	ContentTypeJSON = "application/json"
)

type (
	// Server handle all registry about logic
	Server struct {
		// Currently we support Eureka/Consul
		RegistryType string
		registered   bool

		registryName  string
		serviceName   string
		instanceID    string
		IP            string
		port          int
		serviceLabels map[string]string

		done               chan struct{}
		mutex              sync.RWMutex
		accessableServices atomic.Value

		service  *service.Service
		informer informer.Informer
	}

	// ReadyFunc is a function to check Ingress/Egress ready to work
	ReadyFunc func() bool
)

// NewRegistryCenterServer creates an initialized registry center server.
func NewRegistryCenterServer(registryType string, registryName, serviceName string, IP string, port int, instanceID string,
	serviceLabels map[string]string, service *service.Service, informer informer.Informer) *Server {
	return &Server{
		RegistryType:  registryType,
		registryName:  registryName,
		serviceName:   serviceName,
		service:       service,
		registered:    false,
		mutex:         sync.RWMutex{},
		port:          port,
		IP:            IP,
		instanceID:    instanceID,
		serviceLabels: serviceLabels,
		informer:      informer,

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
	if rcs.Registered() {
		return
	}

	ins := &spec.ServiceInstanceSpec{
		RegistryName: rcs.registryName,
		ServiceName:  rcs.serviceName,
		InstanceID:   rcs.instanceID,
		IP:           rcs.IP,
		Port:         uint32(serviceSpec.Sidecar.IngressPort),
		Labels:       rcs.serviceLabels,
	}

	go rcs.register(ins, ingressReady, egressReady)

	rcs.informer.OnPartOfServiceSpec(rcs.serviceName, "", rcs.onUpdateLocalInfo)
	rcs.informer.OnAllTrafficTargetSpecs(rcs.onAllTrafficTargetSpecs)
}

func (rcs *Server) onAllTrafficTargetSpecs(tts map[string]*spec.TrafficTarget) bool {
	svcs := map[string]bool{}

	for _, tt := range tts {
		for _, src := range tt.Sources {
			if src.Name == rcs.serviceName {
				svcs[tt.Destination.Name] = true
				break
			}
		}
	}

	rcs.accessableServices.Store(svcs)
	return true
}

func (rcs *Server) onUpdateLocalInfo(event informer.Event, serviceSpec *spec.Service) bool {
	switch event.EventType {
	case informer.EventDelete:
		return false
	case informer.EventUpdate:
	}

	return true
}

func needUpdateRecord(originIns, ins *spec.ServiceInstanceSpec) bool {
	if originIns == nil {
		return true
	}

	if originIns.IP != ins.IP || originIns.Port != ins.Port {
		return true
	}

	return false
}

func (rcs *Server) register(ins *spec.ServiceInstanceSpec, ingressReady ReadyFunc, egressReady ReadyFunc) {
	routine := func() (err error) {
		defer func() {
			if err1 := recover(); err1 != nil {
				logger.Errorf("registry center recover from: %v, stack trace:\n%s\n",
					err, debug.Stack())
				err = fmt.Errorf("%v", err1)
			}
		}()

		inReady, eReady := ingressReady(), egressReady()
		if !inReady || !eReady {
			return fmt.Errorf("ingress ready: %v egress ready: %v", inReady, eReady)
		}

		if originIns := rcs.service.GetServiceInstanceSpec(rcs.serviceName, rcs.instanceID); originIns != nil {
			if !needUpdateRecord(originIns, ins) {
				rcs.mutex.Lock()
				rcs.registered = true
				rcs.mutex.Unlock()
				return nil
			}
		}

		ins.Status = spec.ServiceStatusUp
		ins.RegistryTime = time.Now().Format(time.RFC3339)
		rcs.service.PutServiceInstanceSpec(ins)

		rcs.mutex.Lock()
		rcs.registered = true
		rcs.mutex.Unlock()

		return nil
	}

	var tryTimes int
	var firstSucceed bool
	for {
		select {
		case <-rcs.done:
			return
		default:
			tryTimes++

			err := routine()
			if err != nil {
				logger.Errorf("register failed: %v", err)
				time.Sleep(5 * time.Second)
			} else {
				if !firstSucceed {
					logger.Infof("register instance spec succeed")
					firstSucceed = true
				}
			}
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

	logger.Infof("decode consul body SUCC body: %s", string(body))
	return err
}

func (rcs *Server) decodeByEurekaFormat(contentType string, body []byte) error {
	var (
		err       error
		eurekaIns eureka.InstanceInfo
	)

	switch contentType {
	case ContentTypeJSON:
		dec := json.NewDecoder(bytes.NewReader(body))
		if err = dec.Decode(&eurekaIns); err != nil {
			logger.Errorf("decode eureka contentType: %s body: %s failed: %v", contentType, string(body), err)
			return err
		}
	default:
		if err = xml.Unmarshal(body, &eurekaIns); err != nil {
			logger.Errorf("decode eureka contentType: %s body: %s failed: %v", contentType, string(body), err)
			return err
		}
	}
	logger.Infof("decode eureka body SUCC contentType: %s body: %s", contentType, string(body))

	return err
}

// CheckRegistryBody tries to decode Eureka/Consul register request body according to the
// registry type.
func (rcs *Server) CheckRegistryBody(contentType string, reqBody []byte) error {
	var err error

	switch rcs.RegistryType {
	case spec.RegistryTypeEureka:
		err = rcs.decodeByEurekaFormat(contentType, reqBody)
	case spec.RegistryTypeConsul:
		err = rcs.decodeByConsulFormat(reqBody)
	default:
		return fmt.Errorf("BUG: can't recognize registry type: %s req body: %s", rcs.RegistryType, (reqBody))
	}

	return err
}

// CheckRegistryURL tries to decode Nacos register request URL parameters.
func (rcs *Server) CheckRegistryURL(w http.ResponseWriter, r *http.Request) error {
	var err error
	ip := chi.URLParam(r, "ip")
	port := chi.URLParam(r, "port")
	serviceName := chi.URLParam(r, "serviceName")

	if len(ip) == 0 || len(port) == 0 || len(serviceName) == 0 {
		return fmt.Errorf("invalid register parameters, ip: %s, port: %s, serviceName: %s",
			ip, port, serviceName)
	}

	serviceName, err = rcs.SplitNacosServiceName(serviceName)

	if serviceName != rcs.serviceName || err != nil {
		return fmt.Errorf("invalid register serviceName: %s want: %s, err: %v", serviceName, rcs.serviceName, err)
	}
	return err
}
