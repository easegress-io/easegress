package plugins

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/hexdecteam/easegateway/pkg/common"
	"github.com/hexdecteam/easegateway/pkg/logger"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"
)

type latencyLimiterConfig struct {
	common.PluginCommonConfig
	AllowMSec                uint16   `json:"allow_msec"`           // up to 65535
	BackOffTimeoutMSec       int16    `json:"backoff_timeout_msec"` // zero means no queuing, -1 means no timeout
	FlowControlPercentageKey string   `json:"flow_control_percentage_key"`
	LatencyThresholdMSec     uint32   `json:"latency_threshold_msec"` // up to 4294967295
	PluginsConcerned         []string `json:"plugins_concerned"`
	ProbePercentage          uint8    `json:"probe_percentage"` // [1~99]
}

func latencyLimiterConfigConstructor() plugins.Config {
	return &latencyLimiterConfig{
		LatencyThresholdMSec: 800,
		BackOffTimeoutMSec:   1000,
		AllowMSec:            1000,
		ProbePercentage:      10,
	}
}

func (c *latencyLimiterConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
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

	if c.BackOffTimeoutMSec < -1 {
		return fmt.Errorf("invalid queuing timeout, must be >= -1")
	} else if c.BackOffTimeoutMSec == -1 {
		logger.Warnf("[INFINITE timeout of latency limit has been applied, " +
			"no request could be timed out from back off!]")
	} else if c.BackOffTimeoutMSec == 0 {
		logger.Warnf("[ZERO timeout of latency limit has been applied, " +
			"no request could be backed off by limiter!]")
	} else if c.BackOffTimeoutMSec > 10000 {
		return fmt.Errorf("invalid backoff timeout millisecond (requires less than or equal to 10 seconds)")
	}

	if c.ProbePercentage >= 100 || c.ProbePercentage < 1 {
		return fmt.Errorf("invalid probe percentage (requires bigger than zero and less than 100)")
	}
	c.FlowControlPercentageKey = strings.TrimSpace(c.FlowControlPercentageKey)

	return nil
}

////

type latencyWindowLimiter struct {
	conf       *latencyLimiterConfig
	instanceId string
}

func latencyLimiterConstructor(conf plugins.Config) (plugins.Plugin, plugins.PluginType, bool, error) {
	c, ok := conf.(*latencyLimiterConfig)
	if !ok {
		return nil, plugins.ProcessPlugin, false, fmt.Errorf(
			"config type want *latencyWindowLimiterConfig got %T", conf)
	}

	l := &latencyWindowLimiter{
		conf: c,
	}
	l.instanceId = fmt.Sprintf("%p", l)

	return l, plugins.ProcessPlugin, false, nil
}

func (l *latencyWindowLimiter) Prepare(ctx pipelines.PipelineContext) {
	// Register as plugin level indicator, so we don't need to unregister them in CleanUp()
	registerPluginIndicatorForLimiter(ctx, l.Name(), pipelines.STATISTICS_INDICATOR_FOR_ALL_PLUGIN_INSTANCE)
}

// Probe: don't totally fuse outbound requests because we need small amount of requests to probe the concerned target
func (l *latencyWindowLimiter) isProbe(outboundRate float64, inboundRate float64) bool {
	curPercentage := 100 * outboundRate / inboundRate

	// outbound rate is big enough compares to outboundRate
	if curPercentage > float64(l.conf.ProbePercentage) && outboundRate >= 10 {
		return false
	}

	if inboundRate < 10 || // inboundRate is too small so we needs all requests to be probes
		rand.Int31n(100) < int32(l.conf.ProbePercentage) {
		return true
	}
	return false
}

func (l *latencyWindowLimiter) Run(ctx pipelines.PipelineContext, t task.Task) error {
	t.AddFinishedCallback(fmt.Sprintf("%s-checkLatency", l.Name()),
		getTaskFinishedCallbackInLatencyLimiter(ctx, l.conf, l.Name(), l.instanceId))

	go updateInboundThroughputRate(ctx, l.Name()) // ignore error if it occurs

	counter, err := getLatencyLimiterCounter(ctx, l.Name(), l.instanceId, l.conf.AllowMSec)
	if err != nil {
		return nil
	}

	r, err := getInboundThroughputRate1(ctx, l.Name())
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to limit request: %v]", ctx.PipelineName(), err)
		return nil
	}

	outboundRate, err := ctx.Statistics().PluginThroughputRate1(l.Name(), pipelines.AllStatistics)
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to limit request: %v]", ctx.PipelineName(), err)
	}

	inboundRate, _ := r.Get() // ignore error safely
	// use l.conf.AllowMSec to avoid thrashing caused by network, upstream server gc or other factors
	counterThreshold := uint64(float64(l.conf.AllowMSec) / 1000.0 * outboundRate)
	count := counter.Count()
	logger.Debugf("[inboundRate: %.3f, outboundRate: %.3f, counter: %d, counterThreshold: %d]", inboundRate, outboundRate, counter.Count(), counterThreshold)
	if count > counterThreshold { // needs flow control
		go updateFlowControlledThroughputRate(ctx, l.Name())

		if !l.isProbe(outboundRate, inboundRate) {
			if l.conf.BackOffTimeoutMSec == 0 { // don't back off
				// service fusing
				t.SetError(fmt.Errorf("service is unavailable caused by latency limit"),
					task.ResultFlowControl)
				return nil
			}
			if retVal, needReturn := backOff(counter, l.conf.BackOffTimeoutMSec, counterThreshold, t); needReturn {
				return retVal
			}
		}
	}

	if len(l.conf.FlowControlPercentageKey) != 0 {
		percentage, err := getFlowControlledPercentage(ctx, l.Name())
		if err != nil {
			logger.Warnf("[BUG: query flow control percentage data for pipeline %s failed: %v, "+
				"ignored this output]", ctx.PipelineName(), err)
		} else {
			t.WithValue(l.conf.FlowControlPercentageKey, percentage)
		}
	}

	return nil
}

func (l *latencyWindowLimiter) Name() string {
	return l.conf.PluginName()
}

func (l *latencyWindowLimiter) CleanUp(ctx pipelines.PipelineContext) {
	ctx.DeleteBucket(l.Name(), l.instanceId)
}

func (l *latencyWindowLimiter) Close() {
	// Nothing to do.
}

////
// wait until timeout, cancel or latency recoveryed
func backOff(counter *latencyLimiterCounter, backOffTimeoutMSec int16, counterThreshold uint64, t task.Task) (error, bool) {
	var backOffTimeout <-chan time.Time
	if backOffTimeoutMSec != -1 {
		timer := time.NewTimer(time.Duration(backOffTimeoutMSec) * time.Millisecond)
		backOffTimeout = timer.C
		defer timer.Stop()
	}

	var backOffStep int
	if int(backOffTimeoutMSec) <= backOffStep {
		backOffStep = 1
	} else {
		backOffStep = int(backOffTimeoutMSec / 10)
	}

	backOffTicker := time.NewTicker(time.Duration(backOffStep) * time.Millisecond)
	defer backOffTicker.Stop()

	for {
		select {
		case <-backOffTimeout: // receive on a nil channel will always block
			t.SetError(fmt.Errorf("service is unavailable caused by latency limit backoff timeout"),
				task.ResultFlowControl)
			return nil, true
		case <-backOffTicker.C:
			if counter.Count() < counterThreshold { // FIXME(shengdong): counterThreshold needs re-calculate
				logger.Debugf("[successfully passed latency limiter after backed off]")
				return nil, false
			}
		case <-t.Cancel():
			err := fmt.Errorf("task is cancelled by %s", t.CancelCause())
			t.SetError(err, task.ResultTaskCancelled)
			return t.Error(), true
		}
	}
}

////

const (
	latencyLimiterCounterKey = "latencyLimiterCounter"
)

// latencyLimiterCounter count the number of requests that reached the latency limiter
// threshold. It is increased when the request reached the lantency threshold and decreased when
// below.
//
// The maximum counter will be math.max(1, maxCountMSec/1000.0 * outBoundThroughputRate1)
type latencyLimiterCounter struct {
	c       chan *bool
	counter uint64
	closed  bool
}

func newLatencyLimiterCounter(ctx pipelines.PipelineContext, pluginName string, maxCountMSec uint16) *latencyLimiterCounter {
	ret := &latencyLimiterCounter{
		c: make(chan *bool, 32767),
	}

	go func() {
		for {
			select {
			case f := <-ret.c:
				if f == nil {
					return // channel/counter closed, exit
				} else if *f { // increase
					if outboundRate, err := ctx.Statistics().PluginThroughputRate1(pluginName, pipelines.AllStatistics); err == nil {
						max := uint64(outboundRate*float64(maxCountMSec)/1000.0 + 0.5)
						if max == 0 {
							max = 1
						}
						logger.Debugf("[increase counter: %d, counter max: %d, outboundRate: %.1f]", ret.counter, max, outboundRate)
						ret.counter += 1
						if ret.counter > max {
							ret.counter = max
						}
					}
				} else if ret.counter > 0 { // decrease
					ret.counter = ret.counter / 2 // fast recovery
				}
			}
		}
	}()

	return ret
}

func (c *latencyLimiterCounter) Increase() {
	if !c.closed {
		f := true
		c.c <- &f
	}
}

func (c *latencyLimiterCounter) Decrease() {
	if !c.closed {
		f := false
		c.c <- &f
	}
}

func (c *latencyLimiterCounter) Count() uint64 {
	if c.closed {
		return 0
	}

	for len(c.c) > 0 {
		logger.Debugf("[spin to wait counter is updated completely]")
		time.Sleep(time.Millisecond)
	}

	return c.counter
}

func (c *latencyLimiterCounter) Close() error { // io.Closer stub
	c.closed = true
	close(c.c)
	return nil
}

func getLatencyLimiterCounter(ctx pipelines.PipelineContext, pluginName, instanceId string, allowMSec uint16) (*latencyLimiterCounter, error) {
	bucket := ctx.DataBucket(pluginName, instanceId)
	counter, err := bucket.QueryDataWithBindDefault(latencyLimiterCounterKey,
		func() interface{} {
			return newLatencyLimiterCounter(ctx, pluginName, 2*allowMSec) // maxCountMSec may needs tuned
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to limit request: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return counter.(*latencyLimiterCounter), nil
}

func getTaskFinishedCallbackInLatencyLimiter(ctx pipelines.PipelineContext,
	conf *latencyLimiterConfig, pluginName, instanceId string) task.TaskFinished {

	return func(t1 task.Task, _ task.TaskStatus) {
		var latency float64
		var found bool
		latencyThreshold := float64(time.Duration(conf.LatencyThresholdMSec) * time.Millisecond)
		for _, name := range conf.PluginsConcerned {
			if !common.StrInSlice(name, ctx.PluginNames()) {
				continue // ignore safely
			}

			rt, err := ctx.Statistics().PluginExecutionTimePercentile(
				name, pipelines.AllStatistics, 0.9) // value 90% is an option?
			if err != nil {
				logger.Warnf("[BUG: query plugin %s 90%% execution time failed, "+
					"ignored to adjust exceptional latency counter: %v]", pluginName, err)
				return
			}
			logger.Debugf("[concerned plugin %s latency: %.1f, latencyThreshold:%.1f]", name, rt, latencyThreshold)
			if rt < 0 {
				continue // doesn't make sense, defensive
			}

			latency += rt
			found = true
		}

		if !found {
			return
		}

		counter, err := getLatencyLimiterCounter(ctx, pluginName, instanceId, conf.AllowMSec)
		if err != nil { // ignore error safely
			return
		}

		if latency < latencyThreshold {
			counter.Decrease()
		} else {
			counter.Increase()
		}
	}
}
