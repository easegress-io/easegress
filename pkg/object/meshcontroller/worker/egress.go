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

package worker

import (
	"fmt"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/supervisor"
	"gopkg.in/yaml.v2"
)

const egressRPCKey = "X-Mesh-Rpc-Service"

type (
	// EgressServer manages one/many ingress pipelines and one HTTPServer
	EgressServer struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec

		pipelines  map[string]*supervisor.ObjectEntity
		httpServer *supervisor.ObjectEntity

		tc        *trafficcontroller.TrafficController
		namespace string
		inf       informer.Informer
		instaceID string

		serviceName      string
		egressServerName string
		service          *service.Service
		mutex            sync.RWMutex
	}

	httpServerSpecBuilder struct {
		Kind            string `yaml:"kind"`
		Name            string `yaml:"name"`
		httpserver.Spec `yaml:",inline"`
		Cert            *spec.Certificate `yaml:"-"`
	}
)

// NewEgressServer creates an initialized egress server
func NewEgressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor,
	serviceName, instanceID string, service *service.Service, inf informer.Informer) *EgressServer {

	entity, exists := super.GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}

	return &EgressServer{
		super:     super,
		superSpec: superSpec,

		inf:         inf,
		tc:          tc,
		namespace:   fmt.Sprintf("%s/%s", superSpec.Name(), "egress"),
		pipelines:   make(map[string]*supervisor.ObjectEntity),
		serviceName: serviceName,
		service:     service,
		instaceID:   instanceID,
	}
}

func newHTTPServerSpecBuilder(httpServerName string, spec *httpserver.Spec) *httpServerSpecBuilder {
	return &httpServerSpecBuilder{
		Kind: httpserver.Kind,
		Name: httpServerName,
		Spec: *spec,
	}
}

func (b *httpServerSpecBuilder) yamlConfig() string {
	buff, err := yaml.Marshal(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", b, err)
	}
	return string(buff)
}

// InitEgress initializes the Egress HTTPServer.
func (egs *EgressServer) InitEgress(service *spec.Service) error {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	if egs.httpServer != nil {
		return nil
	}

	egs.egressServerName = service.EgressHTTPServerName()
	superSpec, err := service.SideCarEgressHTTPServerSpec()
	if err != nil {
		return err
	}

	entity, err := egs.tc.CreateHTTPServerForSpec(egs.namespace, superSpec)
	if err != nil {
		return fmt.Errorf("create http server %s failed: %v", superSpec.Name(), err)
	}
	egs.httpServer = entity

	if err := egs.inf.OnAllServiceSpecs(egs.reloadBySpecs); err != nil {
		// only return err when its type is not `AlreadyWatched`
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	if err := egs.inf.OnAllServiceInstanceSpecs(egs.reloadByInstances); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	if err := egs.inf.OnServertCert(egs.serviceName, egs.instaceID, egs.reloadByCert); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}
	return nil
}

// Ready checks Egress HTTPServer has been created or not.
// Not need to check pipelines, cause they will be dynamically added.
func (egs *EgressServer) Ready() bool {
	egs.mutex.RLock()
	defer egs.mutex.RUnlock()
	return egs._ready()
}

func (egs *EgressServer) _ready() bool {
	return egs.httpServer != nil
}

func (egs *EgressServer) getCerts() map[string]*spec.Certificate {
	cert := egs.service.GetServiceInstanceCert(egs.serviceName, egs.instaceID)
	certs := make(map[string]*spec.Certificate)
	certs[egs.serviceName] = cert
	return certs
}

func (egs *EgressServer) reloadByCert(event informer.Event, value *spec.Certificate) bool {
	specs := egs.service.ListServiceSpecs()
	mSpecs := make(map[string]*spec.Service)
	for _, v := range specs {
		mSpecs[v.Name] = v
	}

	egs.reloadHTTPServer(mSpecs)
	return false
}

func (egs *EgressServer) reloadByInstances(value map[string]*spec.ServiceInstanceSpec) bool {
	specs := make(map[string]*spec.Service)
	for _, v := range value {
		if _, exist := specs[v.ServiceName]; !exist {
			spec := egs.service.GetServiceSpec(v.ServiceName)
			specs[v.ServiceName] = spec
		}
	}

	return egs.reloadHTTPServer(specs)
}

func (egs *EgressServer) reloadBySpecs(value map[string]*spec.Service) bool {
	return egs.reloadHTTPServer(value)
}

// regex rule: ^(\w+\.)*vet-services\.(\w+)\.svc\..+$
//  can match e.g. _tcp.vet-services.easemesh.svc.cluster.local
//   		   vet-services.easemesh.svc.cluster.local
//   		   _zip._tcp.vet-services.easemesh.svc.com
func (egs *EgressServer) buildHostRegex(serviceName string) string {
	return `^(\w+\.)*` + serviceName + `\.(\w+)\.svc\..+`
}

func (egs *EgressServer) reloadHTTPServer(specs map[string]*spec.Service) bool {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	admSpec := egs.superSpec.ObjectSpec().(*spec.Admin)
	var cert, rootCert *spec.Certificate
	if admSpec.EnablemTLS() {
		cert = egs.service.GetServiceInstanceCert(egs.serviceName, egs.instaceID)
		rootCert = egs.service.GetRootCert()
	}

	pipelines := make(map[string]*supervisor.ObjectEntity)
	serverName2PipelineName := make(map[string]string)

	for _, v := range specs {
		instances := egs.service.ListServiceInstanceSpecs(v.Name)
		pipelineSpec, err := v.SideCarEgressPipelineSpec(instances, cert, rootCert)
		if err != nil {
			logger.Errorf("BUG: gen sidecar egress httpserver spec failed: %v", err)
			continue
		}
		entity, err := egs.tc.CreateHTTPPipelineForSpec(egs.namespace, pipelineSpec)
		if err != nil {
			logger.Errorf("update http pipeline failed: %v", err)
			continue
		}
		pipelines[v.Name] = entity
		serverName2PipelineName[v.Name] = pipelineSpec.Name()
	}

	httpServerSpec := egs.httpServer.Spec().ObjectSpec().(*httpserver.Spec)
	httpServerSpec.Rules = nil

	for serviceName := range pipelines {
		rule := &httpserver.Rule{
			Paths: []*httpserver.Path{
				{
					PathPrefix: "/",
					Headers: []*httpserver.Header{
						{
							Key: egressRPCKey,
							// Value should be the service name
							Values:  []string{serviceName},
							Backend: serverName2PipelineName[serviceName],
						},
					},
					// this name should be the pipeline full name
					Backend: serverName2PipelineName[serviceName],
				},
			},
		}

		// for matching only host name request
		//   1) try exactly matching
		//   2) try matching with regexp
		ruleHost := &httpserver.Rule{
			Host:       serviceName,
			HostRegexp: egs.buildHostRegex(serviceName),
			Paths: []*httpserver.Path{
				{
					PathPrefix: "/",
					// this name should be the pipeline full name
					Backend: serverName2PipelineName[serviceName],
				},
			},
		}

		httpServerSpec.Rules = append(httpServerSpec.Rules, rule, ruleHost)
	}

	builder := newHTTPServerSpecBuilder(egs.egressServerName, httpServerSpec)
	superSpec, err := supervisor.NewSpec(builder.yamlConfig())
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", err)
		return true
	}
	entity, err := egs.tc.UpdateHTTPServerForSpec(egs.namespace, superSpec)
	if err != nil {
		logger.Errorf("update http server %s failed: %v", egs.egressServerName, err)
		return true
	}

	// update local storage
	egs.pipelines = pipelines
	egs.httpServer = entity

	return true
}

// Close closes the Egress HTTPServer and Pipelines
func (egs *EgressServer) Close() {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	if egs._ready() {
		egs.tc.DeleteHTTPServer(egs.namespace, egs.httpServer.Spec().Name())
		for _, entity := range egs.pipelines {
			egs.tc.DeleteHTTPPipeline(egs.namespace, entity.Spec().Name())
		}
	}
}
