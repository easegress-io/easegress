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

	if reqOPLogPull.Timeout < 1*time.Second {
		return nil, fmt.Errorf("timeout is less than 1 second")
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
	if len(req.RequestPayload) == 0 {
		// defensive programming
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
	if len(req.RequestPayload) == 0 {
		// defensive programming
		return
	}

	reqOPLogPull, err := unpackReqOPLogPull(req.RequestPayload)
	if err != nil {
		return
	}

	resp := new(RespOPLogPull)
	resp.SequentialOperations, err = gc.log.retrieve(reqOPLogPull.LocalMaxSeq+1, reqOPLogPull.WantMaxSeq)
	if err != nil {
		logger.Errorf("[retrieve sequential operations from oplog failed: %v]", err)
		return
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
			ms := gc.log.maxSeq()
			reqOPLogPull := ReqOPLogPull{
				Timeout:     gc.conf.OPLogPullTimeout,
				LocalMaxSeq: ms,
				WantMaxSeq:  ms + gc.conf.OPLogPullMaxCountOnce,
			}
			payload, err := cluster.PackWithHeader(&reqOPLogPull, uint8(opLogPullMessage))
			if err != nil {
				logger.Errorf("[BUG: PackWithHeader %#v failed: %v]", reqOPLogPull, err)
				continue LOOP
			}

			member := gc.chooseMemberToPull()
			requestParam := cluster.RequestParam{
				TargetNodeNames: []string{member.NodeName},
			}

			future, _ := gc.cluster.Request("pull_oplog", payload, &requestParam)

			go func() {
				memberResp, ok := <-future.Response()
				if !ok {
					return
				}
				if len(memberResp.Payload) == 0 {
					logger.Errorf("[received empty pull_oplog response]")
					return
				}
				resp, err := unpackRespOPLogPull(memberResp.Payload[1:])
				if err != nil {
					logger.Errorf("[received bad RespOPLogPull: %v]", err)
					return
				}

				err, _ = gc.log.append(resp.SequentialOperations)
				if err != nil {
					logger.Errorf("[append sequential operations to oplog failed: %v]", err)
					return
				}
			}()
		case <-gc.stopChan:
			break LOOP
		}
	}
}
