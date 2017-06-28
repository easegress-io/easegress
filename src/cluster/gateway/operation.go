package gateway

import (
	"fmt"
	"time"

	"cluster"
	"logger"
)

// for api
func (gc *GatewayCluster) issueOperation(group string, timeout time.Duration, requestName string,
	startSeq uint64, syncAll bool, operation *Operation) *ClusterError {

	logger.Debugf("issue operation: requestName(%s) startSeq(%d) operation(%#v) syncAll(%v) timeout(%s)",
		requestName, startSeq, operation, syncAll, timeout)

	req := &ReqOperation{
		OperateAllNodes: syncAll,
		Timeout:         timeout,
		StartSeq:        startSeq,
		Operation:       operation,
	}
	requestPayload, err := cluster.PackWithHeader(req, uint8(operationMessage))
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed: %v]", uint8(operationMessage), req, err)

		return newClusterError(
			fmt.Sprintf("pack request (header=%d) to %#v failed: %v", uint8(operationMessage), req, err),
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
		return newClusterError(fmt.Sprintf("issue operation (sequence=%d) failed: %v", req.StartSeq, err),
			InternalServerError)
	}

	var memberResp *cluster.MemberResponse

	select {
	case r, ok := <-future.Response():
		if !ok {
			return newClusterError(fmt.Sprintf("request operation (sequence=%d) timeout", req.StartSeq),
				TimeoutError)
		}
		memberResp = r
	case <-gc.stopChan:
		return newClusterError(
			fmt.Sprintf("the member gone during issuing operation (sequence=%d)", req.StartSeq),
			IssueMemberGoneError)
	}

	if len(memberResp.Payload) == 0 {
		return newClusterError(
			fmt.Sprintf("request operation (sequence=%d) responds empty response", req.StartSeq),
			InternalServerError)
	}

	resp := new(RespOperation)
	err = cluster.Unpack(memberResp.Payload[1:], resp)
	if err != nil {
		return newClusterError(fmt.Sprintf("unpack operation response failed: %v", err), InternalServerError)
	}

	if resp.Err != nil {
		return resp.Err
	}

	return nil
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

	return reqOperation, nil, NoneClusterError
}

func (gc *GatewayCluster) respondOperation(req *cluster.RequestEvent, resp *RespOperation) {
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

	logger.Debugf("[member %s responded operationMessage message]", gc.clusterConf.NodeName)
}

func (gc *GatewayCluster) respondOperationErr(req *cluster.RequestEvent, typ ClusterErrorType, msg string) {
	resp := &RespOperation{
		Err: newClusterError(msg, typ),
	}
	gc.respondOperation(req, resp)
}

func (gc *GatewayCluster) handleOperationRelay(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqOperation, err, errType := unpackReqOperation(req.RequestPayload[1:])
	if err != nil {
		gc.respondOperationErr(req, errType, err.Error())
		return
	}

	logger.Debugf("received relay operation: %#v", reqOperation)

	ms := gc.log.maxSeq()

	if ms >= reqOperation.StartSeq {
		goto SUCCESS // sync goroutine is so fast
	}

	if reqOperation.StartSeq-ms > uint64(gc.conf.OPLogMaxSeqGapToPull) {
		gc.respondOperationErr(req, OperationLogHugeGapError,
			fmt.Sprintf("can not handle relayed operation request (sequence=%d) "+
				"due to local oplog is too old (sequence=%d)", reqOperation.StartSeq, ms))
		return
	}

	if reqOperation.StartSeq-ms > 1 {
		gc.syncOpLog(ms+1, reqOperation.StartSeq-ms-1)
	}

	// ignore timeout handling on relayed request operation, which is controlled by under layer

	err, errType = gc.log.append(reqOperation.StartSeq, []*Operation{reqOperation.Operation})
	if err != nil {
		switch errType {
		case OperationSeqConflictError:
			logger.Debugf("[the operation (sequence=%d) was synced before, skipped to write oplog]",
				reqOperation.StartSeq)
			goto SUCCESS
		case OperationInvalidSeqError:
			logger.Warnf("[the operation (sequence=%d) is retrieved too early to write oplog]",
				reqOperation.StartSeq)
		default: // does not make sense
			logger.Errorf("[append operation to oplog (completely or partially) failed: %v]", err)
		}

		gc.respondOperationErr(req, errType, err.Error())
		return
	}

SUCCESS:
	gc.respondOperation(req, new(RespOperation))
	return
}

func (gc *GatewayCluster) handleOperation(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqOperation, err, errType := unpackReqOperation(req.RequestPayload[1:])
	if err != nil {
		gc.respondOperationErr(req, errType, err.Error())
		return
	}

	err, errType = gc.log.append(reqOperation.StartSeq, []*Operation{reqOperation.Operation})
	if err != nil {
		logger.Errorf("[append operation to oplog (completely or partially) failed: %v]", err)
		gc.respondOperationErr(req, errType, err.Error())
		return
	}

	logger.Debugf("received operation: %#v", reqOperation)

	if !reqOperation.OperateAllNodes {
		gc.respondOperation(req, new(RespOperation))
		return
	}

	requestMembers := gc.RestAliveMembersInSameGroup()
	if len(requestMembers) == 0 {
		gc.respondOperation(req, new(RespOperation))
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
		Timeout:            reqOperation.Timeout,
		ResponseRelayCount: 1, // fault tolerance on network issue
	}

	requestName := fmt.Sprintf("%s_relay", req.RequestName)
	requestPayload := make([]byte, len(req.RequestPayload))
	copy(requestPayload, req.RequestPayload)
	requestPayload[0] = byte(operationRelayMessage)

	future, err := gc.cluster.Request(requestName, requestPayload, &requestParam)
	if err != nil {
		logger.Errorf("[send operagtion relay message failed: %v]", err)
		gc.respondOperationErr(req, InternalServerError, err.Error())
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
		gc.respondRetrieveErr(req, OperationPartiallyCompleteError,
			fmt.Sprintf("partially succeed in %d nodes", correctMemberRespCount+1)) // add myself
		return
	}

	gc.respondOperation(req, new(RespOperation))
}
