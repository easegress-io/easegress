package plugins

import (
	"fmt"
	"logger"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"common"
	"pipelines"
	"task"
)

type httpHeaderCounterConfig struct {
	CommonConfig
	HeaderConcerned string `json:"header_concerned"`
	ExpirationSec   uint32 `json:"expiration_sec"`
}

func HTTPHeaderCounterConfigConstructor() Config {
	return &httpHeaderCounterConfig{
		ExpirationSec: 60,
	}
}

func (c *httpHeaderCounterConfig) Prepare() error {
	err := c.CommonConfig.Prepare()
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

func HTTPHeaderCounterConstructor(conf Config) (Plugin, error) {
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
			count, err := getRecentHeaderCount(ctx, pluginName)
			if err != nil {
				return nil, err
			}
			return *count, nil
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

	state, err := getHttpHeaderCounterState(ctx, c.Name(), c.instanceId)
	if err != nil {
		return nil, t.ResultCode(), t
	}

	state.Lock()
	defer state.Unlock()

	timer, exists := state.timers[value]
	if !exists {
		count, err := getRecentHeaderCount(ctx, c.Name())
		if err != nil {
			return nil, t.ResultCode(), t
		}

		atomic.AddUint64(count, 1)

		state.timers[value] = time.AfterFunc(time.Duration(c.conf.ExpirationSec)*time.Second, func() {
			count1, err := getRecentHeaderCount(ctx, c.Name())
			if err != nil {
				return
			}

			for !atomic.CompareAndSwapUint64(count1, *count1, *count1-1) {
			}

			state1, err := getHttpHeaderCounterState(ctx, c.Name(), c.instanceId)
			if err != nil {
				return
			}

			state1.Lock()
			delete(state1.timers, value)
			state1.Unlock()
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
	httpHeaderCounterRecentCountKey = "httpHeaderCountKey"
	httpHeaderCounterStateKey       = "httpHeaderCounterStateKey"
)

type httpHeaderCounterTimerState struct {
	sync.Mutex
	timers map[string]*time.Timer
}

func getRecentHeaderCount(ctx pipelines.PipelineContext, pluginName string) (*uint64, error) {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	count, err := bucket.QueryDataWithBindDefault(httpHeaderCounterRecentCountKey,
		func() interface{} {
			var recentHeaderCount uint64
			return &recentHeaderCount
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to count header: %s]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*uint64), nil
}

func getHttpHeaderCounterState(ctx pipelines.PipelineContext, pluginName, instanceId string) (
	*httpHeaderCounterTimerState, error) {

	bucket := ctx.DataBucket(pluginName, instanceId)
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
