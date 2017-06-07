package plugins

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"logger"
	"pipelines"
	"task"
)

type throughputRateLimiterConfig struct {
	CommonConfig
	Tps string `json:"tps,omitempty"`

	tps float64
}

func ThroughputRateLimiterConfigConstructor() Config {
	return new(throughputRateLimiterConfig)
}

func (c *throughputRateLimiterConfig) Prepare(pipelineNames []string) error {
	err := c.CommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.Tps = ts(c.Tps)

	if len(c.Tps) == 0 {
		return fmt.Errorf("invalid throughput rate limit")
	}

	c.tps, err = strconv.ParseFloat(c.Tps, 64)
	if err != nil || c.tps < -1 { // -1 means infinite rate
		return fmt.Errorf("invalid throughput rate limit")
	}

	return nil
}

type throughputRateLimiter struct {
	conf       *throughputRateLimiterConfig
	instanceId string
}

func ThroughputRateLimiterConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*throughputRateLimiterConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *throughputRateLimiterConfig got %T", conf)
	}

	l := &throughputRateLimiter{
		conf: c,
	}

	l.instanceId = fmt.Sprintf("%p", l)

	return l, nil
}

func (l *throughputRateLimiter) Prepare(ctx pipelines.PipelineContext) {
	// Noting to do.
}

func (l *throughputRateLimiter) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	state, err := getThroughputRateLimiterStateData(ctx, l.conf.tps, l.Name(), l.instanceId)
	if err != nil {
		return t, nil
	}

	state.Lock()
	defer state.Unlock()

	if state.limiter == nil {
		t.SetError(fmt.Errorf("service is unavaialbe caused by throughput rate limit"), task.ResultFlowControl)
		return t, nil
	}

	if !state.limiter.Allow() {
		pass := make(chan struct{})

		go func() {
			select {
			case <-pass:
			case <-t.Cancel():
				state.cancelFunc()
			}
		}()

		err := state.limiter.Wait(state.ctx)
		if err != nil {
			// err returns if task was cancelled as well.
			if t.CancelCause() != nil {
				t.SetError(fmt.Errorf("task is cancelled by %s", t.CancelCause()),
					task.ResultTaskCancelled)
			} else {
				t.SetError(err, task.ResultInternalServerError)
			}
		}

		close(pass)
	}

	if t.ResultCode() == task.ResultTaskCancelled {
		return t, t.Error()
	} else {
		return t, nil
	}
}

func (l *throughputRateLimiter) Name() string {
	return l.conf.PluginName()
}

func (l *throughputRateLimiter) Close() {
	// Nothing to do.
}

////

const (
	throughputRateLimiterStateDataKey = "throughputRateLimiterStateDataKey"
)

type throughputRateLimiterStateData struct {
	sync.Mutex
	ctx        context.Context
	cancelFunc context.CancelFunc
	limiter    *rate.Limiter
}

func getThroughputRateLimiterStateData(ctx pipelines.PipelineContext, tps float64,
	pluginName, pluginInstanceId string) (*throughputRateLimiterStateData, error) {

	bucket := ctx.DataBucket(pluginName, pluginInstanceId)
	state, err := bucket.QueryDataWithBindDefault(throughputRateLimiterStateDataKey,
		func() interface{} {
			var limit rate.Limit
			if tps < 0 {
				limit = rate.Inf
			} else {
				limit = rate.Limit(tps)
			}

			cancelCtx, cancel := context.WithCancel(context.Background())
			var limiter *rate.Limiter

			if tps == 0 {
				logger.Warnf("[ZERO throughput rate limit has been applied, " +
					"no request could be processed!]")
			} else {
				limiter = rate.NewLimiter(limit, int(limit)+1)
			}

			return &throughputRateLimiterStateData{
				ctx:        cancelCtx,
				cancelFunc: cancel,
				limiter:    limiter,
			}
		})

	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to limit throughput rate: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return state.(*throughputRateLimiterStateData), nil
}
