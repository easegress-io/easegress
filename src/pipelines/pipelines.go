package pipelines

import (
	"fmt"
	"strings"
	"sync"

	"task"
)

type Pipeline interface {
	Name() string
	Run() error
	Stop()
	Close()
}

////

const DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE = "*"

type PluginPreparationFunc func()

type PipelineContext interface {
	// PipelineName returns pipeline name
	PipelineName() string
	// PluginNames returns sequential plugin names
	PluginNames() []string
	// Parallelism returns number of parallelism
	Parallelism() uint16
	// Statistics returns pipeline statistics
	Statistics() PipelineStatistics
	// DataBucket returns(creates a new one if necessary) pipeline data bucket corresponding with plugin.
	// If the pluginInstanceId doesn't equal to DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE
	// (usually memory address of the instance), the data bucket will be deleted automatically
	// when closing the plugin instance. However if the pluginInstanceId equals to
	// DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE, which indicates all instances of a plugin share one data bucket,
	// the data bucket won't be deleted automatically until the plugin (not the plugin instance) is deleted.
	DataBucket(pluginName, pluginInstanceId string) PipelineContextDataBucket
	// DeleteBucket deletes a data bucket.
	DeleteBucket(pluginName, pluginInstanceId string) PipelineContextDataBucket
	// PreparePlugin is a hook method to invoke Prepare of plugin
	PreparePlugin(pluginName string, fun PluginPreparationFunc)
	// Downstream pipeline calls PushCrossPipelineRequest to commit a request
	CommitCrossPipelineRequest(request *DownstreamRequest, cancel <-chan struct{}) error
	// Upstream pipeline calls PopCrossPipelineRequest to claim a request
	ClaimCrossPipelineRequest() *DownstreamRequest
	// Upstream pipeline calls CrossPipelineWIPRequestsCount to make sure how many requests are waiting process
	CrossPipelineWIPRequestsCount(upstreamPipelineName string) int
	// Close closes a PipelineContext
	Close()
}

////

type DefaultValueFunc func() interface{}

type PipelineContextDataBucket interface {
	// BindData binds data, the type of key must be comparable
	BindData(key, value interface{}) (interface{}, error)
	// QueryData querys data, return nil if not found
	QueryData(key interface{}) interface{}
	// QueryDataWithBindDefault queries data with binding default data if not found, return final value
	QueryDataWithBindDefault(key interface{}, defaultValueFunc DefaultValueFunc) (interface{}, error)
	// UnbindData unbinds data
	UnbindData(key interface{}) interface{}
}

////

type DownstreamRequest struct {
	upstreamPipelineName, downstreamPipelineName string
	data                                         map[interface{}]interface{}
	responseChanLock                             sync.Mutex
	responseChan                                 chan *UpstreamResponse
}

func NewDownstreamRequest(upstreamPipelineName, downstreamPipelineName string,
	data map[interface{}]interface{}) *DownstreamRequest {

	ret := &DownstreamRequest{
		upstreamPipelineName:   upstreamPipelineName,
		downstreamPipelineName: downstreamPipelineName,
		data:         data,
		responseChan: make(chan *UpstreamResponse, 0),
	}

	return ret
}

func (r *DownstreamRequest) UpstreamPipelineName() string {
	return r.upstreamPipelineName
}

func (r *DownstreamRequest) DownstreamPipelineName() string {
	return r.downstreamPipelineName
}

func (r *DownstreamRequest) Respond(response *UpstreamResponse, cancel <-chan struct{}) bool {
	if r.responseChan == nil {
		return false
	}

	return func() (ret bool) {
		defer func() {
			// to prevent send on closed channel due to
			// Close() of the downstream request can be called concurrently
			ret = recover() == nil
		}()

		select {
		case r.responseChan <- response:
			ret = true
		case <-cancel:
			ret = false
		}

		return
	}()
}

func (r *DownstreamRequest) Response() <-chan *UpstreamResponse {
	return r.responseChan
}

func (r *DownstreamRequest) Close() {
	r.responseChanLock.Lock()
	defer r.responseChanLock.Unlock()

	if r.responseChan != nil {
		close(r.responseChan)
		r.responseChan = nil
	}
}

type UpstreamResponse struct {
	UpstreamPipelineName string
	Data                 map[interface{}]interface{}
	TaskError            error
	TaskResultCode       task.TaskResultCode
}

////

type StatisticsKind string

const (
	SuccessStatistics StatisticsKind = "SuccessStatistics"
	FailureStatistics StatisticsKind = "FailureStatistics"
	AllStatistics     StatisticsKind = "AllStatistics"

	STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE = "*"
)

type StatisticsIndicatorEvaluator func(name, indicatorName string) (interface{}, error)

type PipelineThroughputRateUpdated func(name string, latestStatistics PipelineStatistics)
type PipelineExecutionSampleUpdated func(name string, latestStatistics PipelineStatistics)
type PluginThroughputRateUpdated func(name string, latestStatistics PipelineStatistics, kind StatisticsKind)
type PluginExecutionSampleUpdated func(name string, latestStatistics PipelineStatistics, kind StatisticsKind)

type PipelineStatistics interface {
	PipelineThroughputRate1() (float64, error)
	PipelineThroughputRate5() (float64, error)
	PipelineThroughputRate15() (float64, error)
	PipelineExecutionCount() (int64, error)
	PipelineExecutionTimeMax() (int64, error)
	PipelineExecutionTimeMin() (int64, error)
	PipelineExecutionTimePercentile(percentile float64) (float64, error)
	PipelineExecutionTimeStdDev() (float64, error)
	PipelineExecutionTimeVariance() (float64, error)
	PipelineExecutionTimeSum() (int64, error)

	PluginThroughputRate1(pluginName string, kind StatisticsKind) (float64, error)
	PluginThroughputRate5(pluginName string, kind StatisticsKind) (float64, error)
	PluginThroughputRate15(pluginName string, kind StatisticsKind) (float64, error)
	PluginExecutionCount(pluginName string, kind StatisticsKind) (int64, error)
	PluginExecutionTimeMax(pluginName string, kind StatisticsKind) (int64, error)
	PluginExecutionTimeMin(pluginName string, kind StatisticsKind) (int64, error)
	PluginExecutionTimePercentile(
		pluginName string, kind StatisticsKind, percentile float64) (float64, error)
	PluginExecutionTimeStdDev(pluginName string, kind StatisticsKind) (float64, error)
	PluginExecutionTimeVariance(pluginName string, kind StatisticsKind) (float64, error)
	PluginExecutionTimeSum(pluginName string, kind StatisticsKind) (int64, error)

	TaskExecutionCount(kind StatisticsKind) (uint64, error)

	PipelineIndicatorNames() []string
	PipelineIndicatorValue(indicatorName string) (interface{}, error)
	PluginIndicatorNames(pluginName string) []string
	PluginIndicatorValue(pluginName, indicatorName string) (interface{}, error)
	TaskIndicatorNames() []string
	TaskIndicatorValue(indicatorName string) (interface{}, error)

	AddPipelineThroughputRateUpdatedCallback(name string, callback PipelineThroughputRateUpdated,
		overwrite bool) (PipelineThroughputRateUpdated, bool)
	DeletePipelineThroughputRateUpdatedCallback(name string)
	DeletePipelineThroughputRateUpdatedCallbackAfterPluginDelete(name string, pluginName string)
	DeletePipelineThroughputRateUpdatedCallbackAfterPluginUpdate(name string, pluginName string)
	AddPipelineExecutionSampleUpdatedCallback(name string, callback PipelineExecutionSampleUpdated,
		overwrite bool) (PipelineExecutionSampleUpdated, bool)
	DeletePipelineExecutionSampleUpdatedCallback(name string)
	DeletePipelineExecutionSampleUpdatedCallbackAfterPluginDelete(name string, pluginName string)
	DeletePipelineExecutionSampleUpdatedCallbackAfterPluginUpdate(name string, pluginName string)
	AddPluginThroughputRateUpdatedCallback(name string, callback PluginThroughputRateUpdated,
		overwrite bool) (PluginThroughputRateUpdated, bool)
	DeletePluginThroughputRateUpdatedCallback(name string)
	DeletePluginThroughputRateUpdatedCallbackAfterPluginDelete(name string, pluginName string)
	DeletePluginThroughputRateUpdatedCallbackAfterPluginUpdate(name string, pluginName string)
	AddPluginExecutionSampleUpdatedCallback(name string, callback PluginExecutionSampleUpdated,
		overwrite bool) (PluginExecutionSampleUpdated, bool)
	DeletePluginExecutionSampleUpdatedCallback(name string)
	DeletePluginExecutionSampleUpdatedCallbackAfterPluginDelete(name string, pluginName string)
	DeletePluginExecutionSampleUpdatedCallbackAfterPluginUpdate(name string, pluginName string)

	RegisterPluginIndicator(pluginName, pluginInstanceId, indicatorName, desc string,
		evaluator StatisticsIndicatorEvaluator) (bool, error)
	UnregisterPluginIndicator(pluginName, pluginInstanceId, indicatorName string)
	UnregisterPluginIndicatorAfterPluginDelete(pluginName, pluginInstanceId, indicatorName string)
	UnregisterPluginIndicatorAfterPluginUpdate(pluginName, pluginInstanceId, indicatorName string)
}

////

type Config interface {
	PipelineName() string
	PluginNames() []string
	Parallelism() uint16
	CrossPipelineRequestBacklog() uint16
	Prepare() error
}

////

type CommonConfig struct {
	Name                              string   `json:"pipeline_name"`
	Plugins                           []string `json:"plugin_names"`
	ParallelismCount                  uint16   `json:"parallelism"`                    // up to 65535
	CrossPipelineRequestBacklogLength uint16   `json:"cross_pipeline_request_backlog"` // up to 65535
}

func (c *CommonConfig) PipelineName() string {
	return c.Name
}

func (c *CommonConfig) PluginNames() []string {
	return c.Plugins
}

func (c *CommonConfig) Parallelism() uint16 {
	return c.ParallelismCount
}

func (c *CommonConfig) CrossPipelineRequestBacklog() uint16 {
	return c.CrossPipelineRequestBacklogLength
}

func (c *CommonConfig) Prepare() error {
	if len(strings.TrimSpace(c.PipelineName())) == 0 {
		return fmt.Errorf("invalid pipeline name")
	}

	if len(c.PluginNames()) == 0 {
		return fmt.Errorf("pipeline is empty")
	}

	if c.Parallelism() < 1 {
		return fmt.Errorf("invalid pipeline parallelism")
	}

	return nil
}

// Pipelines register authority

var (
	PIPELINE_TYPES = map[string]interface{}{
		"LinearPipeline": nil,
	}
)

func ValidType(t string) bool {
	_, exists := PIPELINE_TYPES[t]
	return exists
}

func GetAllTypes() []string {
	types := make([]string, 0)
	for t := range PIPELINE_TYPES {
		types = append(types, t)
	}
	return types
}
