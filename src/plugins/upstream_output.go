package plugins

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"common"
	"logger"
	"pipelines"
	"task"
)

type routeSelector func(targetPipelines []string, targetWeights []uint16,
	ctx pipelines.PipelineContext, pluginName, instanceId string, t task.Task) string

var routeSelectors = map[string]routeSelector{
	"round_robin":        roundRobinSelector,
	"weight_round_robin": weightRoundRobinSelector,
	"random":             randomSelector,
	"weight_random":      WeightRandomSelector,
	"hashSelector":       hashSelector, // to support use cases for both http source address and header hash
	"least_wip_requests": leastWIPRequestsSelector,
	"sticky_session":     stickySessionSelector,
}

func roundRobinSelector(targetPipelines []string, targetWeights []uint16,
	ctx pipelines.PipelineContext, pluginName, instanceId string, t task.Task) string {

	taskCount, err := getTaskCount(ctx, pluginName)
	if err != nil {
		return randomSelector(targetPipelines, targetWeights, ctx, pluginName, instanceId, t)
	}

	atomic.AddUint64(taskCount, 1)

	return targetPipelines[*taskCount%uint64(len(targetPipelines))]
}

func weightRoundRobinSelector(targetPipelines []string, targetWeights []uint16,
	ctx pipelines.PipelineContext, pluginName, instanceId string, t task.Task) string {

	state, err := getWeightRoundRobinSelectorState(ctx, pluginName, instanceId, targetWeights)
	if err != nil {
		return randomSelector(targetPipelines, targetWeights, ctx, pluginName, instanceId, t)
	}

	state.Lock()
	defer state.Unlock()

	for {
		state.lastPipelineIndex = (state.lastPipelineIndex + 1) % len(targetPipelines)
		if state.lastPipelineIndex == 0 {
			state.lastWeight -= state.weightGCD
			if state.lastWeight <= 0 {
				state.lastWeight = state.maxWeight
			}
		}

		weight := targetWeights[state.lastPipelineIndex]
		if weight >= state.lastWeight {
			return targetPipelines[state.lastPipelineIndex]
		}
	}
}

func randomSelector(targetPipelines []string, targetWeights []uint16,
	ctx pipelines.PipelineContext, pluginName, instanceId string, t task.Task) string {

	return targetPipelines[rand.Intn(len(targetPipelines))]
}

func WeightRandomSelector(targetPipelines []string, targetWeights []uint16,
	ctx pipelines.PipelineContext, pluginName, instanceId string, t task.Task) string {

	return "" // todo(zhiyan)
}

func hashSelector(targetPipelines []string, targetWeights []uint16,
	ctx pipelines.PipelineContext, pluginName, instanceId string, t task.Task) string {

	return "" // todo(zhiyan)
}

func leastWIPRequestsSelector(targetPipelines []string, targetWeights []uint16,
	ctx pipelines.PipelineContext, pluginName, instanceId string, t task.Task) string {

	return "" // todo(zhiyan)
}

func stickySessionSelector(targetPipelines []string, targetWeights []uint16,
	ctx pipelines.PipelineContext, pluginName, instanceId string, t task.Task) string {

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
		return fmt.Errorf("invalid target Weightts")
	}

	useDefaultWeight := len(c.TargetPipelineWeights) == 0

	for idx, pipelineName := range c.TargetPipelineNames {
		c.TargetPipelineNames[idx] = ts(pipelineName)

		if !common.StrInSlice(c.TargetPipelineNames[idx], pipelineNames) {
			return fmt.Errorf("invalid target pipeline")
		}

		if useDefaultWeight {
			c.TargetPipelineWeights = append(c.TargetPipelineWeights, 1)
		}
	}

	if !useDefaultWeight && c.RoutePolicy == "weight_round_robin" {
		var weightGCD uint16
		for i := 0; i < len(c.TargetPipelineWeights); i++ {
			weightGCD = gcd(weightGCD, c.TargetPipelineWeights[i])
		}

		if weightGCD == 0 {
			return fmt.Errorf(
				"invalid target pipeline weights, one of them should be greater or equal to zero")
		}
	}

	if c.TimeoutSec == 0 {
		logger.Warnf("[ZERO timeout has been applied, no task could be cancelled by timeout!]")
	}

	c.selector = routeSelectors[c.RoutePolicy]

	return nil
}

type upstreamOutput struct {
	conf       *upstreamOutputConfig
	instanceId string
}

func UpstreamOutputConstructor(conf Config) (Plugin, error) {
	c, ok := conf.(*upstreamOutputConfig)
	if !ok {
		return nil, fmt.Errorf("config type want *upstreamOutputConfig got %T", conf)
	}

	upstream := &upstreamOutput{
		conf: c,
	}

	upstream.instanceId = fmt.Sprintf("%p", upstream)

	return upstream, nil
}

func (u *upstreamOutput) Prepare(ctx pipelines.PipelineContext) {
	// Nothing to do.
}

func (u *upstreamOutput) Run(ctx pipelines.PipelineContext, t task.Task) (task.Task, error) {
	targetPipelineName := u.conf.selector(
		u.conf.TargetPipelineNames, u.conf.TargetPipelineWeights, ctx, u.conf.PluginName(), u.instanceId, t)
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

////

const (
	upstreamOutputRoundRobinSelectorStateKey       = "upstreamOutputRoundRobinSelectorStateKey"
	upstreamOutputWeightRoundRobinSelectorStateKey = "upstreamOutputWeightRoundRobinSelectorStateKey"
)

func getTaskCount(ctx pipelines.PipelineContext, pluginName string) (*uint64, error) {
	bucket := ctx.DataBucket(pluginName, pipelines.DATA_BUCKET_FOR_ALL_PLUGIN_INSTANCE)
	count, err := bucket.QueryDataWithBindDefault(upstreamOutputRoundRobinSelectorStateKey,
		func() interface{} {
			var taskCount uint64
			return &taskCount
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to count task: %s]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*uint64), nil
}

//

type upstreamOutputRoundWeightRobinSelectorState struct {
	sync.Mutex
	lastPipelineIndex int
	lastWeight        uint16
	weightGCD         uint16
	maxWeight         uint16
}

func gcd(x, y uint16) uint16 {
	for y != 0 {
		x, y = y, x%y
	}
	return x
}

func maxWeight(targetWeights []uint16) uint16 {
	var ret uint16

	for _, v := range targetWeights {
		if v > ret {
			ret = v
		}
	}

	return ret
}

func getWeightRoundRobinSelectorState(
	ctx pipelines.PipelineContext, pluginName, instanceId string, targetWeights []uint16) (
	*upstreamOutputRoundWeightRobinSelectorState, error) {

	bucket := ctx.DataBucket(pluginName, instanceId)
	count, err := bucket.QueryDataWithBindDefault(upstreamOutputWeightRoundRobinSelectorStateKey,
		func() interface{} {
			var weightGCD uint16
			for i := 0; i < len(targetWeights); i++ {
				weightGCD = gcd(weightGCD, targetWeights[i])
			}

			return &upstreamOutputRoundWeightRobinSelectorState{
				lastPipelineIndex: -1,
				weightGCD:         weightGCD,
				maxWeight:         maxWeight(targetWeights),
			}
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to update state of weight round robin selector: %s]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*upstreamOutputRoundWeightRobinSelectorState), nil
}

////

func init() {
	rand.Seed(time.Now().UnixNano())
}
