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

package worker

import (
	"fmt"
	"sync"

	"github.com/megaease/easegress/v2/pkg/filters/builder"
	proxy "github.com/megaease/easegress/v2/pkg/filters/proxies/httpproxy"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/function/spec"
	"github.com/megaease/easegress/v2/pkg/object/httpserver"
	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
	"github.com/megaease/easegress/v2/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const ingressFunctionKey = "X-FaaS-Func-Name"

type (
	// ingressServer manages one/many ingress pipelines and one HTTPServer
	ingressServer struct {
		superSpec *supervisor.Spec

		faasNetworkLayerURL string
		faasHostSuffix      string
		faasNamespace       string

		namespace string
		mutex     sync.RWMutex

		tc             *trafficcontroller.TrafficController
		pipelines      map[string]struct{}
		httpServer     *supervisor.ObjectEntity
		httpServerSpec *supervisor.Spec
	}

	pipelineSpecBuilder struct {
		Kind          string `json:"kind"`
		Name          string `json:"name"`
		pipeline.Spec `json:",inline"`
	}

	httpServerSpecBuilder struct {
		Kind            string `json:"kind"`
		Name            string `json:"name"`
		httpserver.Spec `json:",inline"`
	}
)

// newIngressServer creates an initialized ingress server
func newIngressServer(superSpec *supervisor.Spec, controllerName string) *ingressServer {
	entity, exists := superSpec.Super().GetSystemController(trafficcontroller.Kind)

	if !exists {
		panic(fmt.Errorf("BUG: traffic controller not found"))
	}

	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *TrafficController, got %T", entity.Instance()))
	}
	return &ingressServer{
		pipelines:  make(map[string]struct{}),
		httpServer: nil,
		superSpec:  superSpec,
		mutex:      sync.RWMutex{},
		namespace:  fmt.Sprintf("%s/%s", superSpec.Name(), "ingress"),
		tc:         tc,
	}
}

func newPipelineSpecBuilder(funcName string) *pipelineSpecBuilder {
	return &pipelineSpecBuilder{
		Kind: pipeline.Kind,
		Name: funcName,
		Spec: pipeline.Spec{},
	}
}

func newHTTPServerSpecBuilder(controllerName string) *httpServerSpecBuilder {
	return &httpServerSpecBuilder{
		Kind: httpserver.Kind,
		Name: controllerName,
		Spec: httpserver.Spec{},
	}
}

func (b *httpServerSpecBuilder) buildWithOutRules(spec *httpserver.Spec) *httpServerSpecBuilder {
	var newSpec httpserver.Spec = *spec
	newSpec.Rules = nil // clear the rule, faasController will management them by itself
	b.Spec = newSpec
	return b
}

func (b *httpServerSpecBuilder) buildWithRules(spec *httpserver.Spec) *httpServerSpecBuilder {
	b.Spec = *spec
	return b
}

func (b *httpServerSpecBuilder) jsonConfig() string {
	buff, err := codectool.MarshalJSON(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", b, err)
	}
	return string(buff)
}

func (b *pipelineSpecBuilder) jsonConfig() string {
	buff, err := codectool.MarshalJSON(b)
	if err != nil {
		logger.Errorf("BUG: marshal %#v to json failed: %v", b, err)
	}
	return string(buff)
}

func (b *pipelineSpecBuilder) appendReqAdaptor(funcSpec *spec.Spec, faasNamespace, faasHostSuffix string) *pipelineSpecBuilder {
	adaptorName := "requestAdaptor"
	b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: adaptorName})

	b.Filters = append(b.Filters, map[string]interface{}{
		"kind":   builder.RequestAdaptorKind,
		"name":   adaptorName,
		"method": funcSpec.RequestAdaptor.Method,
		"path":   funcSpec.RequestAdaptor.Path,
		"header": funcSpec.RequestAdaptor.Header,

		// let faas Provider's gateway recognized this function by Host field
		"host": funcSpec.Name + "." + faasNamespace + "." + faasHostSuffix,
	})

	return b
}

func (b *pipelineSpecBuilder) appendProxy(faasNetworkLayerURL string) *pipelineSpecBuilder {
	mainServers := []*proxy.Server{
		{
			URL:      faasNetworkLayerURL,
			KeepHost: true, // Keep the host of the requests as they route to functions.
		},
	}

	backendName := "faasBackend"

	lb := &proxy.LoadBalanceSpec{}

	b.Flow = append(b.Flow, pipeline.FlowNode{FilterName: backendName})
	b.Filters = append(b.Filters, map[string]interface{}{
		"kind": proxy.Kind,
		"name": backendName,
		"mainPool": &proxy.ServerPoolSpec{
			BaseServerPoolSpec: proxy.BaseServerPoolSpec{
				Servers:     mainServers,
				LoadBalance: lb,
			},
		},
	})

	return b
}

// Init creates a default ingress HTTPServer.
func (ings *ingressServer) Init() error {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	if ings.httpServer != nil {
		return nil
	}
	spec := ings.superSpec.ObjectSpec().(*spec.Admin)

	ings.faasNetworkLayerURL = spec.Knative.NetworkLayerURL
	ings.faasHostSuffix = spec.Knative.HostSuffix
	ings.faasNamespace = spec.Knative.Namespace

	builder := newHTTPServerSpecBuilder(ings.superSpec.Name())
	builder.buildWithOutRules(spec.HTTPServer)
	superSpec, err := supervisor.NewSpec(builder.jsonConfig())
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", builder.jsonConfig(), err)
		return err
	}

	ings.httpServerSpec = superSpec
	entity, err := ings.tc.CreateTrafficGateForSpec(ings.namespace, superSpec)
	if err != nil {
		return fmt.Errorf("create http server %s failed: %v", superSpec.Name(), err)
	}
	ings.httpServer = entity
	return nil
}

func (ings *ingressServer) updateHTTPServer(spec *httpserver.Spec) error {
	builder := newHTTPServerSpecBuilder(ings.superSpec.Name())
	builder.buildWithRules(spec)

	var err error
	ings.httpServerSpec, err = supervisor.NewSpec(builder.jsonConfig())
	if err != nil {
		return fmt.Errorf("BUG: new spec: %s failed: %v", builder.jsonConfig(), err)
	}
	_, err = ings.tc.ApplyTrafficGateForSpec(ings.namespace, ings.httpServerSpec)
	if err != nil {
		return fmt.Errorf("apply http server %s failed: %v", ings.httpServerSpec.Name(), err)
	}
	return nil
}

func (ings *ingressServer) find(pipeline string) int {
	spec := ings.httpServerSpec.ObjectSpec().(*httpserver.Spec)
	index := -1
	for idx, v := range spec.Rules {
		for _, p := range v.Paths {
			if p.Backend == pipeline {
				index = idx
				break
			}
		}
	}
	return index
}

func (ings *ingressServer) add(pipeline string) error {
	spec := ings.httpServerSpec.ObjectSpec().(*httpserver.Spec)
	index := ings.find(pipeline)
	// not backend as function's pipeline
	if index == -1 {
		rule := &routers.Rule{
			Paths: []*routers.Path{
				{
					PathPrefix: "/",
					Headers: []*routers.Header{
						{
							Key:    ingressFunctionKey,
							Values: []string{pipeline},
						},
					},
					Backend: pipeline,
				},
			},
		}
		spec.Rules = append(spec.Rules, rule)
		if err := ings.updateHTTPServer(spec); err != nil {
			logger.Errorf("update http server failed: %v ", err)
		}
	}
	return nil
}

func (ings *ingressServer) remove(pipeline string) error {
	spec := ings.httpServerSpec.ObjectSpec().(*httpserver.Spec)
	index := ings.find(pipeline)

	if index != -1 {
		spec.Rules = append(spec.Rules[:index], spec.Rules[index+1:]...)
		return ings.updateHTTPServer(spec)
	}
	return nil
}

// Put puts pipeline named by faas function's name with a requestAdaptor and proxy
func (ings *ingressServer) Put(funcSpec *spec.Spec) error {
	builder := newPipelineSpecBuilder(funcSpec.Name)
	builder.appendReqAdaptor(funcSpec, ings.faasNamespace, ings.faasHostSuffix)
	builder.appendProxy(ings.faasNetworkLayerURL)

	jsonConfig := builder.jsonConfig()
	superSpec, err := supervisor.NewSpec(jsonConfig)
	if err != nil {
		logger.Errorf("new spec for %s failed: %v", jsonConfig, err)
		return err
	}
	if _, err = ings.tc.CreatePipelineForSpec(ings.namespace, superSpec); err != nil {
		return fmt.Errorf("create http pipeline %s failed: %v", superSpec.Name(), err)
	}
	ings.add(funcSpec.Name)
	ings.pipelines[funcSpec.Name] = struct{}{}

	return nil
}

// Delete deletes one ingress pipeline according to the function's name.
func (ings *ingressServer) Delete(functionName string) {
	ings.mutex.Lock()
	_, exist := ings.pipelines[functionName]
	if exist {
		delete(ings.pipelines, functionName)
	}
	ings.mutex.Unlock()
	if exist {
		ings.remove(functionName)
	}
}

// Update updates ingress's all pipeline by all functions map. In Easegress scenario,
// this function can add back all function's pipeline in store.
func (ings *ingressServer) Update(allFunctions map[string]*spec.Function) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()
	for _, v := range allFunctions {
		index := ings.find(v.Spec.Name)
		_, exist := ings.pipelines[v.Spec.Name]

		if v.Status.State == spec.ActiveState {
			// need to add rule in HTTPServer or create this pipeline
			// especially in reboot scenario.
			if index == -1 || !exist {
				err := ings.Put(v.Spec)
				if err != nil {
					logger.Errorf("ingress add back local pipeline: %s, failed: %v",
						v.Spec.Name, err)
					continue
				}
			}
		} else {
			// Function not ready, then remove it from HTTPServer's route rule
			if index != -1 {
				ings.remove(v.Spec.Name)
			}
		}
	}
}

// Stop stops one ingress pipeline according to the function's name.
func (ings *ingressServer) Stop(functionName string) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.remove(functionName)
}

// Start starts one ingress pipeline according to the function's name.
func (ings *ingressServer) Start(functionName string) {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.add(functionName)
}

// Close closes the Egress HTTPServer and Pipelines
func (ings *ingressServer) Close() {
	ings.mutex.Lock()
	defer ings.mutex.Unlock()

	ings.tc.DeleteTrafficGate(ings.namespace, ings.httpServer.Spec().Name())
	for name := range ings.pipelines {
		ings.tc.DeletePipeline(ings.namespace, name)
	}
}
