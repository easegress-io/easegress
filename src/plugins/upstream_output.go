package plugins

import (
	"fmt"
	"strings"
	"task"
	"time"

	"common"
	"logger"
	"pipelines"
)

type routeSelector func(
	targetPipelines []string, targetWeights []uint16, ctx pipelines.PipelineContext, t task.Task) string

var routeSelectors = map[string]routeSelector{
	"round_robin":        roundRobinSelector,
	"weight_round_robin": weightRoundRobinSelector,
	"random":             randomSelector,
	"weight_random":      weightRandomSelector,
	"hashSelector":       hashSelector, // to support use cases for both http source address and header hash
	"least_wip_requests": leastWIPRequestsSelector,
	"sticky_session":     stickySessionSelector,
}

func roundRobinSelector(
	targetPipelines []string, targetWeights []uint16, ctx pipelines.PipelineContext, t task.Task) string {

	return "" // todo(zhiyan)
}

func weightRoundRobinSelector(
	targetPipelines []string, targetWeights []uint16, ctx pipelines.PipelineContext, t task.Task) string {

	return "" // todo(zhiyan)
}

func randomSelector(
	targetPipelines []string, targetWeights []uint16, ctx pipelines.PipelineContext, t task.Task) string {

	return "" // todo(zhiyan)
}

func weightRandomSelector(
	targetPipelines []string, targetWeights []uint16, ctx pipelines.PipelineContext, t task.Task) string {

	return "" // todo(zhiyan)
}

func hashSelector(
	targetPipelines []string, targetWeights []uint16, ctx pipelines.PipelineContext, t task.Task) string {

	return "" // todo(zhiyan)
}

func leastWIPRequestsSelector(
	targetPipelines []string, targetWeights []uint16, ctx pipelines.PipelineContext, t task.Task) string {

	return "" // todo(zhiyan)
}

func stickySessionSelector(
	targetPipelines []string, targetWeights []uint16, ctx pipelines.PipelineContext, t task.Task) string {

	return "" // todo(zhiyan)
}

////

type upstreamOutputConfig struct {
	CommonConfig
	TargetPipelineNames   []string `json:"target_pipelines"`
	TargetPipelineWeights []uint16 `json:"target_weights"` // up to 65535, 0 based
	RoutePolicy           string   `json:"route_policy"`
	TimeoutSec            uint16   `json:"timeout_sec"` // up to 65535, zero means no timeout

	RequestDataKeys   []string `json:"request_data_keys"`
	ValueHashedKey    string   `json:"value_hashed_key"`
	StickySessionKeys []string `json:"sticky_session_keys"`

	selector routeSelector
}

func UpstreamOutputConfigConstructor() Config {
	return &upstreamOutputConfig{
		RoutePolicy: "round_robin",
	}
}

func (c *upstreamOutputConfig) Prepare(pipelineNames []string) error {
	err := c.CommonConfig.Prepare(pipelineNames)
	if err != nil {
		return err
	}

	var policies []string
	for policy := range routeSelectors {
		policies = append(policies, policy)
	}

	if !common.StrInSlice(c.RoutePolicy, policies) {
		return fmt.Errorf("invalid route policy")
	}

	ts := strings.TrimSpace
	c.RoutePolicy = ts(c.RoutePolicy)
	c.ValueHashedKey = ts(c.ValueHashedKey)

	if len(c.TargetPipelineNames) == 0 {
		return fmt.Errorf("invalid target pipelines")
	}

	if len(c.TargetPipelineWeights) > 0 && len(c.TargetPipelineWeights) != len(c.TargetPipelineNames) {
		return fmt.Errorf("invalid target weights")
	}

	useDefaultWeight := len(c.TargetPipelineWeights) == 0

	for idx, pipelineName := range c.TargetPipelineNames {
		c.TargetPipelineNames[idx] = ts(pipelineName)

		if !common.StrInSlice(c.TargetPipelineNames[idx], pipelineNames) {
			return fmt.Errorf("invalid target pipeline")
		}

		if useDefaultWeight {
			c.TargetPipelineWeights = append(c.TargetPipelineWeights, 0)
		}
	}

	if c.TimeoutSec == 0 {
		logger.Warnf("[ZERO timeout has been applied, no task could be cancelled by timeout!]")
	}

	c.selector = routeSelectors[c.RoutePolicy]

	return nil
}

type upstreamOutput struct {
	conf *upstreamOutputConfig
}

func UpstreamOutputConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*upstreamOutputConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *upstreamOutputConfig got %T", conf)
	}

	return &upstreamOutput{
		conf: c,
	}, nil
}

func (u *upstreamOutput) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (u *upstreamOutput) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	targetPipelineName := u.conf.selector(u.conf.TargetPipelineNames, u.conf.TargetPipelineWeights, ctx, t)
	if len(strings.TrimSpace(targetPipelineName)) == 0 {
		logger.Errorf("[BUG: selecting target pipeline returns empty pipeline name]")
		return t, nil
	}

	data := make(map[interface{}]interface{})
	for _, key := range u.conf.RequestDataKeys {
		data[key] = t.Value(key)
	}

	request := pipelines.NewDownstreamRequest(targetPipelineName, u.Name(), data)

	done := make(chan struct{}, 0)

	go func() { // watch on task cancellation
		select {
		case <-t.Cancel():
			if common.CloseChan(done) {
				t.SetError(fmt.Errorf("task is cancelled by %s", t.CancelCause()),
					task.ResultTaskCancelled)
			}
		case <-done:
			// Nothing to do, to exit goroutine
		}
	}()

	timeout := time.Duration(u.conf.TimeoutSec) * time.Second
	if timeout > 0 {
		time.AfterFunc(timeout, func() {
			if common.CloseChan(done) {
				t.SetError(fmt.Errorf("upstream is timeout after %d second(s)", u.conf.TimeoutSec),
					task.ResultServiceUnavailable)
			}
		})
	}

	err := ctx.CommitCrossPipelineRequest(request, done)
	if err != nil && t.Error() != nil {
		// commit failed and error did not cause by task cancellation or request timeout
		t.SetError(err, task.ResultServiceUnavailable)
		return t, nil
	}

	select {
	case response := <-request.Response():
		if response == nil {
			logger.Errorf("[BUG: upstream pipeline %s returns nil response]",
				response.UpstreamPipelineName)
			t.SetError(fmt.Errorf("downstrewam received nil uptream response"),
				task.ResultInternalServerError)
			return t, nil
		}

		if response.UpstreamPipelineName != targetPipelineName {
			logger.Errorf("[BUG: upstream pipeline %s returns the response of "+
				"cross pipeline request to the wrong downstrewam %s]",
				response.UpstreamPipelineName, ctx.PipelineName())
			t.SetError(fmt.Errorf("downstrewam received wrong uptream response"),
				task.ResultInternalServerError)
			return t, nil
		}

		for k, v := range response.Data {
			t1, err := task.WithValue(t, k, v)
			if err != nil {
				t.SetError(err, task.ResultInternalServerError)
				return t, nil
			}

			t = t1
		}

		if response.TaskError != nil {
			t.SetError(response.TaskError, response.TaskResultCode)
			return t, nil
		}

		common.CloseChan(done)
	case <-done:
		// Nothing to do, task is cancelled or request runs timeout before get response from upstream
	}

	return t, nil
}

func (u *upstreamOutput) Name() string {
	return u.conf.PluginName()
}

func (u *upstreamOutput) Close() {
	// Nothing to do.
}
