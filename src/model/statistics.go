package model

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/rcrowley/go-metrics"

	"common"
	"logger"
)

//
// Statistics registry
//

type statRegistry struct {
	sync.RWMutex
	statistics map[string]*PipelineStatistics
	mod        *Model
}

func newStatRegistry(m *Model) *statRegistry {
	ret := &statRegistry{
		statistics: make(map[string]*PipelineStatistics),
		mod:        m,
	}

	m.AddPipelineAddedCallback("addPipelineStatistics", ret.addPipelineStatistics,
		false, common.NORMAL_PRIORITY_CALLBACK)
	m.AddPipelineDeletedCallback("deletePipelineStatistics", ret.deletePipelineStatistics,
		false, common.NORMAL_PRIORITY_CALLBACK)
	m.AddPipelineUpdatedCallback("renewPipelineStatistics", ret.renewPipelineStatistics,
		false, common.NORMAL_PRIORITY_CALLBACK)

	return ret
}

func (r *statRegistry) GetPipelineStatistics(name string) *PipelineStatistics {
	r.RLock()
	defer r.RUnlock()
	return r.statistics[name]
}

func (r *statRegistry) addPipelineStatistics(pipeline *Pipeline) {
	r.Lock()
	defer r.Unlock()

	r.statistics[pipeline.Name()] = NewPipelineStatistics(
		pipeline.config.PipelineName(), pipeline.Config().PluginNames(), r.mod)
	logger.Infof("[pipeline %s statistics is created]", pipeline.Name())
}

func (r *statRegistry) deletePipelineStatistics(pipeline *Pipeline) {
	r.Lock()
	defer r.Unlock()

	statistics, exists := r.statistics[pipeline.Name()]
	// Defensive programming
	if exists {
		go statistics.Close()
	}
	delete(r.statistics, pipeline.Name())
	logger.Infof("[pipeline %s statistics is deleted]", pipeline.Name())
}

func (r *statRegistry) renewPipelineStatistics(pipeline *Pipeline) {
	r.deletePipelineStatistics(pipeline)
	r.addPipelineStatistics(pipeline)
}

//
// Statistics indicator
//

type statisticsIndicator struct {
	name, indicatorName, desc string
	evaluator                 pipelines.StatisticsIndicatorEvaluator
}

func (i *statisticsIndicator) Name() string {
	return i.indicatorName
}

func (i *statisticsIndicator) Description() string {
	return i.desc
}

func (i *statisticsIndicator) Evaluate() (interface{}, error) {
	return i.evaluator(i.name, i.indicatorName)
}

func newPipelineStatisticsIndicator(pipelineName, indicatorName, desc string,
	evaluator pipelines.StatisticsIndicatorEvaluator) (*statisticsIndicator, error) {

	pipelineName = strings.TrimSpace(pipelineName)
	indicatorName = strings.TrimSpace(indicatorName)
	desc = strings.TrimSpace(desc)

	if len(pipelineName) == 0 {
		return nil, fmt.Errorf("pipleine name required")
	}

	if len(indicatorName) == 0 {
		return nil, fmt.Errorf("pipeline statistics indicator name required")
	}

	if evaluator == nil {
		return nil, fmt.Errorf("pipeline statistics indicator evaluator required")
	}

	return &statisticsIndicator{
		name:          pipelineName,
		indicatorName: indicatorName,
		desc:          desc,
		evaluator:     evaluator,
	}, nil
}

func newTaskStatisticsIndicator(pipelineName, indicatorName, desc string,
	evaluator pipelines.StatisticsIndicatorEvaluator) (*statisticsIndicator, error) {

	pipelineName = strings.TrimSpace(pipelineName)
	indicatorName = strings.TrimSpace(indicatorName)
	desc = strings.TrimSpace(desc)

	if len(pipelineName) == 0 {
		return nil, fmt.Errorf("pipleine name required")
	}

	if len(indicatorName) == 0 {
		return nil, fmt.Errorf("task statistics indicator name required")
	}

	if evaluator == nil {
		return nil, fmt.Errorf("task statistics indicator evaluator required")
	}

	return &statisticsIndicator{
		name:          pipelineName,
		indicatorName: indicatorName,
		desc:          desc,
		evaluator:     evaluator,
	}, nil
}

type pluginStatisticsIndicator struct {
	statisticsIndicator

	instanceId string
	version    int64
}

func (i *pluginStatisticsIndicator) Name() string {
	return i.statisticsIndicator.indicatorName
}

func (i *pluginStatisticsIndicator) Description() string {
	return i.statisticsIndicator.desc
}

func (i *pluginStatisticsIndicator) Evaluate() (interface{}, error) {
	return i.statisticsIndicator.evaluator(i.name, i.indicatorName)
}

func (i *pluginStatisticsIndicator) InstanceId() string {
	return i.instanceId
}

func (i *pluginStatisticsIndicator) Version() int64 {
	return i.version
}

func newPluginStatisticsIndicator(pluginName, pluginInstanceId, indicatorName, desc string,
	evaluator pipelines.StatisticsIndicatorEvaluator) (*pluginStatisticsIndicator, error) {

	pluginName = strings.TrimSpace(pluginName)
	pluginInstanceId = strings.TrimSpace(pluginInstanceId)
	indicatorName = strings.TrimSpace(indicatorName)
	desc = strings.TrimSpace(desc)

	if len(pluginName) == 0 {
		return nil, fmt.Errorf("plugin name required")
	}

	if len(pluginInstanceId) == 0 {
		return nil, fmt.Errorf("plugin instance id required")
	}

	if len(indicatorName) == 0 {
		return nil, fmt.Errorf("plugin statistics indicator name required")
	}

	if evaluator == nil {
		return nil, fmt.Errorf("plugin statistics indicator evaluator required")
	}

	return &pluginStatisticsIndicator{
		statisticsIndicator: statisticsIndicator{
			name:          pluginName,
			indicatorName: indicatorName,
			desc:          desc,
			evaluator:     evaluator,
		},
		instanceId: pluginInstanceId,
		version:    time.Now().Unix(),
	}, nil
}

//
// Pipeline statistics
//

type PipelineStatistics struct {
	sync.RWMutex
	pipelineName string

	pipelineThroughputRates1, pipelineThroughputRates5, pipelineThroughputRates15 metrics.EWMA
	pipelineExecutionSample                                                       metrics.Sample

	pluginSuccessThroughputRates1, pluginSuccessThroughputRates5,
	pluginSuccessThroughputRates15, pluginFailureThroughputRates1,
	pluginFailureThroughputRates5, pluginFailureThroughputRates15 map[string]metrics.EWMA
	pluginSuccessExecutionSamples, pluginFailureExecutionSamples map[string]metrics.Sample

	taskSuccessCount, taskFailureCount uint64

	pipelineIndicators map[string]*statisticsIndicator
	pluginIndicators   map[string]map[string][]*pluginStatisticsIndicator
	taskIndicators     map[string]*statisticsIndicator

	done chan struct{}
	mod  *Model

	pipelineThroughputRateUpdatedCallbacks, pipelineExecutionSampleUpdatedCallbacks,
	pluginThroughputRateUpdatedCallbacks, pluginExecutionSampleUpdatedCallbacks []*common.NamedCallback
}

func NewPipelineStatistics(pipelineName string, pluginNames []string, m *Model) *PipelineStatistics {
	ret := &PipelineStatistics{
		pipelineName:                   pipelineName,
		pipelineThroughputRates1:       metrics.NewEWMA1(),
		pipelineThroughputRates5:       metrics.NewEWMA5(),
		pipelineThroughputRates15:      metrics.NewEWMA15(),
		pipelineExecutionSample:        metrics.NewExpDecaySample(514, 0.015),
		pluginSuccessThroughputRates1:  make(map[string]metrics.EWMA),
		pluginSuccessThroughputRates5:  make(map[string]metrics.EWMA),
		pluginSuccessThroughputRates15: make(map[string]metrics.EWMA),
		pluginSuccessExecutionSamples:  make(map[string]metrics.Sample),
		pluginFailureThroughputRates1:  make(map[string]metrics.EWMA),
		pluginFailureThroughputRates5:  make(map[string]metrics.EWMA),
		pluginFailureThroughputRates15: make(map[string]metrics.EWMA),
		pluginFailureExecutionSamples:  make(map[string]metrics.Sample),
		pipelineIndicators:             make(map[string]*statisticsIndicator),
		pluginIndicators:               make(map[string]map[string][]*pluginStatisticsIndicator),
		taskIndicators:                 make(map[string]*statisticsIndicator),
		done:                           make(chan struct{}),
		mod:                            m,
		pipelineThroughputRateUpdatedCallbacks:  make([]*common.NamedCallback, 0, common.CallbacksInitCapacity),
		pipelineExecutionSampleUpdatedCallbacks: make([]*common.NamedCallback, 0, common.CallbacksInitCapacity),
		pluginThroughputRateUpdatedCallbacks:    make([]*common.NamedCallback, 0, common.CallbacksInitCapacity),
		pluginExecutionSampleUpdatedCallbacks:   make([]*common.NamedCallback, 0, common.CallbacksInitCapacity),
	}

	tickFun := func(ewmas []metrics.EWMA) {
		ticker := time.NewTicker(time.Duration(5) * time.Second)

		for {
			select {
			case <-ticker.C:
				for _, ewma := range ewmas {
					ewma.Tick()
				}
			case <-ret.done:
				ticker.Stop()
				return
			}
		}
	}

	go tickFun([]metrics.EWMA{
		ret.pipelineThroughputRates1, ret.pipelineThroughputRates5, ret.pipelineThroughputRates15,
	})

	for _, name := range pluginNames {
		ewma1 := metrics.NewEWMA1()
		ewma5 := metrics.NewEWMA5()
		ewma15 := metrics.NewEWMA15()

		ret.pluginSuccessThroughputRates1[name] = ewma1
		ret.pluginSuccessThroughputRates5[name] = ewma5
		ret.pluginSuccessThroughputRates15[name] = ewma15

		go tickFun([]metrics.EWMA{ewma1, ewma5, ewma15})

		ewma1 = metrics.NewEWMA1()
		ewma5 = metrics.NewEWMA5()
		ewma15 = metrics.NewEWMA15()

		ret.pluginFailureThroughputRates1[name] = ewma1
		ret.pluginFailureThroughputRates5[name] = ewma5
		ret.pluginFailureThroughputRates15[name] = ewma15

		go tickFun([]metrics.EWMA{ewma1, ewma5, ewma15})

		// https://github.com/rcrowley/go-metrics/blob/master/sample_test.go#L62
		// http://dimacs.rutgers.edu/~graham/pubs/papers/fwddecay.pdf
		ret.pluginSuccessExecutionSamples[name] = metrics.NewExpDecaySample(514, 0.015)
		ret.pluginFailureExecutionSamples[name] = metrics.NewExpDecaySample(514, 0.015)
	}

	// Expose pipeline statistics values as indicators
	ret.registerPipelineIndicator("THROUGHPUT_RATE_LAST_1MIN_ALL",
		"Throughput rate of the pipeline in last 1 minute.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineThroughputRate1()
		})
	ret.registerPipelineIndicator("THROUGHPUT_RATE_LAST_5MIN_ALL",
		"Throughput rate of the pipeline in last 5 minute.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineThroughputRate5()
		})
	ret.registerPipelineIndicator("THROUGHPUT_RATE_LAST_15MIN_ALL",
		"Throughput rate of the pipeline in last 15 minute.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineThroughputRate15()
		})
	ret.registerPipelineIndicator("EXECUTION_COUNT_ALL",
		"Total execution count of the pipeline.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionCount()
		})
	ret.registerPipelineIndicator("EXECUTION_TIME_MAX_ALL",
		"Maximal time of execution time of the pipeline in nanosecond.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionTimeMax()
		})
	ret.registerPipelineIndicator("EXECUTION_TIME_MIN_ALL",
		"Minimal time of execution time of the pipeline in nanosecond.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionTimeMin()
		})
	ret.registerPipelineIndicator("EXECUTION_TIME_50_PERCENT_ALL",
		"50% execution time of the pipeline in nanosecond.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionTimePercentile(0.5)
		})
	ret.registerPipelineIndicator("EXECUTION_TIME_90_PERCENT_ALL",
		"90% execution time of the pipeline in nanosecond.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionTimePercentile(0.9)
		})
	ret.registerPipelineIndicator("EXECUTION_TIME_99_PERCENT_ALL",
		"99% execution time of the pipeline in nanosecond.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionTimePercentile(0.99)
		})
	ret.registerPipelineIndicator("EXECUTION_TIME_STD_DEV_ALL",
		"Standard deviation of execution time of the pipeline in nanosecond.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionTimeStdDev()
		})
	ret.registerPipelineIndicator("EXECUTION_TIME_VARIANCE_ALL",
		"Variance of execution time of the pipeline.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionTimeVariance()
		})
	ret.registerPipelineIndicator("EXECUTION_TIME_SUM_ALL",
		"Sum of execution time of the pipeline in nanosecond.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.PipelineExecutionTimeSum()
		})

	// Expose common plugin statistics values as builtin plugin indicators
	for _, pluginName := range pluginNames {
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_1MIN_ALL", "Throughput rate of the plugin in last 1 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate1(pluginName, pipelines.AllStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_5MIN_ALL", "Throughput rate of the plugin in last 5 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate5(pluginName, pipelines.AllStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_15MIN_ALL", "Throughput rate of the plugin in last 15 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate15(pluginName, pipelines.AllStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_1MIN_SUCCESS",
			"Successful throughput rate of the plugin in last 1 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate1(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_5MIN_SUCCESS",
			"Successful throughput rate of the plugin in last 5 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate5(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_15MIN_SUCCESS",
			"Successful throughput rate of the plugin in last 15 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate15(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_1MIN_FAILURE",
			"Failed throughput rate of the plugin in last 1 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate1(pluginName, pipelines.FailureStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_5MIN_FAILURE",
			"Failed throughput rate of the plugin in last 5 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate5(pluginName, pipelines.FailureStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"THROUGHPUT_RATE_LAST_15MIN_FAILURE",
			"Failed throughput rate of the plugin in last 15 minute.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginThroughputRate15(pluginName, pipelines.FailureStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_COUNT_ALL", "Total execution count of the plugin.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionCount(pluginName, pipelines.AllStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_COUNT_SUCCESS", "Successful execution count of the plugin.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionCount(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_COUNT_FAILURE", "Failed execution count of the plugin.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionCount(pluginName, pipelines.FailureStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_MAX_ALL", "Maximal time of execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeMax(pluginName, pipelines.AllStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_MAX_SUCCESS",
			"Maximal time of successful execution of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeMax(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_MAX_FAILURE", "Maximal time of failure execution of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeMax(pluginName, pipelines.FailureStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_MIN_ALL", "Minimal time of execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeMin(pluginName, pipelines.AllStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_MIN_SUCCESS",
			"Minimal time of successful execution of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeMin(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_MIN_FAILURE", "Minimal time of failure execution of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeMin(pluginName, pipelines.FailureStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_50_PERCENT_SUCCESS",
			"50% successful execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimePercentile(
					pluginName, pipelines.SuccessStatistics, 0.5)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_50_PERCENT_FAILURE", "50% failure execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimePercentile(
					pluginName, pipelines.FailureStatistics, 0.5)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_90_PERCENT_SUCCESS",
			"90% successful execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimePercentile(
					pluginName, pipelines.SuccessStatistics, 0.9)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_90_PERCENT_FAILURE", "90% failure execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimePercentile(
					pluginName, pipelines.FailureStatistics, 0.9)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_99_PERCENT_SUCCESS",
			"99% successful execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimePercentile(
					pluginName, pipelines.SuccessStatistics, 0.99)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_99_PERCENT_FAILURE", "99% failure execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimePercentile(
					pluginName, pipelines.FailureStatistics, 0.99)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_STD_DEV_SUCCESS",
			"Standard deviation of successful execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeStdDev(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_STD_DEV_FAILURE",
			"Standard deviation of failure execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeStdDev(pluginName, pipelines.FailureStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_VARIANCE_SUCCESS", "Variance of successful execution time of the plugin.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeVariance(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_VARIANCE_FAILURE", "Variance of failure execution time of the plugin.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeVariance(pluginName, pipelines.FailureStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_SUM_ALL", "Sum of execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeSum(pluginName, pipelines.AllStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_SUM_SUCCESS", "Sum of successful execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeSum(pluginName, pipelines.SuccessStatistics)
			})
		ret.RegisterPluginIndicator(pluginName, pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE,
			"EXECUTION_TIME_SUM_FAILURE", "Sum of failure execution time of the plugin in nanosecond.",
			func(pluginName, indicatorName string) (interface{}, error) {
				return ret.PluginExecutionTimeSum(pluginName, pipelines.FailureStatistics)
			})
	}

	// Expose task statistics values as indicators
	ret.registerTaskIndicator("EXECUTION_COUNT_ALL",
		"Total task execution count.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.TaskExecutionCount(pipelines.AllStatistics)
		})
	ret.registerTaskIndicator("EXECUTION_COUNT_SUCCESS",
		"Successful task execution count.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.TaskExecutionCount(pipelines.SuccessStatistics)
		})
	ret.registerTaskIndicator("EXECUTION_COUNT_FAILURE",
		"Failed task execution count.",
		func(pipelineName, indicatorName string) (interface{}, error) {
			return ret.TaskExecutionCount(pipelines.FailureStatistics)
		})

	return ret
}

func (ps *PipelineStatistics) Close() {
	ps.RLock()
	defer ps.RUnlock()
	close(ps.done)
}

func (ps *PipelineStatistics) PipelineThroughputRate1() (float64, error) {
	return ps.pipelineThroughputRates1.Rate(), nil
}

func (ps *PipelineStatistics) PipelineThroughputRate5() (float64, error) {
	return ps.pipelineThroughputRates5.Rate(), nil
}

func (ps *PipelineStatistics) PipelineThroughputRate15() (float64, error) {
	return ps.pipelineThroughputRates15.Rate(), nil
}

func (ps *PipelineStatistics) PipelineExecutionCount() (int64, error) {
	return ps.pipelineExecutionSample.Count(), nil
}

func (ps *PipelineStatistics) PipelineExecutionTimeMax() (int64, error) {
	return ps.pipelineExecutionSample.Max(), nil
}

func (ps *PipelineStatistics) PipelineExecutionTimeMin() (int64, error) {
	return ps.pipelineExecutionSample.Min(), nil
}

func (ps *PipelineStatistics) PipelineExecutionTimePercentile(percentile float64) (float64, error) {
	return ps.pipelineExecutionSample.Percentile(percentile), nil
}

func (ps *PipelineStatistics) PipelineExecutionTimeStdDev() (float64, error) {
	return ps.pipelineExecutionSample.StdDev(), nil
}

func (ps *PipelineStatistics) PipelineExecutionTimeVariance() (float64, error) {
	return ps.pipelineExecutionSample.Variance(), nil
}

func (ps *PipelineStatistics) PipelineExecutionTimeSum() (int64, error) {
	return ps.pipelineExecutionSample.Sum(), nil
}

func (ps *PipelineStatistics) PluginThroughputRate1(pluginName string,
	kind pipelines.StatisticsKind) (float64, error) {

	switch kind {
	case pipelines.SuccessStatistics:
		return ps.pluginThroughputRate(pluginName, ps.pluginSuccessThroughputRates1)
	case pipelines.FailureStatistics:
		return ps.pluginThroughputRate(pluginName, ps.pluginFailureThroughputRates1)
	case pipelines.AllStatistics:
		v1, err := ps.pluginThroughputRate(pluginName, ps.pluginSuccessThroughputRates1)
		if err != nil {
			return -1, err
		}
		v2, err := ps.pluginThroughputRate(pluginName, ps.pluginFailureThroughputRates1)
		if err != nil {
			return -1, err
		}
		return v1 + v2, nil

	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginThroughputRate5(pluginName string,
	kind pipelines.StatisticsKind) (float64, error) {

	switch kind {
	case pipelines.SuccessStatistics:
		return ps.pluginThroughputRate(pluginName, ps.pluginSuccessThroughputRates5)
	case pipelines.FailureStatistics:
		return ps.pluginThroughputRate(pluginName, ps.pluginFailureThroughputRates5)
	case pipelines.AllStatistics:
		v1, err := ps.pluginThroughputRate(pluginName, ps.pluginSuccessThroughputRates5)
		if err != nil {
			return -1, err
		}
		v2, err := ps.pluginThroughputRate(pluginName, ps.pluginFailureThroughputRates5)
		if err != nil {
			return -1, err
		}
		return v1 + v2, nil

	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginThroughputRate15(pluginName string,
	kind pipelines.StatisticsKind) (float64, error) {

	switch kind {
	case pipelines.SuccessStatistics:
		return ps.pluginThroughputRate(pluginName, ps.pluginSuccessThroughputRates15)
	case pipelines.FailureStatistics:
		return ps.pluginThroughputRate(pluginName, ps.pluginFailureThroughputRates15)
	case pipelines.AllStatistics:
		v1, err := ps.pluginThroughputRate(pluginName, ps.pluginSuccessThroughputRates15)
		if err != nil {
			return -1, err
		}
		v2, err := ps.pluginThroughputRate(pluginName, ps.pluginFailureThroughputRates15)
		if err != nil {
			return -1, err
		}
		return v1 + v2, err
	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginExecutionCount(pluginName string,
	kind pipelines.StatisticsKind) (int64, error) {

	ps.RLock()
	defer ps.RUnlock()

	switch kind {
	case pipelines.SuccessStatistics:
		sample, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Count(), nil
	case pipelines.FailureStatistics:
		sample, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Count(), nil
	case pipelines.AllStatistics:
		sample1, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		sample2, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample1.Count() + sample2.Count(), nil
	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginExecutionTimeMax(pluginName string,
	kind pipelines.StatisticsKind) (int64, error) {

	ps.RLock()
	defer ps.RUnlock()

	switch kind {
	case pipelines.SuccessStatistics:
		sample, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Max(), nil
	case pipelines.FailureStatistics:
		sample, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Max(), nil
	case pipelines.AllStatistics:
		sample1, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		sample2, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		v1, v2 := sample1.Max(), sample2.Max()
		if v1 > v2 {
			return v1, nil
		} else {
			return v2, nil
		}
	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginExecutionTimeMin(pluginName string,
	kind pipelines.StatisticsKind) (int64, error) {

	ps.RLock()
	defer ps.RUnlock()

	switch kind {
	case pipelines.SuccessStatistics:
		sample, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Min(), nil
	case pipelines.FailureStatistics:
		sample, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Min(), nil
	case pipelines.AllStatistics:
		sample1, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		sample2, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		v1, v2 := sample1.Min(), sample2.Min()
		if v1 > v2 {
			return v2, nil
		} else {
			return v1, nil
		}
	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginExecutionTimePercentile(pluginName string,
	kind pipelines.StatisticsKind, percentile float64) (float64, error) {

	ps.RLock()
	defer ps.RUnlock()

	switch kind {
	case pipelines.SuccessStatistics:
		sample, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Percentile(percentile), nil
	case pipelines.FailureStatistics:
		sample, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Percentile(percentile), nil
	case pipelines.AllStatistics:
		// Unsupported, doesn't make sense
		fallthrough
	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginExecutionTimeStdDev(pluginName string,
	kind pipelines.StatisticsKind) (float64, error) {

	ps.RLock()
	defer ps.RUnlock()

	switch kind {
	case pipelines.SuccessStatistics:
		sample, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.StdDev(), nil
	case pipelines.FailureStatistics:
		sample, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.StdDev(), nil
	case pipelines.AllStatistics:
		// Unsupported, doesn't make sense
		fallthrough
	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginExecutionTimeVariance(pluginName string,
	kind pipelines.StatisticsKind) (float64, error) {

	ps.RLock()
	defer ps.RUnlock()

	switch kind {
	case pipelines.SuccessStatistics:
		sample, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Variance(), nil
	case pipelines.FailureStatistics:
		sample, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Variance(), nil
	case pipelines.AllStatistics:
		// Unsupported, doesn't make sense
		fallthrough
	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PluginExecutionTimeSum(pluginName string,
	kind pipelines.StatisticsKind) (int64, error) {

	ps.RLock()
	defer ps.RUnlock()

	switch kind {
	case pipelines.SuccessStatistics:
		sample, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Sum(), nil
	case pipelines.FailureStatistics:
		sample, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample.Sum(), nil
	case pipelines.AllStatistics:
		sample1, exists := ps.pluginSuccessExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		sample2, exists := ps.pluginFailureExecutionSamples[pluginName]
		if !exists {
			return -1, fmt.Errorf("invalid plugin name")
		}
		return sample1.Sum() + sample2.Sum(), nil
	default:
		return -1, fmt.Errorf("invalid plugin statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) TaskExecutionCount(kind pipelines.StatisticsKind) (uint64, error) {
	ps.RLock()
	defer ps.RUnlock()

	switch kind {
	case pipelines.SuccessStatistics:
		return ps.taskSuccessCount, nil
	case pipelines.FailureStatistics:
		return ps.taskFailureCount, nil
	case pipelines.AllStatistics:
		return ps.taskSuccessCount + ps.taskFailureCount, nil
	default:
		return 0, fmt.Errorf("invalid task statistics kind %s", kind)
	}
}

func (ps *PipelineStatistics) PipelineIndicatorNames() []string {
	ret := make([]string, 0)

	ps.RLock()
	defer ps.RUnlock()

	for _, indicator := range ps.pipelineIndicators {
		ret = append(ret, indicator.Name())
	}

	return ret
}

func (ps *PipelineStatistics) PipelineIndicatorValue(indicatorName string) (interface{}, error) {
	ps.RLock()
	defer ps.RUnlock()

	indicator, exists := ps.pipelineIndicators[indicatorName]
	if !exists {
		return nil, fmt.Errorf("pipeline %s statistics indicator %s not found", ps.pipelineName, indicatorName)
	}

	var (
		ret interface{}
		err error
	)

	func() {
		defer func() { // defensive
			e := recover()
			if e != nil {
				err = fmt.Errorf("%v", e)
			}

		}()
		ret, err = indicator.Evaluate()
	}()
	return ret, err
}

func (ps *PipelineStatistics) PipelineIndicatorDescription(indicatorName string) (string, error) {
	ps.RLock()
	defer ps.RUnlock()

	indicator, exists := ps.pipelineIndicators[indicatorName]
	if !exists {
		return "", fmt.Errorf("pipeline %s statistics indicator %s not found", ps.pipelineName, indicatorName)
	}

	return indicator.Description(), nil
}

func (ps *PipelineStatistics) PluginIndicatorNames(pluginName string) []string {
	ret := make([]string, 0)

	ps.RLock()
	defer ps.RUnlock()

	indicators, exists := ps.pluginIndicators[pluginName]
	if !exists {
		return ret
	}

	for name := range indicators {
		ret = append(ret, name)
	}

	return ret
}

func (ps *PipelineStatistics) PluginIndicatorValue(pluginName, indicatorName string) (interface{}, error) {
	ps.RLock()
	defer ps.RUnlock()

	indicators, exists := ps.pluginIndicators[pluginName]
	if !exists {
		return nil, fmt.Errorf("plugin %s statistics not found", pluginName)
	}

	versions, exists := indicators[indicatorName]
	if !exists {
		return nil, fmt.Errorf("plugin %s statistics indicator %s not found", pluginName, indicatorName)
	}

	var (
		ret interface{}
		err error
	)

	func() {
		defer func() { // defensive
			e := recover()
			if e != nil {
				err = fmt.Errorf("%v", e)
			}

		}()

		var (
			max_version int64 = -1
			indicator   *pluginStatisticsIndicator
		)

		for _, version := range versions {
			if version.version > max_version { // use indicator of latest plugin instance
				indicator = version
			}
		}
		ret, err = indicator.Evaluate()
	}()
	return ret, err
}

func (ps *PipelineStatistics) PluginIndicatorDescription(pluginName, indicatorName string) (string, error) {
	ps.RLock()
	defer ps.RUnlock()

	indicators, exists := ps.pluginIndicators[pluginName]
	if !exists {
		return "", fmt.Errorf("plugin %s statistics not found", pluginName)
	}

	versions, exists := indicators[indicatorName]
	if !exists {
		return "", fmt.Errorf("plugin %s statistics indicator %s not found", pluginName, indicatorName)
	}

	var (
		max_version int64 = -1
		indicator   *pluginStatisticsIndicator
	)

	for _, version := range versions {
		if version.version > max_version { // use indicator of latest plugin instance
			indicator = version
		}
	}
	return indicator.Description(), nil
}

func (ps *PipelineStatistics) TaskIndicatorNames() []string {
	ret := make([]string, 0)

	ps.RLock()
	defer ps.RUnlock()

	for _, indicator := range ps.taskIndicators {
		ret = append(ret, indicator.Name())
	}

	return ret
}

func (ps *PipelineStatistics) TaskIndicatorValue(indicatorName string) (interface{}, error) {
	ps.RLock()
	defer ps.RUnlock()

	indicator, exists := ps.taskIndicators[indicatorName]
	if !exists {
		return nil, fmt.Errorf("task statistics indicator %s not found", indicatorName)
	}

	var (
		ret interface{}
		err error
	)

	func() {
		defer func() { // defensive
			e := recover()
			if e != nil {
				err = fmt.Errorf("%v", e)
			}

		}()
		ret, err = indicator.Evaluate()
	}()
	return ret, err
}

func (ps *PipelineStatistics) TaskIndicatorDescription(indicatorName string) (string, error) {
	ps.RLock()
	defer ps.RUnlock()

	indicator, exists := ps.taskIndicators[indicatorName]
	if !exists {
		return "", fmt.Errorf("task statistics indicator %s not found", indicatorName)
	}

	return indicator.Description(), nil
}

func (ps *PipelineStatistics) AddPipelineThroughputRateUpdatedCallback(name string,
	callback pipelines.PipelineThroughputRateUpdated, overwrite bool) (
	pipelines.PipelineThroughputRateUpdated, bool) {

	ps.Lock()
	defer ps.Unlock()

	var oriCallback interface{}
	var added bool
	ps.pipelineThroughputRateUpdatedCallbacks, oriCallback, added = common.AddCallback(
		ps.pipelineThroughputRateUpdatedCallbacks, name, callback, overwrite, common.NORMAL_PRIORITY_CALLBACK)

	if oriCallback == nil {
		return nil, added
	} else {
		return oriCallback.(pipelines.PipelineThroughputRateUpdated), added
	}
}

func (ps *PipelineStatistics) DeletePipelineThroughputRateUpdatedCallback(name string) {
	ps.Lock()
	defer ps.Unlock()

	ps.pipelineThroughputRateUpdatedCallbacks, _ = common.DeleteCallback(
		ps.pipelineThroughputRateUpdatedCallbacks, name)
}

func (ps *PipelineStatistics) AddPipelineExecutionSampleUpdatedCallback(name string,
	callback pipelines.PipelineExecutionSampleUpdated, overwrite bool) (
	pipelines.PipelineExecutionSampleUpdated, bool) {

	ps.Lock()
	defer ps.Unlock()

	var oriCallback interface{}
	var added bool
	ps.pipelineExecutionSampleUpdatedCallbacks, oriCallback, added = common.AddCallback(
		ps.pipelineExecutionSampleUpdatedCallbacks, name, callback, overwrite, common.NORMAL_PRIORITY_CALLBACK)

	if oriCallback == nil {
		return nil, added
	} else {
		return oriCallback.(pipelines.PipelineExecutionSampleUpdated), added
	}
}

func (ps *PipelineStatistics) DeletePipelineExecutionSampleUpdatedCallback(name string) {
	ps.Lock()
	defer ps.Unlock()

	ps.pipelineExecutionSampleUpdatedCallbacks, _ = common.DeleteCallback(
		ps.pipelineExecutionSampleUpdatedCallbacks, name)
}

func (ps *PipelineStatistics) AddPluginThroughputRateUpdatedCallback(name string,
	callback pipelines.PluginThroughputRateUpdated,
	overwrite bool) (pipelines.PluginThroughputRateUpdated, bool) {

	ps.Lock()
	defer ps.Unlock()

	var oriCallback interface{}
	var added bool
	ps.pluginThroughputRateUpdatedCallbacks, oriCallback, added = common.AddCallback(
		ps.pluginThroughputRateUpdatedCallbacks, name, callback, overwrite, common.NORMAL_PRIORITY_CALLBACK)

	if oriCallback == nil {
		return nil, added
	} else {
		return oriCallback.(pipelines.PluginThroughputRateUpdated), added
	}
}

func (ps *PipelineStatistics) DeletePluginThroughputRateUpdatedCallback(name string) {
	ps.Lock()
	defer ps.Unlock()

	ps.pluginThroughputRateUpdatedCallbacks, _ = common.DeleteCallback(
		ps.pluginThroughputRateUpdatedCallbacks, name)
}

func (ps *PipelineStatistics) AddPluginExecutionSampleUpdatedCallback(name string,
	callback pipelines.PluginExecutionSampleUpdated, overwrite bool) (
	pipelines.PluginExecutionSampleUpdated, bool) {

	ps.Lock()
	defer ps.Unlock()

	var oriCallback interface{}
	var added bool
	ps.pluginExecutionSampleUpdatedCallbacks, oriCallback, added = common.AddCallback(
		ps.pluginExecutionSampleUpdatedCallbacks, name, callback, overwrite, common.NORMAL_PRIORITY_CALLBACK)

	if oriCallback == nil {
		return nil, added
	} else {
		return oriCallback.(pipelines.PluginExecutionSampleUpdated), added
	}
}

func (ps *PipelineStatistics) DeletePluginExecutionSampleUpdatedCallback(name string) {
	ps.Lock()
	defer ps.Unlock()

	ps.pluginExecutionSampleUpdatedCallbacks, _ = common.DeleteCallback(
		ps.pluginExecutionSampleUpdatedCallbacks, name)
}

func (ps *PipelineStatistics) registerPipelineIndicator(indicatorName, desc string,
	evaluator pipelines.StatisticsIndicatorEvaluator) error {

	ps.Lock()
	defer ps.Unlock()

	_, exists := ps.pipelineIndicators[indicatorName]
	if exists {
		return fmt.Errorf("duplicate indicator %s of pipeline %s", indicatorName, ps.pipelineName)
	}

	indicator, err := newPipelineStatisticsIndicator(ps.pipelineName, indicatorName, desc, evaluator)
	if err != nil {
		return err
	}

	ps.pipelineIndicators[indicatorName] = indicator

	return nil
}

func (ps *PipelineStatistics) RegisterPluginIndicator(pluginName, pluginInstanceId, indicatorName, desc string,
	evaluator pipelines.StatisticsIndicatorEvaluator) (bool, error) {

	ps.RLock()

	indicators, exists := ps.pluginIndicators[pluginName]
	if exists {
		versions, exists := indicators[indicatorName]
		if exists {
			for _, ver := range versions {
				if ver.InstanceId() == pluginInstanceId {
					ps.RUnlock()
					return false, nil
				}
			}
		}
	}

	ps.RUnlock()

	ps.Lock()
	defer ps.Unlock()

	// DCL
	indicators, exists = ps.pluginIndicators[pluginName]
	if !exists {
		indicators = make(map[string][]*pluginStatisticsIndicator)
		ps.pluginIndicators[pluginName] = indicators
	}

	versions, exists := indicators[indicatorName]
	if exists {
		for _, ver := range versions {
			if ver.InstanceId() == pluginInstanceId {
				return false, nil
			}
		}
	}

	indicator, err := newPluginStatisticsIndicator(pluginName, pluginInstanceId, indicatorName, desc, evaluator)
	if err != nil {
		return false, err
	}

	versions = append(versions, indicator)
	indicators[indicatorName] = versions

	return true, nil
}

func (ps *PipelineStatistics) UnregisterPluginIndicator(pluginName, pluginInstanceId, indicatorName string) {
	ps.Lock()
	defer ps.Unlock()

	indicators, exists := ps.pluginIndicators[pluginName]
	if !exists {
		return
	}

	versions, exists := indicators[indicatorName]
	if !exists {
		return
	}

	for i, ver := range versions {
		if ver.InstanceId() == pluginInstanceId {
			versions = append(versions[:i], versions[i+1:]...)
			break
		}
	}

	if len(versions) > 0 {
		indicators[indicatorName] = versions
	} else {
		delete(indicators, indicatorName)
	}
}

func (ps *PipelineStatistics) registerTaskIndicator(indicatorName, desc string,
	evaluator pipelines.StatisticsIndicatorEvaluator) error {

	ps.Lock()
	defer ps.Unlock()

	_, exists := ps.taskIndicators[indicatorName]
	if exists {
		return fmt.Errorf("duplicate indicator %s of task", indicatorName)
	}

	indicator, err := newTaskStatisticsIndicator(ps.pipelineName, indicatorName, desc, evaluator)
	if err != nil {
		return err
	}

	ps.taskIndicators[indicatorName] = indicator

	return nil
}

func (ps *PipelineStatistics) pluginThroughputRate(pluginName string, slot map[string]metrics.EWMA) (float64, error) {
	ps.RLock()
	defer ps.RUnlock()

	ewma, exists := slot[pluginName]
	if !exists {
		return -1, fmt.Errorf("invalid plugin name")
	}

	return ewma.Rate(), nil
}

func (ps *PipelineStatistics) updatePipelineExecution(duration time.Duration) error {
	ps.pipelineExecutionSample.Update(int64(duration)) // safe conversion

	ps.RLock()

	tmp := make([]*common.NamedCallback, len(ps.pipelineExecutionSampleUpdatedCallbacks))
	copy(tmp, ps.pipelineExecutionSampleUpdatedCallbacks)

	ps.RUnlock()

	for _, callback := range tmp {
		go callback.Callback().(pipelines.PipelineExecutionSampleUpdated)(ps.pipelineName, ps)
	}

	ps.pipelineThroughputRates1.Update(1)
	ps.pipelineThroughputRates5.Update(1)
	ps.pipelineThroughputRates15.Update(1)

	ps.RLock()

	tmp = make([]*common.NamedCallback, len(ps.pipelineThroughputRateUpdatedCallbacks))
	copy(tmp, ps.pipelineThroughputRateUpdatedCallbacks)

	ps.RUnlock()

	for _, callback := range tmp {
		go callback.Callback().(pipelines.PipelineThroughputRateUpdated)(ps.pipelineName, ps)
	}

	return nil
}

func (ps *PipelineStatistics) updatePluginExecution(pluginName string,
	kind pipelines.StatisticsKind, duration time.Duration) error {

	if kind == pipelines.AllStatistics {
		return fmt.Errorf("only supports plugin success and failure statistics kinds")
	}

	err := func() error {
		ps.RLock()
		defer ps.RUnlock()

		switch kind {
		case pipelines.SuccessStatistics:
			sample, exists := ps.pluginSuccessExecutionSamples[pluginName]
			if !exists {
				return fmt.Errorf("invalid plugin name")
			}
			sample.Update(int64(duration)) // safe conversion
		case pipelines.FailureStatistics:
			sample, exists := ps.pluginFailureExecutionSamples[pluginName]
			if !exists {
				return fmt.Errorf("invalid plugin name")
			}
			sample.Update(int64(duration)) // safe conversion
		case pipelines.AllStatistics:
			fallthrough
		default:
			return fmt.Errorf("invalid plugin statistics kind %s", kind)
		}

		return nil
	}()

	if err != nil {
		return err
	}

	ps.RLock()

	tmp := make([]*common.NamedCallback, len(ps.pluginExecutionSampleUpdatedCallbacks))
	copy(tmp, ps.pluginExecutionSampleUpdatedCallbacks)

	ps.RUnlock()

	for _, callback := range tmp {
		go callback.Callback().(pipelines.PluginExecutionSampleUpdated)(pluginName, ps, kind)
		go callback.Callback().(pipelines.PluginExecutionSampleUpdated)(pluginName, ps,
			pipelines.AllStatistics)
	}

	err = func() error {
		ps.RLock()
		defer ps.RUnlock()

		// plugin name is valid if sample has been accessed successfully from map.
		switch kind {
		case pipelines.SuccessStatistics:
			ps.pluginSuccessThroughputRates1[pluginName].Update(1)
			ps.pluginSuccessThroughputRates5[pluginName].Update(1)
			ps.pluginSuccessThroughputRates15[pluginName].Update(1)
		case pipelines.FailureStatistics:
			ps.pluginFailureThroughputRates1[pluginName].Update(1)
			ps.pluginFailureThroughputRates5[pluginName].Update(1)
			ps.pluginFailureThroughputRates15[pluginName].Update(1)
		case pipelines.AllStatistics:
			fallthrough
		default:
			return fmt.Errorf("invalid plugin statistics kind %s", kind)
		}

		return nil
	}()

	if err != nil {
		return err
	}

	ps.RLock()

	tmp = make([]*common.NamedCallback, len(ps.pluginThroughputRateUpdatedCallbacks))
	copy(tmp, ps.pluginThroughputRateUpdatedCallbacks)

	ps.RUnlock()

	for _, callback := range tmp {
		go callback.Callback().(pipelines.PluginThroughputRateUpdated)(pluginName, ps, kind)
		go callback.Callback().(pipelines.PluginThroughputRateUpdated)(pluginName, ps,
			pipelines.AllStatistics)
	}

	return nil
}

func (ps *PipelineStatistics) updateTaskExecution(kind pipelines.StatisticsKind) error {
	switch kind {
	case pipelines.SuccessStatistics:
		atomic.AddUint64(&ps.taskSuccessCount, 1)
	case pipelines.FailureStatistics:
		atomic.AddUint64(&ps.taskFailureCount, 1)
	case pipelines.AllStatistics:
		fallthrough
	default:
		return fmt.Errorf("invalid task statistics kind %s", kind)
	}

	return nil
}
