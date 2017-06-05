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
		logger.Errorf("[BUG: PackWithHeader %d %#v failed: %v]", req.RequestPayload[0], resp, err)
		return
	}

	err = req.Respond(respBuff)
	if err != nil {
		logger.Errorf("[respond %s to request %s, node %s failed: %v]", respBuff, req.RequestName, req.RequestNodeName, err)
		return
	}
}

func respondRetrieveErr(req *cluster.RequestEvent, typ MessageErrType, msg string) {
	resp := &RespRetrieve{
		Err: &MessageErr{
			Type: typ,
			Msg:  msg,
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
		logger.Errorf("[BUG: received empty ReqRetrieve]")
		return nil
	}
	reqRetrieve, err := unpackReqRetrieve(req.RequestPayload[1:])

	if err != nil {
		respondRetrieveErr(req, ErrWrongFormat, err.Error())
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
		respondRetrieveErr(req, ErrInternalServer, err.Error())
		return nil
	}

	return resp
}

func (gc *GatewayCluster) handleRetrieveRelayed(req *cluster.RequestEvent) {
	resp := gc.getLocalRetrieveResp(req)
	if resp == nil {
		return
	}
	respondRetrieve(req, resp)
}

func (gc *GatewayCluster) handleRetrieve(req *cluster.RequestEvent) {
	resp := gc.getLocalRetrieveResp(req)
	if resp == nil {
		return
	}

	respToCompare, err := cluster.PackWithHeader(resp, uint8(retrieveRelayedMessage))
	if err != nil {
		logger.Errorf("[BUG: PackWithHeader %d %#v failed: %v]", req.RequestPayload[0], resp, err)
		return
	}

	// defensive programming
	if len(req.RequestPayload) < 1 {
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

	requestParam := cluster.RequestParam{
		TargetNodeTags: map[string]string{
			groupTagKey: gc.localGroupName(),
		},
	}
	payload := req.RequestPayload
	payload[0] = byte(retrieveRelayedMessage)
	future, err := gc.cluster.Request(req.RequestName+"_relayed", req.RequestPayload, &requestParam)
	if err != nil {
		fmt.Errorf("send request %s")
	}

	members := gc.otherSameGroupMemebers()
	membersRespBook := make(map[string][]byte)
	for _, member := range members {
		membersRespBook[member.NodeName] = nil
	}

	// FIXME: config waitTime
	waitTime := time.Duration(30 * time.Second)
	timer := time.NewTimer(waitTime)

	var memberRespCount int
loop:
	for ; memberRespCount < len(membersRespBook); memberRespCount++ {
		select {
		case <-timer.C:
			break loop
		case memberResp := <-future.Response():
			payload, ok := membersRespBook[memberResp.ResponseNodeName]
			if !ok {
				// maybe not a bug, a new node is up within the same group
				logger.Errorf("[received unexpected response from request %s, node %s]", req.RequestName+"_relayed", memberResp.ResponseNodeName)
				memberRespCount--
				continue loop
			}

			if payload != nil {
				logger.Errorf("[BUG: received multiple response from request %s, node %s]", req.RequestName+"_relayed", memberResp.ResponseNodeName)
				memberRespCount--
				continue loop
			}

			if memberResp.Payload != nil {
				membersRespBook[memberResp.ResponseNodeName] = memberResp.Payload
			} else {
				membersRespBook[memberResp.ResponseNodeName] = []byte("")
			}
		}
	}

	if memberRespCount < len(membersRespBook) {
		respondRetrieveErr(req, ErrRetrieveTimeout, fmt.Sprintf("retrieve timeou %v", waitTime))
		return
	}

	for _, payload := range membersRespBook {
		if bytes.Compare(respToCompare, payload) != 0 {
			respondRetrieveErr(req, ErrRetrieveInconsistency, "retrieve inconsistent content")
			return
		}
	}

	respondRetrieve(req, resp)
}
