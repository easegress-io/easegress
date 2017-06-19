package gateway

import (
	"fmt"
	"time"
)

// operation
func (gc *GatewayCluster) CreatePlugin(group string, timeout time.Duration, seqSnapshot uint64, syncAll bool,
	typ string, conf []byte) *ClusterError {

	operation := Operation{
		ContentCreatePlugin: &ContentCreatePlugin{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)create_plugin", group)
	return gc.issueOperation(group, timeout, requestName, seqSnapshot, syncAll, &operation)
}

func (gc *GatewayCluster) UpdatePlugin(group string, timeout time.Duration, seqSnapshot uint64, syncAll bool,
	typ string, conf []byte) *ClusterError {

	operation := Operation{
		ContentUpdatePlugin: &ContentUpdatePlugin{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)update_plugin", group)

	return gc.issueOperation(group, timeout, requestName, seqSnapshot, syncAll, &operation)
}

func (gc *GatewayCluster) DeletePlugin(group string, timeout time.Duration, seqSnapshot uint64, syncAll bool,
	name string) *ClusterError {

	operation := Operation{
		ContentDeletePlugin: &ContentDeletePlugin{
			Name: name,
		},
	}

	requestName := fmt.Sprintf("(group:%s)delete_plugin", group)

	return gc.issueOperation(group, timeout, requestName, seqSnapshot, syncAll, &operation)
}

func (gc *GatewayCluster) CreatePipeline(group string, timeout time.Duration, seqSnapshot uint64, syncAll bool,
	typ string, conf []byte) *ClusterError {

	operation := Operation{
		ContentCreatePipeline: &ContentCreatePipeline{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)create_pipeline", group)

	return gc.issueOperation(group, timeout, requestName, seqSnapshot, syncAll, &operation)
}

func (gc *GatewayCluster) UpdatePipeline(group string, timeout time.Duration, seqSnapshot uint64, syncAll bool,
	typ string, conf []byte) *ClusterError {
	operation := Operation{
		ContentUpdatePipeline: &ContentUpdatePipeline{
			Type:   typ,
			Config: conf,
		},
	}

	requestName := fmt.Sprintf("(group:%s)update_pipeline", group)

	return gc.issueOperation(group, timeout, requestName, seqSnapshot, syncAll, &operation)
}

func (gc *GatewayCluster) DeletePipeline(group string, timeout time.Duration, seqSnapshot uint64, syncAll bool,
	name string) *ClusterError {

	operation := Operation{
		ContentDeletePipeline: &ContentDeletePipeline{
			Name: name,
		},
	}

	requestName := fmt.Sprintf("(group:%s)delete_pipeline", group)

	return gc.issueOperation(group, timeout, requestName, seqSnapshot, syncAll, &operation)
}

// retrieve
func (gc *GatewayCluster) RetrievePlugins(group string, timeout time.Duration, syncAll bool,
	NamePattern string, types []string) ([]byte, *ClusterError) {

	filter := FilterRetrievePlugins{
		NamePattern: NamePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_plugins", group)

	return gc.issueRetrieve(group, timeout, requestName, syncAll, &filter)
}

func (gc *GatewayCluster) RetrievePipelines(group string, timeout time.Duration, syncAll bool,
	NamePattern string, types []string) ([]byte, *ClusterError) {

	filter := FilterRetrievePipelines{
		NamePattern: NamePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_pipelines", group)

	return gc.issueRetrieve(group, timeout, requestName, syncAll, &filter)
}

func (gc *GatewayCluster) RetrievePluginTypes(group string, timeout time.Duration,
	syncAll bool) ([]byte, *ClusterError) {

	requestName := fmt.Sprintf("(group:%s)retrive_plugin_types", group)

	return gc.issueRetrieve(group, timeout, requestName, syncAll, &FilterRetrievePluginTypes{})
}

func (gc *GatewayCluster) RetrievePipelineTypes(group string, timeout time.Duration,
	syncAll bool) ([]byte, *ClusterError) {

	requestName := fmt.Sprintf("(group:%s)retrive_pipeline_types", group)

	return gc.issueRetrieve(group, timeout, requestName, syncAll, &FilterRetrievePluginTypes{})
}

// statistics
func (gc *GatewayCluster) StatPipelineIndicatorNames(group string, timeout time.Duration,
	pipelineName string) ([]byte, *ClusterError) {

	filter := FilterPipelineIndicatorNames{
		PipelineName: pipelineName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_names)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}

func (gc *GatewayCluster) StatPipelineIndicatorValue(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, *ClusterError) {

	filter := FilterPipelineIndicatorValue{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_value)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}

func (gc *GatewayCluster) StatPipelineIndicatorDesc(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, *ClusterError) {

	filter := FilterPipelineIndicatorDesc{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_pipleine_indicator_desc)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}

func (gc *GatewayCluster) StatPluginIndicatorNames(group string, timeout time.Duration,
	pipelineName, pluginName string) ([]byte, *ClusterError) {

	filter := FilterPluginIndicatorNames{
		PipelineName: pipelineName,
		PluginName:   pluginName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_names)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}

func (gc *GatewayCluster) StatPluginIndicatorValue(group string, timeout time.Duration,
	pipelineName, pluginName, indicatorName string) ([]byte, *ClusterError) {

	filter := FilterPluginIndicatorValue{
		PipelineName:  pipelineName,
		PluginName:    pluginName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_value)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}

func (gc *GatewayCluster) StatPluginIndicatorDesc(group string, timeout time.Duration,
	pipelineName, pluginName, indicatorName string) ([]byte, *ClusterError) {

	filter := FilterPluginIndicatorDesc{
		PipelineName:  pipelineName,
		PluginName:    pluginName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_plugin_indicator_desc)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}

func (gc *GatewayCluster) StatTaskIndicatorNames(group string, timeout time.Duration,
	pipelineName string) ([]byte, *ClusterError) {

	filter := FilterTaskIndicatorNames{
		PipelineName: pipelineName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_names)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}

func (gc *GatewayCluster) StatTaskIndicatorValue(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, *ClusterError) {

	filter := FilterTaskIndicatorValue{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_value)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}

func (gc *GatewayCluster) StatTaskIndicatorDesc(group string, timeout time.Duration,
	pipelineName, indicatorName string) ([]byte, *ClusterError) {

	filter := FilterTaskIndicatorDesc{
		PipelineName:  pipelineName,
		IndicatorName: indicatorName,
	}

	requestName := fmt.Sprintf("(group(%s)stat_task_indicator_desc)", group)

	return gc.issueStat(group, timeout, requestName, &filter)
}
