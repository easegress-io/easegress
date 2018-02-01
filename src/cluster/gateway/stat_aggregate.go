package gateway

import (
	"encoding/json"
	"fmt"
	"sort"

	"common"
	"logger"
)

func aggregateStatResponses(reqStat *ReqStat, respStats map[string]*RespStat) (*RespStat, error) {
	flags := 0
	aggregateNamesFlag := reqStat.FilterPipelineIndicatorNames != nil ||
		reqStat.FilterPluginIndicatorNames != nil ||
		reqStat.FilterTaskIndicatorNames != nil
	if aggregateNamesFlag {
		flags++
	}

	aggregateDescFlag := reqStat.FilterPipelineIndicatorDesc != nil ||
		reqStat.FilterPluginIndicatorDesc != nil ||
		reqStat.FilterTaskIndicatorDesc != nil
	if aggregateDescFlag {
		flags++
	}

	aggregateValueFlag := reqStat.FilterPipelineIndicatorValue != nil ||
		reqStat.FilterPluginIndicatorValue != nil ||
		reqStat.FilterTaskIndicatorValue != nil
	if aggregateValueFlag {
		flags++
	}

	aggregateValuesFlag := reqStat.FilterPipelineIndicatorsValue != nil
	if aggregateValuesFlag {
		flags++
	}

	if flags > 1 {
		return nil, fmt.Errorf("multiple filters flag set: "+
			"names(%v) desc(%v) value(%v) values(%v)",
			aggregateNamesFlag, aggregateDescFlag, aggregateValueFlag,
			aggregateValuesFlag)
	}

	if aggregateNamesFlag {
		return aggregateNames(reqStat, respStats)
	}

	if aggregateDescFlag {
		return aggregateDesc(reqStat, respStats)
	}

	if aggregateValueFlag {
		return aggregateValue(reqStat, respStats)
	}

	if aggregateValuesFlag {
		return aggregateValues(reqStat, respStats)
	}

	return nil, fmt.Errorf("invalid filter")
}

func aggregateNames(reqStat *ReqStat, respStats map[string]*RespStat) (*RespStat, error) {
	memory := make(map[string]struct{})
	ret := new(ResultStatIndicatorNames)

	for memberName, resp := range respStats {
		r := new(ResultStatIndicatorNames)
		err := json.Unmarshal(resp.Names, r)
		if err != nil {
			logger.Warnf("[BUG: unmarshal member %s payload failed: %v]",
				memberName, err)
			continue
		}

		for _, name := range r.Names {
			memory[name] = struct{}{}
		}
	}
	ret.Names = make([]string, 0, len(memory))
	for name := range memory {
		ret.Names = append(ret.Names, name)
	}

	// returns with stable order
	sort.Strings(ret.Names)

	retBuff, err := json.Marshal(ret)
	if err != nil {
		logger.Errorf("[BUG: marshal failed: %v]", err)
		return nil, fmt.Errorf("marshal failed: %v", err)
	}

	return &RespStat{
		Names: retBuff,
	}, nil
}

func aggregateDesc(reqStat *ReqStat, respStats map[string]*RespStat) (*RespStat, error) {
	for memberName, resp := range respStats {
		r := new(ResultStatIndicatorDesc)
		err := json.Unmarshal(resp.Desc, r)
		if err != nil {
			logger.Warnf("[BUG: unmarshal member %s payload failed: %v]",
				memberName, err)
			continue
		}

		return &RespStat{
			Desc: resp.Desc,
		}, nil
	}

	return nil, fmt.Errorf("none of valid response stats")
}

func listValues(reqStat *ReqStat, respStats map[string]*RespStat) map[string]map[string]interface{} {
	var indicatorName string
	switch {
	case reqStat.FilterPipelineIndicatorValue != nil:
		indicatorName = reqStat.FilterPipelineIndicatorValue.IndicatorName
	case reqStat.FilterPluginIndicatorValue != nil:
		indicatorName = reqStat.FilterPluginIndicatorValue.IndicatorName
	case reqStat.FilterTaskIndicatorValue != nil:
		indicatorName = reqStat.FilterTaskIndicatorValue.IndicatorName
	case reqStat.FilterPipelineIndicatorsValue != nil:
	default:
		logger.Errorf("[BUG: no value[s] filters]")
		return nil
	}

	values := make(map[string]map[string]interface{})
	if len(indicatorName) != 0 {
		// single value
		values[indicatorName] = make(map[string]interface{})
		for memberName, respStat := range respStats {
			r := new(ResultStatIndicatorValue)
			err := json.Unmarshal(respStat.Value, r)
			if err != nil {
				logger.Errorf("[BUG: unmarshal ResultStatIndicatorValue in %s failed: %v]",
					memberName, err)
				continue
			}
			values[indicatorName][memberName] = r.Value
		}
	} else {
		// multiple values
		for memberName, respStat := range respStats {
			r := new(ResultStatIndicatorsValue)
			err := json.Unmarshal(respStat.Values, r)
			if err != nil {
				logger.Errorf("[BUG: unmarshal ResultStatIndicatorsValue in %s failed: %v]",
					memberName, err)
				continue
			}
			for indicatorName, value := range r.Values {
				if values[indicatorName] == nil {
					values[indicatorName] = make(map[string]interface{})
				}
				values[indicatorName][memberName] = value
			}
		}
	}

	return values
}

func commonAggregateValue(reqStat *ReqStat, respStats map[string]*RespStat) (map[string]interface{}, error) {
	values := listValues(reqStat, respStats)
	if len(values) == 0 {
		return nil, fmt.Errorf("none of valid values")
	}

	resultValues := make(map[string]interface{})
	for indicatorName, indicatorValues := range values {
		rv := resultStatAggregateValue{}
		aggregator := chooseAggregator(reqStat, indicatorName)
		// If aggregator == nil, it means we meet an indicator without
		// a known built-in aggregator. So we just list the detailed value.
		if aggregator != nil {
			rv.AggregatorName = aggregator.String()
			for memberName, value := range indicatorValues {
				err := aggregator.Aggregate(value)
				if err != nil {
					logger.Errorf("[BUG: aggregate value in %s failed: %v]",
						memberName, err)
					continue
				}
			}
			rv.AggregatedResult = aggregator.Result()
		}
		if reqStat.Detail {
			rv.DetailValues = indicatorValues
		}
		resultValues[indicatorName] = rv
	}

	return resultValues, nil
}

func aggregateValue(reqStat *ReqStat, respStats map[string]*RespStat) (*RespStat, error) {
	resultValues, err := commonAggregateValue(reqStat, respStats)
	if err != nil {
		return nil, err
	}

	result := &ResultStatIndicatorValue{Value: resultValues}
	resultBuff, err := json.Marshal(result)
	if err != nil {
		logger.Errorf("[BUG: marshal %#v failed: %v]", result, err)
		return nil, fmt.Errorf("marshal %#v failed: %v", result, err)
	}

	return &RespStat{Value: resultBuff}, nil
}

func aggregateValues(reqStat *ReqStat, respStats map[string]*RespStat) (*RespStat, error) {
	resultValues, err := commonAggregateValue(reqStat, respStats)
	if err != nil {
		return nil, err
	}

	result := &ResultStatIndicatorsValue{Values: resultValues}
	resultBuff, err := json.Marshal(result)
	if err != nil {
		logger.Errorf("[BUG: marshal %#v failed: %v]", result, err)
		return nil, fmt.Errorf("marshal %#v failed: %v", result, err)
	}

	return &RespStat{Values: resultBuff}, nil
}

var (
	numericMax = func() common.StatAggregator { return new(common.NumericMaxAggregator) }
	numericMin = func() common.StatAggregator { return new(common.NumericMinAggregator) }
	numericSum = func() common.StatAggregator { return new(common.NumericSumAggregator) }
	numericAvg = func() common.StatAggregator { return new(common.NumericAvgAggregator) }
)

func chooseAggregator(reqStat *ReqStat, indicatorName string) common.StatAggregator {
	var aggregator common.StatAggregator
	switch {
	case reqStat.FilterPipelineIndicatorValue != nil ||
		reqStat.FilterPipelineIndicatorsValue != nil:
		aggregator = choosePipelineIndicator(indicatorName)
		if aggregator == nil {
			logger.Warnf("[pipeline indicator %s aggregator not found]", indicatorName)
		}
	case reqStat.FilterPluginIndicatorValue != nil:
		aggregator = choosePluginIndicator(indicatorName)
		if aggregator == nil {
			logger.Warnf("[plugin indicator %s aggregator not found]", indicatorName)
		}
	case reqStat.FilterTaskIndicatorValue != nil:
		aggregator = chooseTaskIndicator(indicatorName)
		if aggregator == nil {
			logger.Warnf("[task indicator %s aggregator not found]", indicatorName)
		}
	default:
		logger.Errorf("[BUG: no value[s]-type filter]")
		return nil
	}

	return aggregator
}
func choosePipelineIndicator(indicatorName string) common.StatAggregator {
	generator := pipelineIndicatorAggregateMap[indicatorName]
	if generator != nil {
		return generator()
	}
	return nil
}
func choosePluginIndicator(indicatorName string) common.StatAggregator {
	generator := pluginIndicatorAggregateMap[indicatorName]
	if generator != nil {
		return generator()
	}
	return nil
}
func chooseTaskIndicator(indicatorName string) common.StatAggregator {
	generator := taskIndicatorAggregateMap[indicatorName]
	if generator != nil {
		return generator()
	}
	return nil
}

var pipelineIndicatorAggregateMap = map[string]func() common.StatAggregator{
	"THROUGHPUT_RATE_LAST_1MIN_ALL":  numericSum,
	"THROUGHPUT_RATE_LAST_5MIN_ALL":  numericSum,
	"THROUGHPUT_RATE_LAST_15MIN_ALL": numericSum,

	"EXECUTION_COUNT_LAST_1MIN_ALL":    numericSum,
	"EXECUTION_TIME_MAX_LAST_1MIN_ALL": numericMax,
	"EXECUTION_TIME_MIN_LAST_1MIN_ALL": numericMin,

	"EXECUTION_TIME_50_PERCENT_LAST_1MIN_ALL": numericMax,
	"EXECUTION_TIME_90_PERCENT_LAST_1MIN_ALL": numericMax,
	"EXECUTION_TIME_99_PERCENT_LAST_1MIN_ALL": numericMax,

	"EXECUTION_TIME_STD_DEV_LAST_1MIN_ALL":  numericMax,
	"EXECUTION_TIME_VARIANCE_LAST_1MIN_ALL": numericMax,
}

var pluginIndicatorAggregateMap = map[string]func() common.StatAggregator{
	"THROUGHPUT_RATE_LAST_1MIN_ALL":      numericSum,
	"THROUGHPUT_RATE_LAST_5MIN_ALL":      numericSum,
	"THROUGHPUT_RATE_LAST_15MIN_ALL":     numericSum,
	"THROUGHPUT_RATE_LAST_1MIN_SUCCESS":  numericSum,
	"THROUGHPUT_RATE_LAST_5MIN_SUCCESS":  numericSum,
	"THROUGHPUT_RATE_LAST_15MIN_SUCCESS": numericSum,
	"THROUGHPUT_RATE_LAST_1MIN_FAILURE":  numericSum,
	"THROUGHPUT_RATE_LAST_5MIN_FAILURE":  numericSum,
	"THROUGHPUT_RATE_LAST_15MIN_FAILURE": numericSum,

	"EXECUTION_COUNT_LAST_1MIN_ALL":     numericSum,
	"EXECUTION_COUNT_LAST_1MIN_SUCCESS": numericSum,
	"EXECUTION_COUNT_LAST_1MIN_FAILURE": numericSum,

	"EXECUTION_TIME_MAX_LAST_1MIN_ALL":     numericMax,
	"EXECUTION_TIME_MAX_LAST_1MIN_SUCCESS": numericMax,
	"EXECUTION_TIME_MAX_LAST_1MIN_FAILURE": numericMax,
	"EXECUTION_TIME_MIN_LAST_1MIN_ALL":     numericMin,
	"EXECUTION_TIME_MIN_LAST_1MIN_SUCCESS": numericMin,
	"EXECUTION_TIME_MIN_LAST_1MIN_FAILURE": numericMin,

	"EXECUTION_TIME_50_PERCENT_LAST_1MIN_ALL":     numericMax,
	"EXECUTION_TIME_50_PERCENT_LAST_1MIN_SUCCESS": numericMax,
	"EXECUTION_TIME_50_PERCENT_LAST_1MIN_FAILURE": numericMax,
	"EXECUTION_TIME_90_PERCENT_LAST_1MIN_ALL":     numericMax,
	"EXECUTION_TIME_90_PERCENT_LAST_1MIN_SUCCESS": numericMax,
	"EXECUTION_TIME_90_PERCENT_LAST_1MIN_FAILURE": numericMax,
	"EXECUTION_TIME_99_PERCENT_LAST_1MIN_ALL":     numericMax,
	"EXECUTION_TIME_99_PERCENT_LAST_1MIN_SUCCESS": numericMax,
	"EXECUTION_TIME_99_PERCENT_LAST_1MIN_FAILURE": numericMax,

	"EXECUTION_TIME_STD_DEV_LAST_1MIN_ALL":      numericMax,
	"EXECUTION_TIME_STD_DEV_LAST_1MIN_SUCCESS":  numericMax,
	"EXECUTION_TIME_STD_DEV_LAST_1MIN_FAILURE":  numericMax,
	"EXECUTION_TIME_VARIANCE_LAST_1MIN_ALL":     numericMax,
	"EXECUTION_TIME_VARIANCE_LAST_1MIN_SUCCESS": numericMax,
	"EXECUTION_TIME_VARIANCE_LAST_1MIN_FAILURE": numericMax,

	// plugin dedicated indicators

	// http_input plugin
	"WAIT_QUEUE_LENGTH": numericSum,
	"WIP_REQUEST_COUNT": numericSum,

	// http_counter plugin
	"RECENT_HEADER_COUNT ": numericSum,
}

var taskIndicatorAggregateMap = map[string]func() common.StatAggregator{
	// task dedicated indicator
	"EXECUTION_COUNT_ALL":     numericSum,
	"EXECUTION_COUNT_SUCCESS": numericSum,
	"EXECUTION_COUNT_FAILURE": numericSum,
}
