package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"cluster"
	"common"
	"config"
	"logger"
	"pipelines"
	"plugins"
)

// for api
func (gc *GatewayCluster) issueRetrieve(group string, timeout time.Duration,
	requestName string, syncAll bool, filter interface{}) ([]byte, *ClusterError) {

	req := &ReqRetrieve{
		RetrieveAllNodes: syncAll,
		Timeout:          timeout,
	}

	switch filter := filter.(type) {
	case *FilterRetrievePlugins:
		req.FilterRetrievePlugins = filter
	case *FilterRetrievePipelines:
		req.FilterRetrievePipelines = filter
	case *FilterRetrievePluginTypes:
		req.FilterRetrievePluginTypes = filter
	case *FilterRetrievePipelineTypes:
		req.FilterRetrievePipelineTypes = filter
	default:
		return nil, newClusterError(fmt.Sprintf("unsupported retrieve filter type %T", filter),
			InternalServerError)
	}

	requestPayload, err := cluster.PackWithHeader(req, uint8(retrieveMessage))
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed: %v]",
			uint8(retrieveMessage), req, err)

		return nil, newClusterError(
			fmt.Sprintf("pack request (header=%d) to %#v failed: %v",
				uint8(retrieveMessage), req, err),
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

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("issue retrieve failed: %v", err), InternalServerError)
	}

	var memberResp *cluster.MemberResponse

	select {
	case r, ok := <-future.Response():
		if !ok {
			return nil, newClusterError("issue retrieve timeout", TimeoutError)
		}
		memberResp = r
	case <-gc.stopChan:
		return nil, newClusterError("the member gone during issuing retrieve", IssueMemberGoneError)
	}

	if len(memberResp.Payload) == 0 {
		return nil, newClusterError("issue retrieve responds empty response", InternalServerError)
	}

	resp := new(RespRetrieve)
	err = cluster.Unpack(memberResp.Payload[1:], resp)
	if err != nil {
		return nil, newClusterError(
			fmt.Sprintf("unpack retrieve response failed: %v", err), InternalServerError)
	}

	if resp.Err != nil {
		return nil, resp.Err
	}

	var ret []byte
	switch filter.(type) {
	case FilterRetrievePlugins:
		ret = resp.ResultRetrievePlugins
	case FilterRetrievePipelines:
		ret = resp.ResultRetrievePipelines
	case FilterRetrievePluginTypes:
		ret = resp.ResultRetrievePluginTypes
	case FilterRetrievePipelineTypes:
		ret = resp.ResultRetrievePipelineTypes
	}

	if ret == nil || len(ret) == 0 {
		return nil, newClusterError("issue retrieve responds invalid result", InternalServerError)
	}

	return ret, nil
}

// for core
func unpackReqRetrieve(payload []byte) (*ReqRetrieve, error, ClusterErrorType) {
	reqRetrieve := new(ReqRetrieve)
	err := cluster.Unpack(payload, reqRetrieve)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to ReqRetrieve failed: %v", payload, err), WrongMessageFormatError
	}

	switch {
	case reqRetrieve.FilterRetrievePlugins != nil:
	case reqRetrieve.FilterRetrievePipelines != nil:
	case reqRetrieve.FilterRetrievePluginTypes != nil:
	case reqRetrieve.FilterRetrievePipelineTypes != nil:
	default:
		return nil, fmt.Errorf("empty retieve filter"), InternalServerError
	}

	return reqRetrieve, nil, NoneError
}

func respondRetrieve(req *cluster.RequestEvent, resp *RespRetrieve) {
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

func respondRetrieveErr(req *cluster.RequestEvent, typ ClusterErrorType, msg string) {
	resp := &RespRetrieve{
		Err: newClusterError(msg, typ),
	}
	respondRetrieve(req, resp)
}

func (gc *GatewayCluster) retrieveResult(filter interface{}) ([]byte, error, ClusterErrorType) {
	var ret interface{}

	switch filter := filter.(type) {
	case *FilterRetrievePlugins:
		plugins, err := gc.mod.GetPlugins(filter.NamePattern, filter.Types)
		if err != nil {
			logger.Errorf("[retrieve plugins from model failed: %v]", err)
			return nil, err, RetrievePluginsError
		}

		r := new(ResultRetrievePlugins)
		for _, plug := range plugins {
			spec := config.PluginSpec{
				Type:   plug.Type(),
				Config: plug.Config(),
			}
			r.Plugins = append(r.Plugins, spec)
		}

		ret = r
	case *FilterRetrievePipelines:
		pipelines, err := gc.mod.GetPipelines(filter.NamePattern, filter.Types)
		if err != nil {
			logger.Errorf("[retrieve pipelines from model failed: %v]", err)
			return nil, err, RetrievePipelinesError
		}

		r := new(ResultRetrievePipelines)
		for _, pipe := range pipelines {
			spec := config.PipelineSpec{
				Type:   pipe.Type(),
				Config: pipe.Config(),
			}
			r.Pipelines = append(r.Pipelines, spec)
		}

		ret = r
	case *FilterRetrievePluginTypes:
		r := new(ResultRetrievePluginTypes)
		r.PluginTypes = make([]string, 0)

		for _, typ := range plugins.GetAllTypes() {
			// defensively
			if !common.StrInSlice(typ, r.PluginTypes) {
				r.PluginTypes = append(r.PluginTypes, typ)
			}
		}

		// returns with stable order
		sort.Strings(r.PluginTypes)

		ret = r
	case *FilterRetrievePipelineTypes:
		r := new(ResultRetrievePipelineTypes)
		r.PipelineTypes = pipelines.GetAllTypes()

		// returns with stable order
		sort.Strings(r.PipelineTypes)

		ret = r
	default:
		return nil, fmt.Errorf("unsupported retrieve filter type %T", filter), InternalServerError
	}

	retBuff, err := json.Marshal(ret)
	if err != nil {
		logger.Errorf("[BUG: marshal retrieve result failed: %v]", err)
		return nil, fmt.Errorf("marshal retrieve result failed: %v", err), InternalServerError
	}

	return retBuff, nil, NoneError
}

func (gc *GatewayCluster) getLocalRetrieveResp(reqRetrieve *ReqRetrieve) (*RespRetrieve, error, ClusterErrorType) {
	ret := new(RespRetrieve)

	// for emphasizing
	var err error
	var errType ClusterErrorType

	switch {
	case reqRetrieve.FilterRetrievePlugins != nil:
		ret.ResultRetrievePlugins, err, errType =
			gc.retrieveResult(reqRetrieve.FilterRetrievePlugins)
	case reqRetrieve.FilterRetrievePipelines != nil:
		ret.ResultRetrievePipelines, err, errType =
			gc.retrieveResult(reqRetrieve.FilterRetrievePipelines)
	case reqRetrieve.FilterRetrievePluginTypes != nil:
		ret.ResultRetrievePluginTypes, err, errType =
			gc.retrieveResult(reqRetrieve.FilterRetrievePluginTypes)
	case reqRetrieve.FilterRetrievePipelineTypes != nil:
		ret.ResultRetrievePipelineTypes, err, errType =
			gc.retrieveResult(reqRetrieve.FilterRetrievePipelineTypes)
	}

	if err != nil {
		return nil, err, errType
	}

	return ret, err, errType
}

func (gc *GatewayCluster) handleRetrieveRelay(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqRetrieve, err, errType := unpackReqRetrieve(req.RequestPayload[1:])
	if err != nil {
		respondRetrieveErr(req, errType, err.Error())
		return
	}

	resp, err, errType := gc.getLocalRetrieveResp(reqRetrieve)
	if err != nil {
		respondRetrieveErr(req, errType, err.Error())
		return
	}

	respondRetrieve(req, resp)
}

func (gc *GatewayCluster) handleRetrieve(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqRetrieve, err, errType := unpackReqRetrieve(req.RequestPayload[1:])
	if err != nil {
		respondRetrieveErr(req, errType, err.Error())
		return
	}

	resp, err, errType := gc.getLocalRetrieveResp(reqRetrieve)
	if err != nil {
		respondRetrieveErr(req, errType, err.Error())
		return
	}

	if !reqRetrieve.RetrieveAllNodes {
		respondRetrieve(req, resp)
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
			modeTagKey:  ReadMode.String(),
		},
		Timeout:            reqRetrieve.Timeout,
		ResponseRelayCount: 1, // fault tolerance on network issue
	}

	requestName := fmt.Sprintf("%s_relay", req.RequestName)
	requestPayload := make([]byte, len(req.RequestPayload))
	copy(requestPayload, req.RequestPayload)
	requestPayload[0] = byte(retrieveRelayMessage)

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		logger.Errorf("[send retrieve relay message failed: %v]", err)
		respondRetrieveErr(req, InternalServerError, err.Error())
		return
	}

	membersRespBook := make(map[string][]byte)
	for _, memberName := range requestMemberNames {
		membersRespBook[memberName] = nil
	}

	gc.recordResp(requestName, future, membersRespBook)

	correctMembersRespCount := 0
	for _, payload := range membersRespBook {
		if len(payload) > 0 {
			correctMembersRespCount++
		}
	}

	if correctMembersRespCount < len(membersRespBook) {
		respondRetrieveErr(req, TimeoutError, "retrieve timeout")
		return
	}

	respToCompare, err := cluster.PackWithHeader(resp, uint8(retrieveRelayMessage))
	if err != nil {
		logger.Errorf("[BUG: pack retrieve relay message failed: %v]", err)
		respondRetrieveErr(req, InternalServerError, err.Error())
		return
	}

	for _, payload := range membersRespBook {
		if bytes.Compare(respToCompare, payload) != 0 {
			respondRetrieveErr(req, RetrieveInconsistencyError,
				"retrieve results from different members are inconsistent")
			return
		}
	}

	respondRetrieve(req, resp)
}
