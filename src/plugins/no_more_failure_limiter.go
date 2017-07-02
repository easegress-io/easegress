package plugins

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"logger"
)

type noMoreFailureLimiterConfig struct {
	CommonConfig
	FailureCountThreshold uint64 `json:"failure_count_threshold"` // up to 18446744073709551615

	// TODO: Supports multiple key and value pairs
	FailureTaskDataKey   string `json:"failure_task_data_key"`
	FailureTaskDataValue string `json:"failure_task_data_value"`
}

func NoMoreFailureLimiterConfigConstructor() plugins.Config {
	return &noMoreFailureLimiterConfig{
		FailureCountThreshold: 1,
	}
}

func (c *noMoreFailureLimiterConfig) Prepare(pipelineNames []string) error {
	err := c.CommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.FailureTaskDataKey, c.FailureTaskDataValue = ts(c.FailureTaskDataKey), ts(c.FailureTaskDataValue)

	if len(c.FailureTaskDataKey) == 0 {
		return fmt.Errorf("invalid failure task data key")
	}

	if c.FailureCountThreshold == 0 {
		logger.Warnf("[ZERO failure count threshold has been applied, no request could be processed!]")
	}

	return nil
}

////

type noMoreFailureLimiter struct {
	conf       *noMoreFailureLimiterConfig
	instanceId string
}

func NoMoreFailureLimiterConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*noMoreFailureLimiterConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *noMoreFailureLimiterConfig got %T", conf)
	}

	l := &noMoreFailureLimiter{
		conf: c,
	}

	l.instanceId = fmt.Sprintf("%p", l)

	return l, nil
}

func (l *noMoreFailureLimiter) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (l *noMoreFailureLimiter) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	t.AddFinishedCallback(fmt.Sprintf("%s-calculateTaskFailure", l.Name()),
		getTaskFinishedCallbackInNoMoreFailureLimiter(ctx, l.conf.FailureTaskDataKey,
			l.conf.FailureTaskDataValue, l.Name(), l.instanceId))

	state, err := getNoMoreFailureLimiterStateData(ctx, l.Name(), l.instanceId)
	if err != nil {
		return t, nil
	}

	state.Lock()
	defer state.Unlock()

	if state.failureCount >= l.conf.FailureCountThreshold {
		// TODO: Adds an option to allow operator provides a special output value as a parameter with task
		t.SetError(fmt.Errorf("service is unavaialbe caused by failure limitation"), task.ResultFlowControl)
		state.failureCount = l.conf.FailureCountThreshold // to prevent overflow
	}

	return t, nil
}

func (l *noMoreFailureLimiter) Name() string {
	return l.conf.PluginName()
}

func (l *noMoreFailureLimiter) Close() {
	// Nothing to do.
}

////

const (
	noMoreFailureLimiterStateKey = "noMoreFailureLimiterStateKey"
)

type noMoreFailureLimiterStateData struct {
	sync.Mutex
	failureCount uint64
}

func getNoMoreFailureLimiterStateData(ctx pipelines.PipelineContext,
	pluginName, pluginInstanceId string) (*noMoreFailureLimiterStateData, error) {

	bucket := ctx.DataBucket(pluginName, pluginInstanceId)
	state, err := bucket.QueryDataWithBindDefault(noMoreFailureLimiterStateKey,
		func() interface{} {
			return &noMoreFailureLimiterStateData{
				failureCount: 0,
			}
		})

	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to handle failure limitation: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return state.(*noMoreFailureLimiterStateData), nil
}

func getTaskFinishedCallbackInNoMoreFailureLimiter(ctx pipelines.PipelineContext,
	failureTaskDataKey, failureTaskDataValue, pluginName, pluginInstanceId string) task.TaskFinished {

	return func(t1 task.Task, _ task.TaskStatus) {
		t1.DeleteFinishedCallback(fmt.Sprintf("%s-calculateTaskFailure", pluginName))

		state, err := getNoMoreFailureLimiterStateData(ctx, pluginName, pluginInstanceId)
		if err != nil {
			return
		}

		state.Lock()
		defer state.Unlock()

		value := fmt.Sprintf("%v", t1.Value(failureTaskDataKey))
		if value == failureTaskDataValue {
			state.failureCount++
		}

		return
	}
}
