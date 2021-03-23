package main

import (
	"fmt"
	yaml "gopkg.in/yaml.v2"

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

// UpdateService updates service.
func (server *ObservabilityManager) UpdateService(newService *spec.Service, version int64) error {

	buff, err := yaml.Marshal(newService)
	if err != nil {
		panic(fmt.Errorf("marsharl new Service %#v failed: %v", err))
	}
	m := make(map[string]string)
	err = yaml.Unmarshal(buff, m)
	if err != nil {
		panic(fmt.Errorf("marsharl new Service %#v failed: %v", err))
	}

	args := []interface{}{m, version}

	logger.Infof("Update Service: %s Observability, new Service is %s", newService.Name, newService)

	_, err = server.jolokiaClient.ExecuteMbeanOperation(easeAgentConfigManager, updateServiceOperation, args)
	if err != nil {
		return fmt.Errorf("UpdateService service: %s  failed: %v", newService.Name, err)
	}

	return nil
}
