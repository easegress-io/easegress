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

		egressEvent chan string
		done        chan struct{}
	}
)

const (
	egressEventChanSize = 100

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
	_service := service.New(superSpec, store)
	registryCenterServer := registrycenter.NewRegistryCenterServer(spec.RegistryType,
		serviceName, applicationIP, applicationPort, instanceID, serviceLabels, _service)
	ingressServer := NewIngressServer(super, serviceName)
	egressEvent := make(chan string, egressEventChanSize)
	egressServer := NewEgressServer(superSpec, super, serviceName, _service, egressEvent)
	observabilityManager := NewObservabilityServer(serviceName)
	inf := informer.NewInformer(store)
	apiServer := NewAPIServer(spec.APIPort)

	wrk := &Worker{
		super:     super,
		superSpec: superSpec,
		spec:      spec,

		serviceName:     serviceName,
		instanceID:      instanceID, // instanceID will be the port ID
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

		egressEvent: egressEvent,
		done:        make(chan struct{}),
	}

	wrk.runAPIServer()

	go wrk.run()

	return wrk
}

func (wrk *Worker) run() {
	var err error
	wrk.heartbeatInterval, err = time.ParseDuration(wrk.spec.HeartbeatInterval)
	if err != nil {
		logger.Errorf("BUG: parse heartbeat interval: %s failed: %v",
			wrk.spec.HeartbeatInterval, err)
		return
	}

	if len(wrk.serviceName) == 0 {
		logger.Errorf("mesh service name is empty")
		return
	} else {
		logger.Infof("%s works for service %s", wrk.serviceName)
	}

	_, err = url.ParseRequestURI(wrk.aliveProbe)
	if err != nil {
		logger.Errorf("parse alive probe: %s to url failed: %v", wrk.aliveProbe, err)
		return
	}

	if wrk.applicationPort == 0 {
		logger.Errorf("empty application port")
		return
	}

	if len(wrk.instanceID) == 0 {
		logger.Errorf("empty env HOSTNAME")
		return
	}

	if len(wrk.applicationIP) == 0 {
		logger.Errorf("empty env APPLICATION_IP")
		return
	}

	startUpRoutine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					wrk.superSpec.Name(), err, debug.Stack())
			}
		}()

		serviceSpec, info := wrk.service.GetServiceSpecWithInfo(wrk.serviceName)

		err := wrk.initTrafficGate()
		if err != nil {
			logger.Errorf("init traffic gate failed: %v", err)
		}

		wrk.registryServer.Register(serviceSpec, wrk.ingressServer.Ready, wrk.egressServer.Ready)

		err = wrk.observabilityManager.UpdateService(serviceSpec, info.Version)
		if err != nil {
			logger.Errorf("update service %s failed: %v", serviceSpec.Name, err)
		}
	}

	startUpRoutine()
	go wrk.heartbeat()
	go wrk.watchEvent()
	go wrk.pushSpecToJavaAgent()
}

func (wrk *Worker) heartbeat() {
	inforJavaAgentReady, trafficGateReady := false, false

	routine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					wrk.superSpec.Name(), err, debug.Stack())
			}
		}()

		if !trafficGateReady {
			err := wrk.initTrafficGate()
			if err != nil {
				logger.Errorf("init traffic gate failed: %v", err)
			} else {
				trafficGateReady = true
			}
		}

		if wrk.registryServer.Registered() {
			if !inforJavaAgentReady {
				err := wrk.informJavaAgent()
				if err != nil {
					logger.Errorf(err.Error())
				} else {
					inforJavaAgentReady = true
				}
			}

			err := wrk.updateHearbeat()
			if err != nil {
				logger.Errorf("update heartbeat failed: %v", err)
			}
		}
	}

	for {
		select {
		case <-wrk.done:
			return
		case <-time.After(wrk.heartbeatInterval):
			routine()
		}
	}
}

func (wrk *Worker) pushSpecToJavaAgent() {
	routine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					wrk.superSpec.Name(), err, debug.Stack())
			}
		}()

		serviceSpec, info := wrk.service.GetServiceSpecWithInfo(wrk.serviceName)
		err := wrk.observabilityManager.UpdateService(serviceSpec, info.Version)
		if err != nil {
			logger.Errorf("update service %s failed: %v", serviceSpec.Name, err)
		}

		globalCanaryHeaders, info := wrk.service.GetGlobalCanaryHeadersWithInfo()
		if globalCanaryHeaders != nil {
			err := wrk.observabilityManager.UpdateCanary(globalCanaryHeaders, info.Version)
			if err != nil {
				logger.Errorf("update canary failed: %v", err)
			}
		}
	}

	for {
		select {
		case <-wrk.done:
			return
		case <-time.After(1 * time.Minute):
			routine()
		}
	}
}

func (wrk *Worker) initTrafficGate() error {
	service := wrk.service.GetServiceSpec(wrk.serviceName)

	if err := wrk.ingressServer.CreateIngress(service, wrk.applicationPort); err != nil {
		return fmt.Errorf("create ingress for service: %s failed: %v", wrk.serviceName, err)
	}

	if err := wrk.egressServer.CreateEgress(service); err != nil {
		return fmt.Errorf("create egress for service: %s failed: %v", wrk.serviceName, err)
	}

	return nil
}

func (wrk *Worker) updateHearbeat() error {
	resp, err := http.Get(wrk.aliveProbe)
	if err != nil {
		return fmt.Errorf("probe: %s check service: %s instanceID: %s heartbeat failed: %v",
			wrk.aliveProbe, wrk.serviceName, wrk.instanceID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe: %s check service: %s instanceID: %s heartbeat failed status code is %d",
			wrk.aliveProbe, wrk.serviceName, wrk.instanceID, resp.StatusCode)
	}

	value, err := wrk.store.Get(layout.ServiceInstanceStatusKey(wrk.serviceName, wrk.instanceID))
	if err != nil {
		return fmt.Errorf("get service: %s instance: %s status failed: %v", wrk.serviceName, wrk.instanceID, err)
	}

	status := &spec.ServiceInstanceStatus{
		ServiceName: wrk.serviceName,
		InstanceID:  wrk.instanceID,
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

	return wrk.store.Put(layout.ServiceInstanceStatusKey(wrk.serviceName, wrk.instanceID), string(buff))
}

func (wrk *Worker) addEgressWatching(serviceName string) {
	handleSerivceSpec := func(event informer.Event, service *spec.Service) (continueWatch bool) {
		continueWatch = true
		switch event.EventType {
		case informer.EventDelete:
			wrk.egressServer.DeletePipeline(serviceName)
			return false
		case informer.EventUpdate:
			defer func() {
				if err := recover(); err != nil {
					logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
						wrk.superSpec.Name(), err, debug.Stack())
				}
			}()
			logger.Infof("handle informer egress service: %s's spec update event", serviceName)
			instanceSpecs := wrk.service.ListServiceInstanceSpecs(service.Name)

			if err := wrk.egressServer.UpdatePipeline(service, instanceSpecs); err != nil {
				logger.Errorf("handle informer egress update service: %s's failed: %v", serviceName, err)
			}
		}
		return
	}
	if err := wrk.informer.OnPartOfServiceSpec(serviceName, informer.AllParts, handleSerivceSpec); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress scope watching service: %s failed: %v", serviceName, err)
			return
		}
	}

	handleServiceInstances := func(instanceKvs map[string]*spec.ServiceInstanceSpec) (continueWatch bool) {
		continueWatch = true
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					wrk.superSpec.Name(), err, debug.Stack())
			}
		}()
		logger.Infof("handle informer egress service: %s's instance update event, ins: %#v", serviceName, instanceKvs)
		serviceSpec := wrk.service.GetServiceSpec(serviceName)

		var instanceSpecs []*spec.ServiceInstanceSpec
		for _, v := range instanceKvs {
			instanceSpecs = append(instanceSpecs, v)
		}
		if err := wrk.egressServer.UpdatePipeline(serviceSpec, instanceSpecs); err != nil {
			logger.Errorf("handle informer egress failed, update service: %s failed: %v", serviceName, err)
		}

		return
	}

	if err := wrk.informer.OnServiceInstanceSpecs(serviceName, handleServiceInstances); err != nil {
		if err != informer.ErrAlreadyWatched {
			logger.Errorf("add egress prefix watching service: %s failed: %v", serviceName, err)
			return
		}
	}
}

func (wrk *Worker) informJavaAgent() error {
	handleServiceSpec := func(event informer.Event, service *spec.Service) bool {
		switch event.EventType {
		case informer.EventDelete:
			return false
		case informer.EventUpdate:
			if err := wrk.observabilityManager.UpdateService(service, event.RawKV.Version); err != nil {
				logger.Errorf("update service %s failed: %v", service.Name, err)
			}
		}

		return true
	}

	err := wrk.informer.OnPartOfServiceSpec(wrk.serviceName, informer.AllParts, handleServiceSpec)
	if err != nil && err != informer.ErrAlreadyWatched {
		return fmt.Errorf("on informer for observability failed: %v", err)
	}

	return nil
}

func (wrk *Worker) watchEvent() {
	for {
		select {
		case <-wrk.done:
			return
		case name := <-wrk.egressEvent:
			logger.Infof("add egress wanted watching service: %s", name)
			wrk.addEgressWatching(name)
		}
	}
}

// Status returns the status of worker.
func (wrk *Worker) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}

// Close close the worker
func (wrk *Worker) Close() {
	close(wrk.done)

	wrk.informer.Close()
	wrk.registryServer.Close()
	wrk.apiServer.Close()
}
