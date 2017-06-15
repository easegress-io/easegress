package gateway

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"time"

	"cluster"
	"logger"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// for api
func (gc *GatewayCluster) chooseMemberToAggregateStat(group string) (string, error) {
	totalMembers := gc.cluster.Members()
	writeMember := ""
	readMembers := make([]string, 0)
	for _, member := range totalMembers {
		if member.NodeTags[groupTagKey] == group &&
			member.Status == cluster.MemberAlive {
			if member.NodeTags[modeTagKey] == ReadMode.String() {
				readMembers = append(readMembers, member.NodeName)
			} else {
				writeMember = member.NodeName
			}
		}
	}

	// choose read mode member preferentially to reduce load of member under write mode
	if len(readMembers) > 0 {
		return readMembers[rand.Int()%len(readMembers)], nil
	}

	// have to choose only alive WriteMode member
	if len(writeMember) > 0 {
		return writeMember, nil
	}

	return "", fmt.Errorf("none of members is alive")
}

func (gc *GatewayCluster) issueStat(group string, timeout time.Duration,
	requestName string, filter interface{}) ([]byte, *HTTPError) {
	req := ReqStat{
		Timeout: timeout,
	}
	switch filter := filter.(type) {
	case FilterPipelineIndicatorNames:
		req.FilterPipelineIndicatorNames = &FilterPipelineIndicatorNames{
			PipelineName: filter.PipelineName,
		}
	case FilterPipelineIndicatorValue:
		req.FilterPipelineIndicatorValue = &FilterPipelineIndicatorValue{
			PipelineName:  filter.PipelineName,
			IndicatorName: filter.IndicatorName,
		}
	case FilterPipelineIndicatorDesc:
		req.FilterPipelineIndicatorDesc = &FilterPipelineIndicatorDesc{
			PipelineName:  filter.PipelineName,
			IndicatorName: filter.IndicatorName,
		}

	case FilterPluginIndicatorNames:
		req.FilterPluginIndicatorNames = &FilterPluginIndicatorNames{
			PipelineName: filter.PipelineName,
			PluginName:   filter.PluginName,
		}
	case FilterPluginIndicatorValue:
		req.FilterPluginIndicatorValue = &FilterPluginIndicatorValue{
			PipelineName:  filter.PipelineName,
			PluginName:    filter.PluginName,
			IndicatorName: filter.IndicatorName,
		}
	case FilterPluginIndicatorDesc:
		req.FilterPluginIndicatorDesc = &FilterPluginIndicatorDesc{
			PipelineName:  filter.PipelineName,
			PluginName:    filter.PluginName,
			IndicatorName: filter.IndicatorName,
		}

	case FilterTaskIndicatorNames:
		req.FilterTaskIndicatorNames = &FilterTaskIndicatorNames{
			PipelineName: filter.PipelineName,
		}
	case FilterTaskIndicatorValue:
		req.FilterTaskIndicatorValue = &FilterTaskIndicatorValue{
			PipelineName:  filter.PipelineName,
			IndicatorName: filter.IndicatorName,
		}
	case FilterTaskIndicatorDesc:
		req.FilterTaskIndicatorDesc = &FilterTaskIndicatorDesc{
			PipelineName:  filter.PipelineName,
			IndicatorName: filter.IndicatorName,
		}
	default:
		return nil, NewHTTPError("unsupported filter type", http.StatusInternalServerError)
	}

	requestPayload, err := cluster.PackWithHeader(&req, uint8(statMessage))
	if err != nil {
		return nil, NewHTTPError(err.Error(), http.StatusInternalServerError)
	}
	targetMember, err := gc.chooseMemberToAggregateStat(group)
	if err != nil {
		return nil, NewHTTPError(err.Error(), http.StatusInternalServerError)
	}
	requestParam := cluster.RequestParam{
		TargetNodeNames: []string{targetMember},
		TargetNodeTags: map[string]string{
			groupTagKey: group,
		},
		Timeout: timeout,
	}

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		return nil, NewHTTPError(err.Error(), http.StatusInternalServerError)
	}

	memberResp, ok := <-future.Response()
	if !ok {
		return nil, NewHTTPError("timeout", http.StatusGatewayTimeout)
	}
	if len(memberResp.Payload) < 1 {
		return nil, NewHTTPError("empty response", http.StatusInternalServerError)
	}

	var resp RespStat
	err = cluster.Unpack(memberResp.Payload[1:], &resp)
	if err != nil {
		return nil, NewHTTPError(err.Error(), http.StatusInternalServerError)
	}

	if resp.Err != nil {
		var code int
		switch resp.Err.Type {
		case WrongMessageFormatError:
			code = http.StatusBadRequest
		case TimeoutError:
			code = http.StatusGatewayTimeout
		case StatNotFoundError:
			code = http.StatusNotFound
		default:
			code = http.StatusInternalServerError
		}
		return nil, NewHTTPError(resp.Err.Message, code)
	}

	var result []byte
	switch filter.(type) {
	case FilterPipelineIndicatorNames:
		result = resp.Names
	case FilterPipelineIndicatorValue:
		result = resp.Value
	case FilterPipelineIndicatorDesc:
		result = resp.Desc

	case FilterPluginIndicatorNames:
		result = resp.Names
	case FilterPluginIndicatorValue:
		result = resp.Value
	case FilterPluginIndicatorDesc:
		result = resp.Desc

	case FilterTaskIndicatorNames:
		result = resp.Names
	case FilterTaskIndicatorValue:
		result = resp.Value
	case FilterTaskIndicatorDesc:
		result = resp.Desc
	default:
		return nil, NewHTTPError("unsupported filter type", http.StatusInternalServerError)
	}
	if result == nil {
		return nil, NewHTTPError("empty result", http.StatusInternalServerError)
	}

	return result, nil
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

	return reqStat, nil, NoneError
}

func respondStat(req *cluster.RequestEvent, resp *RespStat) {
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

		r := new(ResultStatIndicatorValue)
		r.Value, err = stat.PipelineIndicatorValue(filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the value of pipeline %s statistics indicator %s "+
				"from model failed: %v]", filter.PipelineName, filter.IndicatorName, err)
			return nil, err, RetrievePipelineStatValueError
		}

		ret = r
	case *FilterPipelineIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		r := new(ResultStatIndicatorDesc)
		r.Desc, err = stat.PipelineIndicatorDescription(filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the description of pipeline %s statistics indicator %s "+
				"from model failed: %v]", filter.PipelineName, filter.IndicatorName, err)
			return nil, err, RetrievePipelineStatDescError
		}

		ret = r
	case *FilterPluginIndicatorNames:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		r := new(ResultStatIndicatorNames)
		r.Names = stat.PluginIndicatorNames(filter.PluginName)

		// returns with stable order
		sort.Strings(r.Names)

		ret = r
	case *FilterPluginIndicatorValue:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		r := new(ResultStatIndicatorValue)
		r.Value, err = stat.PluginIndicatorValue(filter.PluginName, filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the value of plugin %s statistics indicator %s in pipeline %s "+
				"from model failed: %v]", filter.PluginName, filter.IndicatorName,
				filter.PipelineName, err)
			return nil, err, RetrievePluginStatValueError
		}

		ret = r
	case *FilterPluginIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		r := new(ResultStatIndicatorDesc)
		r.Desc, err = stat.PluginIndicatorDescription(filter.PluginName, filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the description of plugin %s statistics indicator %s "+
				"in pipeline %s from model failed: %v]", filter.PluginName, filter.IndicatorName,
				filter.PipelineName, err)
			return nil, err, RetrievePluginStatDescError
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

		r := new(ResultStatIndicatorValue)
		r.Value, err = stat.TaskIndicatorValue(filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the value of task statistics indicator %s in pipeline %s "+
				"from model failed: %v]", filter.IndicatorName, filter.PipelineName, err)
			return nil, err, RetrieveTaskStatValueError
		}

		ret = r
	case *FilterTaskIndicatorDesc:
		stat := statRegistry.GetPipelineStatistics(filter.PipelineName)
		if stat == nil {
			return nil, fmt.Errorf("pipeline %s statistics not found", filter.PipelineName),
				PipelineStatNotFoundError
		}

		r := new(ResultStatIndicatorDesc)
		r.Desc, err = stat.TaskIndicatorDescription(filter.IndicatorName)
		if err != nil {
			logger.Errorf("[retrieve the description of task statistics indicator %s in pipeline %s "+
				"from model failed: %v]", filter.IndicatorName, filter.PipelineName, err)
			return nil, err, RetrieveTaskStatDescError
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

	return retBuff, nil, NoneError
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
		respondStatErr(req, errType, err.Error())
		return
	}

	resp, err, errType := gc.getLocalStatResp(reqStat)
	if err != nil {
		respondStatErr(req, errType, err.Error())
		return
	}

	respondStat(req, resp)
}

func (gc *GatewayCluster) handleStat(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqStat, err, errType := unpackReqStat(req.RequestPayload[1:])
	if err != nil {
		respondStatErr(req, errType, err.Error())
		return
	}

	localResp, err, errType := gc.getLocalStatResp(reqStat)
	if err != nil {
		respondStatErr(req, errType, err.Error())
		return
	}

	requestMembers := gc.restAliveMembersInSameGroup()
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
		Timeout: reqStat.Timeout,
	}

	requestName := fmt.Sprintf("%s_relay", req.RequestName)
	requestPayload := make([]byte, len(req.RequestPayload))
	copy(requestPayload, req.RequestPayload)
	requestPayload[0] = byte(statRelayMessage)

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		logger.Errorf("[send stat relay message failed: %v]", err)
		respondRetrieveErr(req, InternalServerError, err.Error())
		return
	}

	membersRespBook := make(map[string][]byte)
	for _, memberName := range requestMemberNames {
		membersRespBook[memberName] = nil
	}

	gc.recordResp(requestName, future, membersRespBook)

	var validRespList []*RespStat
	validRespList = append(validRespList, localResp)

	for _, payload := range membersRespBook {
		if len(payload) == 0 {
			continue
		}

		resp := new(RespStat)
		err := cluster.Unpack(payload[1:], resp)
		if err != nil || resp.Err != nil {
			continue
		}

		validRespList = append(validRespList, resp)
	}

	ret := aggregateStatResponses(reqStat, validRespList)
	if ret != nil {
		respondRetrieveErr(req, InternalServerError, "aggreate statistics for cluster memebers failed")
		return
	}

	respondStat(req, ret)
}

type stateAggregator func(values ...[]byte) []byte

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

		resp := new(RespStat)
		resp.Value = aggregator(values...)
		if resp.Value != nil {
			return resp
		}
		return nil
	}

	return nil
}

func NumericSum(typ interface{}, values ...[]byte) []byte {
	if len(values) == 0 {
		// defensive programming
		return nil
	}

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
		}
		ret = sum
	default:
		return nil
	}

	retBuff, err := json.Marshal(ret)
	if err != nil {
		return retBuff
	}

	return retBuff
}

func NumericAvg(typ interface{}, values ...[]byte) []byte {
	if len(values) == 0 {
		// defensive programming
		return nil
	}

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
		}
		if count == 0 {
			return nil
		}
		ret = sum / count
	default:
		return nil
	}

	retBuff, err := json.Marshal(ret)
	if err != nil {
		return retBuff
	}

	return retBuff
}

func sumFloat64(values ...[]byte) []byte {
	return NumericSum(float64(0), values...)
}

func avgFloat64(values ...[]byte) []byte {
	return NumericAvg(float64(0), values...)
}

func sumUint64(values ...[]byte) []byte {
	return NumericSum(uint64(0), values...)
}

func avgUint64(values ...[]byte) []byte {
	return NumericAvg(uint64(0), values...)
}

func sumInt64(values ...[]byte) []byte {
	return NumericSum(int64(0), values...)
}

func avgInt64(values ...[]byte) []byte {
	return NumericAvg(int64(0), values...)
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
