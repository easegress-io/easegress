package plugins

import (
	"fmt"
	"strings"
	"task"

	"common"
)

type routeSelector func(
	targetPipelines []string, targetWeights []uint16, context map[string]interface{}, t task.Task) string

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
	targetPipelines []string, targetWeights []uint16, context map[string]interface{}, t task.Task) string {

	return "" // todo(zhiyan)
}

func weightRoundRobinSelector(
	targetPipelines []string, targetWeights []uint16, context map[string]interface{}, t task.Task) string {

	return "" // todo(zhiyan)
}

func randomSelector(
	targetPipelines []string, targetWeights []uint16, context map[string]interface{}, t task.Task) string {

	return "" // todo(zhiyan)
}

func weightRandomSelector(
	targetPipelines []string, targetWeights []uint16, context map[string]interface{}, t task.Task) string {

	return "" // todo(zhiyan)
}

func hashSelector(
	targetPipelines []string, targetWeights []uint16, context map[string]interface{}, t task.Task) string {

	return "" // todo(zhiyan)
}

func leastWIPRequestsSelector(
	targetPipelines []string, targetWeights []uint16, context map[string]interface{}, t task.Task) string {

	return "" // todo(zhiyan)
}

func stickySessionSelector(
	targetPipelines []string, targetWeights []uint16, context map[string]interface{}, t task.Task) string {

	return "" // todo(zhiyan)
}

////

type upstreamConfig struct {
	CommonConfig
	TargetPipelineNames   []string `json:"target_pipelines"`
	TargetPipelineWeights []uint16 `json:"target_weights"` // up to 65535, 0 based
	RoutePolicy           string   `json:"route_policy"`
	UpstreamTimeoutSec    uint16   `json:"timeout_sec"` // up to 65535, zero means no timeout

	RequestParameterKeys []string `json:"request_parameter_keys"`
	ValueHashedKey       string   `json:"value_hashed_key"`
}

func UpstreamConfigConstructor() Config {
	return &upstreamConfig{
		RoutePolicy: "round_robin",
	}
}

func (c *upstreamConfig) Prepare(pipelineNames []string) error {
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

	return nil
}

type upstream struct {
	conf *upstreamConfig
}
