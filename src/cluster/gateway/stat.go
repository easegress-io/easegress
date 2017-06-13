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

// for api
func (gc *GatewayCluster) chooseMembertoAggregateStat(group string) (string, error) {
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

	// prefer to choose ReadMode member in order to reduce load of WriteMode member
	if len(readMembers) > 0 {
		rand.Seed(time.Now().UnixNano())
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

	requestPayload, err := cluster.PackWithHeader(req, uint8(statMessage))
	if err != nil {
		return nil, NewHTTPError(err.Error(), http.StatusInternalServerError)
	}
	targetMember, err := gc.chooseMembertoAggregateStat(group)
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
		case WrongFormatError:
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

type aggregateFunc func(values ...[]byte) []byte

func aggregateRespStat(reqStat *ReqStat, respStats ...RespStat) *RespStat {
	indicatorName := ""
	var aggrFunc aggregateFunc = nil
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
		if len(indicatorName) == 0 {
			indicatorName = reqStat.FilterPipelineIndicatorValue.IndicatorName
			if aggrFunc == nil {
				aggrFunc = pipelineIndicatorAggregateMap[indicatorName]
			}
		}
		fallthrough
	case reqStat.FilterPluginIndicatorValue != nil:
		if len(indicatorName) == 0 {
			indicatorName = reqStat.FilterPluginIndicatorValue.IndicatorName
			if aggrFunc == nil {
				aggrFunc = pipelineIndicatorAggregateMap[indicatorName]
			}
		}
		fallthrough
	case reqStat.FilterTaskIndicatorValue != nil:
		if len(indicatorName) == 0 {
			indicatorName = reqStat.FilterTaskIndicatorValue.IndicatorName
			if aggrFunc == nil {
				aggrFunc = pipelineIndicatorAggregateMap[indicatorName]
			}
		}

		if len(indicatorName) == 0 {
			return nil
		}

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

		// unknown indicators, just list values
		if aggrFunc == nil {
			resultBuff, err := json.Marshal(resultValues)
			if err != nil {
				return nil
			}
			return &RespStat{
				Value: resultBuff,
			}
		}

		// aggregate known indicators
		values := make([][]byte, 0)
		for _, value := range resultValues {
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
		resp.Value = aggrFunc(values...)
		if resp.Value != nil {
			return resp
		}
		return nil
	}

	return nil
}

func NumericSum(typ interface{}, values ...[]byte) []byte {
	// defensive programming
	if len(values) == 0 {
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
	// defensive programming
	if len(values) == 0 {
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

// In the table-driven design, just use function sumXXX to aggregate
// all known indicators for the time being, will refine them later.

var pipelineIndicatorAggregateMap = map[string]aggregateFunc{
	"THROUGHPUT_RATE_LAST_1MIN_ALL":  sumFloat64,
	"THROUGHPUT_RATE_LAST_5MIN_ALL":  sumFloat64,
	"THROUGHPUT_RATE_LAST_15MIN_ALL": sumFloat64,

	"EXECUTION_COUNT_ALL":    sumInt64,
	"EXECUTION_TIME_MAX_ALL": sumInt64,
	"EXECUTION_TIME_MIN_ALL": sumInt64,

	"EXECUTION_TIME_50_PERCENT_ALL": sumFloat64,
	"EXECUTION_TIME_90_PERCENT_ALL": sumFloat64,
	"EXECUTION_TIME_99_PERCENT_ALL": sumFloat64,

	"EXECUTION_TIME_STD_DEV_ALL":  sumFloat64,
	"EXECUTION_TIME_VARIANCE_ALL": sumFloat64,
	"EXECUTION_TIME_SUM_ALL":      sumInt64,
}

var pluginIndicatorAggregateMap = map[string]aggregateFunc{
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

	"EXECUTION_TIME_MAX_ALL":     sumInt64,
	"EXECUTION_TIME_MAX_SUCCESS": sumInt64,
	"EXECUTION_TIME_MAX_FAILURE": sumInt64,
	"EXECUTION_TIME_MIN_ALL":     sumInt64,
	"EXECUTION_TIME_MIN_SUCCESS": sumInt64,
	"EXECUTION_TIME_MIN_FAILURE": sumInt64,

	"EXECUTION_TIME_50_PERCENT_SUCCESS": sumFloat64,
	"EXECUTION_TIME_50_PERCENT_FAILURE": sumFloat64,
	"EXECUTION_TIME_90_PERCENT_SUCCESS": sumFloat64,
	"EXECUTION_TIME_90_PERCENT_FAILURE": sumFloat64,
	"EXECUTION_TIME_99_PERCENT_SUCCESS": sumFloat64,
	"EXECUTION_TIME_99_PERCENT_FAILURE": sumFloat64,

	"EXECUTION_TIME_STD_DEV_SUCCESS":  sumFloat64,
	"EXECUTION_TIME_STD_DEV_FAILURE":  sumFloat64,
	"EXECUTION_TIME_VARIANCE_SUCCESS": sumFloat64,
	"EXECUTION_TIME_VARIANCE_FAILURE": sumFloat64,

	"EXECUTION_TIME_SUM_ALL":     sumInt64,
	"EXECUTION_TIME_SUM_SUCCESS": sumInt64,
	"EXECUTION_TIME_SUM_FAILURE": sumInt64,

	// Plugin Dedicated Indicator
	// Plugin http_input
	"WAIT_QUEUE_LENGTH": sumUint64,
	"WIP_REQUEST_COUNT": sumUint64,

	// Plugin http_counter
	"RECENT_HEADER_COUNT ": sumUint64,
}

var taskIndicatorAggregateMap = map[string]aggregateFunc{
	// Task Dedicated Indicator
	"EXECUTION_COUNT_ALL":     sumUint64,
	"EXECUTION_COUNT_SUCCESS": sumUint64,
	"EXECUTION_COUNT_FAILURE": sumUint64,
}
