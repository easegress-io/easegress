/**
 * Created by g7tianyi on 14/08/2017
 */

package domain

import (
	"plugins/easenight/common"
)

type EaseStack struct {
	Start       string `json:"start"`
	Stop        string `json:"stop"`
	ApplyConfig string `json:"apply_config"`
	HealthCheck string `json:"health_check"`
}

func ParseEaseStackFromMap(m map[string]interface{}) *EaseStack {
	easeStack := new(EaseStack)
	easeStack.Start = common.ParseStringValue(m["start"])
	easeStack.Stop = common.ParseStringValue(m["stop"])
	easeStack.ApplyConfig = common.ParseStringValue(m["apply_config"])
	easeStack.HealthCheck = common.ParseStringValue(m["health_check"])
	return easeStack
}
