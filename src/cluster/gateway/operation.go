package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cluster"
	"logger"
	"model"
	"plugins"
)

// for api
func (gc *GatewayCluster) queryGroupMaxSeq(group string, timeout time.Duration) (uint64, *HTTPError) {
	requestParam := cluster.RequestParam{
		TargetNodeTags: map[string]string{
			groupTagKey: group,
			modeTagKey:  WriteMode.String(),
		},
		Timeout: timeout,
	}

	requestName := fmt.Sprintf("(group:%s)query_group_max_sequence", group)
	requestPayload, err := cluster.PackWithHeader(&ReqQueryGroupMaxSeq{}, uint8(queryGroupMaxSeqMessage))
	if err != nil {
		return 0, NewHTTPError(err.Error(), http.StatusInternalServerError)
	}

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		return 0, NewHTTPError(err.Error(), http.StatusInternalServerError)
	}

	memberResp, ok := <-future.Response()
	if !ok {
		return 0, NewHTTPError("timeout", http.StatusGatewayTimeout)
	}
	if len(memberResp.Payload) == 0 {
		return 0, NewHTTPError("empty response", http.StatusInternalServerError)
	}

	var resp RespQueryGroupMaxSeq
	err = cluster.Unpack(memberResp.Payload[1:], &resp)
	if err != nil {
		return 0, NewHTTPError(err.Error(), http.StatusInternalServerError)
	}

	return uint64(resp), nil
}

func (gc *GatewayCluster) issueOperation(group string, syncAll bool, timeout time.Duration,
	requestName string, operation Operation) *HTTPError {
	for {
		queryStart := time.Now()
		ms, httpErr := gc.queryGroupMaxSeq(group, timeout)
		if httpErr != nil {
			return httpErr
		}
		expiredDuration := time.Now().Sub(queryStart)
		if timeout <= expiredDuration {
			return NewHTTPError("timeout", http.StatusGatewayTimeout)
		}
		timeout -= expiredDuration

		operation.Seq = ms + 1
		req := ReqOperation{
			OperationAllNodes: syncAll,
			Timeout:           timeout,
			Operation:         operation,
		}
		requestPayload, err := cluster.PackWithHeader(&req, uint8(operationMessage))
		if err != nil {
			return NewHTTPError(err.Error(), http.StatusInternalServerError)
		}
		requestParam := cluster.RequestParam{
			TargetNodeTags: map[string]string{
				groupTagKey: group,
				modeTagKey:  WriteMode.String(),
			},
			Timeout: timeout,
		}

		future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
		if err != nil {
			return NewHTTPError(err.Error(), http.StatusInternalServerError)
		}

		memberResp, ok := <-future.Response()
		if !ok {
			return NewHTTPError("timeout", http.StatusGatewayTimeout)
		}
		if len(memberResp.Payload) == 0 {
			return NewHTTPError("empty response", http.StatusGatewayTimeout)
		}

		var resp RespOperation
		err = cluster.Unpack(memberResp.Payload[1:], &resp)
		if err != nil {
			return NewHTTPError(err.Error(), http.StatusInternalServerError)
		}
		if resp.Err != nil {
			if resp.Err.Type == OperationWrongSeqError {
				continue
			}

			var code int
			switch resp.Err.Type {
			case WrongFormatError, OperationWrongContentError:
				code = http.StatusBadRequest
			case TimeoutError:
				code = http.StatusGatewayTimeout
			case OperationPartiallySucceedError:
				code = http.StatusAccepted
			default:
				code = http.StatusInternalServerError
			}
			return NewHTTPError(resp.Err.Message, code)
		}

		return nil
	}
}

////

// for core
func unpackReqOperation(payload []byte) (*ReqOperation, error) {
	reqOperation := new(ReqOperation)
	err := cluster.Unpack(payload, reqOperation)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to ReqOperation failed: %v", payload, err)
	}

	if reqOperation.Timeout < 1*time.Second {
		return nil, fmt.Errorf("timeout is less than 1 second")
	}
	operation := reqOperation.Operation
	switch {
	case operation.ContentCreatePlugin != nil:
		if len(operation.ContentCreatePlugin.Type) == 0 {
			return nil, fmt.Errorf("empty type")
		}
		if operation.ContentCreatePlugin.Config == nil {
			return nil, fmt.Errorf("empty config")
		}
	case operation.ContentUpdatePlugin != nil:
		if len(operation.ContentUpdatePlugin.Type) == 0 {
			return nil, fmt.Errorf("empty type")
		}
		if operation.ContentUpdatePlugin.Config == nil {
			return nil, fmt.Errorf("empty config")
		}
	case operation.ContentDeletePlugin != nil:
		if len(operation.ContentDeletePlugin.Name) == 0 {
			return nil, fmt.Errorf("empty name")
		}

	case operation.ContentCreatePipeline != nil:
		if len(operation.ContentCreatePipeline.Type) == 0 {
			return nil, fmt.Errorf("empty type")
		}
		if operation.ContentCreatePipeline.Config == nil {
			return nil, fmt.Errorf("empty config")
		}
	case operation.ContentUpdatePipeline != nil:
		if len(operation.ContentUpdatePipeline.Type) == 0 {
			return nil, fmt.Errorf("empty type")
		}
		if operation.ContentUpdatePipeline.Config == nil {
			return nil, fmt.Errorf("empty config")
		}
	case operation.ContentDeletePipeline != nil:
		if len(operation.ContentDeletePipeline.Name) == 0 {
			return nil, fmt.Errorf("empty name")
		}
	default:
		return nil, fmt.Errorf("empty operation content")
	}

	return reqOperation, nil
}

func respondOperation(req *cluster.RequestEvent, resp *RespOperation) {
	// defensive programming
	if len(req.RequestPayload) == 0 {
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

func respondOperationErr(req *cluster.RequestEvent, typ ClusterErrorType, msg string) {
	resp := &RespOperation{
		Err: &ClusterError{
			Type:    typ,
			Message: msg,
		},
	}
	respondOperation(req, resp)
}

func (gc *GatewayCluster) handleOperationRelay(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqOperation, err := unpackReqOperation(req.RequestPayload[1:])
	if err != nil {
		respondOperationErr(req, WrongFormatError, err.Error())
		return
	}

	ms := gc.log.maxSeq()
	if ms+gc.conf.OPLogMaxSeqGapToPull < reqOperation.Operation.Seq {
		respondOperationErr(req, OperationLogHugeGapError,
			fmt.Sprintf("want sync to %d but local is %d, skipped since local is too old",
				reqOperation.Operation.Seq, ms))
		return
	}

	// ignore timeout handling on relayed request operation, which is controlled by under layer

	err, errType := gc.log.append(&reqOperation.Operation)
	if err != nil {
		respondOperationErr(req, errType, err.Error())
		return
	}

	resp := new(RespOperation) // nil Err
	respondOperation(req, resp)
	return

}

func (gc *GatewayCluster) handleOperation(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqOperation, err := unpackReqOperation(req.RequestPayload[1:])
	if err != nil {
		respondOperationErr(req, WrongFormatError, err.Error())
		return
	}

	ms := gc.log.maxSeq()
	if ms+1 != reqOperation.Operation.Seq {
		respondOperationErr(req, OperationWrongSeqError, fmt.Sprintf("we need sequence %d", ms+1))
		return
	}

	err = gc.LandOperation(&reqOperation.Operation)
	if err != nil {
		respondOperationErr(req, OperationWrongContentError, err.Error())
		return
	}

	if !reqOperation.OperationAllNodes {
		resp := new(RespOperation) // nil Err
		respondOperation(req, resp)
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
		Timeout: reqOperation.Timeout,
	}

	requestName := fmt.Sprintf("%s_relay", req.RequestName)
	requestPayload := make([]byte, len(req.RequestPayload))
	copy(requestPayload, req.RequestPayload)
	requestPayload[0] = byte(operationRelayMessage)

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		respondOperationErr(req, InternalServerError,
			fmt.Sprintf("braodcast message failed: %s", err.Error()))
		return
	}

	membersRespBook := make(map[string][]byte)
	for _, memberName := range requestMemberNames {
		membersRespBook[memberName] = nil
	}

	gc.recordResp(requestName, future, membersRespBook)

	memberCorrectRespCount := 0
	for _, payload := range membersRespBook {
		if len(payload) == 0 {
			continue
		}
		resp := new(RespOperation)
		err := cluster.Unpack(payload[1:], resp)
		if err != nil {
			continue
		}

		if resp.Err != nil {
			continue
		}
		memberCorrectRespCount++
	}

	if memberCorrectRespCount < len(membersRespBook) {
		respondRetrieveErr(req, OperationPartiallySucceedError,
			fmt.Sprintf("partially succeed in %d nodes", memberCorrectRespCount+1)) // add self
		return
	}

	resp := new(RespOperation) // nil Err
	respondOperation(req, resp)
}

// LandOperation could be wrapped then added to oplog.operationAppendedCallbacks.
func (gc *GatewayCluster) LandOperation(operation *Operation) error {
	switch {
	case operation.ContentCreatePlugin != nil:
		content := operation.ContentCreatePlugin
		conf, err := plugins.GetConfig(content.Type)
		if err != nil {
			return err
		}
		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			return err
		}
		constructor, err := plugins.GetConstructor(content.Type)
		if err != nil {
			return err
		}

		_, err = gc.mod.AddPlugin(content.Type, conf, constructor)
		if err != nil {
			return err
		}
	case operation.ContentUpdatePlugin != nil:
		content := operation.ContentUpdatePlugin
		conf, err := plugins.GetConfig(content.Type)
		if err != nil {
			return err
		}
		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			return err
		}

		err = gc.mod.UpdatePluginConfig(conf)
		if err != nil {
			return err
		}
	case operation.ContentDeletePlugin != nil:
		content := operation.ContentDeletePlugin
		err := gc.mod.DeletePlugin(content.Name)
		if err != nil {
			return err
		}
	case operation.ContentCreatePipeline != nil:
		content := operation.ContentCreatePipeline
		// FIXME: (@zhiyan) Is it more appropriate to put this method
		// into pipelines package: pipelines.GetConfig(typ string).
		// It's consistent with plugins.GetConfig. Or maybe you have concenrned it.
		conf, err := model.GetPipelineConfig(content.Type)
		if err != nil {
			return err
		}
		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			return err
		}

		_, err = gc.mod.AddPipeline(content.Type, conf)
		if err != nil {
			return err
		}
	case operation.ContentUpdatePipeline != nil:
		content := operation.ContentUpdatePipeline
		conf, err := model.GetPipelineConfig(content.Type)
		if err != nil {
			return err
		}
		err = json.Unmarshal(content.Config, conf)
		if err != nil {
			return err
		}

		err = gc.mod.UpdatePipelineConfig(conf)
		if err != nil {
			return err
		}
	case operation.ContentDeletePipeline != nil:
		content := operation.ContentDeletePipeline
		err := gc.mod.DeletePipeline(content.Name)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("operation with sequence %d has no content", operation.Seq)
	}

	return nil
}
