package gateway

import (
	"testing"
)

func TestAggregateStatResponses(t *testing.T) {
	req := &ReqStat{
		FilterTaskIndicatorValue: &FilterTaskIndicatorValue{
			PipelineName:  "",
			IndicatorName: "EXECUTION_COUNT_ALL",
		},
	}

	resps := []*RespStat{
		&RespStat{
			Value: []byte(`{"value":0}`),
		},
	}

	ret := aggregateStatResponses(req, resps)

	if string(ret.Value) != `{"value":0}` {
		t.Errorf(`want {"value":0} got %s`, ret.Value)
	}
}
