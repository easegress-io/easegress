package nrw

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
	err := Unpack(payload, &reqRetrieve)
	if err != nil {
		return reqRetrieve, fmt.Errorf("wrong payload format: want ReqRetrieve")
	}

	if len(reqRetrieve.Filter) < 1 {
		return reqRetrieve, fmt.Errorf("wrong payload format: filter is empty")
	}

	return reqRetrieve, nil
}

// retrieveResult could decrease degree of coupling between nrw mode and business.
func (nrw *NRW) retrieveResult(payload []byte) ([]byte, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("empty payload")
	}
	reqRetrieve, err := unpackReqRetrieve(payload[1:])

	switch RetrieveType(reqRetrieve.Filter[0]) {
	case RetrievePlugins:
		filter := FilterRetrievePlugins{}
		err := Unpack(reqRetrieve.Filter[1:], &filter)
		if err != nil {
			return nil, fmt.Errorf("wrong filter format: want %T", filter)
		}
		plugs, err := nrw.model.GetPlugins(filter.NamePattern, filter.Types)
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
		resultBuff, err := Pack(result, uint8(RetrievePlugins))
		if err != nil {
			return nil, fmt.Errorf("server error: pack %#v failed: %v", result, err)
		}

		return resultBuff, nil
	case RetrievePipelines:
		filter := FilterRetrievePipelines{}
		err := Unpack(reqRetrieve.Filter[1:], &filter)
		if err != nil {
			return nil, fmt.Errorf("wrong filter format: want %T", filter)
		}
		pipes, err := nrw.model.GetPipelines(filter.NamePattern, filter.Types)
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
		resultBuff, err := Pack(result, uint8(RetrievePipelines))
		if err != nil {
			return nil, fmt.Errorf("server error: pack %#v failed: %v", result, err)
		}

		return resultBuff, nil
	case RetrievePluginTypes:
		result := ResultRetrievePluginTypes{}
		result.PluginTypes = make([]string, 0)
		for _, typ := range plugins.GetAllTypes() {
			if !common.StrInSlice(typ, result.PluginTypes) {
				result.PluginTypes = append(result.PluginTypes, typ)
			}
		}
		sort.Strings(result.PluginTypes)

		resultBuff, err := Pack(result, uint8(RetrievePluginTypes))
		if err != nil {
			return nil, fmt.Errorf("server error: pack %#v failed: %v", result, err)
		}

		return resultBuff, nil
	case RetrievePipelineTypes:
		result := ResultRetrievePipelineTypes{}
		result.PipelineTypes = make([]string, 0)
		for _, typ := range pipelines.GetAllTypes() {
			if !common.StrInSlice(typ, result.PipelineTypes) {
				result.PipelineTypes = append(result.PipelineTypes, typ)
			}
		}
		sort.Strings(result.PipelineTypes)

		resultBuff, err := Pack(result, uint8(RetrievePipelineTypes))
		if err != nil {
			return nil, fmt.Errorf("server error: pack %#v failed: %v", result, err)
		}

		return resultBuff, nil
	default:
		return nil, fmt.Errorf("unsupported filter type")
	}

}

func (nrw *NRW) handleReadModeRetrieve(req *cluster.RequestEvent) {
	respond := func(result []byte, e error) {
		resp := RespRetrieve{
			Err:    NewRetrieveErr(e.Error()),
			Result: result,
		}
		respBuff, err := Pack(resp, uint8(MessageRetrieve))
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

	result, e := nrw.retrieveResult(req.RequestPayload)
	respond(result, e)
}

func (nrw *NRW) handleWriteModeRetrieve(event *cluster.RequestEvent) {
	respond := func(result []byte, e error) {
		resp := RespRetrieve{
			Err:    NewRetrieveErr(e.Error()),
			Result: result,
		}
	}
	// TODO
}
