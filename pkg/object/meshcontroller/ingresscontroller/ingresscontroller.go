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

package ingresscontroller

import (
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/object/httpserver"
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/pkg/supervisor"
)

type (
	// IngressController is the ingress controller.
	IngressController struct {
		mutex sync.RWMutex

		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *spec.Admin

		informer  informer.Informer
		service   *service.Service
		tc        *trafficcontroller.TrafficController
		namespace string

		httpServer      *supervisor.ObjectEntity
		httpPipelines   map[string]*supervisor.ObjectEntity
		ingressBackends map[string]struct{}
		ingressRules    []*spec.IngressRule
	}

	Status = trafficcontroller.StatusInSameNamespace
)

const (
	serviceEventChanSize = 100
)

// New creates a mesh ingress controller.
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *IngressController {
	entity, exists := super.GetSystemController(trafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}

	store := storage.New(superSpec.Name(), super.Cluster())

	ic := &IngressController{
		super:     super,
		superSpec: superSpec,
		spec:      superSpec.ObjectSpec().(*spec.Admin),

		informer:  informer.NewInformer(store),
		service:   service.New(superSpec, super),
		tc:        tc,
		namespace: fmt.Sprintf("%s/%s", superSpec.Name(), "ingresscontroller"),

		httpPipelines:   make(map[string]*supervisor.ObjectEntity),
		ingressBackends: make(map[string]struct{}),
		ingressRules:    []*spec.IngressRule{},
	}

	ic.reloadTraffic()

	err := ic.informer.OnAllIngressSpecs(ic.handleIngress)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("watch ingress failed: %v", err)
	}

	err = ic.informer.OnAllServiceSpecs(ic.handleService)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("watch service failed: %v", err)
	}

	err = ic.informer.OnAllServiceInstanceSpecs(ic.handleServiceInstance)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("watch service instance failed: %v", err)
	}

	return ic
}

func (ic *IngressController) handleIngress(ingresses map[string]*spec.Ingress) (continueWatch bool) {
	continueWatch = true

	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: handleIngress recover from: %v, stack trace:\n%s\n",
				ic.superSpec.Name(), err, debug.Stack())
		}
	}()

	ic.reloadTraffic()

	return
}

func (ic *IngressController) handleService(value map[string]*spec.Service) (continueWatch bool) {
	continueWatch = true

	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: handleService recover from: %v, stack trace:\n%s\n",
				ic.superSpec.Name(), err, debug.Stack())
		}
	}()

	ic.reloadTraffic()

	return
}

func (ic *IngressController) handleServiceInstance(value map[string]*spec.ServiceInstanceSpec) (continueWatch bool) {
	continueWatch = true

	defer func() {
		if err := recover(); err != nil {
			logger.Errorf("%s: handleServiceInstance recover from: %v, stack trace:\n%s\n",
				ic.superSpec.Name(), err, debug.Stack())
		}
	}()

	ic.reloadTraffic()

	return
}

func (ic *IngressController) reloadTraffic() {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	ic._reloadIngress()
	ic._reloadHTTPPipelines()
	ic._reloadHTTPServer()
}

func (ic *IngressController) _reloadIngress() {
	ingressBackends, ingressRules := make(map[string]struct{}), []*spec.IngressRule{}
	for _, ingress := range ic.service.ListIngressSpecs() {
		for _, rule := range ingress.Rules {
			ingressRules = append(ingressRules, rule)
			for _, path := range rule.Paths {
				ingressBackends[path.Backend] = struct{}{}
			}
		}
	}

	ic.ingressBackends, ic.ingressRules = ingressBackends, ingressRules
}

func (ic *IngressController) _reloadHTTPPipelines() {
	for _, entity := range ic.tc.ListHTTPPipelines(ic.namespace) {
		name := entity.Spec().Name()
		if _, exists := ic.ingressBackends[name]; !exists {
			ic.tc.DeleteHTTPPipeline(ic.namespace, name)
		}
	}

	for _, serviceSpec := range ic.service.ListServiceSpecs() {
		if _, exists := ic.ingressBackends[serviceSpec.Name]; !exists {
			continue
		}

		instanceSpecs := ic.service.ListServiceInstanceSpecs(serviceSpec.Name)
		if len(instanceSpecs) == 0 {
			continue
		}

		superSpec, err := serviceSpec.IngressPipelineSpec(instanceSpecs)
		if err != nil {
			logger.Errorf("get ingress pipeline for %s failed: %v",
				serviceSpec.Name, err)
		}

		ic.tc.ApplyHTTPPipelineForSpec(ic.namespace, superSpec)
	}
}

func (ic *IngressController) _reloadHTTPServer() {
	superSpec, err := spec.IngressHTTPServerSpec(ic.spec.IngressPort, ic.ingressRules)
	if err != nil {
		logger.Errorf("get ingress http server spec failed: %v", err)
		return
	}

	entity, err := ic.tc.ApplyHTTPServerForSpec(ic.namespace, superSpec)
	if err != nil {
		logger.Errorf("apply http server failed: %v", err)
		return
	}

	ic.httpServer = entity
}

// Status returns the status of IngressController.
func (ic *IngressController) Status() *supervisor.Status {
	status := &Status{
		Namespace:     ic.namespace,
		HTTPServers:   make(map[string]*httpserver.Status),
		HTTPPipelines: make(map[string]*httppipeline.Status),
	}

	ic.tc.WalkHTTPServers(ic.namespace, func(entity *supervisor.ObjectEntity) bool {
		status.HTTPServers[entity.Spec().Name()] = entity.Instance().Status().ObjectStatus.(*httpserver.Status)
		return true
	})

	ic.tc.WalkHTTPPipelines(ic.namespace, func(entity *supervisor.ObjectEntity) bool {
		status.HTTPPipelines[entity.Spec().Name()] = entity.Instance().Status().ObjectStatus.(*httppipeline.Status)
		return true
	})

	return &supervisor.Status{
		ObjectStatus: status,
	}
}

func (ic *IngressController) Close() {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	ic.informer.Close()
	ic.tc.Clean(ic.namespace)
}
