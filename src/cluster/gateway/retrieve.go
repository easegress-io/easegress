package gateway

import (
	"fmt"
	"sort"

	"cluster"
	"common"
	"config"
	"logger"
	"pipelines"
	"plugins"
)

func unpackReqRetrieve(payload []byte) (ReqRetrieve, error) {
	reqRetrieve := ReqRetrieve{}
	err := cluster.Unpack(payload, &reqRetrieve)
	if err != nil {
		return reqRetrieve, fmt.Errorf("wrong payload format: want ReqRetrieve")
	}

	if len(reqRetrieve.Filter) < 1 {
		return reqRetrieve, fmt.Errorf("wrong payload format: filter is empty")
	}

	return reqRetrieve, nil
}

// retrieveResult could decrease degree of coupling between nrw mode and business.
func (gc *GatewayCluster) retrieveResult(payload []byte) ([]byte, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("empty payload")
	}
	reqRetrieve, err := unpackReqRetrieve(payload[1:])
	if err != nil {
		return nil, err
	}

	switch RetrieveType(reqRetrieve.Filter[0]) {
	case retrievePlugins:
		filter := FilterRetrievePlugins{}
		err := cluster.Unpack(reqRetrieve.Filter[1:], &filter)
		if err != nil {
			return nil, fmt.Errorf("wrong filter format: want %T", filter)
		}
		plugs, err := gc.model.GetPlugins(filter.NamePattern, filter.Types)
		if err != nil {
			return nil, fmt.Errorf("server error: get plugins failed: %v", err)
		}

		result := ResultRetrievePlugins{}
		result.Plugins = make([]config.PluginSpec, 0)
		for _, plug := range plugs {
			spec := config.PluginSpec{
				Type:   plug.Type(),
				Config: plug.Config(),
			}
			result.Plugins = append(result.Plugins, spec)
		}
		resultBuff, err := cluster.Pack(result, uint8(retrievePlugins))
		if err != nil {
			return nil, fmt.Errorf("server error: pack %#v failed: %v", result, err)
		}

		return resultBuff, nil
	case retrievePipelines:
		filter := FilterRetrievePipelines{}
		err := cluster.Unpack(reqRetrieve.Filter[1:], &filter)
		if err != nil {
			return nil, fmt.Errorf("wrong filter format: want %T", filter)
		}
		pipes, err := gc.model.GetPipelines(filter.NamePattern, filter.Types)
		if err != nil {
			return nil, fmt.Errorf("server error: get pipelines failed: %v", err)
		}

		result := ResultRetrievePipelines{}
		result.Pipelines = make([]config.PipelineSpec, 0)
		for _, pipe := range pipes {
			spec := config.PipelineSpec{
				Type:   pipe.Type(),
				Config: pipe.Config(),
			}
			result.Pipelines = append(result.Pipelines, spec)
		}
		resultBuff, err := cluster.Pack(result, uint8(retrievePipelines))
		if err != nil {
			return nil, fmt.Errorf("server error: pack %#v failed: %v", result, err)
		}

		return resultBuff, nil
	case retrievePluginTypes:
		result := ResultRetrievePluginTypes{}
		result.PluginTypes = make([]string, 0)
		for _, typ := range plugins.GetAllTypes() {
			if !common.StrInSlice(typ, result.PluginTypes) {
				result.PluginTypes = append(result.PluginTypes, typ)
			}
		}
		sort.Strings(result.PluginTypes)

		resultBuff, err := cluster.Pack(result, uint8(retrievePluginTypes))
		if err != nil {
			return nil, fmt.Errorf("server error: pack %#v failed: %v", result, err)
		}

		return resultBuff, nil
	case retrievePipelineTypes:
		result := ResultRetrievePipelineTypes{}
		result.PipelineTypes = make([]string, 0)
		for _, typ := range pipelines.GetAllTypes() {
			if !common.StrInSlice(typ, result.PipelineTypes) {
				result.PipelineTypes = append(result.PipelineTypes, typ)
			}
		}
		sort.Strings(result.PipelineTypes)

		resultBuff, err := cluster.Pack(result, uint8(retrievePipelineTypes))
		if err != nil {
			return nil, fmt.Errorf("server error: pack %#v failed: %v", result, err)
		}

		return resultBuff, nil
	default:
		return nil, fmt.Errorf("unsupported filter type")
	}

}

func (gc *GatewayCluster) handleReadModeRetrieve(req *cluster.RequestEvent) {
	respond := func(result []byte, e error) {
		resp := RespRetrieve{
			Err:    NewRetrieveErr(e.Error()),
			Result: result,
		}
		respBuff, err := cluster.Pack(resp, uint8(retrieveMessage))
		if err != nil {
			logger.Errorf("[BUG: pack %#v failed: %v]", resp, err)
			return
		}

		err = req.Respond(respBuff)
		if err != nil {
			logger.Errorf("[respond %s to request %s, node %s failed: %v]", respBuff, req.RequestName, req.RequestNodeName, err)
			return
		}
	}

	result, e := gc.retrieveResult(req.RequestPayload)
	respond(result, e)
}

func (gc *GatewayCluster) handleWriteModeRetrieve(event *cluster.RequestEvent) {
	// TODO
}
