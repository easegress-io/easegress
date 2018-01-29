package gateway

import (
	"common"
)

var ( //
	numericMax = func() common.StatAggregator { return new(common.NumericMaxAggregator) }
	numericMin = func() common.StatAggregator { return new(common.NumericMinAggregator) }
	numericSum = func() common.StatAggregator { return new(common.NumericSumAggregator) }
	numericAvg = func() common.StatAggregator { return new(common.NumericAvgAggregator) }
)

//////
var indicatorValueAggregators = map[string]func() common.StatAggregator{
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
	// task dedicated indicator
	"EXECUTION_COUNT_ALL":     numericSum,
	"EXECUTION_COUNT_SUCCESS": numericSum,
	"EXECUTION_COUNT_FAILURE": numericSum,
}
