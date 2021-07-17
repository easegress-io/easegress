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

package spec

import (
	"fmt"

	k8sresource "k8s.io/apimachinery/pkg/api/resource"

	"github.com/megaease/easegress/pkg/filter/requestadaptor"
	"github.com/megaease/easegress/pkg/object/httpserver"
)

const (
	// AutoScaleMetricCPU is name of cpu metric
	AutoScaleMetricCPU = "cpu"
	// AutoScaleMetricConcurrency is the name of concurrency metric
	AutoScaleMetricConcurrency = "concurrency"
	// AutoScaleMetricRPS is the name of rps metric
	AutoScaleMetricRPS = "rps"

	// ProviderKnative is the FaaS provider Knative.
	ProviderKnative = "knative"
)

type (
	// Admin describes the Function.
	Admin struct {
		// SyncInterval is the interval for reconciling local FaaSFunction state and FaaSProvider's.
		SyncInterval string `yaml:"syncInterval" jsonschema:"required,format=duration"`

		// Provider is the FaaSProvider.
		Provider string `yaml:"provider" jsonschema:"required"`

		// HTTPServer is the HTTP traffic gate for accepting ingress traffic.
		HTTPServer *httpserver.Spec `yaml:"httpServer" jsonschema:"required"`

		// Currently we only supports knative type faas provider, so this filed should be
		// "required" right now.
		Knative *Knative `yaml:"knative" jsonschema:"required"`
	}

	// Function contains the FaaSFunction's spec ,runtime status with a build-in fsm.
	Function struct {
		Spec   *Spec   `yaml:"spec" jsonschema:"required"`
		Status *Status `yaml:"status" jsonschema:"required"`
		Fsm    *FSM    `yaml:"fsm" jsonschema:"omitempty"`
	}

	// Spec is the spec of FaaSFunction.
	Spec struct {
		Name           string `yaml:"name" jsonschema:"required"`
		Image          string `yaml:"image" jsonschema:"required"`
		Port           int    `yaml:"port" jsonschema:"omitempty"`
		AutoScaleType  string `yaml:"autoScaleType" jsonschema:"required"`
		AutoScaleValue string `yaml:"autoScaleValue" jsonschema:"required"`
		MinReplica     int    `yaml:"minReplica" jsonschema:"omitempty"`
		MaxReplica     int    `yaml:"maxReplica" jsonschema:"omitempty"`
		LimitCPU       string `yaml:"limitCPU" jsonschema:"omitempty"`
		LimitMemory    string `yaml:"limitMemory" jsonschema:"omitempty"`
		RequestCPU     string `yaml:"requestCPU" jsonschema:"omitempty"`
		RequestMemory  string `yaml:"requestMemory" jsonschema:"omitempty"`

		RequestAdaptor *requestadaptor.Spec `yaml:"requestAdaptor" jsonschema:"required"`
	}

	// Status is the status of faas function.
	Status struct {
		Name    string            `yaml:"name" jsonschema:"required"`
		State   State             `yaml:"state" jsonschema:"required"`
		Event   Event             `yaml:"event" jsonschema:"required"`
		ExtData map[string]string `yaml:"extData" jsonschema:"omitempty"`
	}

	// Knative is the faas provider Knative.
	Knative struct {
		HostSuffix      string `yaml:"hostSuffix" jsonschema:"required"`
		NetworkLayerURL string `yaml:"networkLayerURL" jsonschema:"required,format=uri"`

		Namespace string `yaml:"namespace" jsonschema:"omitempty"`
		Timeout   string `yaml:"timeout" jsonschema:"omitempty,format=duration"`
	}
)

// Validate valid FaaSFunction's spec.
func (spec *Spec) Validate() error {
	if spec.MinReplica > spec.MaxReplica {
		return fmt.Errorf("invalided minreplica: %d and maxreplica: %d", spec.MinReplica, spec.MaxReplica)
	}

	switch spec.AutoScaleType {
	case AutoScaleMetricCPU, AutoScaleMetricConcurrency, AutoScaleMetricRPS:
		//
	default:
		return fmt.Errorf("unknown autoscale type: %s", spec.AutoScaleType)
	}

	checkK8s := func() (errMsg error) {
		defer func() {
			if err := recover(); err != nil {
				errMsg = fmt.Errorf("k8s resource value check failed: %v", err)
			}
		}()

		// check if k8s resource valid or note
		k8sresource.MustParse(spec.LimitMemory)
		k8sresource.MustParse(spec.LimitCPU)
		k8sresource.MustParse(spec.RequestCPU)
		k8sresource.MustParse(spec.RequestMemory)
		return
	}

	return checkK8s()
}

// Next turns function's states into next states by given event.
func (function *Function) Next(event Event) (updated bool, err error) {
	updated = false
	oldState := function.Fsm.Current()
	err = function.Fsm.Next(event)
	if err != nil {
		return
	}

	if oldState != function.Fsm.Current() {
		updated = true
		function.Status.State = function.Fsm.currentState
	}
	function.Status.Event = event
	return
}
