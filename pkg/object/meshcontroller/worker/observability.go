package worker

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v2"

	"github.com/megaease/easegateway/pkg/object/meshcontroller/spec"
	"github.com/megaease/easegateway/pkg/util/jmxtool"

	yamljsontool "github.com/ghodss/yaml"
)

const (
	easeAgentConfigManager = "com.megaease.easeagent:type=ConfigManager"
	updateServiceOperation = "updateService"
	updateCanaryOperation  = "updateCanary"
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
		return fmt.Errorf("marshal %#v to yaml failed: %v", newService, err)
	}
	jsonBytes, err := yamljsontool.YAMLToJSON(buff)
	if err != nil {
		return fmt.Errorf("convert yaml %s to json failed: %v", buff, err)
	}

	var params interface{}
	err = json.Unmarshal(jsonBytes, &params)
	if err != nil {
		return fmt.Errorf("unmarshal %s to json failed: %v", jsonBytes, err)
	}
	args := []interface{}{params, version}

	_, err = server.jolokiaClient.ExecuteMbeanOperation(easeAgentConfigManager, updateServiceOperation, args)
	if err != nil {
		return fmt.Errorf("execute mbean operation failed: %v", err)
	}

	return nil
}

// UpdateCanary updates canary.
func (server *ObservabilityManager) UpdateCanary(globalHeaders *spec.GlobalCanaryHeaders, version int64) error {
	buff, err := yaml.Marshal(globalHeaders)
	if err != nil {
		return fmt.Errorf("marshal %#v to yaml failed: %v", globalHeaders, err)
	}

	jsonBytes, err := yamljsontool.YAMLToJSON(buff)
	if err != nil {
		return fmt.Errorf("convert yaml %s to json failed: %v", buff, err)
	}

	var params interface{}
	err = json.Unmarshal(jsonBytes, &params)
	if err != nil {
		return fmt.Errorf("unmarshal %s to json failed: %v", jsonBytes, err)
	}

	args := []interface{}{params, version}
	_, err = server.jolokiaClient.ExecuteMbeanOperation(easeAgentConfigManager, updateCanaryOperation, args)
	if err != nil {
		return fmt.Errorf("execute mbean operation failed: %v", err)
	}

	return nil
}
