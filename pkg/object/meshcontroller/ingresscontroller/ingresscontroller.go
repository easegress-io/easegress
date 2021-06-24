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

		tc            *trafficcontroller.TrafficController
		namespace     string
		httpServer    *supervisor.ObjectEntity
		httpPipelines map[string]*supervisor.ObjectEntity
		service       *service.Service
		informer      informer.Informer

		addServiceEvent    chan string
		removeServiceEvent chan string
		done               chan struct{}
	}
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

		tc:            tc,
		namespace:     fmt.Sprintf("%s/%s", superSpec.Name(), "ingresscontroller"),
		httpPipelines: make(map[string]*supervisor.ObjectEntity),
		service:       service.New(superSpec, store),
		informer:      informer.NewInformer(store),

		addServiceEvent:    make(chan string, serviceEventChanSize),
		removeServiceEvent: make(chan string, serviceEventChanSize),
		done:               make(chan struct{}),
	}

	ic.applyHTTPServer()

	go ic.watchEvent()

	return ic
}

// applyHTTPServer applies HTTPServer to its latest status.
func (ic *IngressController) applyHTTPServer() {
	var rules []*spec.IngressRule
	ingresses := ic.service.ListIngressSpecs()
	for _, ingress := range ingresses {
		for i := range ingress.Rules {
			rules = append(rules, &ingress.Rules[i])
		}
	}

	superSpec, err := spec.IngressHTTPServerSpec(ic.spec.IngressPort, rules)
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

func (ic *IngressController) applyPipeline(serviceName string) {
	service := ic.service.GetServiceSpec(serviceName)
	if service == nil {
		logger.Errorf("service spec %s not found", serviceName)
		return
	}

	instanceSpecs := ic.service.ListServiceInstanceSpecs(serviceName)
	if len(instanceSpecs) == 0 {
		logger.Errorf("service %s got 0 instances", serviceName)
		return
	}

	superSpec, err := service.IngressPipelineSpec(instanceSpecs)
	if err != nil {
		logger.Errorf("get pipeline spec %s failed: %v", serviceName, err)
		return
	}

	entity, err := ic.tc.ApplyHTTPPipelineForSpec(ic.namespace, superSpec)
	if err != nil {
		logger.Errorf("apply pipeline %s failed: %v", superSpec.Name(), err)
		return
	}

	ic.httpPipelines[superSpec.Name()] = entity
}

func (ic *IngressController) deletePipeline(serviceName string) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	service := ic.service.GetServiceSpec(serviceName)
	if service == nil {
		logger.Errorf("service spec %s not found", serviceName)
		return
	}

	err := ic.tc.DeleteHTTPPipeline(ic.namespace, service.IngressPipelineName())
	if err != nil {
		logger.Errorf("delete http pipeline failed: %v", err)
		return
	}

	delete(ic.httpPipelines, service.IngressPipelineName())
}

func (ic *IngressController) recover() {
	if err := recover(); err != nil {
		const format = "%s: recover from: %v, stack trace:\n%s\n"
		logger.Errorf(format, ic.superSpec.Name(), err, debug.Stack())
	}
}

func (ic *IngressController) watchIngress() {
	handler := func(ingresses map[string]*spec.Ingress) (continueWatch bool) {
		continueWatch = true
		defer ic.recover()

		logger.Infof("handle informer ingress update event: %#v", ingresses)

		services := make(map[string]struct{})
		for _, ingress := range ingresses {
			for i := range ingress.Rules {
				r := &ingress.Rules[i]
				for j := range r.Paths {
					p := &r.Paths[j]
					services[p.Backend] = struct{}{}
				}
			}
		}

		ic.mutex.Lock()
		defer ic.mutex.Unlock()

		ic.applyHTTPServer()
		for _, entity := range ic.tc.ListHTTPPipelines(ic.namespace) {
			name := entity.Spec().Name()
			if _, exists := services[name]; !exists {
				ic.removeServiceEvent <- name
			}
		}

		return
	}

	err := ic.informer.OnIngressSpecs(handler)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("add scope watching ingress failed: %v", err)
	}
}

func (ic *IngressController) stopWatchService(name string) {
	logger.Infof("stop watching service %s as it is removed from all ingress rules", name)
	ic.informer.StopWatchServiceSpec(name, informer.AllParts)
	ic.informer.StopWatchServiceInstanceSpec(name)
}

func (ic *IngressController) watchService(name string) {
	handleSerivceSpec := func(event informer.Event, service *spec.Service) (continueWatch bool) {
		continueWatch = true
		defer ic.recover()

		switch event.EventType {
		case informer.EventDelete:
			logger.Infof("handle informer service: %s's spec delete event", name)
			ic.deletePipeline(name)
			return false

		case informer.EventUpdate:
			logger.Infof("handle informer service: %s's spec update event", name)
			ic.applyPipeline(service.Name)
		}

		return
	}

	err := ic.informer.OnPartOfServiceSpec(name, informer.AllParts, handleSerivceSpec)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("add scope watching service: %s failed: %v", name, err)
		return
	}

	handleServiceInstances := func(instanceKvs map[string]*spec.ServiceInstanceSpec) (continueWatch bool) {
		continueWatch = true
		defer ic.recover()

		logger.Infof("handle informer service: %s's instance update event, ins: %#v", name, instanceKvs)
		ic.applyPipeline(name)

		return
	}

	err = ic.informer.OnServiceInstanceSpecs(name, handleServiceInstances)
	if err != nil && err != informer.ErrAlreadyWatched {
		logger.Errorf("add prefix watching service: %s failed: %v", name, err)
		return
	}
}

func (ic *IngressController) watchEvent() {
	ic.watchIngress()

	for {
		select {
		case <-ic.done:
			return
		case name := <-ic.addServiceEvent:
			ic.watchService(name)
		case name := <-ic.removeServiceEvent:
			ic.stopWatchService(name)
			ic.deletePipeline(name)
		}
	}
}

// Status returns the status of IngressController.
func (ic *IngressController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}

func (ic *IngressController) Close() {
	close(ic.done)

	ic.mutex.Lock()
	defer ic.mutex.Unlock()

	ic.informer.Close()

	ic.tc.DeleteHTTPServer(ic.namespace, ic.httpServer.Spec().Name())
	for _, entity := range ic.httpPipelines {
		ic.tc.DeleteHTTPPipeline(ic.namespace, entity.Spec().Name())
		delete(ic.httpPipelines, entity.Spec().Name())
	}
}
