package gateway

import (
	"fmt"

	"cluster"
	"logger"
)

func (gc *GatewayCluster) handleStat(req *cluster.RequestEvent) {
	respondErr := func(e error) {
		resp := RespStat{
			Err: NewStatErr(e.Error()),
		}
		respBuff, err := cluster.Pack(resp, uint8(statMessage))
		if err != nil {
			logger.Errorf("[BUG: pack %#v failed: %v]", resp, err)
			return
		}

		err = req.Respond(respBuff)
		if err != nil {
			logger.Errorf("[respond error %v to request %s, node %s failed: %v]", resp, req.RequestName, req.RequestNodeName, err)
			return
		}
	}

	reqStat := ReqStat{}
	err := cluster.Unpack(req.RequestPayload[1:], &reqStat)
	if err != nil {
		respondErr(fmt.Errorf("wrong format: want ReqStat"))
		return
	}
	if len(reqStat.Filter) < 1 {
		respondErr(fmt.Errorf("wrong format: filter is empty"))
		return
	}

	resp := RespRetrieve{}
	_ = resp
	switch StatType(reqStat.Filter[0]) {
	case statPipelineIndicatorNames:
		filter := FilterPipelineIndicatorNames{}
		err := cluster.Unpack(reqStat.Filter[1:], &filter)
		if err != nil {
			respondErr(fmt.Errorf("wrong format: want FilterPipelineIndicatorNames"))
			return
		}
	case statPipelineIndicatorValue:
	case statPipelineIndicatorDesc:
	case statPluginIndicatorNames:
	case statPluginIndicatorValue:
	case statPluginIndicatorDesc:
	case statTaskIndicatorNames:
	case statTaskIndicatorValue:
	case statTaskIndicatorDesc:
	}
}
