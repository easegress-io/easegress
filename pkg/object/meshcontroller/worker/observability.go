package worker

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/util/jmxtool"
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
