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
	"strings"
	"sync/atomic"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
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
	}

	// Spec is the specification for WAFController.
	Spec struct {
		RuleGroups []*protocol.RuleGroupSpec `json:"ruleGroups" jsonschema:"required"`
	}

	Status struct{}
)

func (spec *Spec) Validate() error {
	names := make(map[string]struct{})
	for _, group := range spec.RuleGroups {
		if group.Name == "" {
			return fmt.Errorf("RuleGroup name cannot be empty")
		}
		if _, exists := names[group.Name]; exists {
			return fmt.Errorf("RuleGroup name must be unique: " + group.Name)
		}
		names[group.Name] = struct{}{}

		// best way to validate the rules is to dry create the rule group
		// TODO: update if the rule group contains state and need to close.
		_, err := newRuleGroup(group)
		if err != nil {
			return fmt.Errorf("failed to create rule group %s: %v", group.Name, err)
		}
		// Additional validation for other rule types can be added here.
	}

	return nil
}

func (waf *WAFController) Category() supervisor.ObjectCategory {
	return Category
}

func (waf *WAFController) Kind() string {
	return Kind
}

func (waf *WAFController) DefaultSpec() interface{} {
	return &Spec{
		// Add default enabled rule groups or rules if needed.
	}
}

func (waf *WAFController) Status() *supervisor.Status {
	status := make(map[string]interface{})
	// Here you can add status information about the WAFController.
	return &supervisor.Status{
		ObjectStatus: status,
	}
}

func (waf *WAFController) Close() {
	logger.Infof("Closing WAFController")
	// Perform any necessary cleanup here.
}

func (waf *WAFController) Init(superSpec *supervisor.Spec) {
	waf.superSpec = superSpec
	waf.super = superSpec.Super()
	waf.spec = superSpec.ObjectSpec().(*Spec)

	waf.reload(nil)
}

func (waf *WAFController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	previousGeneration.(*WAFController).Close()
}

func (waf *WAFController) reload(prev *WAFController) {
	waf.ruleGroups = make(map[string]RuleGroup)
	for _, group := range waf.spec.RuleGroups {
		if group == nil || group.Name == "" {
			logger.Errorf("Invalid rule group specification: %v", group)
			continue
		}

		ruleGroup, err := newRuleGroup(group)
		if err != nil {
			// This should not happen since we validate the spec in Validate method.
			panic(fmt.Sprintf("Failed to create rule group %s: %v", group.Name, err))
		}
		waf.ruleGroups[group.Name] = ruleGroup
	}
}

func (waf *WAFController) InheritClose() {
	logger.Infof("close previous generation of WAFController because of inheriting")
	// TODO: Implement the logic to close the previous generation of WAFController.
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

func (waf *WAFController) Handle(ctx *context.Context, ruleGroupName string) string {
	ruleGroup, ok := waf.ruleGroups[ruleGroupName]
	if !ok || ruleGroupName == "" {
		waf.setErrResponse(ctx, fmt.Errorf("rule group %s not found", ruleGroupName))
		return string(ResultRuleGroupNotFoundError)
	}

	result := ruleGroup.Handle(ctx)
	if result.Result != ResultOk {
		// TODO: add metrics for WAF results.
		waf.setWafErrResponse(ctx, result)
		return string(result.Result)
	}
	return string(ResultOk)
}

func (waf *WAFController) setWafErrResponse(ctx *context.Context, result *WAFResult) {
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
