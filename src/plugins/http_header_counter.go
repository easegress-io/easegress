package plugins

import (
	"fmt"
	"logger"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
)

type httpHeaderCounterConfig struct {
	CommonConfig
	HeaderConcerned string `json:"header_concerned"`
	ExpirationSec   uint32 `json:"expiration_sec"`
}

func HTTPHeaderCounterConfigConstructor() plugins.Config {
	return &httpHeaderCounterConfig{
		ExpirationSec: 60,
	}
}

func (c *httpHeaderCounterConfig) Prepare(pipelineNames []string) error {
	err := c.CommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	if c.ExpirationSec == 0 {
		return fmt.Errorf("invalid expiration seconds")
	}

	ts := strings.TrimSpace
	c.HeaderConcerned = ts(c.HeaderConcerned)

	if len(c.HeaderConcerned) == 0 {
		return fmt.Errorf("invalid header concerned")
	}

	return nil
}

////

type httpHeaderCounter struct {
	conf       *httpHeaderCounterConfig
	instanceId string
}

func HTTPHeaderCounterConstructor(conf plugins.Config) (plugins.Plugin, error) {
	c, ok := conf.(*httpHeaderCounterConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *httpHeaderCounterConfig got %T", conf)
	}

	counter := &httpHeaderCounter{
		conf: c,
	}

	counter.instanceId = fmt.Sprintf("%p", counter)

	return counter, nil
}

func (c *httpHeaderCounter) Prepare(ctx pipelines.PipelineContext) {
	added, err := ctx.Statistics().RegisterPluginIndicator(
		c.Name(), c.instanceId, "RECENT_HEADER_COUNT",
		fmt.Sprintf("The count of http requests that the header of each one "+
			"contains a key '%s' in last %d second(s).", c.conf.HeaderConcerned, c.conf.ExpirationSec),
		func(pluginName, indicatorName string) (interface{}, error) {
			state, err := getHTTPHeaderCounterState(ctx, pluginName)
			if err != nil {
				return nil, err
			}
			return state.count, nil
		},
	)
	if err != nil {
		logger.Warnf("[BUG: register plugin %s indicator %s failed, "+
			"ignored to expose customized statistics indicator: %v]", c.Name(), "RECENT_HEADER_COUNT", err)
	} else if added {
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginDelete(
			c.Name(), c.instanceId, "RECENT_HEADER_COUNT",
		)
		ctx.Statistics().UnregisterPluginIndicatorAfterPluginUpdate(
			c.Name(), c.instanceId, "RECENT_HEADER_COUNT")
	}
}

func (c *httpHeaderCounter) count(ctx pipelines.PipelineContext, t task.Task) (error, task.TaskResultCode, task.Task) {
	headerValue := t.Value("HTTP_" + common.UpperCaseAndUnderscore(c.conf.HeaderConcerned))
	value, ok := headerValue.(string)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", c.conf.HeaderConcerned, headerValue),
			task.ResultMissingInput, t
	}

	state, err := getHTTPHeaderCounterState(ctx, c.Name())
	if err != nil {
		return nil, t.ResultCode(), t
	}

	state.Lock()
	defer state.Unlock()

	timer, exists := state.timers[value]
	if !exists {
		atomic.AddUint64(&state.count, 1)

		state.timers[value] = time.AfterFunc(time.Duration(c.conf.ExpirationSec)*time.Second, func() {
			state, err := getHTTPHeaderCounterState(ctx, c.Name())
			if err != nil {
				return
			}

			// substitute for illegal atomic.AddUint64(&state.count, -1)
			// whose second parameter is negative.
			for !atomic.CompareAndSwapUint64(&state.count, state.count, state.count-1) {
			}

			state.Lock()
			delete(state.timers, value)
			state.Unlock()
		})
	} else {
		timer.Reset(time.Duration(c.conf.ExpirationSec) * time.Second)
	}

	return nil, t.ResultCode(), t
}

func (c *httpHeaderCounter) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	err, resultCode, t := c.count(ctx, t)
	if err != nil {
		t.SetError(err, resultCode)
	}
	return t, nil
}

func (c *httpHeaderCounter) Name() string {
	return c.conf.Name
}

func (c *httpHeaderCounter) Close() {
	// Nothing to do.
}

////

const (
	httpHeaderCounterStateKey = "httpHeaderCounterStateKey"
)

type httpHeaderCounterTimerState struct {
	// count caches len(timers) and needs not be protected by sync.Mutex.
	count uint64

	sync.Mutex
	timers map[string]*time.Timer
}

func getHTTPHeaderCounterState(ctx pipelines.PipelineContext, pluginName string) (
	*httpHeaderCounterTimerState, error) {

	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	state, err := bucket.QueryDataWithBindDefault(httpHeaderCounterStateKey,
		func() interface{} {
			return &httpHeaderCounterTimerState{
				timers: make(map[string]*time.Timer),
			}
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to handle header counter timer: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return state.(*httpHeaderCounterTimerState), nil
}
