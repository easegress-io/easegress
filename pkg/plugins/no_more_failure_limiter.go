package plugins

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/pipelines"
	"github.com/megaease/easegateway/pkg/task"
)

type noMoreFailureLimiterConfig struct {
	PluginCommonConfig
	FailureCountThreshold uint64 `json:"failure_count_threshold"` // up to 18446744073709551615

	// TODO: Supports multiple key and value pairs
	FailureTaskDataKey   string `json:"failure_task_data_key"`
	FailureTaskDataValue string `json:"failure_task_data_value"`
}

func noMoreFailureLimiterConfigConstructor() Config {
	return &noMoreFailureLimiterConfig{
		FailureCountThreshold: 1,
	}
}

func (c *noMoreFailureLimiterConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.FailureTaskDataKey, c.FailureTaskDataValue = ts(c.FailureTaskDataKey), ts(c.FailureTaskDataValue)

	if len(c.FailureTaskDataKey) == 0 {
		return fmt.Errorf("invalid failure task data key")
	}

	if c.FailureCountThreshold == 0 {
		logger.Warnf("ZERO failure count threshold has been applied, no request could be processed!")
	}

	return nil
}

////

type noMoreFailureLimiter struct {
	conf       *noMoreFailureLimiterConfig
	instanceId string
}

func noMoreFailureLimiterConstructor(conf Config) (Plugin, PluginType, bool, error) {
	c, ok := conf.(*noMoreFailureLimiterConfig)
	if !ok {
		return nil, ProcessPlugin, false, fmt.Errorf(
			"config type want *noMoreFailureLimiterConfig got %T", conf)
	}

	l := &noMoreFailureLimiter{
		conf: c,
	}

	l.instanceId = fmt.Sprintf("%p", l)

	return l, ProcessPlugin, false, nil
}

func (l *noMoreFailureLimiter) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (l *noMoreFailureLimiter) Run(ctx pipelines.PipelineContext, t task.Task) error {
	t.AddFinishedCallback(fmt.Sprintf("%s-calculateTaskFailure", l.Name()),
		getTaskFinishedCallbackInNoMoreFailureLimiter(ctx, l.conf.FailureTaskDataKey,
			l.conf.FailureTaskDataValue, l.Name(), l.instanceId))

	counter, err := getNoMoreFailureCounter(ctx, l.Name(), l.instanceId)
	if err != nil {
		return nil
	}

	if *counter >= l.conf.FailureCountThreshold {
		// TODO: Adds an option to allow operator provides a special output value as a parameter with task
		t.SetError(fmt.Errorf("service is unavailable caused by failure limitation"), task.ResultFlowControl)
		atomic.StoreUint64(counter, l.conf.FailureCountThreshold) // to prevent overflow
	}

	return nil
}

func (l *noMoreFailureLimiter) Name() string {
	return l.conf.PluginName()
}

func (l *noMoreFailureLimiter) CleanUp(ctx pipelines.PipelineContext) {
	ctx.DeleteBucket(l.Name(), l.instanceId)
}

func (l *noMoreFailureLimiter) Close() {
	// Nothing to do.
}

////

const (
	noMoreFailureLimiterCounterKey = "noMoreFailureLimiterCounterKey"
)

func getNoMoreFailureCounter(ctx pipelines.PipelineContext, pluginName, pluginInstanceId string) (*uint64, error) {
	bucket := ctx.DataBucket(pluginName, pluginInstanceId)
	counter, err := bucket.QueryDataWithBindDefault(noMoreFailureLimiterCounterKey,
		func() interface{} {
			var failureCount uint64
			return &failureCount
		})

	if err != nil {
		logger.Warnf("BUG: query failure counter for pipeline %s failed, "+
			"ignored to handle failure limitation: %v", ctx.PipelineName(), err)
		return nil, err
	}

	return counter.(*uint64), nil
}

func getTaskFinishedCallbackInNoMoreFailureLimiter(ctx pipelines.PipelineContext,
	failureTaskDataKey, failureTaskDataValue, pluginName, pluginInstanceId string) task.TaskFinished {

	return func(t1 task.Task, _ task.TaskStatus) {
		counter, err := getNoMoreFailureCounter(ctx, pluginName, pluginInstanceId)
		if err != nil {
			return
		}

		value := fmt.Sprintf("%v", t1.Value(failureTaskDataKey))
		if value == failureTaskDataValue {
			atomic.AddUint64(counter, 1)
		}

		return
	}
}
