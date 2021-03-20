package worker

import (
	"fmt"

	"github.com/fatih/structs"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/util/jmxtool"
)

const (
	easeAgentConfigManager = "com.megaease.easeagent:type=ConfigManager"
)

type (
	// ObservabilityManager is the manager for observability.
	ObservabilityManager struct {
		serviceName   string
		jolokiaClient *jmxtool.JolokiaClient
	}
)

// NewObservabilityServer creates an ObservabilityServer.
func NewObservabilityServer(serviceName string) *ObservabilityManager {
	client := jmxtool.NewJolokiaClient("localhost", "8778", "jolokia")
	return &ObservabilityManager{
		serviceName:   serviceName,
		jolokiaClient: client,
	}
}

// UpdateObservability updates observability.
func (server *ObservabilityManager) UpdateObservability(serviceName string, newObservability *spec.Observability) error {
	paramsMap := structs.Map(newObservability)
	args := []interface{}{paramsMap}

	logger.Infof("Update Service %s Observability, new Observability is %s.", serviceName, newObservability)

	_, err := server.jolokiaClient.ExecuteMbeanOperation(easeAgentConfigManager, "updateObservability", args)
	if err != nil {
		return fmt.Errorf("updateObservability service %s observability failed: %v", serviceName, err)
	}

	return nil
}
