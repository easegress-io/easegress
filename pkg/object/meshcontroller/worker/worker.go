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
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/pkg/object/meshcontroller/label"
	"github.com/megaease/easegress/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegress/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegress/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/pkg/supervisor"
)

type (
	// Worker is a sidecar in service mesh.
	Worker struct {
		super             *supervisor.Supervisor
		superSpec         *supervisor.Spec
		spec              *spec.Admin
		heartbeatInterval time.Duration

		// mesh service fields
		serviceName     string
		instanceID      string
		aliveProbe      string
		applicationPort uint32
		applicationIP   string
		serviceLabels   map[string]string

		store    storage.Storage
		service  *service.Service
		informer informer.Informer

		registryServer       *registrycenter.Server
		ingressServer        *IngressServer
		egressServer         *EgressServer
		observabilityManager *ObservabilityManager
		apiServer            *apiServer

		done chan struct{}
	}
)

func decodeLabels(labels string) map[string]string {
	mLabels := make(map[string]string)
	if len(labels) == 0 {
		return mLabels
	}
	strLabel, err := url.QueryUnescape(labels)
	if err != nil {
		logger.Errorf("query unescape: %s failed: %v ", labels, err)
		return mLabels
	}

	arrLabel := strings.Split(strLabel, "&")

	for _, v := range arrLabel {
		kv := strings.Split(v, "=")
		if len(kv) == 2 {
			mLabels[kv[0]] = kv[1]
		} else {
			logger.Errorf("serviceLabel: %s invalid format: %s", strLabel, v)
		}
	}
	return mLabels
}

// New creates a mesh worker.
func New(superSpec *supervisor.Spec) *Worker {
	super := superSpec.Super()
	_spec := superSpec.ObjectSpec().(*spec.Admin)
	serviceName := super.Options().Labels[label.KeyServiceName]
	aliveProbe := super.Options().Labels[label.KeyAliveProbe]
	serviceLabels := decodeLabels(super.Options().Labels[label.KeyServiceLabels])
	applicationPort, err := strconv.Atoi(super.Options().Labels[label.KeyApplicationPort])
	if err != nil {
		logger.Errorf("parse %s failed: %v", super.Options().Labels[label.KeyApplicationPort], err)
	}

	instanceID := os.Getenv(spec.PodEnvHostname)
	applicationIP := os.Getenv(spec.PodEnvApplicationIP)
	store := storage.New(superSpec.Name(), super.Cluster())
	_service := service.New(superSpec)

	inf := informer.NewInformer(store, serviceName)
	registryCenterServer := registrycenter.NewRegistryCenterServer(_spec.RegistryType,
		superSpec.Name(), serviceName, applicationIP, applicationPort,
		instanceID, serviceLabels, _service, inf)
	ingressServer := NewIngressServer(superSpec, super, serviceName, instanceID, _service)
	egressServer := NewEgressServer(superSpec, super, serviceName, instanceID, _service)

	observabilityManager := NewObservabilityServer(serviceName)
	apiServer := newAPIServer(_spec.APIPort)

	worker := &Worker{
		super:     super,
		superSpec: superSpec,
		spec:      _spec,

		serviceName:     serviceName,
		instanceID:      instanceID, // instanceID will be the pod ID valued by HOSTNAME env.
		aliveProbe:      aliveProbe,
		applicationPort: uint32(applicationPort),
		applicationIP:   applicationIP,
		serviceLabels:   serviceLabels,

		store:    store,
		service:  _service,
		informer: inf,

		registryServer:       registryCenterServer,
		ingressServer:        ingressServer,
		egressServer:         egressServer,
		observabilityManager: observabilityManager,
		apiServer:            apiServer,

		done: make(chan struct{}),
	}

	worker.runAPIServer()

	go worker.run()

	return worker
}

func (worker *Worker) validate() error {
	var err error
	worker.heartbeatInterval, err = time.ParseDuration(worker.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat interval: %s failed: %v",
			worker.spec.HeartbeatInterval, err)
		return err
	}

	if len(worker.serviceName) == 0 {
		errMsg := "empty service name"
		logger.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}

	_, err = url.ParseRequestURI(worker.aliveProbe)
	if err != nil {
		logger.Errorf("parse alive probe: %s to url failed: %v", worker.aliveProbe, err)
		return err
	}

	if worker.applicationPort == 0 {
		errMsg := "empty application port"
		logger.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}

	if len(worker.instanceID) == 0 {
		errMsg := "empty env HOSTNAME"
		logger.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}

	if len(worker.applicationIP) == 0 {
		errMsg := "empty env APPLICATION_IP"
		logger.Errorf(errMsg)
		return fmt.Errorf(errMsg)
	}
	logger.Infof("sidecar works for service: %s", worker.serviceName)
	return nil
}

func (worker *Worker) run() {
	if err := worker.validate(); err != nil {
		return
	}
	startUpRoutine := func() bool {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					worker.superSpec.Name(), err, debug.Stack())
			}
		}()

		serviceSpec, info := worker.service.GetServiceSpecWithInfo(worker.serviceName)
		if serviceSpec == nil || !serviceSpec.Runnable() {
			return false
		}

		err := worker.initTrafficGate()
		if err != nil {
			logger.Errorf("init traffic gate failed: %v", err)
		}

		worker.registryServer.Register(serviceSpec, worker.ingressServer.Ready, worker.egressServer.Ready)

		err = worker.observabilityManager.UpdateService(serviceSpec, info.Version)
		if err != nil {
			logger.Errorf("update service %s failed: %v", serviceSpec.Name, err)
		}

		return true
	}

	if runnable := startUpRoutine(); !runnable {
		logger.Errorf("service: %s is not runnable, check the service spec or ignore if mock is enable", worker.superSpec.Name())
		return
	}
	go worker.heartbeat()
	go worker.pushSpecToJavaAgent()
}

func (worker *Worker) heartbeat() {
	informJavaAgentReady, trafficGateReady := false, false

	routine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					worker.superSpec.Name(), err, debug.Stack())
			}
		}()

		if !trafficGateReady {
			err := worker.initTrafficGate()
			if err != nil {
				logger.Errorf("init traffic gate failed: %v", err)
			} else {
				trafficGateReady = true
			}
		}

		if worker.registryServer.Registered() {
			if !informJavaAgentReady {
				err := worker.informJavaAgent()
				if err != nil {
					logger.Errorf(err.Error())
				} else {
					informJavaAgentReady = true
				}
			}

			err := worker.updateHeartbeat()
			if err != nil {
				logger.Errorf("update heartbeat failed: %v", err)
			}
		}
	}

	for {
		select {
		case <-worker.done:
			return
		case <-time.After(worker.heartbeatInterval):
			routine()
		}
	}
}

func (worker *Worker) pushSpecToJavaAgent() {
	routine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					worker.superSpec.Name(), err, debug.Stack())
			}
		}()

		serviceSpec, info := worker.service.GetServiceSpecWithInfo(worker.serviceName)
		err := worker.observabilityManager.UpdateService(serviceSpec, info.Version)
		if err != nil {
			logger.Errorf("update service %s failed: %v", serviceSpec.Name, err)
		}

		globalCanaryHeaders, info := worker.service.GetGlobalCanaryHeadersWithInfo()
		if globalCanaryHeaders != nil {
			err := worker.observabilityManager.UpdateCanary(globalCanaryHeaders, info.Version)
			if err != nil {
				logger.Errorf("update canary failed: %v", err)
			}
		}
	}

	for {
		select {
		case <-worker.done:
			return
		case <-time.After(1 * time.Minute):
			routine()
		}
	}
}

func (worker *Worker) initTrafficGate() error {
	service := worker.service.GetServiceSpec(worker.serviceName)
	if service == nil {
		logger.Errorf("service %s not found", worker.serviceName)
		return spec.ErrServiceNotFound
	}

	if err := worker.ingressServer.InitIngress(service, worker.applicationPort); err != nil {
		return fmt.Errorf("create ingress for service: %s failed: %v", worker.serviceName, err)
	}

	if err := worker.egressServer.InitEgress(service); err != nil {
		return fmt.Errorf("create egress for service: %s failed: %v", worker.serviceName, err)
	}

	return nil
}

func (worker *Worker) updateHeartbeat() error {
	resp, err := http.Get(worker.aliveProbe)
	if err != nil {
		return fmt.Errorf("probe: %s check service: %s instanceID: %s heartbeat failed: %v",
			worker.aliveProbe, worker.serviceName, worker.instanceID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe: %s check service: %s instanceID: %s heartbeat failed status code is %d",
			worker.aliveProbe, worker.serviceName, worker.instanceID, resp.StatusCode)
	}

	value, err := worker.store.Get(layout.ServiceInstanceStatusKey(worker.serviceName, worker.instanceID))
	if err != nil {
		return fmt.Errorf("get service: %s instance: %s status failed: %v", worker.serviceName, worker.instanceID, err)
	}

	status := &spec.ServiceInstanceStatus{
		ServiceName: worker.serviceName,
		InstanceID:  worker.instanceID,
	}
	if value != nil {
		err := yaml.Unmarshal([]byte(*value), status)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", *value, err)

			// NOTE: This is a little strict, maybe we could use the brand new status to update.
			return err
		}
	}

	status.LastHeartbeatTime = time.Now().Format(time.RFC3339)
	buff, err := yaml.Marshal(status)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to yaml failed: %v", status, err)
		return err
	}

	return worker.store.Put(layout.ServiceInstanceStatusKey(worker.serviceName, worker.instanceID), string(buff))
}

func (worker *Worker) informJavaAgent() error {
	handleServiceSpec := func(event informer.Event, service *spec.Service) bool {
		switch event.EventType {
		case informer.EventDelete:
			return false
		case informer.EventUpdate:
			if err := worker.observabilityManager.UpdateService(service, event.RawKV.Version); err != nil {
				logger.Errorf("update service %s failed: %v", service.Name, err)
			}
		}

		return true
	}

	err := worker.informer.OnPartOfServiceSpec(worker.serviceName, informer.AllParts, handleServiceSpec)
	if err != nil && err != informer.ErrAlreadyWatched {
		return fmt.Errorf("on informer for observability failed: %v", err)
	}

	return nil
}

// Status returns the status of worker.
func (worker *Worker) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}

// Close closes the worker
func (worker *Worker) Close() {
	close(worker.done)

	// close informer firstly.
	worker.informer.Close()
	worker.egressServer.Close()
	worker.ingressServer.Close()
	worker.registryServer.Close()
	worker.apiServer.Close()
}
