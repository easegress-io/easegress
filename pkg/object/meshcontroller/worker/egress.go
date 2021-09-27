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

		serviceName      string
		egressServerName string
		service          *service.Service
		mutex            sync.RWMutex
	}

	httpServerSpecBuilder struct {
		Kind            string `yaml:"kind"`
		Name            string `yaml:"name"`
		httpserver.Spec `yaml:",inline"`
	}
)

// NewEgressServer creates an initialized egress server
func NewEgressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor,
	serviceName string, service *service.Service, inf informer.Informer) *EgressServer {

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
			logger.Errorf("add ingress spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	if err := egs.inf.OnAllServiceInstanceSpecs(egs.reloadByInstances); err != nil {
		// only return err when its type is not `AlreadyWatched`
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add ingress spec watching service: %s failed: %v", service.Name, err)
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

func (egs *EgressServer) reloadHTTPServer(specs map[string]*spec.Service) bool {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	pipelines := make(map[string]*supervisor.ObjectEntity)
	serverName2PipelineName := make(map[string]string)

	for _, v := range specs {
		instances := egs.service.ListServiceInstanceSpecs(v.Name)
		pipelineSpec, err := v.SideCarEgressPipelineSpec(instances)
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

	for k := range pipelines {
		rule := &httpserver.Rule{
			Paths: []*httpserver.Path{
				{
					PathPrefix: "/",
					Headers: []*httpserver.Header{
						{
							Key: egressRPCKey,
							// Value should be the service name
							Values:  []string{k},
							Backend: serverName2PipelineName[k],
						},
					},
					// this name should be the pipeline full name
					Backend: serverName2PipelineName[k],
				},
			},
		}

		httpServerSpec.Rules = append(httpServerSpec.Rules, rule)
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
