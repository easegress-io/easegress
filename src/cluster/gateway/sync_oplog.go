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

//

func unpackReqOPLogPull(payload []byte) (*ReqOPLogPull, error) {
	reqOPLogPull := new(ReqOPLogPull)
	err := cluster.Unpack(payload, reqOPLogPull)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to reqOPLogPull failed: %v", payload, err)
	}

	return reqOPLogPull, nil
}

func unpackRespOPLogPull(payload []byte) (*RespOPLogPull, error) {
	respOPLogPull := new(RespOPLogPull)
	err := cluster.Unpack(payload, respOPLogPull)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to RespOPLogPull failed: %v", payload, err)
	}

	return respOPLogPull, nil
}

func (gc *GatewayCluster) respondOPLog(req *cluster.RequestEvent, resp *RespOPLogPull) {
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

	logger.Debugf("[member %s responded opLogPullMessage message]", gc.clusterConf.NodeName)
}

func (gc *GatewayCluster) handleOPLogPull(req *cluster.RequestEvent) {
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqOPLogPull, err := unpackReqOPLogPull(req.RequestPayload[1:])
	if err != nil {
		logger.Errorf("[BUG: invalid pull_oplog %s requested from %s: %v]",
			req.RequestName, req.RequestNodeName, err)
		// swallow the failure, requester will wait to timeout and retry later
		return
	}

	resp := new(RespOPLogPull)
	resp.StartSeq = reqOPLogPull.StartSeq
	resp.SequentialOperations, err, _ = gc.log.retrieve(reqOPLogPull.StartSeq, reqOPLogPull.CountLimit)
	if err != nil {
		logger.Errorf("[retrieve sequential operation(s) from oplog failed: %v]", err)
		// swallow the failure, requester will wait until timeout and retry later
		return
	}

	gc.respondOPLog(req, resp)
}

func (gc *GatewayCluster) chooseMemberToPull() *cluster.Member {
	members := gc.RestAliveMembersInSameGroup()

	if len(members) == 0 {
		return nil
	}

	var member *cluster.Member
	r := rand.Int()

	// about half of the probability to choose WriteMode node
	if r%2 != 0 {
		for _, m := range members {
			if m.NodeTags[modeTagKey] == WriteMode.String() {
				member = &m
				break
			}
		}
	}

	if member == nil {
		member = &members[r%len(members)]
	}

	return member
}

func (gc *GatewayCluster) syncOpLogLoop() {
LOOP:
	for {
		select {
		case <-time.After(gc.conf.OPLogPullInterval):
			gc.syncOpLog(gc.log.maxSeq()+1, uint64(gc.conf.OPLogPullMaxCountOnce))
		case <-gc.stopChan:
			break LOOP
		}
	}
}

func (gc *GatewayCluster) syncOpLog(startSeq, countLimit uint64) {
	gc.syncOpLogLock.Lock()
	defer gc.syncOpLogLock.Unlock()

	if startSeq+countLimit-1 <= gc.log.maxSeq() {
		// nothing to do, the log has been synced before acquire the lock
		return
	}

	// adjust count limitation of sync in according to the latest max sequence after acquire the lock
	countLimit -= gc.log.maxSeq() + 1 - startSeq

	reqOPLogPull := ReqOPLogPull{
		StartSeq:   gc.log.maxSeq() + 1, // operations are sequential in the log
		CountLimit: countLimit,
	}

	payload, err := cluster.PackWithHeader(&reqOPLogPull, uint8(opLogPullMessage))
	if err != nil {
		logger.Errorf("[BUG: pack request (header=%d) to %#v failed, oplog sync skipped: %v]",
			opLogPullMessage, &reqOPLogPull, err)
		return
	}

	member := gc.chooseMemberToPull()
	if member == nil {
		logger.Warnf("[peer member not found, oplog sync skipped]")
		return
	}

	requestParam := cluster.RequestParam{
		TargetNodeNames: []string{member.NodeName},
		// TargetNodeNames is enough but TargetNodeTags could make rule strict
		TargetNodeTags: map[string]string{
			groupTagKey: gc.localGroupName(),
			modeTagKey:  member.NodeTags[modeTagKey],
		},
		Timeout:            gc.conf.OPLogPullTimeout,
		ResponseRelayCount: 1,
	}

	future, _ := gc.cluster.Request(fmt.Sprintf("pull_oplog(StartSeq=%d,Count=%d)",
		reqOPLogPull.StartSeq, reqOPLogPull.CountLimit), payload, &requestParam)
	select {
	case memberResp, ok := <-future.Response():
		if !ok {
			logger.Warnf("[peer member responds too late, oplog sync skipped]")
			return
		}

		if len(memberResp.Payload) == 0 {
			logger.Errorf("[BUG: receive empty pull_oplog response]")
			return
		}

		resp, err := unpackRespOPLogPull(memberResp.Payload[1:])
		if err != nil {
			logger.Errorf("[BUG: invalid pull_oplog response from %s: %v]",
				memberResp.ResponseNodeName, err)
			return
		}

		err, _ = gc.log.append(resp.StartSeq, resp.SequentialOperations)
		if err != nil {
			logger.Errorf("[append operation(s) to oplog failed: %v]", err)
			return
		}
	case <-gc.stopChan:
		return
	}
}
