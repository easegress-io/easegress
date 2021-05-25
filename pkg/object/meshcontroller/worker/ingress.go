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

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/object/httpserver"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/protocol"
	"github.com/megaease/easegateway/pkg/supervisor"
)

// ErrIngressClosed is the error when operating in a closed Ingress server
var ErrIngressClosed = fmt.Errorf("ingress has been closed")

type (
	// IngressServer manages one ingress pipeline and one HTTPServer
	IngressServer struct {
		super       *supervisor.Supervisor
		serviceName string

		// port is the Java business process's listening port
		// not the ingress HTTPServer's port
		// NOTE: Not used for now.
		// port  uint32
		mutex sync.RWMutex

		// running EG objects, accept other service instances' traffic
		// in mesh and hand over to local Java business process
		pipelines  map[string]*httppipeline.HTTPPipeline
		httpServer *httpserver.HTTPServer
	}
)

// NewIngressServer creates a initialized ingress server
func NewIngressServer(super *supervisor.Supervisor, serviceName string) *IngressServer {
	return &IngressServer{
		super:       super,
		pipelines:   make(map[string]*httppipeline.HTTPPipeline),
		httpServer:  nil,
		serviceName: serviceName,
		mutex:       sync.RWMutex{},
	}
}

// Get gets pipeline object for httpServer, it implements httpServer's MuxMapper interface
func (ings *IngressServer) Get(name string) (protocol.HTTPHandler, bool) {
	ings.mutex.RLock()
	defer ings.mutex.RUnlock()

	p, ok := ings.pipelines[name]
	return p, ok
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
		pipeline := &httppipeline.HTTPPipeline{}
		pipeline.Init(superSpec, ings.super)
		ings.pipelines[service.IngressPipelineName()] = pipeline
	}

	if ings.httpServer == nil {
		superSpec, err := service.SideCarIngressHTTPServerSpec()
		if err != nil {
			return err
		}

		httpServer := &httpserver.HTTPServer{}
		httpServer.Init(superSpec, ings.super)
		httpServer.InjectMuxMapper(ings)
		ings.httpServer = httpServer
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

	pipeline, ok := ings.pipelines[service.IngressPipelineName()]
	if !ok {
		return fmt.Errorf("can't find service: %s's ingress pipeline", ings.serviceName)
	}

	superSpec, err := supervisor.NewSpec(newSpec)
	if err != nil {
		logger.Errorf("BUG: update ingress pipeline spec: %s new super spec failed: %v", newSpec, err)
		return err
	}

	var newPipeline httppipeline.HTTPPipeline
	newPipeline.Inherit(superSpec, pipeline, ings.super)
	ings.pipelines[service.IngressPipelineName()] = &newPipeline

	return err
}

// Close closes the Ingress HTTPServer and Pipeline
func (ings *IngressServer) Close() {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.httpServer.Close()
	for _, v := range ings.pipelines {
		v.Close()
	}
}
