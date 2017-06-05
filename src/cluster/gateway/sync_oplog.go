package gateway

import (
	"fmt"
	"math/rand"
	"time"

	"cluster"
	"logger"
)

func unpackReqOPLogPull(payload []byte) (*ReqOPLogPull, error) {
	reqOPLogPull := new(ReqOPLogPull)
	err := cluster.Unpack(payload, reqOPLogPull)
	if err != nil {
		return nil, fmt.Errorf("unpack %s to ReqOPLogPull failed: %v", payload, err)
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

func respondOPLog(req *cluster.RequestEvent, resp *RespOPLogPull) {
	// defensive programming
	if len(req.RequestPayload) < 1 {
		return
	}

	respBuff, err := cluster.PackWithHeader(resp, uint8(req.RequestPayload[0]))
	if err != nil {
		logger.Errorf("[BUG: Pack header(%d) %#v failed: %v]", req.RequestPayload[0], resp, err)
		return
	}

	err = req.Respond(respBuff)
	if err != nil {
		logger.Errorf("[respond %s to request %s, node %s failed: %v]",
			respBuff, req.RequestName, req.RequestNodeName, err)
		return
	}
}

func (gc *GatewayCluster) handleOPLogPull(req *cluster.RequestEvent) {
	if len(req.RequestPayload) < 1 {
		// defensive programming
		return
	}

	reqOPLogPull, err := unpackReqOPLogPull(req.RequestPayload)
	if err != nil {
		return
	}

	resp := new(RespOPLogPull)
	operations, err := gc.log.retrieve(reqOPLogPull.LocalMaxSeq+1, reqOPLogPull.WantMaxSeq)
	if err != nil {
		return
	}

	resp.SequentialOperations = make([]Operation, 0)
	for _, operation := range operations {
		resp.SequentialOperations = append(resp.SequentialOperations, *operation)
	}

	respondOPLog(req, resp)
}

func (gc *GatewayCluster) chooseMemberToPull() cluster.Member {
	// About half of the probability to choose WriteMode node.

	members := gc.restAliveMembersInSameGroup()

	rand.Seed(time.Now().UnixNano())
	if rand.Int()%2 != 0 {
		for _, member := range members {
			if member.NodeTags[modeTagKey] == string(WriteMode) {
				return member
			}
		}
	}

	return members[rand.Int()%len(members)]
}

func (gc *GatewayCluster) syncOPLog() {
LOOP:
	for {
		select {
		case <-time.After(gc.conf.OPLogPullTimeout):
			if gc.Mode() == WriteMode {
				// WriteMode doesn't have to, because the mode
				// could be changed in runtime so it's necessary
				// to check intermittently.
				continue LOOP
			}

			ms := gc.log.maxSeq()
			reqOPLogPull := ReqOPLogPull{
				Timeout:     gc.conf.OPLogPullTimeout,
				LocalMaxSeq: ms,
				WantMaxSeq:  ms + gc.conf.OPLogPullMaxCountOnce,
			}
			payload, err := cluster.PackWithHeader(reqOPLogPull, uint8(opLogPullMessage))
			if err != nil {
				logger.Errorf("[BUG: PackWithHeader %#v failed: %v]", reqOPLogPull, err)
				continue LOOP
			}

			member := gc.chooseMemberToPull()
			requestParam := cluster.RequestParam{
				TargetNodeNames: []string{member.NodeName},
			}

			future, err := gc.cluster.Request("pull_oplog", payload, &requestParam)

			go func(future *cluster.Future) {
				memberResp, ok := <-future.Response()
				if !ok {
					return
				}
				if len(memberResp.Payload) < 1 {
					logger.Errorf("received empty pull_oplog response")
					return
				}
				resp, err := unpackRespOPLogPull(memberResp.Payload[1:])
				if err != nil {
					logger.Errorf("received bad RespOPLogPull: %v", err)
					return
				}
				gc.log.append(resp.SequentialOperations...)
			}(future)

		case <-gc.stopChan:
			break LOOP
		}
	}
}
