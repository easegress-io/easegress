package gateway

import (
	"fmt"
	"time"

	"cluster"
	"logger"
	"net/http"
)

// operation
func (gc *GatewayCluster) QueryGroupMaxSeq(group string, timeout time.Duration) (uint64, *ClusterError) {
	req := new(ReqQueryGroupMaxSeq)
	requestPayload, err := cluster.PackWithHeader(req, uint8(queryGroupMaxSeqMessage))
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed: %v]",
			uint8(queryGroupMaxSeqMessage), req, err)

		return 0, newClusterError(
			fmt.Sprintf("pack request (header=%d) to %#v failed: %v",
				uint8(queryGroupMaxSeqMessage), req, err),
			InternalServerError)
	}

	requestParam := cluster.RequestParam{
		TargetNodeTags: map[string]string{
			groupTagKey: group,
			modeTagKey:  WriteMode.String(),
		},
		Timeout:            timeout,
		ResponseRelayCount: 1, // fault tolerance on network issue
	}

	requestName := fmt.Sprintf("(group:%s)query_group_max_sequence", group)

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		return 0, newClusterError(fmt.Sprintf("query max sequence failed: %v", err), InternalServerError)
	}

	memberResp, ok := <-future.Response()
	if !ok {
		return 0, newClusterError("query max sequence in the group timeout", TimeoutError)
	}
	if len(memberResp.Payload) == 0 {
		return 0, newClusterError("query max sequence responds empty response", InternalServerError)
	}

	var resp RespQueryGroupMaxSeq
	err = cluster.Unpack(memberResp.Payload[1:], &resp)
	if err != nil {
		return 0, newClusterError(fmt.Sprintf("unpack max sequence response failed: %v", err),
			InternalServerError)
	}

	return uint64(resp), nil
}

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
	NamePattern string, types []string) (*ResultRetrievePlugins, *ClusterError) {

	filter := FilterRetrievePlugins{
		NamePattern: NamePattern,
		Types:       types,
	}

	requestName := fmt.Sprintf("(group:%s)retrive_plugins", group)

	resp, err := gc.issueRetrieve(group, timeout, requestName, syncAll, &filter)
	if err != nil {
		return nil, err
	}

	ret, ok := resp.(*ResultRetrievePlugins)
	if !ok {
		logger.Errorf("[BUG: retrieve plugins returns invalid result, got type %T]", resp)
		return nil, newClusterError("retrieve plugins returns invalid result", InternalServerError)
	}

	return ret, nil
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
