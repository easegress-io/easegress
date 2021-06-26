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

const (
	// from k8s pod's env value
	podEnvHostname      = "HOSTNAME"
	podEnvApplicationIP = "APPLICATION_IP"
)

func decodeLables(lables string) map[string]string {
	mLabels := make(map[string]string)
	if len(lables) == 0 {
		return mLabels
	}
	strLabel, err := url.QueryUnescape(lables)
	if err != nil {
		logger.Errorf("query unescape: %s failed: %v ", lables, err)
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
func New(superSpec *supervisor.Spec, super *supervisor.Supervisor) *Worker {
	spec := superSpec.ObjectSpec().(*spec.Admin)

	serviceName := super.Options().Labels[label.KeyServiceName]
	aliveProbe := super.Options().Labels[label.KeyAliveProbe]
	serviceLabels := decodeLables(super.Options().Labels[label.KeyServiceLables])
	applicationPort, err := strconv.Atoi(super.Options().Labels[label.KeyApplicationPort])
	if err != nil {
		logger.Errorf("parse %s failed: %v", super.Options().Labels[label.KeyApplicationPort], err)
	}

	instanceID := os.Getenv(podEnvHostname)
	applicationIP := os.Getenv(podEnvApplicationIP)

	store := storage.New(superSpec.Name(), super.Cluster())
	_service := service.New(superSpec, super)
	registryCenterServer := registrycenter.NewRegistryCenterServer(spec.RegistryType,
		serviceName, applicationIP, applicationPort, instanceID, serviceLabels, _service)

	// FIXME: check service Name
	inf := informer.NewInformer(store, serviceName)
	ingressServer := NewIngressServer(superSpec, super, serviceName, inf)
	egressServer := NewEgressServer(superSpec, super, serviceName, _service, inf)

	observabilityManager := NewObservabilityServer(serviceName)
	apiServer := NewAPIServer(spec.APIPort)

	worker := &Worker{
		super:     super,
		superSpec: superSpec,
		spec:      spec,

		serviceName:     serviceName,
		instanceID:      instanceID, // instanceID will be the pod ID
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

func (worker *Worker) run() {
	var err error
	worker.heartbeatInterval, err = time.ParseDuration(worker.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat interval: %s failed: %v",
			worker.spec.HeartbeatInterval, err)
		return
	}

	if len(worker.serviceName) == 0 {
		logger.Errorf("mesh service name is empty")
		return
	} else {
		logger.Infof("%s works for service %s", worker.serviceName)
	}

	_, err = url.ParseRequestURI(worker.aliveProbe)
	if err != nil {
		logger.Errorf("parse alive probe: %s to url failed: %v", worker.aliveProbe, err)
		return
	}

	if worker.applicationPort == 0 {
		logger.Errorf("empty application port")
		return
	}

	if len(worker.instanceID) == 0 {
		logger.Errorf("empty env HOSTNAME")
		return
	}

	if len(worker.applicationIP) == 0 {
		logger.Errorf("empty env APPLICATION_IP")
		return
	}

	startUpRoutine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					worker.superSpec.Name(), err, debug.Stack())
			}
		}()

		serviceSpec, info := worker.service.GetServiceSpecWithInfo(worker.serviceName)

		err := worker.initTrafficGate()
		if err != nil {
			logger.Errorf("init traffic gate failed: %v", err)
		}

		worker.registryServer.Register(serviceSpec, worker.ingressServer.Ready, worker.egressServer.Ready)

		err = worker.observabilityManager.UpdateService(serviceSpec, info.Version)
		if err != nil {
			logger.Errorf("update service %s failed: %v", serviceSpec.Name, err)
		}
	}

	startUpRoutine()
	go worker.heartbeat()
	go worker.pushSpecToJavaAgent()
}

func (worker *Worker) heartbeat() {
	inforJavaAgentReady, trafficGateReady := false, false

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
			if !inforJavaAgentReady {
				err := worker.informJavaAgent()
				if err != nil {
					logger.Errorf(err.Error())
				} else {
					inforJavaAgentReady = true
				}
			}

			err := worker.updateHearbeat()
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

	if err := worker.ingressServer.InitIngress(service, worker.applicationPort); err != nil {
		return fmt.Errorf("create ingress for service: %s failed: %v", worker.serviceName, err)
	}

	if err := worker.egressServer.InitEgress(service); err != nil {
		return fmt.Errorf("create egress for service: %s failed: %v", worker.serviceName, err)
	}

	return nil
}

func (worker *Worker) updateHearbeat() error {
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

			// NOTE: This is a little strict, maybe we could use the brand new status to udpate.
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

// Close close the worker
func (worker *Worker) Close() {
	close(worker.done)

	worker.egressServer.Close()
	worker.ingressServer.Close()
	worker.informer.Close()
	worker.registryServer.Close()
	worker.apiServer.Close()
}
