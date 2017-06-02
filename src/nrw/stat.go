package nrw

import (
	"fmt"

	"cluster"
	"logger"
)

func (nrw *NRW) handleStat(req *cluster.RequestEvent) {
	respondErr := func(e error) {
		resp := RespStat{
			Err: NewStatErr(e.Error()),
		}
		respBuff, err := Pack(resp, uint8(MessasgeStat))
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
	err := Unpack(req.RequestPayload[1:], &reqStat)
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
	case StatPipelineIndicatorNames:
		filter := FilterPipelineIndicatorNames{}
		err := Unpack(reqStat.Filter[1:], &filter)
		if err != nil {
			respondErr(fmt.Errorf("wrong format: want FilterPipelineIndicatorNames"))
			return
		}
	case StatPipelineIndicatorValue:
	case StatPipelineIndicatorDesc:
	case StatPluginIndicatorNames:
	case StatPluginIndicatorValue:
	case StatPluginIndicatorDesc:
	case StatTaskIndicatorNames:
	case StatTaskIndicatorValue:
	case StatTaskIndicatorDesc:
	}
}
