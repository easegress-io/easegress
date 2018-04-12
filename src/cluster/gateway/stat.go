package gateway

import (
	"fmt"
	"math/rand"
	"time"

	"cluster"
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

func (gc *GatewayCluster) issueStat(group string, timeout time.Duration, detail bool,
	requestName string, filter Stater) (StatResult, *ClusterError) {
	req := &ReqStat{
		Detail: detail,
		Stat:   filter,
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
	requestParam := newRequestParam([]string{targetMember.NodeName}, group, Mode(targetMember.NodeTags[modeTagKey]), timeout)
	future, err := gc.cluster.Request(requestName, requestPayload, requestParam)
	if err != nil {
		return nil, newClusterError(
			fmt.Sprintf("issue statistics aggregation failed: %v", err), InternalServerError)
	}

	var memberResp *cluster.MemberResponse

	select {
	case r, ok := <-future.Response():
		if !ok {
			return nil, newClusterError(fmt.Sprintf("issue statistics aggregation timeout %.2fs", timeout.Seconds()), TimeoutError)
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

	return resp.Payload, nil
}

// for core
func unpackReqStat(payload []byte) (*ReqStat, error, ClusterErrorType) {
	reqStat := new(ReqStat)
	if err := cluster.Unpack(payload, reqStat); err != nil {
		return nil, fmt.Errorf("unpack %s to ReqStat failed: %v", payload, err), WrongMessageFormatError
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

func (gc *GatewayCluster) statResult(s Stater) (StatResult, error, ClusterErrorType) {
	result, err, errType := s.Execute(gc.localMemberName(), gc.mod)
	if err != nil {
		return nil, err, errType
	}

	return result, nil, NoneClusterError
}

func (gc *GatewayCluster) getLocalStatResp(s Stater) (*RespStat, error, ClusterErrorType) {
	if err, errType := s.Validate(); err != nil {
		return nil, fmt.Errorf("validate ReqStat.Stat failed: %s", err), errType
	}
	payload, err, errType := s.Execute(gc.localMemberName(), gc.mod)
	if err != nil {
		return nil, err, errType
	}

	return &RespStat{Payload: payload}, nil, NoneClusterError
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

	resp, err, errType := gc.getLocalStatResp(reqStat.Stat)
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

	var firstResp *RespStat
	otherRespPayloads := make(map[string]StatResult)

	localResp, localErr, errType := gc.getLocalStatResp(reqStat.Stat)
	if localErr != nil {
		logger.Warnf("[get local statistics failed: %v]", localErr)
	} else {
		firstResp = localResp
	}

	requestMembers := gc.RestAliveMembersInSameGroup()
	if len(requestMembers) > 0 {
		requestMemberNames := make([]string, 0)
		for _, member := range requestMembers {
			requestMemberNames = append(requestMemberNames, member.NodeName)
		}

		requestParam := newRequestParam(requestMemberNames, gc.localGroupName(), NilMode, req.Timeout())
		requestName := fmt.Sprintf("%s_relay", req.RequestName)
		requestPayload := make([]byte, len(req.RequestPayload))
		copy(requestPayload, req.RequestPayload)
		requestPayload[0] = byte(statRelayMessage)

		future, err := gc.cluster.Request(requestName, requestPayload, requestParam)
		if err != nil {
			logger.Errorf("[send stat relay message failed: %v]", err)
			gc.respondStatErr(req, InternalServerError, err.Error())
			return
		}

		membersRespBook := make(map[string][]byte)
		for _, memberName := range requestMemberNames {
			membersRespBook[memberName] = nil
		}

		gc.recordResp(requestName, future, membersRespBook)

		for memberName, payload := range membersRespBook {
			if len(payload) == 0 {
				continue
			}

			resp := new(RespStat)
			err := cluster.Unpack(payload[1:], resp)
			if err != nil || resp.Err != nil {
				logger.Warnf("unpack payload from %s failed: %v", memberName, err)
				continue
			}
			if firstResp == nil {
				firstResp = resp
			}
			otherRespPayloads[memberName] = resp.Payload
		}
		if len(otherRespPayloads) == 0 {
			msg := fmt.Sprintf("[expected %d member to respond, but got %d]", len(requestMembers), 0)
			logger.Warnf(msg)
		}
	}
	if firstResp == nil {
		msg := "[aggregate statistics for cluster members failed: no valid response received]"
		logger.Warnf(msg)
		gc.respondStatErr(req, InternalServerError, msg)
		return
	}
	aggregatedPayload, err := firstResp.Payload.Aggregate(reqStat.Detail, otherRespPayloads)
	if err != nil {
		msg := fmt.Sprintf("[aggregate statistics for cluster members failed: %v]", err)
		logger.Warnf(msg)
		gc.respondStatErr(req, InternalServerError, msg)
		return
	}

	respStat := &RespStat{Payload: aggregatedPayload}
	gc.respondStat(req, respStat)
}
