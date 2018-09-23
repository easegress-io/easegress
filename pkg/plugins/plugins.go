package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/hexdecteam/easegateway/pkg/common"
	"github.com/hexdecteam/easegateway/pkg/logger"
	"github.com/hexdecteam/easegateway/pkg/option"
	apm "github.com/hexdecteam/easegateway/pkg/plugins/easemonitor"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"
)

// Plugins Register Authority

type pluginEntry struct {
	pluginConstructor plugins.Constructor
	configConstructor plugins.ConfigConstructor
}

var (
	PLUGIN_ENTRIES = map[string]pluginEntry{
		// generic plugins
		"HTTPServer": {
			httpServerConstructor, HTTPServerConfigConstructor},
		"HTTPInput": {
			httpInputConstructor, HTTPInputConfigConstructor},
		"JSONValidator": {
			jsonValidatorConstructor, jsonValidatorConfigConstructor},
		"KafkaOutput": {
			kafkaOutputConstructor, kafkaOutputConfigConstructor},
		"ThroughputRateLimiter": {
			throughputRateLimiterConstructor, throughputRateLimiterConfigConstructor},
		"LatencyLimiter": {
			latencyLimiterConstructor, latencyLimiterConfigConstructor},
		"ServiceCircuitBreaker": {
			serviceCircuitBreakerConstructor, serviceCircuitBreakerConfigConstructor},
		"StaticProbabilityLimiter": {
			staticProbabilityLimiterConstructor, staticProbabilityLimiterConfigConstructor},
		"NoMoreFailureLimiter": {
			noMoreFailureLimiterConstructor, noMoreFailureLimiterConfigConstructor},
		"SimpleCommonMock": {
			simpleCommonMockConstructor, simpleCommonMockConfigConstructor},
		"HTTPHeaderCounter": {
			httpHeaderCounterConstructor, httpHeaderCounterConfigConstructor},
		"HTTPOutput": {
			httpOutputConstructor, HTTPOutputConfigConstructor},
		"IOReader": {
			ioReaderConstructor, ioReaderConfigConfigConstructor},
		"SimpleCommonCache": {
			simpleCommonCacheConstructor, simpleCommonCacheConfigConstructor},
		"UpstreamOutput": {
			upstreamOutputConstructor, upstreamOutputConfigConstructor},
		"DownstreamInput": {
			downstreamInputConstructor, downstreamInputConfigConstructor},
		"Python": {
			pythonConstructor, pythonConfigConstructor},
		"Shell": {
			shellConstructor, shellConfigConstructor},

		// Ease Monitor product dedicated plugins
		"EaseMonitorGraphiteGidExtractor": {
			apm.GraphiteGidExtractorConstructor, apm.GraphiteGidExtractorConfigConstructor},
		"EaseMonitorGraphiteValidator": {
			apm.GraphiteValidatorConstructor, apm.GraphiteValidatorConfigConstructor},
		"EaseMonitorJSONGidExtractor": {
			apm.JSONGidExtractorConstructor, apm.JSONGidExtractorConfigConstructor},
	}
)

func ValidType(t string) bool {
	_, ok := PLUGIN_ENTRIES[t]
	return ok
}

func GetAllTypes() []string {
	types := make([]string, 0, len(PLUGIN_ENTRIES))
	for t := range PLUGIN_ENTRIES {
		types = append(types, t)
	}
	return types
}

func GetConstructor(t string) (plugins.Constructor, error) {
	if !ValidType(t) {
		return nil, fmt.Errorf("invalid plugin type %s", t)
	}

	return PLUGIN_ENTRIES[t].pluginConstructor, nil
}

func GetConfig(t string) (plugins.Config, error) {
	if !ValidType(t) {
		return nil, fmt.Errorf("invalid plugin type %s", t)
	}

	return PLUGIN_ENTRIES[t].configConstructor(), nil
}

// Out-tree plugin type loading

func LoadOutTreePluginTypes() error {
	logger.Debugf("[load all out-tree plugin types]")

	err := os.MkdirAll(common.PLUGIN_HOME_DIR, 0700)
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	count := 0

	err = filepath.Walk(common.PLUGIN_HOME_DIR, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			logger.Warnf("[access %s failed, out-tree plugin types skipped in the file: %v]", path, err)
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		typeNames, failedTypeName, err := loadOutTreePluginTypes(path)
		if err != nil {
			if failedTypeName == "" {
				logger.Errorf("[load out-tree plugin types from %s failed, skipped: %v]", path, err)
			} else {
				logger.Errorf("[load out-tree plugin type %s from %s failed, skipped: %v]",
					failedTypeName, path, err)
			}
		}

		for _, name := range typeNames {
			logger.Debugf("[out-tree plugin type %s is loaded from %s successfully]", name, path)
			count++
		}

		return nil
	})

	if err != nil {
		logger.Errorf("[load out-tree plugin types failed: %v]", err)
		return err
	}

	logger.Infof("[out-tree plugin types are loaded successfully (total=%d)]", count)
	return nil
}

func loadOutTreePluginTypes(path string) ([]string, string, error) {
	ret := make([]string, 0)

	p, err := plugin.Open(path)
	if err != nil {
		return ret, "", err
	}

	f, err := p.Lookup("GetPluginTypeNames")
	if err != nil {
		return ret, "", err
	}

	getTypeNames, ok := f.(func() ([]string, error))
	if !ok {
		return ret, "", fmt.Errorf("invalid plugin type names definition (%T)", f)
	}

	names, err := getTypeNames()
	if err != nil {
		return ret, "", err
	}

	for _, name := range names {
		// check if definition is existing
		_, exists := PLUGIN_ENTRIES[name]
		if exists {
			logger.Warnf("[plugin type %s definied in %s is conflicting, skipped]", name, path)
			continue
		}

		// plugin constructor
		f, err := p.Lookup(fmt.Sprintf("Get%sPluginConstructor", strings.Title(name)))
		if err != nil {
			return ret, name, err
		}

		getConstructor, ok := f.(func() (plugins.Constructor, error))
		if !ok {
			return ret, name, fmt.Errorf("invalid plugin constructor definition (%T)", f)
		}

		var c plugins.Constructor
		var e error

		if common.PanicToErr(func() { c, e = getConstructor() }, &err) {
			return ret, name, err
		} else if e != nil {
			return ret, name, e
		}

		// plugin configuration constructor
		f, err = p.Lookup(fmt.Sprintf("Get%sPluginConfigConstructor", strings.Title(name)))
		if err != nil {
			return ret, name, err
		}

		getConfigConstructor, ok := f.(func() (plugins.ConfigConstructor, error))
		if !ok {
			return ret, name, fmt.Errorf("invalid plugin config constructor definition (%T)", f)
		}

		var cc plugins.ConfigConstructor
		e = nil

		if common.PanicToErr(func() { cc, e = getConfigConstructor() }, &err) {
			return ret, name, err
		} else if e != nil {
			return ret, name, e
		}

		// register
		PLUGIN_ENTRIES[name] = pluginEntry{
			pluginConstructor: c,
			configConstructor: cc,
		}
	}

	return ret, "", nil
}

// Plugins utils

func ReplaceTokensInPattern(t task.Task, pattern string) (string, error) {
	visitor := func(pos int, token string) (bool, string) {
		var ret string

		v := t.Value(token)
		if v != nil {
			ret = task.ToString(v, option.PluginIODataFormatLengthLimit)
		}

		// always do replacement even it is empty (no value found in the task with a certain data key)
		return true, ret
	}

	removeEscapeChar := strings.Contains(pattern, common.TOKEN_ESCAPE_CHAR)

	return common.ScanTokens(pattern, removeEscapeChar, visitor)
}

// limiter related utils

const limiterInboundThroughputRate1Key = "limiterInboundThroughputRate1"
const limiterFlowControlledThroughputRate1Key = "limiterFlowControlledThroughputRate1"

func getInboundThroughputRate1(ctx pipelines.PipelineContext,
	pluginName string) (*common.ThroughputStatistic, error) {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	rate, err := bucket.QueryDataWithBindDefault(limiterInboundThroughputRate1Key, func() interface{} {
		return common.NewThroughputStatistic(common.ThroughputRate1)
	})
	if err != nil {
		logger.Warnf("[BUG: query in-throughput rate data for pipeline %s failed, "+
			"ignored statistic: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return rate.(*common.ThroughputStatistic), nil
}

func getFlowControlledThroughputRate1(ctx pipelines.PipelineContext,
	pluginName string) (*common.ThroughputStatistic, error) {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	rate, err := bucket.QueryDataWithBindDefault(limiterFlowControlledThroughputRate1Key, func() interface{} {
		return common.NewThroughputStatistic(common.ThroughputRate1)
	})
	if err != nil {
		logger.Warnf("[BUG: query flow controlled throughput rate data for pipeline %s failed, "+
			"ignored statistic: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return rate.(*common.ThroughputStatistic), nil
}

func getFlowControlledPercentage(ctx pipelines.PipelineContext, pluginName string) (int, error) {
	rate, err := getFlowControlledThroughputRate1(ctx, pluginName)
	if err != nil {
		return 0, err
	}
	pluginFlowControlledRate, err := rate.Get()
	if err != nil {
		return 0, err
	}
	rate, err = getInboundThroughputRate1(ctx, pluginName)
	if err != nil {
		return 0, err
	}
	pluginInThroughputRate, err := rate.Get()
	if pluginInThroughputRate == 0 { // avoid divide by zero
		return 0, nil
	}
	return int(100.0 * pluginFlowControlledRate / pluginInThroughputRate), nil
}

func registerPluginIndicatorForLimiter(ctx pipelines.PipelineContext, pluginName, pluginInstanceId string) {
	_, err := ctx.Statistics().RegisterPluginIndicator(pluginName, pluginInstanceId,
		"THROUGHPUT_RATE_LAST_1MIN_FLOWCONTROLLED", "Flow controlled throughput rate of the plugin in last 1 minute.",
		func(pluginName, indicatorName string) (interface{}, error) {
			rate, err := getFlowControlledThroughputRate1(ctx, pluginName)
			if err != nil {
				return nil, err
			}
			return rate.Get()
		})
	if err != nil {
		logger.Warnf("[BUG: register plugin indicator for pipeline %s plugin %s failed: %v]",
			ctx.PipelineName(), pluginName, err)
	}

	// We don't use limiter plugin's THROUGHPUT_RATE_LAST_1MIN_ALL
	// because it indicates the throughput rate after applying flow control
	_, err = ctx.Statistics().RegisterPluginIndicator(pluginName, pluginInstanceId,
		"THROUGHPUT_RATE_LAST_1MIN_INBOUND", "All inbound throughput rate of the plugin in last 1 minute.",
		func(pluginName, indicatorName string) (interface{}, error) {
			rate, err := getInboundThroughputRate1(ctx, pluginName)
			if err != nil {
				return nil, err
			}
			return rate.Get()
		})
	if err != nil {
		logger.Warnf("[BUG: register plugin indicator for pipeline %s plugin %s failed: %v]",
			ctx.PipelineName(), pluginName, err)
	}
}

func updateFlowControlledThroughputRate(ctx pipelines.PipelineContext, pluginName string) error {
	flowControlledRate1, err := getFlowControlledThroughputRate1(ctx, pluginName)
	if err != nil {
		return err
	}
	flowControlledRate1.Update(1)
	return nil
}

func updateInboundThroughputRate(ctx pipelines.PipelineContext, pluginName string) error {
	inRate1, err := getInboundThroughputRate1(ctx, pluginName)
	if err != nil {
		return err
	}
	inRate1.Update(1)
	return nil
}
