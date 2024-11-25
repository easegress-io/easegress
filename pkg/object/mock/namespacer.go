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

// Package mock implements some mock objects.
package mock

import (
	"fmt"
	"strings"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/httpserver"
	"github.com/megaease/easegress/v2/pkg/object/pipeline"
	"github.com/megaease/easegress/v2/pkg/object/trafficcontroller"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// Category is the category of MockNamespacer.
	Category = supervisor.CategoryBusinessController
	// Kind is the kind of MockNamespacer
	Kind = "MockNamespacer"
)

func init() {
	supervisor.Register(&MockNamespacer{})
	api.RegisterObject(&api.APIResource{
		Category: Category,
		Kind:     Kind,
		Name:     strings.ToLower(Kind),
		Aliases:  []string{"mocknamespacer"},
	})
}

type (
	// MockNamespacer create a mock namespace for test
	MockNamespacer struct {
		spec *Spec
		tc   *trafficcontroller.TrafficController
	}

	// Spec is the MockNamespacer spec
	Spec struct {
		Namespace   string            `json:"namespace"`
		HTTPServers []NamedHTTPServer `json:"httpServers"`
		Pipelines   []NamedPipeline   `json:"pipelines"`
	}

	NamedPipeline struct {
		Kind          string `json:"kind"`
		Name          string `json:"name"`
		pipeline.Spec `json:",inline"`
	}

	NamedHTTPServer struct {
		Kind            string `json:"kind"`
		Name            string `json:"name"`
		httpserver.Spec `json:",inline"`
	}
)

// Validate validates the MockNamespacer Spec.
func (spec *Spec) Validate() error {
	if spec.Namespace == "" {
		return fmt.Errorf("namespace is empty")
	}
	for _, server := range spec.HTTPServers {
		if server.Name == "" {
			return fmt.Errorf("httpserver name is empty")
		}
	}
	for _, pipeline := range spec.Pipelines {
		if pipeline.Name == "" {
			return fmt.Errorf("pipeline name is empty")
		}
	}
	return nil
}

// Category returns the category of MockNamespacer.
func (mn *MockNamespacer) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of MockNamespacer.
func (mn *MockNamespacer) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of MockNamespacer.
func (mn *MockNamespacer) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes MockNamespacer.
func (mn *MockNamespacer) Init(superSpec *supervisor.Spec) {
	spec := superSpec.ObjectSpec().(*Spec)
	mn.spec = spec
	mn.tc = getTrafficController(superSpec.Super())

	for _, server := range mn.spec.HTTPServers {
		jsonConfig, err := codectool.MarshalJSON(server)
		if err != nil {
			logger.Errorf("marshal httpserver failed: %v", err)
			continue
		}
		s, err := supervisor.NewSpec(string(jsonConfig))
		if err != nil {
			logger.Errorf("new httpserver spec failed: %v", err)
			continue
		}
		mn.tc.CreateTrafficGateForSpec(spec.Namespace, s)
	}
	for _, pipeline := range mn.spec.Pipelines {
		jsonConfig, err := codectool.MarshalJSON(pipeline)
		if err != nil {
			logger.Errorf("marshal pipeline failed: %v", err)
			continue
		}
		s, err := supervisor.NewSpec(string(jsonConfig))
		if err != nil {
			logger.Errorf("new pipeline spec failed: %v", err)
			continue
		}
		mn.tc.CreatePipelineForSpec(spec.Namespace, s)
	}
}

// Inherit inherits previous generation of MockNamespacer.
func (mn *MockNamespacer) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.Close()
	mn.Init(superSpec)
}

func getTrafficController(super *supervisor.Supervisor) *trafficcontroller.TrafficController {
	entity, exists := super.GetSystemController(trafficcontroller.Kind)
	if !exists {
		return nil
	}
	tc, ok := entity.Instance().(*trafficcontroller.TrafficController)
	if !ok {
		return nil
	}
	return tc
}

// Status returns the status of MockNamespacer.
func (mn *MockNamespacer) Status() *supervisor.Status {
	return &supervisor.Status{
		ObjectStatus: nil,
	}
}

// Close closes MockNamespacer.
func (mn *MockNamespacer) Close() {
	for _, server := range mn.spec.HTTPServers {
		mn.tc.DeleteTrafficGate(mn.spec.Namespace, server.Name)
	}
	for _, pipeline := range mn.spec.Pipelines {
		mn.tc.DeletePipeline(mn.spec.Namespace, pipeline.Name)
	}
}
