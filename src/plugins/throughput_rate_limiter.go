package plugins

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"
	"golang.org/x/time/rate"

	"common"
	"logger"
)

type throughputRateLimiterConfig struct {
	common.PluginCommonConfig
	Tps         string `json:"tps,omitempty"`
	TimeoutMSec uint32 `json:"timeout_msec"` // up to 4294967295, zero means no queuing

	tps float64
}

func ThroughputRateLimiterConfigConstructor() plugins.Config {
	return &throughputRateLimiterConfig{
		TimeoutMSec: 200,
	}
}

func (c *throughputRateLimiterConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	ts := strings.TrimSpace
	c.Tps = ts(c.Tps)

	if len(c.Tps) == 0 {
		return fmt.Errorf("invalid throughput rate limit")
	}

	if c.TimeoutMSec == 0 {
		logger.Warnf("[ZERO timeout has been applied, no request could be queued by limiter!]")
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

func ThroughputRateLimiterConstructor(conf plugins.Config) (plugins.Plugin, error) {
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
	limiter, err := getThroughputRateLimiter(ctx, l.conf.tps, l.Name(), l.instanceId)
	if err != nil {
		return t, nil
	}

	if limiter == nil {
		t.SetError(fmt.Errorf("service is unavaialbe caused by throughput rate limit"), task.ResultFlowControl)
		return t, nil
	}

	if !limiter.Allow() {
		if l.conf.TimeoutMSec == 0 {
			t.SetError(fmt.Errorf("service is unavaialbe caused by throughput rate limit (without queuing)"),
				task.ResultFlowControl)
			return t, nil
		}

		pass := make(chan struct{})
		cancelCtx, cancel := context.WithTimeout(
			context.Background(), time.Duration(l.conf.TimeoutMSec)*time.Millisecond)

		go func() {
			select {
			case <-pass:
			case <-t.Cancel():
				cancel()
			}
		}()

		err = limiter.Wait(cancelCtx)
		if err != nil {
			switch err {
			case context.Canceled:
				if t.CancelCause() != nil { // task was cancelled
					t.SetError(fmt.Errorf("task is cancelled by %s", t.CancelCause()),
						task.ResultTaskCancelled)
				} else {
					logger.Warnf("[BUG: limiter context was canceled but task still running]")
				}
			default: // task queuing timeout
				// type of error is context.DeadlineExceeded or limiter predicts waiting would exceed context deadline
				t.SetError(fmt.Errorf("service is unavaialbe caused by throughput rate limit (queuing timeout)"),
					task.ResultFlowControl)
			}
		}

		close(pass)
	}

	return t, nil
}

func (l *throughputRateLimiter) Name() string {
	return l.conf.PluginName()
}

func (l *throughputRateLimiter) Close() {
	// Nothing to do.
}

////

const (
	throughputRateLimiterKey = "throughputRateLimiterKey"
)

func getThroughputRateLimiter(ctx pipelines.PipelineContext, tps float64,
	pluginName, pluginInstanceId string) (*rate.Limiter, error) {

	bucket := ctx.DataBucket(pluginName, pluginInstanceId)
	limiter, err := bucket.QueryDataWithBindDefault(throughputRateLimiterKey,
		func() interface{} {
			var limit rate.Limit
			if tps < 0 {
				limit = rate.Inf
			} else {
				limit = rate.Limit(tps)
			}

			var limiter *rate.Limiter

			if tps == 0 {
				logger.Warnf("[ZERO throughput rate limit has been applied, " +
					"no request could be processed!]")
			} else {
				limiter = rate.NewLimiter(limit, int(limit)+1)
			}

			return limiter
		})

	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to limit throughput rate: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return limiter.(*rate.Limiter), nil
}
