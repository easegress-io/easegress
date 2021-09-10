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

	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegress/pkg/util/jmxtool"
)

const (
	easeAgentConfigManager = "com.megaease.easeagent:type=ConfigManager"
	updateServiceOperation = "updateService"
	updateCanaryOperation  = "updateCanary"
)

type (
	// ObservabilityManager is the manager for observability.
	ObservabilityManager struct {
		serviceName string
		agentClient *jmxtool.AgentClient
	}
)

// NewObservabilityServer creates an ObservabilityServer.
func NewObservabilityServer(serviceName string) *ObservabilityManager {
	client := jmxtool.NewAgentClient("localhost", "9900")
	return &ObservabilityManager{
		serviceName: serviceName,
		agentClient: client,
	}
}

// UpdateService updates service.
func (server *ObservabilityManager) UpdateService(newService *spec.Service, version int64) error {
	err := server.agentClient.UpdateService(newService, version)
	if err != nil {
		return fmt.Errorf("Update Service Spec failed: %v ", err)
	}

	return nil
}

// UpdateCanary updates canary.
func (server *ObservabilityManager) UpdateCanary(globalHeaders *spec.GlobalCanaryHeaders, version int64) error {
	err := server.agentClient.UpdateCanary(globalHeaders, version)
	if err != nil {
		return fmt.Errorf("Update Canary Spec: %v ", err)
	}
	return nil
}
