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

package spec

import (
	"fmt"

	"github.com/megaease/easegress/v2/pkg/filters/builder"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"

	"github.com/megaease/easegress/v2/pkg/object/httpserver"
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
		SyncInterval string `json:"syncInterval" jsonschema:"required,format=duration"`

		// Provider is the FaaSProvider.
		Provider string `json:"provider" jsonschema:"required"`

		// HTTPServer is the HTTP traffic gate for accepting ingress traffic.
		HTTPServer *httpserver.Spec `json:"httpServer" jsonschema:"required"`

		// Currently we only supports knative type faas provider, so this filed should be
		// "required" right now.
		Knative *Knative `json:"knative" jsonschema:"required"`
	}

	// Function contains the FaaSFunction's spec ,runtime status with a build-in fsm.
	Function struct {
		Spec   *Spec   `json:"spec" jsonschema:"required"`
		Status *Status `json:"status" jsonschema:"required"`
		Fsm    *FSM    `json:"fsm,omitempty"`
	}

	// Spec is the spec of FaaSFunction.
	Spec struct {
		Name           string `json:"name" jsonschema:"required"`
		Image          string `json:"image" jsonschema:"required"`
		Port           int    `json:"port,omitempty"`
		AutoScaleType  string `json:"autoScaleType" jsonschema:"required"`
		AutoScaleValue string `json:"autoScaleValue" jsonschema:"required"`
		MinReplica     int    `json:"minReplica,omitempty"`
		MaxReplica     int    `json:"maxReplica,omitempty"`
		LimitCPU       string `json:"limitCPU,omitempty"`
		LimitMemory    string `json:"limitMemory,omitempty"`
		RequestCPU     string `json:"requestCPU,omitempty"`
		RequestMemory  string `json:"requestMemory,omitempty"`

		RequestAdaptor *builder.RequestAdaptorSpec `json:"requestAdaptor" jsonschema:"required"`
	}

	// Status is the status of faas function.
	Status struct {
		Name    string            `json:"name" jsonschema:"required"`
		State   State             `json:"state" jsonschema:"required"`
		Event   Event             `json:"event" jsonschema:"required"`
		ExtData map[string]string `json:"extData,omitempty"`
	}

	// Knative is the faas provider Knative.
	Knative struct {
		HostSuffix      string `json:"hostSuffix" jsonschema:"required"`
		NetworkLayerURL string `json:"networkLayerURL" jsonschema:"required,format=uri"`

		Namespace string `json:"namespace,omitempty"`
		Timeout   string `json:"timeout,omitempty" jsonschema:"format=duration"`
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
