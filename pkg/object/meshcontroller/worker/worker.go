/*
 * Copyright (c) 2017, The Easegress Authors
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

// Package worker provides the worker for mesh controller.
package worker

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/informer"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/label"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/layout"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/registrycenter"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/service"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/storage"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
	"github.com/megaease/easegress/v2/pkg/util/jmxtool"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
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

		registryType         string
		registryServer       *registrycenter.Server
		ingressServer        *IngressServer
		egressServer         *EgressServer
		observabilityManager *ObservabilityManager
		apiServer            *apiServer

		done chan struct{}
	}
)

func decodeLabels(labelStr string) map[string]string {
	labelMap := make(map[string]string)
	if len(labelStr) == 0 {
		return labelMap
	}

	labelSlice := strings.Split(labelStr, ",")

	for _, v := range labelSlice {
		kv := strings.Split(v, "=")
		if len(kv) == 2 {
			labelMap[kv[0]] = kv[1]
		} else {
			logger.Errorf("%s: invalid label: %s", labelStr, v)
		}
	}

	return labelMap
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

	_informer := informer.NewInformer(store, serviceName)
	observabilityManager := NewObservabilityServer(serviceName)
	instanceSpec := &spec.ServiceInstanceSpec{
		RegistryName: superSpec.Name(),
		ServiceName:  serviceName,
		InstanceID:   instanceID,
		IP:           applicationIP,
		// Port is assigned when registered.
		Labels: serviceLabels,
	}

	registryCenterServer := registrycenter.NewRegistryCenterServer(_spec.RegistryType,
		instanceSpec, _service, _informer, observabilityManager.agentClient)

	ingressServer := NewIngressServer(superSpec, super, serviceName, instanceID, _service)
	egressServer := NewEgressServer(superSpec, super, serviceName, instanceID, _service)

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
		informer: _informer,

		registryType:         _spec.RegistryType,
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

		serviceSpec := worker.service.GetServiceSpec(worker.serviceName)
		if serviceSpec == nil {
			return false
		}

		err := worker.initTrafficGate()
		if err != nil {
			logger.Errorf("init traffic gate failed: %v", err)
		}

		worker.registryServer.Register(serviceSpec, worker.ingressServer.Ready, worker.egressServer.Ready)

		return true
	}

	if runnable := startUpRoutine(); !runnable {
		logger.Errorf("service: %s is not runnable, please check the service spec", worker.superSpec.Name())
		return
	}
	go worker.heartbeat()
	go worker.updateAgentConfig()
}

func (worker *Worker) heartbeat() {
	trafficGateReady := false

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

func (worker *Worker) updateAgentConfig() {
	routine := func() {
		defer func() {
			if err := recover(); err != nil {
				logger.Errorf("%s: recover from: %v, stack trace:\n%s\n",
					worker.superSpec.Name(), err, debug.Stack())
			}
		}()

		decodeBase64 := func(s string) string {
			result, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				panic(err)
			}

			return string(result)
		}

		agentConfig := &jmxtool.AgentConfig{}

		serviceSpec := worker.service.GetServiceSpec(worker.serviceName)
		agentConfig.Service = *serviceSpec

		canaries := worker.service.ListServiceCanaries()
		headersMap := map[string]struct{}{spec.ServiceCanaryHeaderKey: {}}
		for _, canary := range canaries {
			for key := range canary.TrafficRules.Headers {
				headersMap[key] = struct{}{}
			}
		}
		canaryHeaders := []string{}
		for key := range headersMap {
			canaryHeaders = append(canaryHeaders, key)
		}
		sort.Strings(canaryHeaders)
		agentConfig.Headers = strings.Join(canaryHeaders, ",")

		if worker.spec.MonitorMTLS != nil && worker.spec.MonitorMTLS.Enabled {
			for _, monitorCert := range worker.spec.MonitorMTLS.Certs {
				if stringtool.StrInSlice(worker.serviceName, monitorCert.Services) {
					reporterType := worker.spec.MonitorMTLS.ReporterAppendType
					if reporterType == "" {
						reporterType = "http"
					}
					agentConfig.Reporter = &jmxtool.AgentReporter{
						ReporterTLS: &jmxtool.AgentReporterTLS{
							Enable: true,
							CACert: decodeBase64(worker.spec.MonitorMTLS.CaCertBase64),
							Cert:   decodeBase64(monitorCert.CertBase64),
							Key:    decodeBase64(monitorCert.KeyBase64),
						},
						AppendType:      reporterType,
						BootstrapServer: worker.spec.MonitorMTLS.URL,
						Username:        worker.spec.MonitorMTLS.Username,
						Password:        worker.spec.MonitorMTLS.Password,
					}
					break
				}
			}
		}

		worker.observabilityManager.agentClient.UpdateAgentConfig(agentConfig)
	}

	routine()

	for {
		select {
		case <-worker.done:
			return
		case <-time.After(30 * time.Second):
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
		err := codectool.Unmarshal([]byte(*value), status)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to json failed: %v", *value, err)

			// NOTE: This is a little strict, maybe we could use the brand new status to update.
			return err
		}
	}

	status.LastHeartbeatTime = time.Now().Format(time.RFC3339)
	buff, err := codectool.MarshalJSON(status)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", status, err)
		return err
	}

	return worker.store.Put(layout.ServiceInstanceStatusKey(worker.serviceName, worker.instanceID), string(buff))
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
