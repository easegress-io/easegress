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

// TODO: Divide the switch-case into smaller 2, merge its common part.
func (nrw *NRW) handleRetrieve(req *cluster.RequestEvent) {
	respondErr := func(e error) {
		resp := RespRetrieve{
			Err: e,
		}
		respBuff, err := Pack(resp, uint8(MessageRetrieve))
		if err != nil {
			logger.Errorf("[BUG: pack %#v failed: %v]", resp, err)
			return
		}

		err = req.Respond(respBuff)
		if err != nil {
			logger.Errorf("[respond error %v to request %s, node %s failed: %v]", resp, req.RequestName, req.RequestNodeName, err)
			return
		}
	}

	reqRetrieve := ReqRetrieve{}
	err := Unpack(req.RequestPayload[1:], &reqRetrieve)
	if err != nil {
		respondErr(fmt.Errorf("wrong format: want ReqRetrieve"))
		return
	}

	if len(reqRetrieve.Filter) < 1 {
		respondErr(fmt.Errorf("wrong format: filter is empty"))
		return
	}

	resp := RespRetrieve{}
	switch RetrieveType(reqRetrieve.Filter[0]) {
	case RetrievePlugins:
		filter := FilterRetrievePlugins{}
		err := Unpack(reqRetrieve.Filter[1:], &filter)
		if err != nil {
			respondErr(fmt.Errorf("wrong format: want FilterRetrievePlugins"))
			return
		}
		plugs, err := nrw.model.GetPlugins(filter.NamePattern, filter.Types)
		if err != nil {
			respondErr(fmt.Errorf("server error: get plugins failed: %v", err))
			return
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
		buffResult, err := Pack(result, uint8(RetrievePlugins))
		if err != nil {
			logger.Errorf("[BUG: pack %#v failed: %v]", result, err)
			respondErr(fmt.Errorf("server error: pack %#v failed: %v", result, err))
			return
		}

		resp.Result = buffResult
	case RetrievePipelines:
		filter := FilterRetrievePipelines{}
		err := Unpack(reqRetrieve.Filter[1:], &filter)
		if err != nil {
			respondErr(fmt.Errorf("wrong format: want FilterRetrievePipelines"))
			return
		}
		pipes, err := nrw.model.GetPipelines(filter.NamePattern, filter.Types)
		if err != nil {
			respondErr(fmt.Errorf("server error: get pipelines failed: %v", err))
			return
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
		buffResult, err := Pack(result, uint8(RetrievePipelines))
		if err != nil {
			logger.Errorf("[BUG: pack %#v failed: %v]", result, err)
			respondErr(fmt.Errorf("server error: pack %#v failed: %v", result, err))
			return
		}

		resp.Result = buffResult
	case RetrievePluginTypes:
		result := ResultRetrievePluginTypes{}
		result.PluginTypes = make([]string, 0)
		for _, typ := range plugins.GetAllTypes() {
			if !common.StrInSlice(typ, result.PluginTypes) {
				result.PluginTypes = append(result.PluginTypes, typ)
			}
		}
		sort.Strings(result.PluginTypes)

		buffResult, err := Pack(result, uint8(RetrievePluginTypes))
		if err != nil {
			logger.Errorf("[BUG: pack %#v failed: %v]", result, err)
			respondErr(fmt.Errorf("server error: pack %#v failed: %v", result, err))
			return
		}

		resp.Result = buffResult
	case RetrievePipelineTypes:
		result := ResultRetrievePipelineTypes{}
		result.PipelineTypes = make([]string, 0)
		for _, typ := range pipelines.GetAllTypes() {
			if !common.StrInSlice(typ, result.PipelineTypes) {
				result.PipelineTypes = append(result.PipelineTypes, typ)
			}
		}
		sort.Strings(result.PipelineTypes)

		buffResult, err := Pack(result, uint8(RetrievePipelineTypes))
		if err != nil {
			logger.Errorf("[BUG: pack %#v failed: %v]", result, err)
			respondErr(fmt.Errorf("server error: pack %#v failed: %v", result, err))
			return
		}

		resp.Result = buffResult
	}

	respBuff, err := Pack(resp, uint8(MessageRetrieve))
	if err != nil {
		logger.Errorf("[BUG: pack %#v failed: %v]", resp, err)
		respondErr(fmt.Errorf("server error: pack %#v failed: %v", resp, err))
		return
	}
	err = req.Respond(respBuff)
	if err != nil {
		logger.Errorf("[respond %v to request %s, node %s failed: %v]", resp, req.RequestName, req.RequestNodeName, err)
		return
	}
}
