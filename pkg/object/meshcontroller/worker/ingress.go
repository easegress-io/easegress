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
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/supervisor"
)

// ErrIngressClosed is the error when operating in a closed Ingress server
var ErrIngressClosed = fmt.Errorf("ingress has been closed")

type (
	// IngressServer manages one ingress pipeline and one HTTPServer
	IngressServer struct {
		super       *supervisor.Supervisor
		serviceName string

		mutex sync.RWMutex

		tc              *trafficcontroller.TrafficController
		applicationPort uint32
		namespace       string
		inf             informer.Informer

		pipelines  map[string]*supervisor.ObjectEntity
		httpServer *supervisor.ObjectEntity
	}
)

// NewIngressServer creates a initialized ingress server
func NewIngressServer(superSpec *supervisor.Spec, super *supervisor.Supervisor, serviceName string, inf informer.Informer) *IngressServer {
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
		inf:         inf,
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

// InitIngress creates local default pipeline and httpServer for ingress
func (ings *IngressServer) InitIngress(service *spec.Service, port uint32) error {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.applicationPort = port

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

	if err := ings.inf.OnPartOfServiceSpec(service.Name, informer.AllParts, ings.reloadTraffic); err != nil {
		// Only return err when its type is not `AlreadyWatched`
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add ingress spec watching service: %s failed: %v", service.Name, err)
			return err
		}
	}

	return nil
}

func (ings *IngressServer) reloadTraffic(event informer.Event, serviceSpec *spec.Service) bool {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if event.EventType == informer.EventDelete {
		logger.Infof("receive delete event: %#v", event)
		return false
	}

	superSpec, err := serviceSpec.SideCarIngressPipelineSpec(ings.applicationPort)
	if err != nil {
		logger.Errorf("BUG: update ingress pipeline spec: %s new super spec failed: %v",
			superSpec.YAMLConfig(), err)
		return true
	}

	entity, err := ings.tc.UpdateHTTPPipelineForSpec(ings.namespace, superSpec)
	if err != nil {
		return true
	}

	ings.pipelines[ings.serviceName] = entity
	return true
}

// Close closes the Ingress HTTPServer and Pipeline
func (ings *IngressServer) Close() {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if ings.Ready() {
		ings.tc.DeleteHTTPServer(ings.namespace, ings.httpServer.Spec().Name())
		for _, entity := range ings.pipelines {
			ings.tc.DeleteHTTPPipeline(ings.namespace, entity.Spec().Name())
		}
	}
}
