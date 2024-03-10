/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package registrycenter provides registry center server.
package registrycenter

import (
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

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/jmxtool"
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
		registryType string
		instanceSpec *spec.ServiceInstanceSpec
		service      *service.Service
		informer     informer.Informer
		jmxClient    *jmxtool.AgentClient

		serviceName        string
		registered         bool
		done               chan struct{}
		mutex              sync.RWMutex
		accessableServices atomic.Value
	}

	// ReadyFunc is a function to check Ingress/Egress ready to work
	ReadyFunc func() bool
)

// NewRegistryCenterServer creates an initialized registry center server.
func NewRegistryCenterServer(registryType string, instanceSpec *spec.ServiceInstanceSpec,
	service *service.Service, informer informer.Informer, jmxAgent *jmxtool.AgentClient,
) *Server {
	return &Server{
		registryType: registryType,
		instanceSpec: instanceSpec,
		service:      service,
		informer:     informer,
		jmxClient:    jmxAgent,

		serviceName: instanceSpec.ServiceName,
		done:        make(chan struct{}),
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

	rcs.instanceSpec.Port = uint32(serviceSpec.Sidecar.IngressPort)

	go rcs.register(rcs.instanceSpec, ingressReady, egressReady)

	rcs.informer.OnPartOfServiceSpec(rcs.serviceName, rcs.onUpdateLocalInfo)
	rcs.informer.OnAllTrafficTargetSpecs(rcs.onAllTrafficTargetSpecs)
}

func (rcs *Server) updateAgentType() {
	if rcs.instanceSpec.AgentType == "" {
		rcs.instanceSpec.AgentType = "None"
	}

	if rcs.instanceSpec.AgentType != "None" {
		return
	}

	agentInfo, err := rcs.jmxClient.GetAgentInfo()
	if err != nil {
		logger.Warnf("get agent info failed: %v", err)
		return
	}

	if agentInfo.Type == "" {
		return
	}

	rcs.instanceSpec.AgentType = agentInfo.Type
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

		rcs.updateAgentType()

		inReady, eReady := ingressReady(), egressReady()
		if !inReady || !eReady {
			return fmt.Errorf("ingress ready: %v egress ready: %v", inReady, eReady)
		}

		if originIns := rcs.service.GetServiceInstanceSpec(rcs.instanceSpec.ServiceName,
			rcs.instanceSpec.InstanceID); originIns != nil {
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

	var firstSucceed bool
	ticker := time.NewTicker(5 * time.Second)
	for {
		err := routine()
		if err != nil {
			logger.Errorf("register failed: %v", err)
		} else if !firstSucceed {
			logger.Infof("register instance spec succeed")
			firstSucceed = true
		}

		select {
		case <-rcs.done:
			ticker.Stop()
			return
		case <-ticker.C:
		}
	}
}

func (rcs *Server) decodeByConsulFormat(body []byte) error {
	var (
		err error
		reg consul.AgentServiceRegistration
	)

	err = codectool.UnmarshalJSON(body, &reg)
	if err != nil {
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
		if err = codectool.UnmarshalJSON(body, &eurekaIns); err != nil {
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

	switch rcs.registryType {
	case spec.RegistryTypeEureka:
		err = rcs.decodeByEurekaFormat(contentType, reqBody)
	case spec.RegistryTypeConsul:
		err = rcs.decodeByConsulFormat(reqBody)
	default:
		return fmt.Errorf("BUG: can't recognize registry type: %s req body: %s",
			rcs.registryType, (reqBody))
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

	if serviceName != rcs.instanceSpec.ServiceName || err != nil {
		return fmt.Errorf("invalid register serviceName: %s want: %s, err: %v", serviceName, rcs.serviceName, err)
	}
	return err
}
