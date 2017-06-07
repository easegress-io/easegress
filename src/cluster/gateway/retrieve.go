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

func unpackReqRetrieve(payload []byte) (*ReqRetrieve, error) {
	reqRetrieve := new(ReqRetrieve)
	err := cluster.Unpack(payload, reqRetrieve)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to ReqRetrieve failed: %v", payload, err)
	}

	if reqRetrieve.Timeout < 1*time.Second {
		return nil, fmt.Errorf("timeout is less than 1 second")
	}

	switch {
	case reqRetrieve.FilterRetrievePlugins != nil:
	case reqRetrieve.FilterRetrievePipelines != nil:
	case reqRetrieve.FilterRetrievePluginTypes != nil:
	case reqRetrieve.FilterRetrievePipelineTypes != nil:
	default:
		return nil, fmt.Errorf("empty filter")
	}

	return reqRetrieve, nil
}

func respondRetrieve(req *cluster.RequestEvent, resp *RespRetrieve) {
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

func respondRetrieveErr(req *cluster.RequestEvent, typ ClusterErrorType, msg string) {
	resp := &RespRetrieve{
		Err: &ClusterError{
			Type:    typ,
			Message: msg,
		},
	}
	respondRetrieve(req, resp)
}

func (gc *GatewayCluster) retrieveResult(filter interface{}) ([]byte, error) {
	var ret interface{}

	switch filter := filter.(type) {
	case *FilterRetrievePlugins:
		plugs, err := gc.mod.GetPlugins(filter.NamePattern, filter.Types)
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
		ret = result
	case *FilterRetrievePipelines:
		pipes, err := gc.mod.GetPipelines(filter.NamePattern, filter.Types)
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
		ret = result
	case *FilterRetrievePluginTypes:
		result := ResultRetrievePluginTypes{}
		result.PluginTypes = make([]string, 0)
		for _, typ := range plugins.GetAllTypes() {
			if !common.StrInSlice(typ, result.PluginTypes) {
				result.PluginTypes = append(result.PluginTypes, typ)
			}
		}
		sort.Strings(result.PluginTypes)
		ret = result
	case *FilterRetrievePipelineTypes:
		result := ResultRetrievePipelineTypes{}
		result.PipelineTypes = make([]string, 0)
		for _, typ := range pipelines.GetAllTypes() {
			if !common.StrInSlice(typ, result.PipelineTypes) {
				result.PipelineTypes = append(result.PipelineTypes, typ)
			}
		}
		sort.Strings(result.PipelineTypes)
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

func (gc *GatewayCluster) getLocalRetrieveResp(req *cluster.RequestEvent) *RespRetrieve {
	if len(req.RequestPayload) < 1 {
		return nil
	}
	reqRetrieve, err := unpackReqRetrieve(req.RequestPayload[1:])
	if err != nil {
		respondRetrieveErr(req, WrongFormatError, err.Error())
		return nil
	}

	resp := new(RespRetrieve)
	err = nil // for emphasizing
	switch {
	case reqRetrieve.FilterRetrievePlugins != nil:
		resp.ResultRetrievePlugins, err = gc.retrieveResult(reqRetrieve.FilterRetrievePlugins)
	case reqRetrieve.FilterRetrievePipelines != nil:
		resp.ResultRetrievePipelines, err = gc.retrieveResult(reqRetrieve.FilterRetrievePipelines)
	case reqRetrieve.FilterRetrievePluginTypes != nil:
		resp.ResultRetrievePluginTypes, err = gc.retrieveResult(reqRetrieve.FilterRetrievePluginTypes)
	case reqRetrieve.FilterRetrievePipelineTypes != nil:
		resp.ResultRetrievePipelineTypes, err = gc.retrieveResult(reqRetrieve.FilterRetrievePipelineTypes)
	}

	if err != nil {
		respondRetrieveErr(req, InternalServerError, err.Error())
		return nil
	}

	return resp
}

func (gc *GatewayCluster) handleRetrieveRelay(req *cluster.RequestEvent) {
	if len(req.RequestPayload) < 1 {
		// defensive programming
		return
	}

	resp := gc.getLocalRetrieveResp(req)
	if resp == nil {
		return
	}

	respondRetrieve(req, resp)
}

func (gc *GatewayCluster) handleRetrieve(req *cluster.RequestEvent) {
	if len(req.RequestPayload) < 1 {
		// defensive programming
		return
	}

	resp := gc.getLocalRetrieveResp(req)
	if resp == nil {
		return
	}

	reqRetrieve, err := unpackReqRetrieve(req.RequestPayload[1:])
	if err != nil {
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
		// TargetNodeNames is enough but TargetNodeTags could make rule strict.
		TargetNodeTags: map[string]string{
			groupTagKey: gc.localGroupName(),
			modeTagKey:  ReadMode.String(),
		},
		Timeout: reqRetrieve.Timeout,
	}

	requestName := fmt.Sprintf("%s_relay", req.RequestName)
	requestPayload := make([]byte, len(req.RequestPayload))
	copy(requestPayload, req.RequestPayload)
	requestPayload[0] = byte(retrieveRelayMessage)

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

	membersRespCount := 0
	for _, payload := range membersRespBook {
		if payload != nil {
			membersRespCount++
		}
	}
	if membersRespCount < len(membersRespBook) {
		respondRetrieveErr(req, RetrieveTimeoutError, "retrieve timeout")
		return
	}

	respToCompare, err := cluster.PackWithHeader(resp, uint8(retrieveRelayMessage))
	if err != nil {
		logger.Errorf("[BUG: pack retrieve relay message failed: %v]", err)
		return
	}

	for _, payload := range membersRespBook {
		if bytes.Compare(respToCompare, payload) != 0 {
			respondRetrieveErr(req, RetrieveInconsistencyError, "retrieve inconsistent content")
			return
		}
	}

	respondRetrieve(req, resp)
}
