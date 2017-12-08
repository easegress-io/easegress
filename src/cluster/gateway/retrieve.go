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
	requestName string, syncAll bool, filter interface{}) (interface{}, *ClusterError) {
	req := &ReqRetrieve{
		RetrieveAllNodes: syncAll,
		Timeout:          timeout,
	}

	switch filter := filter.(type) {
	case *FilterRetrievePlugin:
		req.FilterRetrievePlugin = filter
	case *FilterRetrievePlugins:
		req.FilterRetrievePlugins = filter
	case *FilterRetrievePipeline:
		req.FilterRetrievePipeline = filter
	case *FilterRetrievePipelines:
		req.FilterRetrievePipelines = filter
	case *FilterRetrievePluginTypes:
		req.FilterRetrievePluginTypes = filter
	case *FilterRetrievePipelineTypes:
		req.FilterRetrievePipelineTypes = filter
	default:
		return nil, newClusterError(
			fmt.Sprintf("unsupported retrieve filter type %T", filter), InternalServerError)
	}

	requestPayload, err := cluster.PackWithHeader(req, uint8(retrieveMessage))
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed: %v]", uint8(retrieveMessage), req, err)

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
		return nil, newClusterError(
			fmt.Sprintf("issue retrieve failed: %v", err), InternalServerError)
	}

	var memberResp *cluster.MemberResponse

	select {
	case r, ok := <-future.Response():
		if !ok {
			return nil, newClusterError("issue retrieve timeout", TimeoutError)
		}
		memberResp = r
	case <-gc.stopChan:
		return nil, newClusterError(
			"the member gone during issuing retrieve", IssueMemberGoneError)
	}

	if len(memberResp.Payload) == 0 {
		return nil, newClusterError(
			"issue retrieve responds empty response", InternalServerError)
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

	switch filter.(type) {
	case *FilterRetrievePlugin:
		ret := new(ResultRetrievePlugin)
		err = json.Unmarshal(resp.ResultRetrievePlugin, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarsh retrieve plugin response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarsh retrieve plugin response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterRetrievePlugins:
		ret := new(ResultRetrievePlugins)
		err = json.Unmarshal(resp.ResultRetrievePlugins, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarsh retrieve plugins response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarsh retrieve plugins response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterRetrievePipeline:
		ret := new(ResultRetrievePipeline)
		err = json.Unmarshal(resp.ResultRetrievePipeline, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarsh retrieve pipeline response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarsh retrieve pipeline response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterRetrievePipelines:
		ret := new(ResultRetrievePipelines)
		err = json.Unmarshal(resp.ResultRetrievePipelines, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarsh retrieve pipelines response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarsh retrieve pipelines response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterRetrievePluginTypes:
		ret := new(ResultRetrievePluginTypes)
		err = json.Unmarshal(resp.ResultRetrievePluginTypes, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarsh retrieve plugin types response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarsh retrieve plugin types response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	case *FilterRetrievePipelineTypes:
		ret := new(ResultRetrievePipelineTypes)
		err = json.Unmarshal(resp.ResultRetrievePipelineTypes, ret)
		if err != nil {
			logger.Errorf("[BUG: unmarsh retrieve pipeline types response failed: %v]", err)
			return nil, newClusterError(
				fmt.Sprintf("unmarsh retrieve pipeline types response failed: %v", err),
				InternalServerError)
		}

		return ret, nil
	}

	return nil, newClusterError(fmt.Sprintf("unmarsh retrieve response failed: %v", err), InternalServerError)
}

// for core
func unpackReqRetrieve(payload []byte) (*ReqRetrieve, error, ClusterErrorType) {
	reqRetrieve := new(ReqRetrieve)
	err := cluster.Unpack(payload, reqRetrieve)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to ReqRetrieve failed: %v", payload, err), WrongMessageFormatError
	}

	switch {
	case reqRetrieve.FilterRetrievePlugin != nil:
	case reqRetrieve.FilterRetrievePlugins != nil:
	case reqRetrieve.FilterRetrievePipeline != nil:
	case reqRetrieve.FilterRetrievePipelines != nil:
	case reqRetrieve.FilterRetrievePluginTypes != nil:
	case reqRetrieve.FilterRetrievePipelineTypes != nil:
	default:
		return nil, fmt.Errorf("empty retieve filter"), InternalServerError
	}

	return reqRetrieve, nil, NoneClusterError
}

func (gc *GatewayCluster) respondRetrieve(req *cluster.RequestEvent, resp *RespRetrieve) {
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

	logger.Debugf("[member %s responded operationRelayMessage message]", gc.clusterConf.NodeName)
}

func (gc *GatewayCluster) respondRetrieveErr(req *cluster.RequestEvent, typ ClusterErrorType, msg string) {
	resp := &RespRetrieve{
		Err: newClusterError(msg, typ),
	}
	gc.respondRetrieve(req, resp)
}

func (gc *GatewayCluster) retrieveResult(filter interface{}) ([]byte, error, ClusterErrorType) {
	var ret interface{}

	switch filter := filter.(type) {
	case *FilterRetrievePlugin:
		plug, _ := gc.mod.GetPlugin(filter.Name)
		if plug == nil {
			logger.Errorf("[plugin %s not found]", filter.Name)
			return nil, fmt.Errorf("plugin %s not found", filter.Name), RetrievePluginNotFoundError
		}

		r := new(ResultRetrievePlugin)
		r.Plugin = config.PluginSpec{
			Type:   plug.Type(),
			Config: plug.Config(),
		}

		ret = r
	case *FilterRetrievePlugins:
		plugins, err := gc.mod.GetPlugins(filter.NamePattern, filter.Types)
		if err != nil {
			logger.Errorf("[retrieve plugins from model failed: %v]", err)
			return nil, err, RetrievePluginsError
		}

		r := new(ResultRetrievePlugins)
		r.Plugins = make([]config.PluginSpec, 0)

		for _, plug := range plugins {
			spec := config.PluginSpec{
				Type:   plug.Type(),
				Config: plug.Config(),
			}
			r.Plugins = append(r.Plugins, spec)
		}

		ret = r
	case *FilterRetrievePipeline:
		pipe := gc.mod.GetPipeline(filter.Name)
		if pipe == nil {
			logger.Errorf("[pipeline %s not found]", filter.Name)
			return nil, fmt.Errorf("pipeline %s not found", filter.Name), RetrievePipelineNotFoundError
		}

		r := new(ResultRetrievePipeline)
		r.Pipeline = config.PipelineSpec{
			Type:   pipe.Type(),
			Config: pipe.Config(),
		}

		ret = r
	case *FilterRetrievePipelines:
		pipelines, err := gc.mod.GetPipelines(filter.NamePattern, filter.Types)
		if err != nil {
			logger.Errorf("[retrieve pipelines from model failed: %v]", err)
			return nil, err, RetrievePipelinesError
		}

		r := new(ResultRetrievePipelines)
		r.Pipelines = make([]config.PipelineSpec, 0)

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

	return retBuff, nil, NoneClusterError
}

func (gc *GatewayCluster) getLocalRetrieveResp(reqRetrieve *ReqRetrieve) (*RespRetrieve, error, ClusterErrorType) {
	ret := new(RespRetrieve)

	// for emphasizing
	var err error
	var errType ClusterErrorType

	switch {
	case reqRetrieve.FilterRetrievePlugin != nil:
		ret.ResultRetrievePlugin, err, errType =
			gc.retrieveResult(reqRetrieve.FilterRetrievePlugin)
	case reqRetrieve.FilterRetrievePlugins != nil:
		ret.ResultRetrievePlugins, err, errType =
			gc.retrieveResult(reqRetrieve.FilterRetrievePlugins)
	case reqRetrieve.FilterRetrievePipeline != nil:
		ret.ResultRetrievePipeline, err, errType =
			gc.retrieveResult(reqRetrieve.FilterRetrievePipeline)
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
		gc.respondRetrieveErr(req, errType, err.Error())
		return
	}

	resp, err, errType := gc.getLocalRetrieveResp(reqRetrieve)
	if err != nil {
		gc.respondRetrieveErr(req, errType, err.Error())
		return
	}

	gc.respondRetrieve(req, resp)
}

func (gc *GatewayCluster) handleRetrieve(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqRetrieve, err, errType := unpackReqRetrieve(req.RequestPayload[1:])
	if err != nil {
		gc.respondRetrieveErr(req, errType, err.Error())
		return
	}

	resp, err, errType := gc.getLocalRetrieveResp(reqRetrieve)
	if err != nil {
		gc.respondRetrieveErr(req, errType, err.Error())
		return
	}

	if !reqRetrieve.RetrieveAllNodes {
		gc.respondRetrieve(req, resp)
		return
	}

	requestMembers := gc.RestAliveMembersInSameGroup()
	if len(requestMembers) == 0 {
		gc.respondRetrieve(req, resp)
		return
	}

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
		gc.respondRetrieveErr(req, InternalServerError, err.Error())
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
		gc.respondRetrieveErr(req, TimeoutError, "retrieve timeout")
		return
	}

	respToCompare, err := cluster.PackWithHeader(resp, uint8(retrieveRelayMessage))
	if err != nil {
		logger.Errorf("[BUG: pack retrieve relay message failed: %v]", err)
		gc.respondRetrieveErr(req, InternalServerError, err.Error())
		return
	}

	for member, payload := range membersRespBook {
		if bytes.Compare(respToCompare, payload) != 0 {
			gc.respondRetrieveErr(req, RetrieveInconsistencyError,
				fmt.Sprintf("retrieve results from different members (%s, %s) are inconsistent", gc.conf.ClusterMemberName, member))
			return
		}
	}

	gc.respondRetrieve(req, resp)
}
