package gateway

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"cluster"
	"logger"
)

func unpackReqStat(payload []byte) (*ReqStat, error) {
	reqStat := new(ReqStat)
	err := cluster.Unpack(payload, reqStat)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to ReqStat failed: %v", payload, err)
	}

	if reqStat.Timeout < 1*time.Second {
		return nil, fmt.Errorf("timeout is less than 1 second")
	}

	emptyString := func(s string) bool {
		return len(s) == 0
	}

	switch {
	case reqStat.FilterPipelineIndicatorNames != nil:
		if emptyString(reqStat.FilterPipelineIndicatorNames.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
	case reqStat.FilterPipelineIndicatorValue != nil:
		if emptyString(reqStat.FilterPipelineIndicatorValue.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
		if emptyString(reqStat.FilterPipelineIndicatorValue.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name")
		}
	case reqStat.FilterPipelineIndicatorDesc != nil:
		if emptyString(reqStat.FilterPipelineIndicatorDesc.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
		if emptyString(reqStat.FilterPipelineIndicatorDesc.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name")
		}
	case reqStat.FilterPluginIndicatorNames != nil:
		if emptyString(reqStat.FilterPluginIndicatorNames.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
		if emptyString(reqStat.FilterPluginIndicatorNames.PluginName) {
			return nil, fmt.Errorf("empty plugin name")
		}
	case reqStat.FilterPluginIndicatorValue != nil:
		if emptyString(reqStat.FilterPluginIndicatorValue.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
		if emptyString(reqStat.FilterPluginIndicatorValue.PluginName) {
			return nil, fmt.Errorf("empty plugin name")
		}
		if emptyString(reqStat.FilterPluginIndicatorValue.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name")
		}
	case reqStat.FilterPluginIndicatorDesc != nil:
		if emptyString(reqStat.FilterPluginIndicatorDesc.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
		if emptyString(reqStat.FilterPluginIndicatorDesc.PluginName) {
			return nil, fmt.Errorf("empty plugin name")
		}
		if emptyString(reqStat.FilterPluginIndicatorDesc.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name")
		}
	case reqStat.FilterTaskIndicatorNames != nil:
		if emptyString(reqStat.FilterTaskIndicatorNames.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
	case reqStat.FilterTaskIndicatorValue != nil:
		if emptyString(reqStat.FilterTaskIndicatorValue.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
		if emptyString(reqStat.FilterTaskIndicatorValue.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name")
		}
	case reqStat.FilterTaskIndicatorDesc != nil:
		if emptyString(reqStat.FilterTaskIndicatorDesc.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name")
		}
		if emptyString(reqStat.FilterTaskIndicatorDesc.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name")
		}
	default:
		return nil, fmt.Errorf("empty filter")
	}

	return reqStat, nil
}

func respondStat(req *cluster.RequestEvent, resp *RespStat) {
	// defensive programming
	if len(req.RequestPayload) < 1 {
		return
	}

	respBuff, err := cluster.PackWithHeader(resp, uint8(req.RequestPayload[0]))
	if err != nil {
		logger.Errorf("[BUG: pack header(%d) %#v failed: %v]", req.RequestPayload[0], resp, err)
		return
	}

	err = req.Respond(respBuff)
	if err != nil {
		logger.Errorf("[respond %s to request %s, node %s failed: %v]",
			respBuff, req.RequestName, req.RequestNodeName, err)
		return
	}
}

func respondStatErr(req *cluster.RequestEvent, typ ClusterErrorType, msg string) {
	resp := &RespStat{
		Err: &ClusterError{
			Type:    typ,
			Message: msg,
		},
	}
	respondStat(req, resp)
}

func (gc *GatewayCluster) statResult(filter interface{}) ([]byte, error) {
	var ret interface{}
	var err error

	statRegistry := gc.mod.StatRegistry()
	switch filter := filter.(type) {
	case *FilterPipelineIndicatorNames:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorNames{}
		result.Names = stat.PipelineIndicatorNames()
		sort.Strings(result.Names)
		ret = result
	case *FilterPipelineIndicatorValue:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorValue{}
		result.Value, err = stat.PipelineIndicatorValue(filter.IndicatorName)
		if err != nil {
			return nil, err
		}
		ret = result
	case *FilterPipelineIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorDesc{}
		result.Desc, err = stat.PipelineIndicatorDescription(filter.IndicatorName)
		if err != nil {
			return nil, err
		}
		ret = result
	case *FilterPluginIndicatorNames:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorNames{}
		result.Names = stat.PluginIndicatorNames(filter.PluginName)
		sort.Strings(result.Names)
		ret = result
	case *FilterPluginIndicatorValue:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorValue{}
		result.Value, err = stat.PluginIndicatorValue(filter.PluginName, filter.IndicatorName)
		if err != nil {
			return nil, err
		}
		ret = result
	case *FilterPluginIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorDesc{}
		result.Desc, err = stat.PluginIndicatorDescription(filter.PluginName, filter.IndicatorName)
		if err != nil {
			return nil, err
		}
		ret = result
	case *FilterTaskIndicatorNames:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorNames{}
		result.Names = stat.TaskIndicatorNames()
		sort.Strings(result.Names)
		ret = result
	case *FilterTaskIndicatorValue:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorValue{}
		result.Value, err = stat.TaskIndicatorValue(filter.IndicatorName)
		if err != nil {
			return nil, err
		}
		ret = result
	case *FilterTaskIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName)
		}
		result := ResultStatIndicatorDesc{}
		result.Desc, err = stat.TaskIndicatorDescription(filter.IndicatorName)
		if err != nil {
			return nil, err
		}
		ret = result
	default:
		return nil, fmt.Errorf("unsupported filter type")
	}

	retBuff, err := json.Marshal(ret)
	if err != nil {
		logger.Errorf("[BUG: marshal %#v failed: %v]", ret, err)
		return nil, fmt.Errorf("server error: marshal %#v failed: %v", ret, err)
	}

	return retBuff, nil
}

func (gc *GatewayCluster) getLocalStatResp(req *cluster.RequestEvent) *RespStat {
	if len(req.RequestPayload) < 1 {
		return nil
	}

	reqStat, err := unpackReqStat(req.RequestPayload[1:])
	if err != nil {
		respondStatErr(req, WrongFormatError, err.Error())
		return nil
	}

	resp := new(RespStat)
	err = nil // for emphasizing
	switch {
	case reqStat.FilterPipelineIndicatorNames != nil:
		resp.Names, err = gc.statResult(reqStat.FilterPipelineIndicatorNames)
	case reqStat.FilterPipelineIndicatorValue != nil:
		resp.Value, err = gc.statResult(reqStat.FilterPipelineIndicatorValue)
	case reqStat.FilterPipelineIndicatorDesc != nil:
		resp.Desc, err = gc.statResult(reqStat.FilterPipelineIndicatorDesc)
	case reqStat.FilterPluginIndicatorNames != nil:
		resp.Names, err = gc.statResult(reqStat.FilterPluginIndicatorNames)
	case reqStat.FilterPluginIndicatorValue != nil:
		resp.Value, err = gc.statResult(reqStat.FilterPluginIndicatorValue)
	case reqStat.FilterPluginIndicatorDesc != nil:
		resp.Desc, err = gc.statResult(reqStat.FilterPluginIndicatorDesc)
	case reqStat.FilterTaskIndicatorNames != nil:
		resp.Names, err = gc.statResult(reqStat.FilterTaskIndicatorNames)
	case reqStat.FilterTaskIndicatorValue != nil:
		resp.Value, err = gc.statResult(reqStat.FilterTaskIndicatorValue)
	case reqStat.FilterTaskIndicatorDesc != nil:
		resp.Desc, err = gc.statResult(reqStat.FilterTaskIndicatorDesc)
	}

	if err != nil {
		resp.Err = &ClusterError{
			Type:    StatNotFoundError,
			Message: err.Error(),
		}
	}

	return resp
}

func (gc *GatewayCluster) handleStatRelay(req *cluster.RequestEvent) {
	if len(req.RequestPayload) < 1 {
		// defensive programming
		return
	}

	resp := gc.getLocalStatResp(req)
	if resp == nil {
		return
	}

	respondStat(req, resp)
}

// Choose ReadMode node preferentially to reduce load of WrieMode
func (gc *GatewayCluster) handleStat(req *cluster.RequestEvent) {
	if len(req.RequestPayload) < 1 {
		// defensive programming
		return
	}

	// Maybe the result is not found locally which is OK to go on.
	localResp := gc.getLocalStatResp(req)
	if localResp == nil {
		return
	}

	reqStat, err := unpackReqStat(req.RequestPayload[1:])
	if err != nil {
		return
	}

	requestMembers := gc.restAliveMembersInSameGroup()
	requestMemberNames := make([]string, 0)
	for _, member := range requestMembers {
		requestMemberNames = append(requestMemberNames, member.NodeName)
	}
	requestParam := cluster.RequestParam{
		TargetNodeNames: requestMemberNames,
		TargetNodeTags: map[string]string{
			groupTagKey: gc.localGroupName(),
		},
		Timeout: reqStat.Timeout,
	}

	requestName := fmt.Sprintf("%s_relay", req.RequestName)
	requestPayload := make([]byte, len(req.RequestPayload))
	copy(requestPayload, req.RequestPayload)
	requestPayload[0] = byte(statRelayMessage)

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		respondRetrieveErr(req, InternalServerError, fmt.Sprintf("braodcast message failed: %s", err.Error()))
		return
	}

	membersRespBook := make(map[string][]byte)
	for _, memberName := range requestMemberNames {
		membersRespBook[memberName] = nil
	}

	gc.recordResp(requestName, future, membersRespBook)

	validResp := make([]RespStat, 0)
	if localResp.Err != nil {
		validResp = append(validResp, *localResp)
	}
	for _, payload := range membersRespBook {
		if len(payload) < 1 {
			continue
		}

		resp := new(RespStat)
		err := cluster.Unpack(payload[1:], resp)
		if err != nil {
			continue
		}

		if resp.Err != nil {
			continue
		}
		validResp = append(validResp, *resp)
	}

	resp := aggregateRespStat(reqStat, validResp...)
	if resp != nil {
		respondStat(req, resp)
		return
	}
	respondStat(req, localResp)
}

func aggregateRespStat(reqStat *ReqStat, respStats ...RespStat) *RespStat {
	switch {
	case reqStat.FilterPipelineIndicatorNames != nil:
		fallthrough
	case reqStat.FilterPluginIndicatorNames != nil:
		fallthrough
	case reqStat.FilterTaskIndicatorNames != nil:
		namesMark := make(map[string]struct{})
		for _, resp := range respStats {
			result := &ResultStatIndicatorNames{}
			err := json.Unmarshal(resp.Names, result)
			if err != nil {
				continue
			}
			for _, name := range result.Names {
				namesMark[name] = struct{}{}
			}
		}

		if len(namesMark) == 0 {
			return nil
		}
		result := &ResultStatIndicatorNames{
			Names: make([]string, 0),
		}
		for name := range namesMark {
			result.Names = append(result.Names, name)
		}
		sort.Strings(result.Names)
		resultBuff, err := json.Marshal(result)
		if err != nil {
			return nil
		}
		return &RespStat{
			Names: resultBuff,
		}

	case reqStat.FilterPipelineIndicatorDesc != nil:
		fallthrough
	case reqStat.FilterPluginIndicatorDesc != nil:
		fallthrough
	case reqStat.FilterTaskIndicatorDesc != nil:
		for _, resp := range respStats {
			result := &ResultStatIndicatorDesc{}
			err := json.Unmarshal(resp.Desc, result)
			if err != nil || result.Desc == nil {
				continue
			}
			return &RespStat{
				Desc: resp.Desc,
			}
		}
		return nil

	case reqStat.FilterPipelineIndicatorValue != nil:
		fallthrough
	case reqStat.FilterPluginIndicatorValue != nil:
		fallthrough
	case reqStat.FilterTaskIndicatorValue != nil:
		resultValues := make([]ResultStatIndicatorValue, 0)
		for _, resp := range respStats {
			result := &ResultStatIndicatorValue{}
			err := json.Unmarshal(resp.Value, result)
			if err != nil || result.Value == nil {
				continue
			}
			resultValues = append(resultValues, *result)
		}
		if len(resultValues) == 0 {
			return nil
		}
		result := aggregateResultValues(resultValues...)
		resultBuff, err := json.Marshal(result)
		if err != nil {
			return nil
		}
		return &RespStat{
			Value: resultBuff,
		}
	}

	return nil
}

func aggregateResultValues(resultValues ...ResultStatIndicatorValue) ResultStatIndicatorValue {
	// TODO
	return ResultStatIndicatorValue{}
}
