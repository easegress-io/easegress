package worker

import (
	"github.com/fatih/structs"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/util/jmxtool"
)

const (
	easeAgentConfigManager = "com.megaease.easeagent:type=ConfigManager"
)

type ObservabilityManager struct {
	serviceName   string
	jolokiaClient *jmxtool.JolokiaClient
}

func NewObservabilityServer(serviceName string) *ObservabilityManager {
	client := jmxtool.NewJolokiaClient("localhost", "8778", "jolokia")
	return &ObservabilityManager{
		serviceName:   serviceName,
		jolokiaClient: client,
	}
}

func (server *ObservabilityManager) UpdateObservability(serviceName string, newObservability *spec.Observability) error {
	paramsMap := structs.Map(newObservability)
	args := []interface{}{paramsMap}
	result, err := server.jolokiaClient.ExecuteMbeanOperation(easeAgentConfigManager, "updateObservability", args)
	if err != nil {
		logger.Errorf("UpdateObservability service :%s observability failed, Observability : %v , Result: %v,err : %v", serviceName, newObservability, result, err)
		return err
	}
	return nil
}
