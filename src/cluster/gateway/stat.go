package gateway

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"cluster"
	"common"
	"logger"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// for api
func (gc *GatewayCluster) chooseMemberToAggregateStat(group string) (*cluster.Member, error) {
	totalMembers := gc.cluster.Members()
	var readMembers, writeMembers []cluster.Member

	for _, member := range totalMembers {
		if member.NodeTags[groupTagKey] == group &&
			member.Status == cluster.MemberAlive {
			if member.NodeTags[modeTagKey] == ReadMode.String() {
				readMembers = append(readMembers, member)
			} else {
				writeMembers = append(writeMembers, member)
			}
		}
	}

	// choose read mode member preferentially to reduce load of member under write mode
	if len(readMembers) > 0 {
		return &readMembers[rand.Int()%len(readMembers)], nil
	}

	// have to choose only alive WriteMode member
	if len(writeMembers) > 0 {
		return &writeMembers[rand.Int()%len(writeMembers)], nil
	}

	return nil, fmt.Errorf("none of members is alive to aggregate statistics")
}

func (gc *GatewayCluster) issueStat(group string, timeout time.Duration,
	requestName string, filter interface{}) (interface{}, *ClusterError) {
	req := &ReqStat{
		Timeout: timeout,
	}

	switch filter := filter.(type) {
	case *FilterPipelineIndicatorNames:
		req.FilterPipelineIndicatorNames = filter
	case *FilterPipelineIndicatorValue:
		req.FilterPipelineIndicatorValue = filter
	case *FilterPipelineIndicatorDesc:
		req.FilterPipelineIndicatorDesc = filter
	case *FilterPluginIndicatorNames:
		req.FilterPluginIndicatorNames = filter
	case *FilterPluginIndicatorValue:
		req.FilterPluginIndicatorValue = filter
	case *FilterPluginIndicatorDesc:
		req.FilterPluginIndicatorDesc = filter
	case *FilterTaskIndicatorNames:
		req.FilterTaskIndicatorNames = filter
	case *FilterTaskIndicatorValue:
		req.FilterTaskIndicatorValue = filter
	case *FilterTaskIndicatorDesc:
		req.FilterTaskIndicatorDesc = filter
	default:
		return nil, newClusterError(fmt.Sprintf("unsupported statistics filter type %T", filter),
			InternalServerError)
	}

	requestPayload, err := cluster.PackWithHeader(req, uint8(statMessage))
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed: %v]",
			uint8(statMessage), req, err)

		return nil, newClusterError(
			fmt.Sprintf("pack request (header=%d) to %#v failed: %v",
				uint8(statMessage), req, err),
			InternalServerError)
	}

	targetMember, err := gc.chooseMemberToAggregateStat(group)
	if err != nil {
		return nil, newClusterError(
			fmt.Sprintf("choose member to aggregate statistics failed: %v", err), InternalServerError)
	}

	requestParam := cluster.RequestParam{
		TargetNodeNames: []string{targetMember.NodeName},
		// TargetNodeNames is enough but TargetNodeTags could make rule strict
		TargetNodeTags: map[string]string{
			groupTagKey: group,
			modeTagKey:  targetMember.NodeTags[modeTagKey],
		},
		Timeout:            timeout,
		ResponseRelayCount: 1, // fault tolerance on network issue
	}

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		return nil, newClusterError(
			fmt.Sprintf("issue statistics aggregation failed: %v", err), InternalServerError)
	}

	var memberResp *cluster.MemberResponse

	select {
	case r, ok := <-future.Response():
		if !ok {
			return nil, newClusterError("issue statistics aggregation timeout", TimeoutError)
		}
		memberResp = r
	case <-gc.stopChan:
		return nil, newClusterError(
			"the member gone during issuing statistics aggregation", IssueMemberGoneError)
	}

	if len(memberResp.Payload) == 0 {
		return nil, newClusterError(
			"issue statistics aggregation responds empty response", InternalServerError)
	}

	var resp RespStat
	err = cluster.Unpack(memberResp.Payload[1:], &resp)
	if err != nil {
		return nil, newClusterError(
			fmt.Sprintf("unpack statistics aggregation response failed: %v", err), InternalServerError)
	}

	if resp.Err != nil {
		return nil, resp.Err
	}

	switch filter.(type) {
	case *FilterPipelineIndicatorNames:
		ret := new(ResultStatIndicatorNames)
		err = json.Unmarshal(resp.Names, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat pipeline indicator names response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat pipeline indicator names response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterPipelineIndicatorValue:
		ret := new(ResultStatIndicatorValue)
		err = json.Unmarshal(resp.Value, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat pipeline indicator value response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat pipeline indicator value response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterPipelineIndicatorDesc:
		ret := new(ResultStatIndicatorDesc)
		err = json.Unmarshal(resp.Desc, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat pipeline indicator desc response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat pipeline indicator desc response failed: %v", err),
				InternalServerError)
		}

		return ret, nil

	case *FilterPluginIndicatorNames:
		ret := new(ResultStatIndicatorNames)
		err = json.Unmarshal(resp.Names, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat plugin indicator names response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat plugin indicator names response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterPluginIndicatorValue:
		ret := new(ResultStatIndicatorValue)
		err = json.Unmarshal(resp.Value, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat plugin indicator value response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat plugin indicator value response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterPluginIndicatorDesc:
		ret := new(ResultStatIndicatorDesc)
		err = json.Unmarshal(resp.Desc, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat plugin indicator desc response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat plugin indicator desc response failed: %v", err),
				InternalServerError)
		}

		return ret, nil

	case *FilterTaskIndicatorNames:
		ret := new(ResultStatIndicatorNames)
		err = json.Unmarshal(resp.Names, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat task indicator names response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat task indicator names response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterTaskIndicatorValue:
		ret := new(ResultStatIndicatorValue)
		err = json.Unmarshal(resp.Value, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat task indicator value response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat task indicator value response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterTaskIndicatorDesc:
		ret := new(ResultStatIndicatorDesc)
		err = json.Unmarshal(resp.Desc, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarshal stat task indicator desc response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarshal stat task indicator desc response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	}

	return nil, newClusterError(fmt.Sprintf("unmarshal stat response failed: %v", err), InternalServerError)
}

// for core
func unpackReqStat(payload []byte) (*ReqStat, error, ClusterErrorType) {
	reqStat := new(ReqStat)
	err := cluster.Unpack(payload, reqStat)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to ReqStat failed: %v", payload, err), WrongMessageFormatError
	}

	emptyString := func(s string) bool {
		return len(s) == 0
	}

	switch {
	case reqStat.FilterPipelineIndicatorNames != nil:
		if emptyString(reqStat.FilterPipelineIndicatorNames.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retireve " +
				"pipeline statistics indicator names"), InternalServerError
		}
	case reqStat.FilterPipelineIndicatorValue != nil:
		if emptyString(reqStat.FilterPipelineIndicatorValue.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retrieve " +
				"pipeline statistics indicator value"), InternalServerError
		}
		if emptyString(reqStat.FilterPipelineIndicatorValue.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name in filter to " +
				"retrieve pipeline statistics indicator value"), InternalServerError
		}
	case reqStat.FilterPipelineIndicatorDesc != nil:
		if emptyString(reqStat.FilterPipelineIndicatorDesc.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retrieve " +
				"pipeline statistics indicator description"), InternalServerError
		}
		if emptyString(reqStat.FilterPipelineIndicatorDesc.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name in filter to retrieve " +
				"pipeline statistics indicator description"), InternalServerError
		}
	case reqStat.FilterPluginIndicatorNames != nil:
		if emptyString(reqStat.FilterPluginIndicatorNames.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retrieve " +
				"plugin statistics indicator names"), InternalServerError
		}
		if emptyString(reqStat.FilterPluginIndicatorNames.PluginName) {
			return nil, fmt.Errorf("empty plugin name in filter to retrieve " +
				"plugin statistics indicator names"), InternalServerError
		}
	case reqStat.FilterPluginIndicatorValue != nil:
		if emptyString(reqStat.FilterPluginIndicatorValue.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retrieve " +
				"plugin statistics indicator value"), InternalServerError
		}
		if emptyString(reqStat.FilterPluginIndicatorValue.PluginName) {
			return nil, fmt.Errorf("empty plugin name in filter to retrieve " +
				"plugin statistics indicator value"), InternalServerError
		}
		if emptyString(reqStat.FilterPluginIndicatorValue.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name in filter to retrieve " +
				"plugin statistics indicator value"), InternalServerError
		}
	case reqStat.FilterPluginIndicatorDesc != nil:
		if emptyString(reqStat.FilterPluginIndicatorDesc.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retrieve " +
				"plugin statistics indicator description"), InternalServerError
		}
		if emptyString(reqStat.FilterPluginIndicatorDesc.PluginName) {
			return nil, fmt.Errorf("empty plugin name in filter to retrieve " +
				"plugin statistics indicator description"), InternalServerError
		}
		if emptyString(reqStat.FilterPluginIndicatorDesc.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name in filter to retrieve " +
				"plugin statistics indicator description"), InternalServerError
		}
	case reqStat.FilterTaskIndicatorNames != nil:
		if emptyString(reqStat.FilterTaskIndicatorNames.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retrieve " +
				"task statistics indicator names"), InternalServerError
		}
	case reqStat.FilterTaskIndicatorValue != nil:
		if emptyString(reqStat.FilterTaskIndicatorValue.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retrieve " +
				"task statistics indicator value"), InternalServerError
		}
		if emptyString(reqStat.FilterTaskIndicatorValue.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name in filter to retrieve " +
				"task statistics indicator value"), InternalServerError
		}
	case reqStat.FilterTaskIndicatorDesc != nil:
		if emptyString(reqStat.FilterTaskIndicatorDesc.PipelineName) {
			return nil, fmt.Errorf("empty pipeline name in filter to retrieve " +
				"task statistics indicator description"), InternalServerError
		}
		if emptyString(reqStat.FilterTaskIndicatorDesc.IndicatorName) {
			return nil, fmt.Errorf("empty indicator name in filter to retrieve " +
				"task statistics indicator description"), InternalServerError
		}
	default:
		return nil, fmt.Errorf("empty statistics filter"), InternalServerError
	}

	return reqStat, nil, NoneClusterError
}

func (gc *GatewayCluster) respondStat(req *cluster.RequestEvent, resp *RespStat) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	respBuff, err := cluster.PackWithHeader(resp, uint8(req.RequestPayload[0]))
	if err != nil {
		logger.Errorf("[BUG: pack response (header=%d) to %#v failed: %v]", req.RequestPayload[0], resp, err)
		return
	}

	err = req.Respond(respBuff)
	if err != nil {
		logger.Errorf("[respond request %s to member %s failed: %v]",
			req.RequestName, req.RequestNodeName, err)
		return
	}

	logger.Debugf("[member %s responded statMessage message]", gc.clusterConf.NodeName)
}

func (gc *GatewayCluster) respondStatErr(req *cluster.RequestEvent, typ ClusterErrorType, msg string) {
	resp := &RespStat{
		Err: newClusterError(msg, typ),
	}
	gc.respondStat(req, resp)
}

func (gc *GatewayCluster) statResult(filter interface{}) ([]byte, error, ClusterErrorType) {
	var ret interface{}
	var err error

	statRegistry := gc.mod.StatRegistry()

	switch filter := filter.(type) {
	case *FilterPipelineIndicatorNames:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		r := new(ResultStatIndicatorNames)
		r.Names = stat.PipelineIndicatorNames()

		// returns with stable order
		sort.Strings(r.Names)

		ret = r
	case *FilterPipelineIndicatorValue:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		indicatorNames := stat.PipelineIndicatorNames()
		if !common.StrInSlice(filter.IndicatorName, indicatorNames) {
			return nil, fmt.Errorf("indicator %s not found", filter.IndicatorName),
				RetrievePipelineStatIndicatorNotFoundError
		}

		r := new(ResultStatIndicatorValue)
		r.Value, err = stat.PipelineIndicatorValue(filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the value of pipeline %s statistics indicator %s "+
				"from model failed: %v]", filter.PipelineName, filter.IndicatorName, err)
			return nil, fmt.Errorf("evaluate indicator %s value failed", filter.IndicatorName),
				RetrievePipelineStatValueError
		}

		ret = r
	case *FilterPipelineIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		indicatorNames := stat.PipelineIndicatorNames()
		if !common.StrInSlice(filter.IndicatorName, indicatorNames) {
			return nil, fmt.Errorf("indicator %s not found", filter.IndicatorName),
				RetrievePipelineStatIndicatorNotFoundError
		}

		r := new(ResultStatIndicatorDesc)
		r.Desc, err = stat.PipelineIndicatorDescription(filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the description of pipeline %s statistics indicator %s "+
				"from model failed: %v]", filter.PipelineName, filter.IndicatorName, err)
			return nil, fmt.Errorf("describe indicator %s failed", filter.IndicatorName),
				RetrievePipelineStatDescError
		}

		ret = r
	case *FilterPluginIndicatorNames:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		names := stat.PluginIndicatorNames(filter.PluginName)
		if names == nil {
			return nil, fmt.Errorf("plugin %s statistics not found", filter.PluginName),
				PluginStatNotFoundError
		}

		r := new(ResultStatIndicatorNames)
		r.Names = names

		// returns with stable order
		sort.Strings(r.Names)

		ret = r
	case *FilterPluginIndicatorValue:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		indicatorNames := stat.PluginIndicatorNames(filter.PluginName)
		if indicatorNames == nil {
			return nil, fmt.Errorf("plugin %s statistics not found", filter.PluginName),
				PluginStatNotFoundError
		}

		if !common.StrInSlice(filter.IndicatorName, indicatorNames) {
			return nil, fmt.Errorf("indicator %s not found", filter.IndicatorName),
				RetrievePluginStatIndicatorNotFoundError
		}

		r := new(ResultStatIndicatorValue)
		r.Value, err = stat.PluginIndicatorValue(filter.PluginName, filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the value of plugin %s statistics indicator %s in pipeline %s "+
				"from model failed: %v]", filter.PluginName, filter.IndicatorName,
				filter.PipelineName, err)
			return nil, fmt.Errorf("evaluate indicator %s value failed", filter.IndicatorName),
				RetrievePluginStatValueError
		}

		ret = r
	case *FilterPluginIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		indicatorNames := stat.PluginIndicatorNames(filter.PluginName)
		if indicatorNames == nil {
			return nil, fmt.Errorf("plugin %s statistics not found", filter.PluginName),
				PluginStatNotFoundError
		}

		if !common.StrInSlice(filter.IndicatorName, indicatorNames) {
			return nil, fmt.Errorf("indicator %s not found", filter.IndicatorName),
				RetrievePluginStatIndicatorNotFoundError
		}

		r := new(ResultStatIndicatorDesc)
		r.Desc, err = stat.PluginIndicatorDescription(filter.PluginName, filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the description of plugin %s statistics indicator %s "+
				"in pipeline %s from model failed: %v]", filter.PluginName, filter.IndicatorName,
				filter.PipelineName, err)
			return nil, fmt.Errorf("describe indicator %s failed", filter.IndicatorName),
				RetrievePluginStatDescError
		}

		ret = r
	case *FilterTaskIndicatorNames:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		r := new(ResultStatIndicatorNames)
		r.Names = stat.TaskIndicatorNames()

		// returns with stable order
		sort.Strings(r.Names)

		ret = r
	case *FilterTaskIndicatorValue:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		indicatorNames := stat.TaskIndicatorNames()
		if !common.StrInSlice(filter.IndicatorName, indicatorNames) {
			return nil, fmt.Errorf("indicator %s not found", filter.IndicatorName),
				RetrieveTaskStatIndicatorNotFoundError
		}

		r := new(ResultStatIndicatorValue)
		r.Value, err = stat.TaskIndicatorValue(filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the value of task statistics indicator %s in pipeline %s "+
				"from model failed: %v]", filter.IndicatorName, filter.PipelineName, err)
			return nil, fmt.Errorf("evaluate indicator %s value failed", filter.IndicatorName),
				RetrieveTaskStatValueError
		}

		ret = r
	case *FilterTaskIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		indicatorNames := stat.TaskIndicatorNames()
		if !common.StrInSlice(filter.IndicatorName, indicatorNames) {
			return nil, fmt.Errorf("indicator %s not found", filter.IndicatorName),
				RetrieveTaskStatIndicatorNotFoundError
		}

		r := new(ResultStatIndicatorDesc)
		r.Desc, err = stat.TaskIndicatorDescription(filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the description of task statistics indicator %s in pipeline %s "+
				"from model failed: %v]", filter.IndicatorName, filter.PipelineName, err)
			return nil, fmt.Errorf("describe indicator %s failed", filter.IndicatorName),
				RetrieveTaskStatDescError
		}

		ret = r
	default:
		return nil, fmt.Errorf("unsupported statistics filter type %T", filter), InternalServerError
	}

	retBuff, err := json.Marshal(ret)
	if err != nil {
		logger.Errorf("[BUG: marshal statistics result failed: %v]", err)
		return nil, fmt.Errorf("marshal statistics result failed: %v", err), InternalServerError
	}

	return retBuff, nil, NoneClusterError
}

func (gc *GatewayCluster) getLocalStatResp(reqStat *ReqStat) (*RespStat, error, ClusterErrorType) {
	resp := new(RespStat)

	// for emphasizing
	var err error
	var errType ClusterErrorType

	switch {
	case reqStat.FilterPipelineIndicatorNames != nil:
		resp.Names, err, errType = gc.statResult(reqStat.FilterPipelineIndicatorNames)
	case reqStat.FilterPipelineIndicatorValue != nil:
		resp.Value, err, errType = gc.statResult(reqStat.FilterPipelineIndicatorValue)
	case reqStat.FilterPipelineIndicatorDesc != nil:
		resp.Desc, err, errType = gc.statResult(reqStat.FilterPipelineIndicatorDesc)
	case reqStat.FilterPluginIndicatorNames != nil:
		resp.Names, err, errType = gc.statResult(reqStat.FilterPluginIndicatorNames)
	case reqStat.FilterPluginIndicatorValue != nil:
		resp.Value, err, errType = gc.statResult(reqStat.FilterPluginIndicatorValue)
	case reqStat.FilterPluginIndicatorDesc != nil:
		resp.Desc, err, errType = gc.statResult(reqStat.FilterPluginIndicatorDesc)
	case reqStat.FilterTaskIndicatorNames != nil:
		resp.Names, err, errType = gc.statResult(reqStat.FilterTaskIndicatorNames)
	case reqStat.FilterTaskIndicatorValue != nil:
		resp.Value, err, errType = gc.statResult(reqStat.FilterTaskIndicatorValue)
	case reqStat.FilterTaskIndicatorDesc != nil:
		resp.Desc, err, errType = gc.statResult(reqStat.FilterTaskIndicatorDesc)
	}

	if err != nil {
		return nil, err, errType
	}

	return resp, err, errType
}

func (gc *GatewayCluster) handleStatRelay(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqStat, err, errType := unpackReqStat(req.RequestPayload[1:])
	if err != nil {
		gc.respondStatErr(req, errType, err.Error())
		return
	}

	resp, err, errType := gc.getLocalStatResp(reqStat)
	if err != nil {
		gc.respondStatErr(req, errType, err.Error())
		return
	}

	gc.respondStat(req, resp)
}

func (gc *GatewayCluster) handleStat(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqStat, err, errType := unpackReqStat(req.RequestPayload[1:])
	if err != nil {
		gc.respondStatErr(req, errType, err.Error())
		return
	}

	var validRespList []*RespStat

	localResp, err, errType := gc.getLocalStatResp(reqStat)
	if err != nil {
		logger.Warnf("[get local statistics failed: %v]", err)
	}

	if localResp != nil {
		validRespList = append(validRespList, localResp)
	}

	requestMembers := gc.RestAliveMembersInSameGroup()

	if len(requestMembers) > 0 {
		requestMemberNames := make([]string, 0)
		for _, member := range requestMembers {
			requestMemberNames = append(requestMemberNames, member.NodeName)
		}

		requestParam := cluster.RequestParam{
			TargetNodeNames: requestMemberNames,
			// TargetNodeNames is enough but TargetNodeTags could make rule strict
			TargetNodeTags: map[string]string{
				groupTagKey: gc.localGroupName(),
			},
			Timeout:            reqStat.Timeout,
			ResponseRelayCount: 1, // fault tolerance on network issue
		}

		requestName := fmt.Sprintf("%s_relay", req.RequestName)
		requestPayload := make([]byte, len(req.RequestPayload))
		copy(requestPayload, req.RequestPayload)
		requestPayload[0] = byte(statRelayMessage)

		future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
		if err != nil {
			logger.Errorf("[send stat relay message failed: %v]", err)
			gc.respondRetrieveErr(req, InternalServerError, err.Error())
			return
		}

		membersRespBook := make(map[string][]byte)
		for _, memberName := range requestMemberNames {
			membersRespBook[memberName] = nil
		}

		gc.recordResp(requestName, future, membersRespBook)

		for _, payload := range membersRespBook {
			if len(payload) == 0 {
				continue
			}

			resp := new(RespStat)
			err := cluster.Unpack(payload[1:], resp)
			if err != nil || resp.Err != nil {
				// FIXME: when aggregateStatResponses() supports to return "details" for each member,
				// provide error in the dedicated section of each member as the part of result
				continue
			}

			validRespList = append(validRespList, resp)
		}
	}

	ret := aggregateStatResponses(reqStat, validRespList)
	if ret == nil {
		gc.respondRetrieveErr(req, InternalServerError, "aggreate statistics for cluster memebers failed")
		return
	}

	gc.respondStat(req, ret)
}

type stateAggregator func(values ...[]byte) interface{}

func aggregateStatResponses(reqStat *ReqStat, respStats []*RespStat) *RespStat {
	var indicatorName string
	var aggregator stateAggregator = nil

	switch {
	case reqStat.FilterPipelineIndicatorNames != nil:
		fallthrough
	case reqStat.FilterPluginIndicatorNames != nil:
		fallthrough
	case reqStat.FilterTaskIndicatorNames != nil:
		memory := make(map[string]struct{})
		ret := new(ResultStatIndicatorNames)
		ret.Names = make([]string, 0)

		for _, resp := range respStats {
			r := new(ResultStatIndicatorNames)
			err := json.Unmarshal(resp.Names, r)
			if err != nil {
				continue
			}

			for _, name := range r.Names {
				_, exists := memory[name]
				if !exists {
					ret.Names = append(ret.Names, name)
					memory[name] = struct{}{}
				}
			}
		}

		// returns with stable order
		sort.Strings(ret.Names)

		retBuff, err := json.Marshal(ret)
		if err != nil {
			return nil
		}

		return &RespStat{
			Names: retBuff,
		}
	case reqStat.FilterPipelineIndicatorDesc != nil:
		fallthrough
	case reqStat.FilterPluginIndicatorDesc != nil:
		fallthrough
	case reqStat.FilterTaskIndicatorDesc != nil:
		for _, resp := range respStats {
			r := new(ResultStatIndicatorDesc)
			err := json.Unmarshal(resp.Desc, r)
			if err != nil || r.Desc == nil {
				continue
			}

			return &RespStat{
				Desc: resp.Desc,
			}
		}

		return nil
	case reqStat.FilterPipelineIndicatorValue != nil:
		if len(indicatorName) == 0 {
			indicatorName = reqStat.FilterPipelineIndicatorValue.IndicatorName
			if aggregator == nil {
				aggregator = pipelineIndicatorAggregateMap[indicatorName]
			}
		}
		fallthrough
	case reqStat.FilterPluginIndicatorValue != nil:
		if len(indicatorName) == 0 {
			indicatorName = reqStat.FilterPluginIndicatorValue.IndicatorName
			if aggregator == nil {
				aggregator = pluginIndicatorAggregateMap[indicatorName]
			}
		}
		fallthrough
	case reqStat.FilterTaskIndicatorValue != nil:
		if len(indicatorName) == 0 {
			indicatorName = reqStat.FilterTaskIndicatorValue.IndicatorName
			if aggregator == nil {
				aggregator = taskIndicatorAggregateMap[indicatorName]
			}
		}

		if len(indicatorName) == 0 {
			return nil
		}

		indicatorValues := make([]*ResultStatIndicatorValue, 0)
		for _, resp := range respStats {
			r := new(ResultStatIndicatorValue)
			err := json.Unmarshal(resp.Value, r)
			if err != nil || r.Value == nil {
				continue
			}

			indicatorValues = append(indicatorValues, r)
		}

		// TODO: Add node names about detailed list

		// unknown indicators, just list values
		if aggregator == nil {
			retBuff, err := json.Marshal(indicatorValues)
			if err != nil {
				return nil
			}

			return &RespStat{
				Value: retBuff,
			}
		}

		// aggregate known indicators
		values := make([][]byte, 0)
		for _, value := range indicatorValues {
			valueBuff, err := json.Marshal(value.Value)
			if err != nil {
				continue
			}
			values = append(values, valueBuff)
		}
		if len(values) == 0 {
			return nil
		}

		result := new(ResultStatIndicatorValue)
		result.Value = aggregator(values...)
		if result.Value == nil {
			return nil
		}

		resp := new(RespStat)
		var err error
		resp.Value, err = json.Marshal(result)
		if err != nil {
			return nil
		}
		return resp
	}

	return nil
}

func numericMax(typ interface{}, values ...[]byte) interface{} {
	if len(values) == 0 {
		// defensive programming
		return nil
	}

	handledAny := false
	var ret interface{}
	switch typ.(type) {
	case float64:
		var max float64 = math.NaN()
		for _, value := range values {
			var v float64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			if math.IsNaN(max) {
				max = v
			} else {
				max = math.Max(max, v)
			}
			handledAny = true
		}
		ret = max
	case uint64:
		var max uint64 = 0
		for _, value := range values {
			var v uint64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			if v > max {
				max = v
			}
			handledAny = true
		}
		ret = max
	case int64:
		var max int64 = math.MinInt64
		for _, value := range values {
			var v int64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			if v > max {
				max = v
			}
			handledAny = true
		}
		ret = max
	default:
		return nil
	}

	if !handledAny {
		return nil
	}

	return ret
}

func numericMin(typ interface{}, values ...[]byte) interface{} {
	if len(values) == 0 {
		// defensive programming
		return nil
	}

	handledAny := false
	var ret interface{}
	switch typ.(type) {
	case float64:
		var min float64 = math.NaN()
		for _, value := range values {
			var v float64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			if math.IsNaN(min) {
				min = v
			} else {
				min = math.Min(min, v)
			}
			handledAny = true
		}
		ret = min
	case uint64:
		var min uint64 = math.MaxUint64
		for _, value := range values {
			var v uint64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			if v < min {
				min = v
			}
			handledAny = true
		}
		ret = min
	case int64:
		var min int64 = math.MaxInt64
		for _, value := range values {
			var v int64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			if v < min {
				min = v
			}
			handledAny = true
		}
		ret = min
	default:
		return nil
	}

	if !handledAny {
		return nil
	}

	return ret
}

func numericSum(typ interface{}, values ...[]byte) interface{} {
	if len(values) == 0 {
		// defensive programming
		return nil
	}

	handledAny := false
	var ret interface{}
	switch typ.(type) {
	case float64:
		var sum float64 = 0
		for _, value := range values {
			var v float64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			sum += v
			handledAny = true
		}
		ret = sum
	case uint64:
		var sum uint64 = 0
		for _, value := range values {
			var v uint64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			sum += v
			handledAny = true
		}
		ret = sum
	case int64:
		var sum int64 = 0
		for _, value := range values {
			var v int64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			sum += v
			handledAny = true
		}
		ret = sum
	default:
		return nil
	}

	if !handledAny {
		return nil
	}

	return ret
}

func numericAvg(typ interface{}, values ...[]byte) interface{} {
	if len(values) == 0 {
		// defensive programming
		return nil
	}

	handledAny := false
	var ret interface{}
	switch typ.(type) {
	case float64:
		var sum float64 = 0
		var count float64 = 0
		for _, value := range values {
			var v float64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			sum += v
			count += 1
			handledAny = true
		}
		if count == 0 {
			return nil
		}
		ret = sum / count
	case uint64:
		var sum uint64 = 0
		var count uint64 = 0
		for _, value := range values {
			var v uint64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			sum += v
			count += 1
			handledAny = true
		}
		if count == 0 {
			return nil
		}
		ret = sum / count
	case int64:
		var sum int64 = 0
		var count int64 = 0
		for _, value := range values {
			var v int64
			err := json.Unmarshal(value, &v)
			if err != nil {
				continue
			}
			sum += v
			count += 1
			handledAny = true
		}
		if count == 0 {
			return nil
		}
		ret = sum / count
	default:
		return nil
	}

	if !handledAny {
		return nil
	}

	return ret
}

func maxFloat64(values ...[]byte) interface{} {
	return numericMax(float64(0), values...)
}

func minFloat64(values ...[]byte) interface{} {
	return numericMin(float64(0), values...)
}

func sumFloat64(values ...[]byte) interface{} {
	return numericSum(float64(0), values...)
}

func avgFloat64(values ...[]byte) interface{} {
	return numericAvg(float64(0), values...)
}

////

func maxUint64(values ...[]byte) interface{} {
	return numericMax(uint64(0), values...)
}

func minUint64(values ...[]byte) interface{} {
	return numericMin(uint64(0), values...)
}

func sumUint64(values ...[]byte) interface{} {
	return numericSum(uint64(0), values...)
}

func avgUint64(values ...[]byte) interface{} {
	return numericAvg(uint64(0), values...)
}

////

func maxInt64(values ...[]byte) interface{} {
	return numericMax(int64(0), values...)
}

func minInt64(values ...[]byte) interface{} {
	return numericMin(int64(0), values...)
}

func sumInt64(values ...[]byte) interface{} {
	return numericSum(int64(0), values...)
}

func avgInt64(values ...[]byte) interface{} {
	return numericAvg(int64(0), values...)
}

////

var pipelineIndicatorAggregateMap = map[string]stateAggregator{
	"THROUGHPUT_RATE_LAST_1MIN_ALL":  sumFloat64,
	"THROUGHPUT_RATE_LAST_5MIN_ALL":  sumFloat64,
	"THROUGHPUT_RATE_LAST_15MIN_ALL": sumFloat64,

	"EXECUTION_COUNT_ALL":    sumInt64,
	"EXECUTION_TIME_MAX_ALL": maxInt64,
	"EXECUTION_TIME_MIN_ALL": minInt64,

	"EXECUTION_TIME_50_PERCENT_ALL": maxFloat64,
	"EXECUTION_TIME_90_PERCENT_ALL": maxFloat64,
	"EXECUTION_TIME_99_PERCENT_ALL": maxFloat64,

	"EXECUTION_TIME_STD_DEV_ALL":  maxFloat64,
	"EXECUTION_TIME_VARIANCE_ALL": maxFloat64,
	"EXECUTION_TIME_SUM_ALL":      sumInt64,
}

var pluginIndicatorAggregateMap = map[string]stateAggregator{
	"THROUGHPUT_RATE_LAST_1MIN_ALL":      sumFloat64,
	"THROUGHPUT_RATE_LAST_5MIN_ALL":      sumFloat64,
	"THROUGHPUT_RATE_LAST_15MIN_ALL":     sumFloat64,
	"THROUGHPUT_RATE_LAST_1MIN_SUCCESS":  sumFloat64,
	"THROUGHPUT_RATE_LAST_5MIN_SUCCESS":  sumFloat64,
	"THROUGHPUT_RATE_LAST_15MIN_SUCCESS": sumFloat64,
	"THROUGHPUT_RATE_LAST_1MIN_FAILURE":  sumFloat64,
	"THROUGHPUT_RATE_LAST_5MIN_FAILURE":  sumFloat64,
	"THROUGHPUT_RATE_LAST_15MIN_FAILURE": sumFloat64,

	"EXECUTION_COUNT_ALL":     sumInt64,
	"EXECUTION_COUNT_SUCCESS": sumInt64,
	"EXECUTION_COUNT_FAILURE": sumInt64,

	"EXECUTION_TIME_MAX_ALL":     maxInt64,
	"EXECUTION_TIME_MAX_SUCCESS": maxInt64,
	"EXECUTION_TIME_MAX_FAILURE": maxInt64,
	"EXECUTION_TIME_MIN_ALL":     minInt64,
	"EXECUTION_TIME_MIN_SUCCESS": minInt64,
	"EXECUTION_TIME_MIN_FAILURE": minInt64,

	"EXECUTION_TIME_50_PERCENT_SUCCESS": maxFloat64,
	"EXECUTION_TIME_50_PERCENT_FAILURE": maxFloat64,
	"EXECUTION_TIME_90_PERCENT_SUCCESS": maxFloat64,
	"EXECUTION_TIME_90_PERCENT_FAILURE": maxFloat64,
	"EXECUTION_TIME_99_PERCENT_SUCCESS": maxFloat64,
	"EXECUTION_TIME_99_PERCENT_FAILURE": maxFloat64,

	"EXECUTION_TIME_STD_DEV_SUCCESS":  maxFloat64,
	"EXECUTION_TIME_STD_DEV_FAILURE":  maxFloat64,
	"EXECUTION_TIME_VARIANCE_SUCCESS": maxFloat64,
	"EXECUTION_TIME_VARIANCE_FAILURE": maxFloat64,

	"EXECUTION_TIME_SUM_ALL":     sumInt64,
	"EXECUTION_TIME_SUM_SUCCESS": sumInt64,
	"EXECUTION_TIME_SUM_FAILURE": sumInt64,

	// plugin dedicated indicators

	// http_input plugin
	"WAIT_QUEUE_LENGTH": sumUint64,
	"WIP_REQUEST_COUNT": sumUint64,

	// http_counter plugin
	"RECENT_HEADER_COUNT ": sumUint64,
}

var taskIndicatorAggregateMap = map[string]stateAggregator{
	// task dedicated indicator
	"EXECUTION_COUNT_ALL":     sumUint64,
	"EXECUTION_COUNT_SUCCESS": sumUint64,
	"EXECUTION_COUNT_FAILURE": sumUint64,
}
