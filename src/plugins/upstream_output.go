package plugins

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdecteam/easegateway-types/pipelines"
	"github.com/hexdecteam/easegateway-types/plugins"
	"github.com/hexdecteam/easegateway-types/task"

	"common"
	"logger"
	"option"
)

type target struct {
	pipelineName string
	needResponse bool
}

type routeSelector func(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target

var routeSelectors = map[string]routeSelector{
	"round_robin":          roundRobinSelector,
	"weighted_round_robin": weightedRoundRobinSelector,
	"random":               randomSelector,
	"weighted_random":      weightedRandomSelector,
	"least_wip_requests":   leastWIPRequestsSelector,

	"hash":   hashSelector,   // to support use cases for http source address and header hash, sticky session
	"filter": filterSelector, // to support blue/green deployment and A/B testing

	"fanout": fanoutSelector,
}

func roundRobinSelector(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target {
	taskCount, err := getTaskCount(ctx, u.Name())
	if err != nil {
		return randomSelector(u, ctx, t)
	}

	atomic.AddUint64(taskCount, 1)

	return []*target{{
		pipelineName: u.conf.TargetPipelineNames[*taskCount%uint64(len(u.conf.TargetPipelineNames))],
		needResponse: true,
	}}
}

func weightedRoundRobinSelector(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target {
	state, err := getWeightedRoundRobinSelectorState(ctx, u.Name(), u.instanceId, u.conf.TargetPipelineWeights)
	if err != nil {
		return randomSelector(u, ctx, t)
	}

	state.Lock()
	defer state.Unlock()

	for {
		state.lastPipelineIndex = (state.lastPipelineIndex + 1) % len(u.conf.TargetPipelineNames)
		if state.lastPipelineIndex == 0 {
			state.lastWeight -= state.weightGCD
			if state.lastWeight <= 0 {
				state.lastWeight = state.maxWeight
			}
		}

		weight := u.conf.TargetPipelineWeights[state.lastPipelineIndex]
		if weight >= state.lastWeight {
			return []*target{{
				pipelineName: u.conf.TargetPipelineNames[state.lastPipelineIndex],
				needResponse: true,
			}}
		}
	}
}

func randomSelector(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target {
	return []*target{{
		pipelineName: u.conf.TargetPipelineNames[rand.Intn(len(u.conf.TargetPipelineNames))],
		needResponse: true,
	}}
}

func weightedRandomSelector(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target {
	sum, err := getWeightSum(ctx, u.Name(), u.instanceId, u.conf.TargetPipelineWeights)
	if err != nil {
		return randomSelector(u, ctx, t)
	}

	r := rand.Uint64() % sum

	for idx := 0; idx < len(u.conf.TargetPipelineWeights); idx++ {
		if r < uint64(u.conf.TargetPipelineWeights[idx]) {
			return []*target{{
				pipelineName: u.conf.TargetPipelineNames[idx],
				needResponse: true,
			}}
		}

		r -= uint64(u.conf.TargetPipelineWeights[idx])
	}

	logger.Errorf("[BUG: calculation in weighted random selector is wrong, should not reach here]")

	return randomSelector(u, ctx, t)
}

func leastWIPRequestsSelector(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target {
	ret := u.conf.TargetPipelineNames[0]
	leastWIP := ctx.CrossPipelineWIPRequestsCount(ret)

	for idx := 1; idx < len(u.conf.TargetPipelineNames)-1; idx++ {
		count := ctx.CrossPipelineWIPRequestsCount(u.conf.TargetPipelineNames[idx])
		if count < leastWIP {
			leastWIP = count
			ret = u.conf.TargetPipelineNames[idx]
		}
	}

	return []*target{{
		pipelineName: ret,
		needResponse: true,
	}}
}

func hashSelector(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target {
	h := fnv.New32a()

	for _, key := range u.conf.ValueHashedKeys {
		v := t.Value(key)
		if v == nil {
			continue
		}

		h.Write([]byte(task.ToString(v, option.PluginIODataFormatLengthLimit)))
	}

	return []*target{{
		pipelineName: u.conf.TargetPipelineNames[h.Sum32()%uint32(len(u.conf.TargetPipelineNames))],
		needResponse: true,
	}}
}

func filterSelector(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target {
	selectedIdx := -1

	for idx, conditionSet := range u.conf.FilterConditions {
		matched := true

		for key := range conditionSet {
			r := u.conf.filterRegexMapList[idx][key]
			v := t.Value(key)

			if v == nil || !r.MatchString(task.ToString(v, option.PluginIODataFormatLengthLimit)) {
				matched = false
				break
			}
		}

		if matched {
			selectedIdx = idx
			break
		}
	}

	if selectedIdx == -1 {
		logger.Warnf("[no target pipeline matched the condition, filter selector chooses nothing]")
		return nil
	}

	return []*target{{
		pipelineName: u.conf.TargetPipelineNames[selectedIdx],
		needResponse: true,
	}}
}

func fanoutSelector(u *upstreamOutput, ctx pipelines.PipelineContext, t task.Task) []*target {
	var ret []*target

	for idx, pipelineName := range u.conf.TargetPipelineNames {
		ret = append(ret, &target{
			pipelineName: pipelineName,
			needResponse: u.conf.TargetPipelineResponseFlags[idx],
		})
	}

	return ret
}

////

type upstreamOutputConfig struct {
	common.PluginCommonConfig
	TargetPipelineNames []string `json:"target_pipelines"`
	RoutePolicy         string   `json:"route_policy"`
	TimeoutSec          uint16   `json:"timeout_sec"` // up to 65535, zero means no timeout

	RequestDataKeys []string `json:"request_data_keys"`
	// for weighted_round_robin and weighted_random policies
	TargetPipelineWeights []uint16 `json:"target_weights"` // weight up to 65535, 0 based
	// for hash policy
	ValueHashedKeys []string `json:"value_hashed_keys"`
	// for filter policy
	// each map in the list as the condition set for the target pipeline according to the index
	// map key is the key of value in the task, map value is the match condition, support regex
	FilterConditions []map[string]string `json:"filter_conditions"`
	// for fanout policy
	TargetPipelineResponseFlags []bool `json:"target_response_flags"`

	selector           routeSelector
	filterRegexMapList []map[string]*regexp.Regexp
}

func UpstreamOutputConfigConstructor() plugins.Config {
	return &upstreamOutputConfig{
		RoutePolicy: "round_robin",
	}
}

func (c *upstreamOutputConfig) Prepare(pipelineNames []string) error {
	err := c.PluginCommonConfig.Prepare(pipelineNames)
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

	if len(c.TargetPipelineNames) == 0 {
		return fmt.Errorf("invalid upstream pipelines")
	}

	if strings.HasPrefix(c.RoutePolicy, "weighted_") {
		if len(c.TargetPipelineWeights) > 0 && len(c.TargetPipelineWeights) != len(c.TargetPipelineNames) {
			return fmt.Errorf("invalid upstream pipeline weight")
		}

		useDefaultWeight := len(c.TargetPipelineWeights) == 0

		for idx, pipelineName := range c.TargetPipelineNames {
			c.TargetPipelineNames[idx] = ts(pipelineName)

			if !common.StrInSlice(c.TargetPipelineNames[idx], pipelineNames) {
				logger.Warnf("[upstream pipeline %s not found]", c.TargetPipelineNames[idx])
			}

			if useDefaultWeight {
				c.TargetPipelineWeights = append(c.TargetPipelineWeights, 1)
			}
		}

		if !useDefaultWeight {
			var weightGCD uint16
			for i := 0; i < len(c.TargetPipelineWeights); i++ {
				weightGCD = gcd(weightGCD, c.TargetPipelineWeights[i])
			}

			if weightGCD == 0 {
				return fmt.Errorf("invalid target pipeline weights, " +
					"one of them should be greater or equal to zero")
			}
		}
	}

	if c.RoutePolicy == "hash" && len(c.ValueHashedKeys) == 0 {
		return fmt.Errorf("invalid hash keys")
	}

	if c.RoutePolicy == "filter" {
		if len(c.FilterConditions) != len(c.TargetPipelineNames) {
			return fmt.Errorf("invalid filter conditions")
		}

		for idx, conditionSet := range c.FilterConditions {
			if len(conditionSet) == 0 {
				return fmt.Errorf("invalid filter conditons of target pipeline %s",
					c.TargetPipelineNames[idx])
			}

			regexMap := make(map[string]*regexp.Regexp)

			for key, condition := range conditionSet {
				regexMap[key], err = regexp.Compile(condition)
				if err != nil {
					return fmt.Errorf("invalid filter condition: %v", err)
				}
			}

			c.filterRegexMapList = append(c.filterRegexMapList, regexMap)
		}
	}

	if c.RoutePolicy == "fanout" {
		if len(c.TargetPipelineResponseFlags) > 0 && len(c.TargetPipelineResponseFlags) != len(c.TargetPipelineNames) {
			return fmt.Errorf("invalid upstream pipeline response flag")
		}

		useDefaultFlag := len(c.TargetPipelineResponseFlags) == 0

		for idx, pipelineName := range c.TargetPipelineNames {
			c.TargetPipelineNames[idx] = ts(pipelineName)

			if !common.StrInSlice(c.TargetPipelineNames[idx], pipelineNames) {
				logger.Warnf("[upstream pipeline %s not found]", c.TargetPipelineNames[idx])
			}

			if useDefaultFlag {
				c.TargetPipelineResponseFlags = append(c.TargetPipelineResponseFlags, false)
			}
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

func UpstreamOutputConstructor(conf plugins.Config) (plugins.Plugin, error) {
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
	targets := u.conf.selector(u, ctx, t)
	if len(targets) == 0 {
		t.SetError(fmt.Errorf("target pipeline selector of %s returns empty pipeline name", u.conf.RoutePolicy),
			task.ResultServiceUnavailable)
		return t, nil
	}

	var requests []*pipelines.DownstreamRequest
	var waitResponses []string

	for _, target := range targets {
		data := make(map[interface{}]interface{})
		for _, key := range u.conf.RequestDataKeys {
			data[key] = t.Value(key)
		}

		request := pipelines.NewDownstreamRequest(target.pipelineName, u.Name(), data)
		requests = append(requests, request)

		if target.needResponse {
			waitResponses = append(waitResponses, target.pipelineName)
		}
	}

	// close request without response at last to prevent upstream ignores closed request directly when it scheduled
	defer func() {
		for _, request := range requests {
			request.Close()
		}
	}()

	done := make(chan struct{}, 0)

	go func() { // watch on task cancellation
		select {
		case <-t.Cancel():
			if common.CloseChan(done) {
				t.SetError(fmt.Errorf("task is cancelled by %s", t.CancelCause()), task.ResultTaskCancelled)
			}
		case <-done:
			// Nothing to do, exit goroutine
		}
	}()

	timeout := time.Duration(u.conf.TimeoutSec) * time.Second
	if timeout > 0 { // zero value means no timeout
		go func() { // watch on request timeout
			select {
			case <-time.After(timeout):
				if common.CloseChan(done) {
					t.SetError(
						fmt.Errorf("upstream is timeout after %d second(s)", u.conf.TimeoutSec),
						task.ResultServiceUnavailable)
				}
			case <-done:
				// Nothing to do, exit goroutine
			}
		}()
	}

	defer common.CloseChan(done)

LOOP:
	for _, request := range requests {
		err := ctx.CommitCrossPipelineRequest(request, done)
		if err != nil {
			if t.Error() == nil { // commit failed and it was not caused by task cancellation or request timeout
				t.SetError(err, task.ResultServiceUnavailable)
			}

			return t, nil
		}

		if !common.StrInSlice(request.UpstreamPipelineName(), waitResponses) {
			continue LOOP
		}

		// synchronized call
		select {
		case response := <-request.Response():
			if response == nil {
				logger.Errorf("[BUG: upstream pipeline %s returns nil response]",
					response.UpstreamPipelineName)

				t.SetError(fmt.Errorf("downstream received nil upstream response"),
					task.ResultInternalServerError)
				return t, nil
			}

			if response.UpstreamPipelineName != request.UpstreamPipelineName() {
				logger.Errorf("[BUG: upstream pipeline %s returns the response of "+
					"cross pipeline request to the wrong downstream %s]",
					response.UpstreamPipelineName, ctx.PipelineName())

				t.SetError(fmt.Errorf("downstream received wrong upstream response"),
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
		case <-done:
			// stop loop, task is cancelled or requests running timeout before get all responses from upstreams
			break LOOP
		}
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
	upstreamOutputRoundRobinSelectorStateKey         = "upstreamOutputRoundRobinSelectorStateKey"
	upstreamOutputWeightedRoundRobinSelectorStateKey = "upstreamOutputWeightedRoundRobinSelectorStateKey"
	upstreamOutputWeightedRandomSelectorStateKey     = "upstreamOutputWeightedRandomSelectorStateKey"
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
			"ignored to count task: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*uint64), nil
}

//

type upstreamOutputRoundWeightedRobinSelectorState struct {
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

func getWeightedRoundRobinSelectorState(
	ctx pipelines.PipelineContext, pluginName, instanceId string, targetWeights []uint16) (
	*upstreamOutputRoundWeightedRobinSelectorState, error) {

	bucket := ctx.DataBucket(pluginName, instanceId)
	count, err := bucket.QueryDataWithBindDefault(upstreamOutputWeightedRoundRobinSelectorStateKey,
		func() interface{} {
			var weightGCD uint16
			for i := 0; i < len(targetWeights); i++ {
				weightGCD = gcd(weightGCD, targetWeights[i])
			}

			return &upstreamOutputRoundWeightedRobinSelectorState{
				lastPipelineIndex: -1,
				weightGCD:         weightGCD,
				maxWeight:         maxWeight(targetWeights),
			}
		})
	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to update state of weighted round robin selector: %v]", ctx.PipelineName(), err)
		return nil, err
	}

	return count.(*upstreamOutputRoundWeightedRobinSelectorState), nil
}

//

func getWeightSum(
	ctx pipelines.PipelineContext, pluginName, instanceId string, targetWeights []uint16) (uint64, error) {

	bucket := ctx.DataBucket(pluginName, instanceId)
	sum, err := bucket.QueryDataWithBindDefault(upstreamOutputWeightedRandomSelectorStateKey,
		func() interface{} {
			var ret uint64
			for _, weight := range targetWeights {
				// FIXME(zhiyan): to use better algorithm of weighted random if we meet overflow issue
				ret += uint64(weight)
			}

			return ret
		})

	if err != nil {
		logger.Warnf("[BUG: query state data for pipeline %s failed, "+
			"ignored to get state of weighted random selector: %v]", ctx.PipelineName(), err)
		return 0, err
	}

	return sum.(uint64), nil
}

////

func init() {
	rand.Seed(time.Now().UnixNano())
}
