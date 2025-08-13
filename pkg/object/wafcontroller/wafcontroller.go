/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package wafcontroller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/common"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/metrics"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

type ResultError string

const (
	// Category is the category of the WAFController.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of the WAFController.
	Kind = "WAFController"

	// ResultRuleGroupNotFoundError indicates that the rule group was not found.
	ResultRuleGroupNotFoundError ResultError = "ruleGroupNotFoundError"
)

var (
	aliases = []string{
		"wafcontroller",
		"wafcontrollers",
		"waf",
	}

	globalWAFController atomic.Value
)

type (
	// WAFHandler is used to handle WAF requests.
	WAFHandler interface {
		Handle(ctx *context.Context, ruleGroupName string) string
	}

	// WAFController is the controller for WAF.
	WAFController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		ruleGroups map[string]RuleGroup
		metricHub  *metrics.MetricHub
	}

	// Spec is the specification for WAFController.
	Spec struct {
		RuleGroups []*protocol.RuleGroupSpec `json:"ruleGroups" jsonschema:"required"`
	}

	// Status is the status of WAFController.
	Status struct{}
)

// Validate validates the WAFController specification.
func (spec *Spec) Validate() error {
	names := make(map[string]struct{})
	for _, group := range spec.RuleGroups {
		if group.Name == "" {
			return fmt.Errorf("RuleGroup name cannot be empty")
		}
		if common.ValidateName(group.Name) != nil {
			return fmt.Errorf("RuleGroup name %s is invalid: %v", group.Name, common.ValidateName(group.Name))
		}
		if _, exists := names[group.Name]; exists {
			return fmt.Errorf("RuleGroup name must be unique: %s", group.Name)
		}
		names[group.Name] = struct{}{}

		// dry-run to validate the rule group
		rg, err := newRuleGroup(group)
		if err != nil {
			return fmt.Errorf("failed to create rule group %s: %v", group.Name, err)
		}
		rg.Close()
	}
	return nil
}

// Category returns the category of the WAFController.
func (waf *WAFController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of the WAFController.
func (waf *WAFController) Kind() string {
	return Kind
}

// DefaultSpec returns the default specification for WAFController.
func (waf *WAFController) DefaultSpec() interface{} {
	return &Spec{}
}

// Status returns the status of the WAFController.
func (waf *WAFController) Status() *supervisor.Status {
	stats := waf.metricHub.GetStats()
	status := make(map[string]interface{})
	status["ruleGroupStatus"] = stats
	return &supervisor.Status{
		ObjectStatus: status,
	}
}

// Close closes the WAFController and releases resources.
func (waf *WAFController) Close() {
	logger.Infof("Closing WAFController")
	waf.metricHub.Close()
	waf.unregisterAPIs()
	for _, ruleGroup := range waf.ruleGroups {
		ruleGroup.Close()
	}
	globalWAFController.CompareAndSwap(waf, (*WAFController)(nil))
}

// Init initializes the WAFController with the provided supervisor specification.
func (waf *WAFController) Init(superSpec *supervisor.Spec) {
	waf.superSpec = superSpec
	waf.super = superSpec.Super()
	waf.spec = superSpec.ObjectSpec().(*Spec)

	waf.reload(nil)
}

// Inherit initializes the WAFController with the previous generation's specification.
func (waf *WAFController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	prev := previousGeneration.(*WAFController)

	waf.superSpec = superSpec
	waf.spec = superSpec.ObjectSpec().(*Spec)
	waf.super = superSpec.Super()
	waf.reload(prev)
}

func (waf *WAFController) reload(prev *WAFController) {
	waf.ruleGroups = make(map[string]RuleGroup)
	for _, group := range waf.spec.RuleGroups {
		if group == nil || group.Name == "" {
			logger.Errorf("Invalid rule group specification: group name and spec cannot be empty")
			continue
		}

		ruleGroup, err := newRuleGroup(group)
		if err != nil {
			// This should not happen since we validate the spec in Validate method.
			logger.Errorf("[CRITICAL]!!!! Failed to create rule group %s: %v， the controller can not run correctly", group.Name, err)
			continue
		}
		waf.ruleGroups[group.Name] = ruleGroup
	}

	if prev != nil {
		if prev.metricHub != nil {
			waf.metricHub = prev.metricHub
		}
		prev.inheritClose()
	} else {
		waf.metricHub = metrics.NewMetrics(waf.superSpec)
	}
	globalWAFController.Store(waf)
	waf.registerAPIs()
}

func (waf *WAFController) inheritClose() {
	logger.Infof("close previous generation of WAFController because of inheriting")
	waf.unregisterAPIs()
	for _, ruleGroup := range waf.ruleGroups {
		ruleGroup.Close()
	}
	globalWAFController.CompareAndSwap(waf, (*WAFController)(nil))
}

func init() {
	supervisor.Register(&WAFController{})
	api.RegisterObject(&api.APIResource{
		Category:    Category,
		Kind:        Kind,
		Name:        strings.ToLower(Kind),
		Aliases:     aliases,
		ValiateHook: validateHook,
	})
}

func validateHook(operationType api.OperationType, spec *supervisor.Spec) error {
	if operationType != api.OperationTypeCreate || spec.Kind() != Kind {
		return nil
	}

	agcs := []string{}
	supervisor.GetGlobalSuper().WalkControllers(func(controller *supervisor.ObjectEntity) bool {
		if controller.Spec().Kind() == Kind {
			agcs = append(agcs, controller.Spec().Name())
		}
		return true
	})

	if len(agcs) >= 1 {
		return fmt.Errorf("only one WAFController is allowed, existed: %v", agcs)
	}

	return nil
}

// Handle processes the request and returns a WAF response.
func (waf *WAFController) Handle(ctx *context.Context, ruleGroupName string) string {
	ruleGroup, ok := waf.ruleGroups[ruleGroupName]
	if !ok || ruleGroupName == "" {
		waf.setErrResponse(ctx, fmt.Errorf("rule group %s not found", ruleGroupName))
		return string(ResultRuleGroupNotFoundError)
	}

	result := ruleGroup.Handle(ctx)
	if result.Result != protocol.ResultOk {
		if result.Interruption != nil {
			waf.metricHub.Update(&metrics.Metric{
				RuleGroup: ruleGroupName,
				RuleID:    strconv.Itoa(result.Interruption.RuleID),
				Source:    result.Interruption.Data,
			})
		}
		waf.setWafErrResponse(ctx, result)
		return string(result.Result)
	}
	return string(protocol.ResultOk)
}

func (waf *WAFController) setWafErrResponse(ctx *context.Context, result *protocol.WAFResult) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}
	if result.Interruption != nil && result.Interruption.Status != 0 {
		resp.SetStatusCode(result.Interruption.Status)
	} else {
		resp.SetStatusCode(http.StatusForbidden)
	}
	errMsg := map[string]string{
		"Message": result.Message,
	}
	data, _ := codectool.MarshalJSON(errMsg)
	resp.SetPayload(data)
	ctx.SetOutputResponse(resp)
}

func (waf *WAFController) setErrResponse(ctx *context.Context, err error) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}
	resp.SetStatusCode(http.StatusInternalServerError)
	errMsg := map[string]string{
		"Message": err.Error(),
	}
	data, _ := codectool.MarshalJSON(errMsg)
	resp.SetPayload(data)
	ctx.SetOutputResponse(resp)
}

func GetGlobalWAFController() (*WAFController, error) {
	waf := globalWAFController.Load()
	if waf == nil {
		return nil, fmt.Errorf("WAFController is not initialized or has been closed")
	}
	w, ok := waf.(*WAFController)
	if !ok {
		return nil, fmt.Errorf("global WAFController is not of type *WAFController, got %T", waf)
	}

	return w, nil
}
