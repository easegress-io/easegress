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
	"net/http"
	"sync"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
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

		serviceName string
		service     *service.Service
		mutex       sync.RWMutex
		watch       chan<- string
	}
)

// NewEgressServer creates a initialized egress server
func NewEgressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor,
	serviceName string, service *service.Service, watch chan<- string) *EgressServer {

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

		tc:          tc,
		namespace:   fmt.Sprintf("%s/%s", superSpec.Name(), "egress"),
		pipelines:   make(map[string]*supervisor.ObjectEntity),
		serviceName: serviceName,
		service:     service,
		watch:       watch,
	}
}

// Get gets egressServer itself as the default backend.
// egress server will handle the pipeline routing by itself.
func (egs *EgressServer) GetHandler(name string) (protocol.HTTPHandler, bool) {
	egs.mutex.RLock()
	defer egs.mutex.RUnlock()
	return egs, true
}

// CreateEgress creates a default Egress HTTPServer.
func (egs *EgressServer) CreateEgress(service *spec.Service) error {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	if egs.httpServer != nil {
		return nil
	}

	superSpec, err := service.SideCarEgressHTTPServerSpec()
	if err != nil {
		return err
	}

	entity, err := egs.tc.CreateHTTPServerForSpec(egs.namespace, superSpec)
	if err != nil {
		return fmt.Errorf("create http server %s failed: %v", superSpec.Name(), err)
	}
	egs.httpServer = entity

	return nil
}

// Ready checks Egress HTTPServer has been created or not.
// Not need to check pipelines, cause they will be dynamically added.
func (egs *EgressServer) Ready() bool {
	egs.mutex.RLock()
	defer egs.mutex.RUnlock()
	return egs.httpServer != nil
}

func (egs *EgressServer) addPipeline(serviceName string) (*supervisor.ObjectEntity, error) {
	service := egs.service.GetServiceSpec(serviceName)
	if service == nil {
		return nil, spec.ErrServiceNotFound
	}

	instanceSpec := egs.service.ListServiceInstanceSpecs(serviceName)
	if len(instanceSpec) == 0 {
		logger.Errorf("found service: %s with empty instances", serviceName)
		return nil, spec.ErrServiceNotavailable
	}

	superSpec, err := service.SideCarEgressPipelineSpec(instanceSpec)
	if err != nil {
		return nil, err
	}
	logger.Infof("add pipeline spec: %s", superSpec.YAMLConfig())

	entity, err := egs.tc.CreateHTTPPipelineForSpec(egs.namespace, superSpec)
	if err != nil {
		return nil, fmt.Errorf("create http pipeline %s failed: %v", superSpec.Name(), err)
	}
	egs.pipelines[service.Name] = entity

	return entity, nil
}

// DeletePipeline deletes one Egress pipeline according to the serviceName.
func (egs *EgressServer) DeletePipeline(serviceName string) {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	if pipeline, exists := egs.pipelines[serviceName]; exists {
		egs.tc.DeleteHTTPPipeline(egs.namespace, pipeline.Spec().Name())
		delete(egs.pipelines, serviceName)
	}
}

// UpdatePipeline updates a local pipeline according to the informer.
func (egs *EgressServer) UpdatePipeline(service *spec.Service, instanceSpec []*spec.ServiceInstanceSpec) error {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	superSpec, err := service.SideCarEgressPipelineSpec(instanceSpec)
	if err != nil {
		return err
	}

	entity, err := egs.tc.UpdateHTTPPipelineForSpec(egs.namespace, superSpec)
	if err != nil {
		return fmt.Errorf("update http pipeline %s failed: %v", superSpec.Name(), err)
	}

	egs.pipelines[service.Name] = entity

	return nil
}

func (egs *EgressServer) getPipeline(serviceName string) (*supervisor.ObjectEntity, error) {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	if pipeline, ok := egs.pipelines[serviceName]; ok {
		egs.watch <- serviceName
		return pipeline, nil
	}

	pipeline, err := egs.addPipeline(serviceName)
	if err == nil {
		egs.watch <- serviceName
	}
	return pipeline, err
}

// Handle handles all egress traffic and route to desired pipeline according
// to the "X-MESH-RPC-SERVICE" field in header.
func (egs *EgressServer) Handle(ctx context.HTTPContext) {
	serviceName := ctx.Request().Header().Get(egressRPCKey)

	if len(serviceName) == 0 {
		logger.Errorf("handle egress RPC without setting service name in: %s header: %#v",
			egressRPCKey, ctx.Request().Header())
		ctx.Response().SetStatusCode(http.StatusNotFound)
		return
	}

	pipeline, err := egs.getPipeline(serviceName)
	if err != nil {
		if err == spec.ErrServiceNotFound {
			logger.Errorf("handle egress RPC unknown service: %s", serviceName)
			ctx.Response().SetStatusCode(http.StatusNotFound)
		} else {
			logger.Errorf("handle egress RPC service: %s get pipeline failed: %v", serviceName, err)
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
		}
		return
	}
	pipeline.Instance().(*httppipeline.HTTPPipeline).Handle(ctx)
	logger.Infof("hanlde service name:%s finished, status code: %d", serviceName, ctx.Response().StatusCode())
}

// Close closes the Egress HTTPServer and Pipelines
func (egs *EgressServer) Close() {
	egs.mutex.Lock()
	defer egs.mutex.Unlock()

	egs.tc.DeleteHTTPServer(egs.namespace, egs.httpServer.Spec().Name())
	for _, entity := range egs.pipelines {
		egs.tc.DeleteHTTPPipeline(egs.namespace, entity.Spec().Name())
	}
}
