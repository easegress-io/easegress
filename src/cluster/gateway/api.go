package gateway

import (
	"fmt"
	"time"
)

// operation
func (gc *GatewayCluster) CreatePlugin(group string, syncAll bool, timeout time.Duration,
	typ string, conf []byte) error {
	operation := Operation{
		ContentCreatePlugin: &ContentCreatePlugin{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)create_plugin", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, operation)
}

func (gc *GatewayCluster) UpdatePlugin(group string, syncAll bool, timeout time.Duration,
	typ string, conf []byte) error {
	operation := Operation{
		ContentUpdatePlugin: &ContentUpdatePlugin{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)update_plugin", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, operation)
}

func (gc *GatewayCluster) DeletePlugin(group string, syncAll bool, timeout time.Duration,
	name string) error {
	operation := Operation{
		ContentDeletePlugin: &ContentDeletePlugin{
			Name: name,
		},
	}

	requestName := fmt.Sprintf("(group:%s)delete_plugin", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, operation)
}

func (gc *GatewayCluster) CreatePipeline(group string, syncAll bool, timeout time.Duration,
	typ string, conf []byte) error {
	operation := Operation{
		ContentCreatePipeline: &ContentCreatePipeline{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)create_pipeline", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, operation)
}

func (gc *GatewayCluster) UpdatePipeline(group string, syncAll bool, timeout time.Duration,
	typ string, conf []byte) error {
	operation := Operation{
		ContentUpdatePipeline: &ContentUpdatePipeline{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)update_pipeline", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, operation)
}

func (gc *GatewayCluster) DeletePipeline(group string, syncAll bool, timeout time.Duration,
	name string) error {
	operation := Operation{
		ContentDeletePipeline: &ContentDeletePipeline{
			Name: name,
		},
	}

	requestName := fmt.Sprintf("(group:%s)delete_pipeline", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, operation)
}

// retrive
func (gc *GatewayCluster) RetrievePlugins(group string, syncAll bool, timeout time.Duration,
	NamePattern string, types []string) ([]byte, error) {
	return nil, nil
}

func (hc *GatewayCluster) RetrievePipelines(group string, syncAll bool, timeout time.Duration,
	NamePattern string, types []string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) RetrievePluginTypes(group string, syncAll bool, timeout time.Duration) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) RetrievePipelineTypes(group string, syncAll bool, timeout time.Duration) ([]byte, error) {
	return nil, nil
}

// stat
func (gc *GatewayCluster) StatPipelineIndicatorNames(group string, timeout time.Duration,
	pipelineName string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) StatPipelineIndicatorValue(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) StatPipelineIndicatorDesc(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) StatPluginIndicatorNames(group string, timeout time.Duration,
	pipelineName, pluginName string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) StatPluginIndicatorValue(group string, timeout time.Duration,
	pipelineName, pluginName, indicatorName string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) StatPluginIndicatorDesc(group string, timeout time.Duration,
	pipelineName, pluginName, indicatorName string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) StatTaskIndicatorNames(group string, timeout time.Duration,
	pipelineName string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) StatTaskIndicatorValue(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, error) {
	return nil, nil
}

func (gc *GatewayCluster) StatTaskIndicatorDesc(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, error) {
	return nil, nil
}
