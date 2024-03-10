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

// Package ingresscontroller implements a K8s ingress controller.
package ingresscontroller

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/httpserver"
	"github.com/megaease/easegress/v2/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the name of ingress controller
	Category = supervisor.CategoryBusinessController
	// Kind is the kind of ingress controller
	Kind = "IngressController"

	defaultIngressClass          = "easegress"
	defaultIngressControllerName = "megaease.com/ingress-controller"
	k8sIngressClassAnnotation    = "kubernetes.io/ingress.class"
)

func init() {
	supervisor.Register(&IngressController{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"ingresscontrollers", "ingress"},
	})
}

type (
	// IngressController implements a K8s ingress controller
	IngressController struct {
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
		HTTPServer   *httpserver.Spec `json:"httpServer" jsonschema:"required"`
		KubeConfig   string           `json:"kubeConfig,omitempty"`
		MasterURL    string           `json:"masterURL,omitempty"`
		Namespaces   []string         `json:"namespaces,omitempty"`
		IngressClass string           `json:"ingressClass,omitempty"`
	}
)

// Category returns the category of IngressController.
func (ic *IngressController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of IngressController.
func (ic *IngressController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of IngressController.
func (ic *IngressController) DefaultSpec() interface{} {
	return &Spec{
		HTTPServer: &httpserver.Spec{
			KeepAlive:        true,
			KeepAliveTimeout: "60s",
			MaxConnections:   10240,
		},
		IngressClass: defaultIngressClass,
	}
}

// Init initializes IngressController.
func (ic *IngressController) Init(superSpec *supervisor.Spec) {
	ic.superSpec = superSpec
	ic.spec = superSpec.ObjectSpec().(*Spec)
	ic.super = superSpec.Super()
	ic.reload()
}

// Inherit inherits previous generation of IngressController.
func (ic *IngressController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	ic.Init(superSpec)
}

func (ic *IngressController) reload() {
	if entity, exists := ic.super.GetSystemController(trafficcontroller.Kind); !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	} else if tc, ok := entity.Instance().(*trafficcontroller.TrafficController); !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	} else {
		ic.tc = tc
	}

	ic.namespace = fmt.Sprintf("%s/%s", ic.superSpec.Name(), "ingresscontroller")
	ic.stopCh = make(chan struct{})

	ic.wg.Add(1)
	go ic.run()
}

func (ic *IngressController) run() {
	defer ic.wg.Done()

	// connect to kubernetes
	for {
		k8sClient, err := newK8sClient(ic.spec.MasterURL, ic.spec.KubeConfig)
		if err == nil {
			ic.k8sClient = k8sClient
			break
		}
		logger.Errorf("failed to create kubernetes client: %v", err)

		select {
		case <-ic.stopCh:
			return
		case <-time.After(10 * time.Second):
		}
	}
	logger.Infof("successfully connect to kubernetes")

	// watch ingress related resources
	var (
		stopCh chan struct{}
		err    error
	)
	for {
		stopCh, err = ic.k8sClient.watch(ic.spec.Namespaces)
		if err == nil {
			break
		}
		logger.Errorf("failed to watch ingress related resources: %v", err)

		select {
		case <-ic.stopCh:
			return
		case <-time.After(10 * time.Second):
		}
	}
	logger.Infof("successfully watched ingress related resources")

	// process resource update events
	for {
		select {
		case <-ic.stopCh:
			close(stopCh) // close stopCh to stop goroutines created by watch
			return

		case <-ic.k8sClient.event():
			err = ic.translate()

		// retry if last translation failed, the k8sClient.event() won't send
		// a new event in this case, so we need a timer
		case <-time.After(10 * time.Second):
			if err != nil {
				logger.Infof("last translation failed, retry")
				err = ic.translate()
			}
		}
	}
}

// Status returns the status of IngressController.
func (ic *IngressController) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}

// Close closes IngressController.
func (ic *IngressController) Close() {
	close(ic.stopCh)
	ic.wg.Wait()
	ic.tc.Clean(ic.namespace)
}

func (ic *IngressController) translate() error {
	logger.Debugf("begin translate kubernetes ingress to easegress configuration")
	st := newSpecTranslator(ic.k8sClient, ic.spec.IngressClass, ic.spec.HTTPServer)
	err := st.translate()
	if err != nil {
		logger.Errorf("failed to translate kubernetes ingress: %v", err)
		return err
	}
	logger.Debugf("end translate kubernetes ingress to easegress configuration")

	pipelines := st.pipelineSpecs()
	for _, spec := range pipelines {
		_, err = ic.tc.ApplyPipelineForSpec(ic.namespace, spec)
		if err != nil {
			logger.Errorf("BUG: failed to apply pipeline spec to %s: %v", spec.Name(), err)
		}
	}
	logger.Debugf("pipelines updated")

	spec := st.httpServerSpec()
	_, err = ic.tc.ApplyTrafficGateForSpec(ic.namespace, spec)
	if err != nil {
		logger.Errorf("BUG: failed to apply http server spec: %v", err)
	} else {
		logger.Debugf("http server updated")
	}

	for _, p := range ic.tc.ListPipelines(ic.namespace) {
		if _, ok := pipelines[p.Spec().Name()]; !ok {
			ic.tc.DeletePipeline(ic.namespace, p.Spec().Name())
		}
	}

	return nil
}
