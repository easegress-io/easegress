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

// Package gateway implements k8s gateway API.
package gatewaycontroller

import (
	"fmt"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the name of GatewayController
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of GatewayController
	Kind = "GatewayController"

	gatewayControllerName = "megaease.com/gateway-controller"
)

type (
	// GatewayController implements a k8s gateway API controller
	GatewayController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		tc        *trafficcontroller.TrafficController
		namespace string
		k8sClient *k8sClient

		stopCh chan struct{}
		wg     sync.WaitGroup
	}

	// Spec is the ingress controller spec
	Spec struct {
		KubeConfig string   `json:"kubeConfig,omitempty"`
		MasterURL  string   `json:"masterURL,omitempty"`
		Namespaces []string `json:"namespaces,omitempty"`
	}
)

func init() {
	supervisor.Register(&GatewayController{})
}

// Category returns the category of GatewayController.
func (gc *GatewayController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of GatewayController.
func (gc *GatewayController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of GatewayController.
func (gc *GatewayController) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes GatewayController.
func (gc *GatewayController) Init(superSpec *supervisor.Spec) {
	gc.superSpec = superSpec
	gc.spec = superSpec.ObjectSpec().(*Spec)
	gc.super = superSpec.Super()
	gc.reload()
}

// Inherit inherits previous generation of GatewayController.
func (gc *GatewayController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	gc.Init(superSpec)
}

func (gc *GatewayController) reload() {
	if entity, exists := gc.super.GetSystemController(trafficcontroller.Kind); !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	} else if tc, ok := entity.Instance().(*trafficcontroller.TrafficController); !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	} else {
		gc.tc = tc
	}

	gc.namespace = fmt.Sprintf("%s/%s", gc.superSpec.Name(), "gatewaycontroller")
	gc.stopCh = make(chan struct{})

	gc.wg.Add(1)
	go gc.run()
}

func (gc *GatewayController) run() {
	defer gc.wg.Done()

	// connect to kubernetes
	for {
		k8sClient, err := newK8sClient(gc.spec.MasterURL, gc.spec.KubeConfig)
		if err == nil {
			gc.k8sClient = k8sClient
			break
		}
		logger.Errorf("failed to create kubernetes client: %v", err)

		select {
		case <-gc.stopCh:
			return
		case <-time.After(10 * time.Second):
		}
	}
	logger.Infof("successfully connect to kubernetes")

	// watch gateway related resources
	var (
		stopCh chan struct{}
		err    error
	)
	for {
		stopCh, err = gc.k8sClient.watch(gc.spec.Namespaces)
		if err == nil {
			break
		}
		logger.Errorf("failed to watch gateway related resources: %v", err)

		select {
		case <-gc.stopCh:
			return
		case <-time.After(10 * time.Second):
		}
	}
	logger.Infof("successfully watched gateway related resources")

	gc.translate()
	// process resource update events
	for {
		select {
		case <-gc.stopCh:
			close(stopCh) // close stopCh to stop goroutines created by watch
			return

		case <-gc.k8sClient.event():
			err = gc.translate()

		// retry if last translation failed, the k8sClient.event() won't send
		// a new event in this case, so we need a timer
		case <-time.After(10 * time.Second):
			if err != nil {
				logger.Infof("last translation failed, retry")
				err = gc.translate()
			}
		}
	}
}

func (gc *GatewayController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}

func (gc *GatewayController) Close() {
	close(gc.stopCh)
	gc.wg.Wait()
	gc.tc.Clean(gc.namespace)
}

func (gc *GatewayController) translate() error {
	logger.Debugf("begin translate kubernetes gateway to easegress configuration")

	st := newSpecTranslator(gc.k8sClient)
	err := st.translate()
	if err != nil {
		logger.Errorf("failed to translate kubernetes gateway: %v", err)
		return err
	}
	logger.Debugf("end translate kubernetes gateway to easegress configuration")

	specs := st.pipelineSpecs()
	for _, spec := range specs {
		_, err = gc.tc.ApplyPipelineForSpec(gc.namespace, spec)
		if err != nil {
			logger.Errorf("BUG: failed to apply pipeline spec to %s: %v", spec.Name(), err)
		}
	}

	for _, p := range gc.tc.ListPipelines(gc.namespace) {
		if _, ok := specs[p.Spec().Name()]; !ok {
			gc.tc.DeletePipeline(gc.namespace, p.Spec().Name())
		}
	}

	logger.Debugf("pipelines updated")

	specs = st.httpServerSpecs()
	for _, spec := range st.httpServerSpecs() {
		_, err = gc.tc.ApplyTrafficGateForSpec(gc.namespace, spec)
		if err != nil {
			logger.Errorf("BUG: failed to apply http server spec: %v", err)
		}
	}

	for _, p := range gc.tc.ListTrafficGates(gc.namespace) {
		if _, ok := specs[p.Spec().Name()]; !ok {
			gc.tc.DeleteTrafficGate(gc.namespace, p.Spec().Name())
		}
	}

	logger.Debugf("http servers updated")

	return nil
}
