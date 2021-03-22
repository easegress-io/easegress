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
	updateServiceOperation = "updateService"
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

	logger.Infof("Update Service: %s Observability, new Observability is %s", serviceName, newObservability)

	_, err := server.jolokiaClient.ExecuteMbeanOperation(easeAgentConfigManager, updateServiceOperation, args)
	if err != nil {
		return fmt.Errorf("update service: %s observability failed: %v", serviceName, err)
	}

	return nil
}


// UpdateService updates service.
func (server *ObservabilityManager) UpdateService(newService *spec.Service) error {
	paramsMap := structs.Map(newService)
	args := []interface{}{paramsMap}

	logger.Infof("Update Service: %s Observability, new Service is %s", newService.Name, newService)

	_, err := server.jolokiaClient.ExecuteMbeanOperation(easeAgentConfigManager, updateServiceOperation, args)
	if err != nil {
		return fmt.Errorf("UpdateService service: %s  failed: %v", newService.Name, err)
	}

	return nil
}
