package gateway

import (
	"fmt"
	"strings"
	"time"

	"cluster"
	"common"
	"logger"
	"option"
)

// for api

// QueryWriterSequence query oplog sequence information from writer node of specified group
func (gc *GatewayCluster) QueryWriterSequence(group string, timeout time.Duration) (*RespQuerySeq, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query writer sequence due to cluster gone",
			IssueMemberGoneError)
	}

	reqParam := newRequestParam(nil, group, WriteMode, timeout)
	requestName := fmt.Sprintf("(group:%s)query_writer_sequence", group)

	req := new(ReqQuerySeq)
	respPayload, err, clusterErrorType := gc.querySingleMember(req, uint8(querySeqMessage), requestName, reqParam)
	if err != nil {
		return nil, newClusterError(
			fmt.Sprintf("query group %s writer sequence failed: %v", group, err),
			clusterErrorType)
	}

	var resp RespQuerySeq
	err = cluster.Unpack(respPayload[1:], &resp)
	if err != nil {
		return nil, newClusterError(fmt.Sprintf("unpack sequence response failed: %v", err),
			InternalServerError)
	}

	return &resp, nil
}

// QueryGroupSequence query oplog sequence information on writer node for all reader nodes of specified group
// Will return response and QueryPartiallyCompleteError if query is partially complete on readers
func (gc *GatewayCluster) QueryReadersSequence(group string, timeout time.Duration) (map[string]*RespQuerySeq, *ClusterError) {
	if gc.Stopped() {
		return nil, newClusterError("can not query readers sequence due to cluster gone",
			IssueMemberGoneError)
	}
	if gc.Mode() != WriteMode {
		return nil, newClusterError("can not query readers sequence on reader node, it must be called on writer node",
			IssueMemberGoneError)
	}

	readers := gc.aliveNodesInCluster(ReadMode)
	if len(readers) == 0 {
		logger.Debugf("[no alive readers in group %s]", group)
		return nil, nil
	}

	reqParam := newRequestParam(readers, group, ReadMode, timeout)
	requestName := fmt.Sprintf("(group:%s)query_readers_sequence", group)

	req := new(ReqQuerySeq)
	respPayloads, queryErr, clusterErrorType := gc.queryMultipleMembers(req, uint8(querySeqMessage), requestName, reqParam)
	if queryErr != nil && clusterErrorType != QueryPartiallyCompleteError {
		return nil, newClusterError(
			fmt.Sprintf("query readers sequence for group %s failed: %v", group, queryErr),
			clusterErrorType)
	}

	rets := make(map[string]*RespQuerySeq, len(respPayloads))
	for name := range respPayloads {
		var resp RespQuerySeq
		err := cluster.Unpack(respPayloads[name][1:], &resp)
		if err != nil {
			return nil, newClusterError(fmt.Sprintf("unpack sequence response failed: %v", err),
				InternalServerError)
		}
		rets[name] = &resp
	}

	// Query partially complete, return response in case caller need it
	if queryErr != nil && clusterErrorType == QueryPartiallyCompleteError {
		return rets, newClusterError(queryErr.Error(), clusterErrorType)
	}
	return rets, nil
}

// for core
func (gc *GatewayCluster) handleQuerySequence(req *cluster.RequestEvent) {
	max := gc.log.MaxSeq()
	min := gc.log.MinSeq()

	resp := RespQuerySeq{
		Max: max,
		Min: min,
	}
	payload, err := cluster.PackWithHeader(resp, uint8(querySeqMessage))
	if err != nil {
		logger.Errorf("[BUG: PackWithHeader sequence %v failed: %v]", resp, err)
		return
	}

	err = req.Respond(payload)
	if err != nil {
		logger.Errorf("[respond sequence to request %s, node %s failed: %v]",
			req.RequestName, req.RequestNodeName, err)
	}

	logger.Debugf("[member %s responded querySequenceMessage message]", gc.clusterConf.NodeName)
}

func (gc *GatewayCluster) handleQueryMember(req *cluster.RequestEvent) {
	resp := RespQueryMember{
		ClusterResp:     ClusterResp{common.Now()},
		MemberInnerInfo: *gc.getMemberInnerInfo(req.Timeout()),
	}
	gc.handleResp(req, uint8(queryMemberMessage), resp)

	logger.Debugf("[member %s responded queryMemberMessage message]", gc.clusterConf.NodeName)
}

func (gc *GatewayCluster) handleQueryMembersList(req *cluster.RequestEvent) {
	resp := RespQueryMembersList{
		ClusterResp: ClusterResp{common.Now()},
		MembersInfo: *gc.getMembersInfo(),
	}
	gc.handleResp(req, uint8(queryMembersListMessage), resp)

	logger.Debugf("[member %s responded queryMembers message]", gc.clusterConf.NodeName)
}

func (gc *GatewayCluster) handleQueryGroup(req *cluster.RequestEvent) {
	// TODO(shengdong) don we need more accurate timeout mechanism for intra-cluster communication?
	opLogGroupInfo, err := gc.retrieveOpLogGroupInfo(gc.localGroupName(), req.Timeout()-2*time.Second)
	var resp RespQueryGroup
	if err != nil && err.Type != QueryPartiallyCompleteError {
		resp = RespQueryGroup{
			Err: err,
		}
	} else {
		resp = RespQueryGroup{
			RespQueryGroupPayload: RespQueryGroupPayload{
				ClusterResp:    ClusterResp{common.Now()},
				MembersInfo:    *gc.getMembersInfo(),
				OpLogGroupInfo: *opLogGroupInfo,
			},
			Err: err,
		}
	}
	gc.handleResp(req, uint8(queryMemberMessage), resp)

	logger.Debugf("[member %s responded queryGroupMessage message]", gc.clusterConf.NodeName)
}

// getMembersInfo assemble members information from local group
func (gc *GatewayCluster) getMembersInfo() *MembersInfo {
	members := gc.cluster.Members()
	aliveMembers := make([]MemberInfo, 0)
	failedMembers := make([]MemberInfo, 0)

	memberToMemberInfo := func(m *cluster.Member) (string, MemberInfo) {
		return m.NodeName, MemberInfo{
			Name:     m.NodeName,
			Endpoint: fmt.Sprintf("%s:%d", m.Address, m.Port),
			Mode:     strings.ToLower(m.NodeTags[modeTagKey]),
		}
	}

	for idx := range members {
		m := members[idx]
		if m.NodeTags[groupTagKey] == gc.localGroupName() {
			_, member := memberToMemberInfo(&m)
			if m.Status == cluster.MemberAlive {
				aliveMembers = append(aliveMembers, member)
			} else if m.Status == cluster.MemberFailed {
				failedMembers = append(failedMembers, member)
			}
		}
	}
	return &MembersInfo{
		FailedMembersCount: uint64(len(failedMembers)),
		FailedMembers:      failedMembers,
		AliveMembersCount:  uint64(len(aliveMembers)),
		AliveMembers:       aliveMembers,
	}
}

// retrieveOpLogGroupInfo retrieve operation log information for specific group
func (gc *GatewayCluster) retrieveOpLogGroupInfo(group string, timeout time.Duration) (*OpLogGroupInfo, *ClusterError) {
	synced := false
	maxSequence := int64(-1)
	minSequence := int64(-1)

	groupSeq, writerError := gc.QueryWriterSequence(group, timeout)
	var readerError *ClusterError
	var readersSeq map[string]*RespQuerySeq
	var unSyncedMembers []string // default is nil
	if writerError == nil {
		maxSequence = int64(groupSeq.Max)
		minSequence = int64(groupSeq.Min)
		readersSeq, readerError = gc.QueryReadersSequence(group, timeout)
		if readerError == nil || readerError.Type == QueryPartiallyCompleteError {
			unSyncedMembers = make([]string, 0)
			for name, seq := range readersSeq {
				if int64(seq.Max) != maxSequence {
					unSyncedMembers = append(unSyncedMembers, name)
				}
			}
			if len(unSyncedMembers) == 0 {
				synced = true
			}
		} else {
			logger.Errorf("[%v]", readerError)
			return nil, readerError
		}
	} else {
		logger.Errorf("[%v]", writerError)
		return nil, writerError
	}
	ret := &OpLogGroupInfo{
		OpLogStatus: OpLogStatus{
			Synced:      synced,
			MaxSequence: maxSequence,
			MinSequence: minSequence,
		},
		UnSyncedMembers:      unSyncedMembers,
		UnSyncedMembersCount: uint64(len(unSyncedMembers)),
	}

	return ret, readerError
}

// getMemberInnerInfo get current node's member info
func (gc *GatewayCluster) getMemberInnerInfo(timeout time.Duration) *MemberInnerInfo {
	memberListConf := gc.cluster.GetMemberListConfig()
	groupSeq, err := gc.QueryWriterSequence(gc.localGroupName(), timeout)

	synced := false
	syncProgress := int8(-1)
	syncLag := int64(-1)

	ownMaxSeq := gc.log.MaxSeq()
	ownMinSeq := gc.log.MinSeq()

	if err == nil {
		synced = ownMaxSeq == groupSeq.Max
		ownDiff := ownMaxSeq - ownMinSeq + 1
		groupDiff := groupSeq.Max - groupSeq.Min + 1
		if ownDiff <= groupDiff { //defensive programming
			syncProgress = int8(100 * ownDiff / groupDiff)
			syncLag = int64(groupSeq.Max - ownMaxSeq)
		}
	}

	memberInnerInfo := MemberInnerInfo{
		Config: memberConfig{
			ClusterDefaultOpTimeoutSec: option.ClusterDefaultOpTimeout / time.Second,
			GossipConfig: GossipConfig{
				GossipPacketSizeBytes:  memberListConf.UDPBufferSize,
				GossipIntervalMillisec: memberListConf.GossipInterval / time.Millisecond,
			},
			OpLogConfig: OpLogConfig{
				MaxSeqGapToPull:     gc.conf.OPLogMaxSeqGapToPull,
				PullMaxCountOnce:    gc.conf.OPLogPullMaxCountOnce,
				PullIntervalSeconds: gc.conf.OPLogPullInterval / time.Second,
				PullTimeoutSeconds:  gc.conf.OPLogPullTimeout / time.Second,
			},
		},
		OpLogInfo: OpLogInfo{
			OpLogInnerInfo: OpLogInnerInfo{
				DataPath:     gc.log.Path(),
				SizeBytes:    gc.log.Size(),
				SyncProgress: syncProgress,
				SyncLag:      syncLag,
			},
			OpLogStatus: OpLogStatus{
				Synced:      synced,
				MaxSequence: int64(gc.log.MaxSeq()),
				MinSequence: int64(gc.log.MinSeq()),
			},
		},
	}
	return &memberInnerInfo
}

func (gc *GatewayCluster) queryMultipleMembers(req interface{}, header uint8, requestName string, reqParam *cluster.RequestParam) (map[string][]byte, error, ClusterErrorType) {
	memberCount := len(reqParam.TargetNodeNames)
	if memberCount == 0 {
		return nil, fmt.Errorf("[BUG: must specify TargetNodeNames in RequestParam]"), InternalServerError
	}

	requestPayload, err := cluster.PackWithHeader(req, header)
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed: %v]",
			header, req, err)
		return nil, fmt.Errorf("pack request (header=%d) to %#v failed: %v",
			header, req, err), InternalServerError
	}

	future, err := gc.cluster.Request(requestName, requestPayload, reqParam)
	if err != nil {
		return nil, err, InternalServerError
	}

	membersRespBook := make(map[string][]byte, memberCount)
	for _, memberName := range reqParam.TargetNodeNames {
		membersRespBook[memberName] = nil
	}

	gc.recordResp(requestName, future, membersRespBook)

	correctMemberRespCount := 0
	payloads := make(map[string][]byte, memberCount)
	for name, payload := range membersRespBook {
		if len(payload) == 0 {
			continue
		}
		payloads[name] = payload
		correctMemberRespCount++
	}

	if correctMemberRespCount > 0 && correctMemberRespCount < memberCount {
		// return non-nil payloads when partially complete, so caller can deal with the payloads
		return payloads, fmt.Errorf("partially succeed in %d of %d nodes", correctMemberRespCount, memberCount), QueryPartiallyCompleteError
	}

	return payloads, nil, NoneClusterError
}

func (gc *GatewayCluster) querySingleMember(req interface{}, header uint8, requestName string, reqParam *cluster.RequestParam) ([]byte, error, ClusterErrorType) {
	if !(len(reqParam.TargetNodeNames) == 0 && reqParam.TargetNodeTags != nil && reqParam.TargetNodeTags[modeTagKey] == "^"+WriteMode.String()+"$") && // target is not writer node
		len(reqParam.TargetNodeNames) != 1 {
		return nil, fmt.Errorf("[BUG: multiple target when call querySingleMember]"), InternalServerError
	}
	requestPayload, err := cluster.PackWithHeader(req, header)
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed: %v]",
			header, req, err)
		return nil, fmt.Errorf("pack request (header=%d) to %#v failed: %v",
			header, req, err), InternalServerError
	}

	future, err := gc.cluster.Request(requestName, requestPayload, reqParam)
	if err != nil {
		return nil, err, InternalServerError
	}

	var memberResp *cluster.MemberResponse

	select {
	case r, ok := <-future.Response():
		if !ok {
			return nil, fmt.Errorf("query timeout %.2fs", reqParam.Timeout.Seconds()), TimeoutError
		}
		memberResp = r
	case <-gc.stopChan:
		return nil, fmt.Errorf("the member gone during issuing query"), IssueMemberGoneError
	}

	if len(memberResp.Payload) == 0 {
		return nil, fmt.Errorf("query responds empty response"), InternalServerError
	}
	return memberResp.Payload, nil, NoneClusterError
}
