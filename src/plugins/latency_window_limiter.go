package plugins

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"common"
	"logger"
	"pipelines"
	"task"
)

type latencyWindowLimiterConfig struct {
	CommonConfig
	PluginsConcerned     []string `json:"plugins_concerned"`
	LatencyThresholdMSec uint32   `json:"latency_threshold_msec"` // up to 4294967295
	BackOffMSec          uint16   `json:"backoff_msec"`           // up to 65535
	WindowSizeMax        uint64   `json:"window_size_max"`        // up to 18446744073709551615
	WindowSizeInit       uint64   `json:"windows_size_init"`
}

func LatencyWindowLimiterConfigConstructor() Config {
	return &latencyWindowLimiterConfig{
		LatencyThresholdMSec: 800,
		BackOffMSec:          100,
		WindowSizeMax:        65535,
		WindowSizeInit:       512,
	}
}

func (c *latencyWindowLimiterConfig) Prepare(pipelineNames []string) error {
	err := c.CommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	if len(c.PluginsConcerned) == 0 {
		return fmt.Errorf("invalid plugins concerned")
	}

	for _, pluginName := range c.PluginsConcerned {
		if len(strings.TrimSpace(pluginName)) == 0 {
			return fmt.Errorf("invalid plugin name")
		}
	}

	if c.LatencyThresholdMSec < 1 {
		return fmt.Errorf("invalid latency millisecond threshold")
	}

	if c.BackOffMSec > 10000 {
		return fmt.Errorf("invalid backoff millisecond (requires less than or equal to 10 seconds)")
	}

	if c.WindowSizeMax < 1 {
		return fmt.Errorf("invalid maximal windows size")
	}

	if c.WindowSizeInit < 1 || c.WindowSizeInit > c.WindowSizeMax {
		return fmt.Errorf("invalid initial windows size")
	}

	return nil
}

////

type latencyWindowLimiter struct {
	conf                               *latencyWindowLimiterConfig
	instanceId                         string
	executionSampleUpdatedCallbackName string
}

func LatencyWindowLimiterConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*latencyWindowLimiterConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *latencyWindowLimiterConfig got %T", conf)
	}

	l := &latencyWindowLimiter{
		conf: c,
	}

	l.instanceId = fmt.Sprintf("%p", l)
	l.executionSampleUpdatedCallbackName = fmt.Sprintf(
		"LatencyWindowLimiter-pluginExecutionSampleUpdatedForPluginInstance@%p", l)

	return l, nil
}

func (l *latencyWindowLimiter) Prepare(ctx pipelines.PipelineContext) {
	_, added := ctx.Statistics().AddPluginExecutionSampleUpdatedCallback(
		l.executionSampleUpdatedCallbackName,
		l.getPluginExecutionSampleUpdatedCallback(ctx),
		false)
	if added {
		ctx.Statistics().DeletePluginExecutionSampleUpdatedCallbackAfterPluginDelete(
			l.executionSampleUpdatedCallbackName, l.Name())
		ctx.Statistics().DeletePluginExecutionSampleUpdatedCallbackAfterPluginUpdate(
			l.executionSampleUpdatedCallbackName, l.Name())
	}
}

func (l *latencyWindowLimiter) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	t.AddFinishedCallback(fmt.Sprintf("%s-giveWindowBack", l.Name()),
		getTaskFinishedCallbackInLatencyWindowLimiter(ctx, l.conf.WindowSizeInit, l.Name(), l.instanceId))

	for {
		lock, err := getLatencyWindowLimiterWindowLock(ctx, l.Name(), l.instanceId)
		if err != nil {
			return t, nil
		}

		lock.Lock()

		window, err := getLatencyWindowLimiterWindow(ctx, l.conf.WindowSizeInit, l.Name(), l.instanceId)
		if err != nil {
			lock.Unlock()
			return t, nil
		}

		if window < 1 {
			lock.Unlock()
			if l.conf.BackOffMSec < 1 {
				// service fusing
				t.SetError(fmt.Errorf("service is unavaialbe caused by sliding window limit"),
					task.ResultFlowControl)
				return t, nil
			}

			select {
			case <-time.After(time.Duration(l.conf.BackOffMSec) * time.Millisecond):
			case <-t.Cancel():
				err := fmt.Errorf("task is cancelled by %s", t.CancelCause())
				t.SetError(err, task.ResultTaskCancelled)
				return t, err
			}

			continue
		}

		if window > l.conf.WindowSizeMax {
			window = l.conf.WindowSizeMax
		}

		window--

		setLatencyWindowLimiterWindow(ctx, window, l.Name(), l.instanceId)
		lock.Unlock()
		break
	}

	return t, nil
}

func (l *latencyWindowLimiter) Name() string {
	return l.conf.PluginName()
}

func (l *latencyWindowLimiter) Close() {
	// Nothing to do.
}

func (l *latencyWindowLimiter) getPluginExecutionSampleUpdatedCallback(
	ctx pipelines.PipelineContext) pipelines.PluginExecutionSampleUpdated {

	return func(pluginName string, latestStatistics pipelines.PipelineStatistics,
		kind pipelines.StatisticsKind) {

		if kind != pipelines.SuccessStatistics {
			return // only successful plugin execution can effect window
		}

		var latency float64

		for _, name := range l.conf.PluginsConcerned {
			if !common.StrInSlice(name, ctx.PluginNames()) {
				continue // ignore safely
			}

			rt, err := latestStatistics.PluginExecutionTimePercentile(
				name, pipelines.SuccessStatistics, 0.9) // value 90% is an option?
			if err != nil {
				logger.Warnf("[BUG: query plugin %s 90% execution time failed, "+
					"ignored to adjust sliding window: %v]", pluginName, err)
				return
			}

			if rt < 0 {
				continue // doesn't make sense, defensive
			}

			latency += rt
		}

		if latency == 0 {
			return
		}

		lock, err := getLatencyWindowLimiterWindowLock(ctx, l.Name(), l.instanceId)
		if err != nil {
			return
		}

		lock.Lock()
		defer lock.Unlock()

		window, err := getLatencyWindowLimiterWindow(ctx, l.conf.WindowSizeInit, l.Name(), l.instanceId)
		if err != nil {
			return
		}

		if window > l.conf.WindowSizeMax {
			window = l.conf.WindowSizeMax
		}

		if latency < float64(time.Duration(l.conf.LatencyThresholdMSec)*time.Millisecond) {
			if window < l.conf.WindowSizeMax {
				window++
			}
		} else {
			if window > 0 {
				window--
			}
		}

		setLatencyWindowLimiterWindow(ctx, window, l.Name(), l.instanceId)
	}
}

////

const (
	latencyWindowLimiterSlidingWindowKey     = "latencyWindowLimiterSlidingWindowKey"
	latencyWindowLimiterSlidingWindowLockKey = "latencyWindowLimiterSlidingWindowLockKey"
)

func getLatencyWindowLimiterWindow(ctx pipelines.PipelineContext, windowSizeInit uint64,
	pluginName, pluginInstanceId string) (uint64, error) {

	bucket := ctx.DataBucket(pluginName, pluginInstanceId)
	window, err := bucket.QueryDataWithBindDefault(latencyWindowLimiterSlidingWindowKey,
		func() interface{} { return windowSizeInit })
	if err != nil {
		logger.Warnf("[BUG: query sliding window for pipeline %s failed, "+
			"ignored to adjust sliding window: %v]", ctx.PipelineName(), err)
		return 0, err
	}

	return window.(uint64), nil
}

func setLatencyWindowLimiterWindow(ctx pipelines.PipelineContext, window uint64,
	pluginName, pluginInstanceId string) error {

	bucket := ctx.DataBucket(pluginName, pluginInstanceId)
	_, err := bucket.BindData(latencyWindowLimiterSlidingWindowKey, window)
	if err != nil {
		logger.Errorf("[BUG: save sliding window for pipeline %s failed, ignored but it is critical: %v]",
			ctx.PipelineName(), err)
	}

	return nil
}

func getLatencyWindowLimiterWindowLock(ctx pipelines.PipelineContext,
	pluginName, pluginInstanceId string) (*sync.Mutex, error) {

	bucket := ctx.DataBucket(pluginName, pluginInstanceId)
	lock, err := bucket.QueryDataWithBindDefault(latencyWindowLimiterSlidingWindowLockKey,
		func() interface{} { return &sync.Mutex{} })
	if err != nil {
		logger.Warnf("[BUG: query sliding window lock for pipeline %s failed, "+
			"ignored to adjust sliding window: %s]", ctx.PipelineName(), err)
		return nil, err
	}

	return lock.(*sync.Mutex), nil
}

func getTaskFinishedCallbackInLatencyWindowLimiter(ctx pipelines.PipelineContext, windowSizeInit uint64,
	pluginName, pluginInstanceId string) task.TaskFinished {

	return func(t1 task.Task, _ task.TaskStatus) {
		t1.DeleteFinishedCallback(fmt.Sprintf("%s-giveWindowBack", pluginName))

		if !task.SuccessfulResult(t1.ResultCode()) {
			return // only successful plugin execution can effect window
		}

		lock, err := getLatencyWindowLimiterWindowLock(ctx, pluginName, pluginInstanceId)
		if err != nil {
			return
		}

		lock.Lock()
		defer lock.Unlock()

		window, err := getLatencyWindowLimiterWindow(ctx, windowSizeInit, pluginName, pluginInstanceId)
		if err != nil {
			return
		}

		// Do not check maximal window size as limitation at the moment due to the
		// configuration could be updated after this callback was registered on the task.
		window++

		setLatencyWindowLimiterWindow(ctx, window, pluginName, pluginInstanceId)
	}
}
