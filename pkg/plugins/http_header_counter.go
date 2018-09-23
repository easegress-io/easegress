package plugins

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hexdecteam/easegateway/pkg/common"
	"github.com/hexdecteam/easegateway/pkg/logger"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"
)

type httpHeaderCounterConfig struct {
	common.PluginCommonConfig
	HeaderConcerned string `json:"header_concerned"`
	ExpirationSec   uint32 `json:"expiration_sec"`
}

func httpHeaderCounterConfigConstructor() plugins.Config {
	return &httpHeaderCounterConfig{
		ExpirationSec: 60,
	}
}

func (c *httpHeaderCounterConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
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
	conf           *httpHeaderCounterConfig
	instanceId     string
	indicatorAdded bool
}

func httpHeaderCounterConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, bool, error) {
	c, ok := conf.(*httpHeaderCounterConfig)
	if !ok {
		return nil, plugins.ProcessPlugin, false, fmt.Errorf(
			"config type want *httpHeaderCounterConfig got %T", conf)
	}

	counter := &httpHeaderCounter{
		conf: c,
	}

	counter.instanceId = fmt.Sprintf("%p", counter)

	return counter, plugins.ProcessPlugin, false, nil
}

func (c *httpHeaderCounter) Prepare(ctx pipelines.PipelineContext) {
	added, err := ctx.Statistics().RegisterPluginIndicator(
		c.Name(), c.instanceId, "RECENT_HEADER_COUNT",
		fmt.Sprintf("The count of http requests that the header of each one "+
			"contains a key '%s' in last %d second(s).", c.conf.HeaderConcerned, c.conf.ExpirationSec),
		func(pluginName, indicatorName string) (interface{}, error) {
			state, err := getHTTPHeaderCounterState(ctx, c.Name(), c.instanceId)
			if err != nil {
				return nil, err
			}
			// without locking to accelerate read
			return len(state.timers), nil
		},
	)
	if err != nil {
		logger.Warnf("[BUG: register plugin %s indicator %s failed, "+
			"ignored to expose customized statistics indicator: %v]", c.Name(), "RECENT_HEADER_COUNT", err)
	}

	c.indicatorAdded = added
}

func (c *httpHeaderCounter) count(ctx pipelines.PipelineContext, t task.Task) (error, task.TaskResultCode, task.Task) {
	headerValue := t.Value("HTTP_" + common.UpperCaseAndUnderscore(c.conf.HeaderConcerned))
	value, ok := headerValue.(string)
	if !ok {
		return fmt.Errorf("input %s got wrong value: %#v", c.conf.HeaderConcerned, headerValue),
			task.ResultMissingInput, t
	}

	state, err := getHTTPHeaderCounterState(ctx, c.Name(), c.instanceId)
	if err != nil {
		return nil, t.ResultCode(), t
	}

	state.Lock()
	defer state.Unlock()

	book, exists := state.timers[value]
	if !exists {
		timer := time.AfterFunc(time.Duration(c.conf.ExpirationSec)*time.Second, func() {
			state, err := getHTTPHeaderCounterState(ctx, c.Name(), c.instanceId)
			if err != nil {
				return
			}

			state.Lock()
			defer state.Unlock()

			book := state.timers[value]
			if book.recycled {
				book.recycled = false // reset for next handling
				return
			}

			delete(state.timers, value)
		})

		state.timers[value] = &httpHeaderCounterTimerBook{
			timer:    timer,
			recycled: false,
		}
	} else {
		expired := !book.timer.Stop()
		if expired { // timeout callback is triggered concurrently
			book.recycled = true
		}

		book.timer.Reset(time.Duration(c.conf.ExpirationSec) * time.Second)
	}

	return nil, t.ResultCode(), t
}

func (c *httpHeaderCounter) Run(ctx pipelines.PipelineContext, t task.Task) error {
	err, resultCode, t := c.count(ctx, t)
	if err != nil {
		t.SetError(err, resultCode)
	}

	return nil
}

func (c *httpHeaderCounter) Name() string {
	return c.conf.Name
}

func (c *httpHeaderCounter) CleanUp(ctx pipelines.PipelineContext) {
	if c.indicatorAdded {
		ctx.Statistics().UnregisterPluginIndicator(c.Name(), c.instanceId, "RECENT_HEADER_COUNT")
	}

	ctx.DeleteBucket(c.Name(), c.instanceId)
}

func (c *httpHeaderCounter) Close() {
	// Nothing to do.
}

////

const httpHeaderCounterStateKey = "httpHeaderCounterStateKey"

type httpHeaderCounterTimerBook struct {
	timer    *time.Timer
	recycled bool
}
type httpHeaderCounterTimerState struct {
	sync.Mutex
	timers map[string]*httpHeaderCounterTimerBook
}

func getHTTPHeaderCounterState(ctx pipelines.PipelineContext, pluginName, instanceId string) (
	*httpHeaderCounterTimerState, error) {

	bucket := ctx.DataBucket(pluginName, instanceId)
	state, err := bucket.QueryDataWithBindDefault(httpHeaderCounterStateKey,
		func() interface{} {
			return &httpHeaderCounterTimerState{
				timers: make(map[string]*httpHeaderCounterTimerBook),
			}
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to handle header counter timer: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return state.(*httpHeaderCounterTimerState), nil
}
