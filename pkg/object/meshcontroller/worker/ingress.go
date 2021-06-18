/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://wwwrk.apache.org/licenses/LICENSE-2.0
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
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/supervisor"
)

// ErrIngressClosed is the error when operating in a closed Ingress server
var ErrIngressClosed = fmt.Errorf("ingress has been closed")

type (
	// IngressServer manages one ingress pipeline and one HTTPServer
	IngressServer struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec

		serviceName string

		// port is the Java business process's listening port
		// not the ingress HTTPServer's port
		// NOTE: Not used for nowrk.
		// port  uint32
		mutex sync.RWMutex

		tc        *trafficcontroller.TrafficController
		namespace string
		// running EG objects, accept other service instances' traffic
		// in mesh and hand over to local Java business process
		pipelines  map[string]*supervisor.ObjectEntity
		httpServer *supervisor.ObjectEntity
	}
)

// NewIngressServer creates a initialized ingress server
func NewIngressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor, serviceName string) *IngressServer {
	entity, exists := super.GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}

	return &IngressServer{
		super: super,

		tc:        tc,
		namespace: fmt.Sprintf("%s/%s", superSpec.Name(), "ingress"),

		pipelines:   make(map[string]*supervisor.ObjectEntity),
		httpServer:  nil,
		serviceName: serviceName,
		mutex:       sync.RWMutex{},
	}
}

// Ready checks ingress's pipeline and HTTPServer are created or not
func (ings *IngressServer) Ready() bool {
	ings.mutex.RLock()
	defer ings.mutex.RUnlock()

	serviceSpec := &spec.Service{
		Name: ings.serviceName,
	}

	_, pipelineReady := ings.pipelines[serviceSpec.IngressPipelineName()]

	return pipelineReady && (ings.httpServer != nil)
}

// CreateIngress creates local default pipeline and httpServer for ingress
func (ings *IngressServer) CreateIngress(service *spec.Service, port uint32) error {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if _, ok := ings.pipelines[service.IngressPipelineName()]; !ok {
		superSpec, err := service.SideCarIngressPipelineSpec(port)
		if err != nil {
			return err
		}
		entity, err := ings.tc.CreateHTTPPipelineForSpec(ings.namespace, superSpec)
		if err != nil {
			return fmt.Errorf("create http pipeline %s failed: %v", superSpec.Name(), err)
		}
		ings.pipelines[service.IngressPipelineName()] = entity
	}

	if ings.httpServer == nil {
		superSpec, err := service.SideCarIngressHTTPServerSpec()
		if err != nil {
			return err
		}

		entity, err := ings.tc.CreateHTTPServerForSpec(ings.namespace, superSpec)
		if err != nil {
			return fmt.Errorf("create http server %s failed: %v", superSpec.Name(), err)
		}
		ings.httpServer = entity
	}

	return nil
}

// UpdatePipeline accepts new pipeline specs, and uses it to update
// ingress's HTTPPipeline with inheritance
func (ings *IngressServer) UpdatePipeline(newSpec string) error {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	service := &spec.Service{
		Name: ings.serviceName,
	}

	superSpec, err := supervisor.NewSpec(newSpec)
	if err != nil {
		logger.Errorf("BUG: update ingress pipeline spec: %s new super spec failed: %v", newSpec, err)
		return err
	}

	entity, err := ings.tc.UpdateHTTPPipelineForSpec(ings.namespace, superSpec)
	if err != nil {
		return fmt.Errorf("update http pipeline %s failed: %v", superSpec.Name(), err)
	}

	ings.pipelines[service.IngressPipelineName()] = entity

	return err
}

// Close closes the Ingress HTTPServer and Pipeline
func (ings *IngressServer) Close() {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.tc.DeleteHTTPServer(ings.namespace, ings.httpServer.Spec().Name())
	for _, entity := range ings.pipelines {
		ings.tc.DeleteHTTPPipeline(ings.namespace, entity.Spec().Name())
	}
}
