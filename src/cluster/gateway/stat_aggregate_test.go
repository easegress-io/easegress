package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"
)

func (rs *RespStat) String() string {
	return fmt.Sprintf("\n\terr: %s\n\tnames: %s\n\tvalue: %s\n\tvalues:%s\n\tdesc:%s\n\t",
		rs.Err.Error(), string(rs.Names),
		string(rs.Value), string(rs.Values), string(rs.Desc))
}

func assertStatRespEqual(t *testing.T, want *RespStat, got *RespStat) {
	wantString := want.String()
	gotString := got.String()
	if wantString != gotString {
		t.Fatalf("\nwant: %s\ngot : %s", want, got)
	}
}

func TestMultpleFilters(t *testing.T) {
	req := &ReqStat{
		FilterPipelineIndicatorNames: &FilterPipelineIndicatorNames{
			PipelineName: "pipeline-001",
		},
		FilterTaskIndicatorValue: &FilterTaskIndicatorValue{
			PipelineName:  "pipeline-001",
			IndicatorName: "EXECUTION_COUNT_ALL",
		},
	}
	_, err := aggregateStatResponses(req, nil)
	if err == nil {
		t.Fatalf("want multiple filters error got <nil>")
	}
}

func TestAggregateNames(t *testing.T) {
	req := &ReqStat{
		FilterPipelineIndicatorNames: &FilterPipelineIndicatorNames{
			PipelineName: "pipeline-001",
		},
	}
	resps := map[string]*RespStat{
		"node-001": {
			Names: []byte(`{"names": ["pipeline-001", "pipeline-003"]}`),
		},
		"node-002": {
			Names: []byte(`{"names": ["pipeline-002", "pipeline-003", "pipeline-004"]}`),
		},
	}

	wantResult := ResultStatIndicatorNames{
		Names: []string{"pipeline-001", "pipeline-002", "pipeline-003", "pipeline-004"},
	}
	wantResultBuff, err := json.Marshal(wantResult)
	if err != nil {
		log.Fatalf("BUG: marhsal %#v failed: err", wantResult, err)
	}
	want := &RespStat{
		Names: wantResultBuff,
	}

	got, err := aggregateStatResponses(req, resps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStatRespEqual(t, want, got)
}

func TestAggregateDesc(t *testing.T) {
	req := &ReqStat{
		FilterPluginIndicatorDesc: &FilterPluginIndicatorDesc{
			PipelineName:  "pipeline-001",
			PluginName:    "plugin-001",
			IndicatorName: "WAIT_QUEUE_LENGTH",
		},
	}
	resps := map[string]*RespStat{
		"node-001": {
			Desc: []byte(`{"desc":"The length of wait queue which contains requests wait to be handled by a pipeline."}`),
		},
		"node-002": {
			Desc: []byte(`{"desc":"The length of wait queue which contains requests wait to be handled by a pipeline."}`),
		},
	}

	wantResult := ResultStatIndicatorDesc{
		Desc: "The length of wait queue which contains requests wait to be handled by a pipeline.",
	}
	wantResultBuff, err := json.Marshal(wantResult)
	if err != nil {
		log.Fatalf("BUG: marhsal %#v failed: err", wantResult, err)
	}
	want := &RespStat{
		Desc: wantResultBuff,
	}

	got, err := aggregateStatResponses(req, resps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStatRespEqual(t, want, got)
}

func TestAggregateTaskValue(t *testing.T) {
	req := &ReqStat{
		Detail: false,
		FilterTaskIndicatorValue: &FilterTaskIndicatorValue{
			PipelineName:  "pipeline-001",
			IndicatorName: "EXECUTION_COUNT_ALL",
		},
	}
	resps := map[string]*RespStat{
		"node-001": {
			Value: []byte(`{"value": 11}`),
		},
		"node-002": {
			Value: []byte(`{"value": 22}`),
		},
	}

	wantResult := ResultStatIndicatorValue{
		Value: map[string]interface{}{
			"EXECUTION_COUNT_ALL": resultStatAggregateValue{
				AggregatedResult: 33,
				AggregatorName:   "numeric_sum",
				// DetailValues: map[string]interface{}{
				// 	"node-001": 11,
				// 	"node-002": 22,
				// },
			},
		},
	}
	wantResultBuff, err := json.Marshal(wantResult)
	if err != nil {
		log.Fatalf("BUG: marhsal %#v failed: err", wantResult, err)
	}
	want := &RespStat{
		Value: wantResultBuff,
	}

	got, err := aggregateStatResponses(req, resps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStatRespEqual(t, want, got)
}

func TestAggregatePluginValue(t *testing.T) {
	req := &ReqStat{
		Detail: true,
		FilterPluginIndicatorValue: &FilterPluginIndicatorValue{
			PipelineName:  "pipeline-001",
			PluginName:    "plugin-001",
			IndicatorName: "EXECUTION_COUNT_LAST_1MIN_ALL",
		},
	}
	resps := map[string]*RespStat{
		"node-001": {
			Value: []byte(`{"value": 123}`),
		},
		"node-002": {
			Value: []byte(`{"value": 321}`),
		},
	}

	wantResult := ResultStatIndicatorValue{
		Value: map[string]interface{}{
			"EXECUTION_COUNT_LAST_1MIN_ALL": resultStatAggregateValue{
				AggregatedResult: 444,
				AggregatorName:   "numeric_sum",
				DetailValues: map[string]interface{}{
					"node-001": 123,
					"node-002": 321,
				},
			},
		},
	}
	wantResultBuff, err := json.Marshal(wantResult)
	if err != nil {
		log.Fatalf("BUG: marhsal %#v failed: err", wantResult, err)
	}
	want := &RespStat{
		Value: wantResultBuff,
	}

	got, err := aggregateStatResponses(req, resps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStatRespEqual(t, want, got)
}

func TestAggregatePipelineValues(t *testing.T) {
	req := &ReqStat{
		Detail: true,
		FilterPipelineIndicatorsValue: &FilterPipelineIndicatorsValue{
			PipelineName: "pipeline-001",
			IndicatorNames: []string{
				"EXECUTION_COUNT_LAST_1MIN_ALL",
				"EXECUTION_TIME_MAX_LAST_1MIN_ALL",
				"EXECUTION_TIME_MIN_LAST_1MIN_ALL",
				"INVALID_INDICATOR"},
		},
	}
	resps := map[string]*RespStat{
		"node-001": {
			Values: []byte(`{"values":{
                                         "EXECUTION_COUNT_LAST_1MIN_ALL": 11,
                                         "EXECUTION_TIME_MAX_LAST_1MIN_ALL": 0.22,
                                         "EXECUTION_TIME_MIN_LAST_1MIN_ALL": 0.44,
                                         "INVALID_INDICATOR": null}}`),
		},
		"node-002": {
			Values: []byte(`{"values": {
                                         "EXECUTION_COUNT_LAST_1MIN_ALL": 22,
                                         "EXECUTION_TIME_MAX_LAST_1MIN_ALL": 1.22,
                                         "EXECUTION_TIME_MIN_LAST_1MIN_ALL": 0.99,
                                         "INVALID_INDICATOR": null}}`),
		},
		"node-003": {
			Values: []byte(`{"values": {
                                         "EXECUTION_COUNT_LAST_1MIN_ALL": 33,
                                         "EXECUTION_TIME_MAX_LAST_1MIN_ALL": 2.22,
                                         "EXECUTION_TIME_MIN_LAST_1MIN_ALL": 1.99,
                                         "INVALID_INDICATOR": null}}`),
		},
	}

	wantResult := ResultStatIndicatorsValue{
		Values: map[string]interface{}{
			"EXECUTION_COUNT_LAST_1MIN_ALL": resultStatAggregateValue{
				AggregatedResult: 66,
				AggregatorName:   "numeric_sum",
				DetailValues: map[string]interface{}{
					"node-001": 11,
					"node-002": 22,
					"node-003": 33,
				},
			},
			"EXECUTION_TIME_MAX_LAST_1MIN_ALL": resultStatAggregateValue{
				AggregatedResult: 2.22,
				AggregatorName:   "numeric_max",
				DetailValues: map[string]interface{}{
					"node-001": 0.22,
					"node-002": 1.22,
					"node-003": 2.22,
				},
			},
			"EXECUTION_TIME_MIN_LAST_1MIN_ALL": resultStatAggregateValue{
				AggregatedResult: 0.44,
				AggregatorName:   "numeric_min",
				DetailValues: map[string]interface{}{
					"node-001": 0.44,
					"node-002": 0.99,
					"node-003": 1.99,
				},
			},
			"INVALID_INDICATOR": resultStatAggregateValue{
				AggregatedResult: nil,
				AggregatorName:   "",
				DetailValues: map[string]interface{}{
					"node-001": nil,
					"node-002": nil,
					"node-003": nil,
				},
			},
		},
	}
	wantResultBuff, err := json.Marshal(wantResult)
	if err != nil {
		log.Fatalf("BUG: marhsal %#v failed: err", wantResult, err)
	}
	want := &RespStat{
		Values: wantResultBuff,
	}

	got, err := aggregateStatResponses(req, resps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertStatRespEqual(t, want, got)
}
