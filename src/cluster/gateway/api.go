package gateway

import (
	"fmt"
	"time"
)

// operation
func (gc *GatewayCluster) CreatePlugin(group string, syncAll bool, timeout time.Duration,
	typ string, conf []byte) *HTTPError {
	operation := Operation{
		ContentCreatePlugin: &ContentCreatePlugin{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)create_plugin", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, &operation)
}

func (gc *GatewayCluster) UpdatePlugin(group string, syncAll bool, timeout time.Duration,
	typ string, conf []byte) *HTTPError {
	operation := Operation{
		ContentUpdatePlugin: &ContentUpdatePlugin{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)update_plugin", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, &operation)
}

func (gc *GatewayCluster) DeletePlugin(group string, syncAll bool, timeout time.Duration,
	name string) *HTTPError {
	operation := Operation{
		ContentDeletePlugin: &ContentDeletePlugin{
			Name: name,
		},
	}

	requestName := fmt.Sprintf("(group:%s)delete_plugin", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, &operation)
}

func (gc *GatewayCluster) CreatePipeline(group string, syncAll bool, timeout time.Duration,
	typ string, conf []byte) *HTTPError {
	operation := Operation{
		ContentCreatePipeline: &ContentCreatePipeline{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)create_pipeline", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, &operation)
}

func (gc *GatewayCluster) UpdatePipeline(group string, syncAll bool, timeout time.Duration,
	typ string, conf []byte) *HTTPError {
	operation := Operation{
		ContentUpdatePipeline: &ContentUpdatePipeline{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)update_pipeline", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, &operation)
}

func (gc *GatewayCluster) DeletePipeline(group string, syncAll bool, timeout time.Duration,
	name string) *HTTPError {
	operation := Operation{
		ContentDeletePipeline: &ContentDeletePipeline{
			Name: name,
		},
	}

	requestName := fmt.Sprintf("(group:%s)delete_pipeline", group)
	return gc.issueOperation(group, syncAll, timeout, requestName, &operation)
}

// retrive
func (gc *GatewayCluster) RetrievePlugins(group string, syncAll bool, timeout time.Duration,
	NamePattern string, types []string) ([]byte, *HTTPError) {
	filter := FilterRetrievePlugins{
		NamePattern: NamePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_plugins", group)
	return gc.issueRetrieve(group, syncAll, timeout, requestName, &filter)
}

func (gc *GatewayCluster) RetrievePipelines(group string, syncAll bool, timeout time.Duration,
	NamePattern string, types []string) ([]byte, *HTTPError) {
	filter := FilterRetrievePipelines{
		NamePattern: NamePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_pipelines", group)
	return gc.issueRetrieve(group, syncAll, timeout, requestName, &filter)
}

func (gc *GatewayCluster) RetrievePluginTypes(group string, syncAll bool,
	timeout time.Duration) ([]byte, *HTTPError) {

	requestName := fmt.Sprintf("(group:%s)retrive_plugin_types", group)
	return gc.issueRetrieve(group, syncAll, timeout, requestName, &FilterRetrievePluginTypes{})
}

func (gc *GatewayCluster) RetrievePipelineTypes(group string, syncAll bool,
	timeout time.Duration) ([]byte, *HTTPError) {

	requestName := fmt.Sprintf("(group:%s)retrive_pipeline_types", group)
	return gc.issueRetrieve(group, syncAll, timeout, requestName, &FilterRetrievePluginTypes{})
}

// stat
func (gc *GatewayCluster) StatPipelineIndicatorNames(group string, timeout time.Duration,
	pipelineName string) ([]byte, *HTTPError) {
	filter := FilterPipelineIndicatorNames{
		PipelineName: pipelineName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_names)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}

func (gc *GatewayCluster) StatPipelineIndicatorValue(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, *HTTPError) {
	filter := FilterPipelineIndicatorValue{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_value)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}

func (gc *GatewayCluster) StatPipelineIndicatorDesc(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, *HTTPError) {
	filter := FilterPipelineIndicatorDesc{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_desc)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}

func (gc *GatewayCluster) StatPluginIndicatorNames(group string, timeout time.Duration,
	pipelineName, pluginName string) ([]byte, *HTTPError) {
	filter := FilterPluginIndicatorNames{
		PipelineName: pipelineName,
		PluginName:   pluginName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_names)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}

func (gc *GatewayCluster) StatPluginIndicatorValue(group string, timeout time.Duration,
	pipelineName, pluginName, indicatorName string) ([]byte, *HTTPError) {
	filter := FilterPluginIndicatorValue{
		PipelineName:  pipelineName,
		PluginName:    pluginName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_value)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}

func (gc *GatewayCluster) StatPluginIndicatorDesc(group string, timeout time.Duration,
	pipelineName, pluginName, indicatorName string) ([]byte, *HTTPError) {
	filter := FilterPluginIndicatorDesc{
		PipelineName:  pipelineName,
		PluginName:    pluginName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_desc)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}

func (gc *GatewayCluster) StatTaskIndicatorNames(group string, timeout time.Duration,
	pipelineName string) ([]byte, *HTTPError) {
	filter := FilterTaskIndicatorNames{
		PipelineName: pipelineName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_names)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}

func (gc *GatewayCluster) StatTaskIndicatorValue(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, *HTTPError) {
	filter := FilterTaskIndicatorValue{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_value)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}

func (gc *GatewayCluster) StatTaskIndicatorDesc(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, *HTTPError) {
	filter := FilterTaskIndicatorDesc{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_desc)", group)
	return gc.issueStat(group, timeout, requestName, filter)
}
