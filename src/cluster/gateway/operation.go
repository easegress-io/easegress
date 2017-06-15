package gateway

import (
	"fmt"
	"net/http"
	"time"

	"cluster"
	"logger"
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
	requestName string, operation *Operation) *HTTPError {
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
			OperateAllNodes: syncAll,
			Timeout:         timeout,
			Operation:       operation,
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
			if resp.Err.Type == OperationSeqConflictError {
				continue
			}

			var code int
			switch resp.Err.Type {
			// FIXME(zhiyan): I think continue above is wrong, returns StatusConflict seems better
			//case OperationSeqConflictError:
			//	code = http.StatusConflict
			case WrongMessageFormatError, OperationInvalidContentError, OperationSeqInterruptError:
				code = http.StatusBadRequest
			case TimeoutError:
				code = http.StatusGatewayTimeout
			case OperationPartiallyCompleteError:
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
func unpackReqOperation(payload []byte) (*ReqOperation, error, ClusterErrorType) {
	reqOperation := new(ReqOperation)
	err := cluster.Unpack(payload, reqOperation)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to ReqOperation failed: %v", payload, err), WrongMessageFormatError
	}

	operation := reqOperation.Operation
	switch {
	case operation.ContentCreatePlugin != nil:
		if len(operation.ContentCreatePlugin.Type) == 0 {
			return nil, fmt.Errorf("empty plugin type for creation"), InternalServerError
		}
		if operation.ContentCreatePlugin.Config == nil {
			return nil, fmt.Errorf("empty plugin config for creation"), InternalServerError
		}
	case operation.ContentUpdatePlugin != nil:
		if len(operation.ContentUpdatePlugin.Type) == 0 {
			return nil, fmt.Errorf("empty plugin type for update"), InternalServerError
		}
		if operation.ContentUpdatePlugin.Config == nil {
			return nil, fmt.Errorf("empty plugin config for update"), InternalServerError
		}
	case operation.ContentDeletePlugin != nil:
		if len(operation.ContentDeletePlugin.Name) == 0 {
			return nil, fmt.Errorf("empty plugin name for deletion"), InternalServerError
		}
	case operation.ContentCreatePipeline != nil:
		if len(operation.ContentCreatePipeline.Type) == 0 {
			return nil, fmt.Errorf("empty pipeline type for creation"), InternalServerError
		}
		if operation.ContentCreatePipeline.Config == nil {
			return nil, fmt.Errorf("empty pipeline config for creation"), InternalServerError
		}
	case operation.ContentUpdatePipeline != nil:
		if len(operation.ContentUpdatePipeline.Type) == 0 {
			return nil, fmt.Errorf("empty pipeline type for update"), InternalServerError
		}
		if operation.ContentUpdatePipeline.Config == nil {
			return nil, fmt.Errorf("empty pipeline config for update"), InternalServerError
		}
	case operation.ContentDeletePipeline != nil:
		if len(operation.ContentDeletePipeline.Name) == 0 {
			return nil, fmt.Errorf("empty pipeline name for deletion"), InternalServerError
		}
	default:
		return nil, fmt.Errorf("empty operation request"), InternalServerError
	}

	return reqOperation, nil, NoneError
}

func respondOperation(req *cluster.RequestEvent, resp *RespOperation) {
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

	reqOperation, err, errType := unpackReqOperation(req.RequestPayload[1:])
	if err != nil {
		respondOperationErr(req, errType, err.Error())
		return
	}

	// ignore timeout handling on relayed request operation, which is controlled by under layer

	err, errType = gc.log.append(reqOperation.StartSeq, []*Operation{reqOperation.Operation})
	if err != nil {
		switch errType {
		case OperationSeqConflictError:
			logger.Warnf("[the operation (sequence=%d) was synced before, skipped to write oplog]",
				reqOperation.StartSeq)
			goto SUCCESS
		case OperationInvalidSeqError:
			logger.Warnf("[the operation (sequence=%d) is retrieved too early to write oplog]",
				reqOperation.StartSeq)
		default: // does not make sense
			logger.Errorf("[append operation to oplog failed: %v]", err)
		}

		respondOperationErr(req, errType, err.Error())
		return
	}
SUCCESS:
	respondOperation(req, new(RespOperation))
	return
}

func (gc *GatewayCluster) handleOperation(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqOperation, err, errType := unpackReqOperation(req.RequestPayload[1:])
	if err != nil {
		respondOperationErr(req, errType, err.Error())
		return
	}

	err, errType = gc.log.append(reqOperation.StartSeq, []**Operation{reqOperation.Operation})
	if err != nil {
		logger.Errorf("[append operation to oplog failed: %v]", err)
		respondOperationErr(req, errType, err.Error())
		return
	}

	if !reqOperation.OperateAllNodes {
		respondOperation(req, new(RespOperation))
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
		Timeout: reqOperation.Timeout,
	}

	requestName := fmt.Sprintf("%s_relay", req.RequestName)
	requestPayload := make([]byte, len(req.RequestPayload))
	copy(requestPayload, req.RequestPayload)
	requestPayload[0] = byte(operationRelayMessage)

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		logger.Errorf("[send operagtion relay message failed: %v]", err)
		respondOperationErr(req, InternalServerError, err.Error())
		return
	}

	membersRespBook := make(map[string][]byte)
	for _, memberName := range requestMemberNames {
		membersRespBook[memberName] = nil
	}

	gc.recordResp(requestName, future, membersRespBook)

	correctMemberRespCount := 0
	for _, payload := range membersRespBook {
		if len(payload) == 0 {
			continue
		}

		resp := new(RespOperation)
		err := cluster.Unpack(payload[1:], resp)
		if err != nil || resp.Err != nil {
			continue
		}

		correctMemberRespCount++
	}

	if correctMemberRespCount < len(membersRespBook) {
		respondRetrieveErr(req, OperationPartiallyCompleteError,
			fmt.Sprintf("partially succeed in %d nodes", correctMemberRespCount+1)) // add myself
		return
	}

	respondOperation(req, new(RespOperation))
}
