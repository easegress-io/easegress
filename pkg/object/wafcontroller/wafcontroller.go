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
	"github.com/megaease/easegress/v2/pkg/context"
	"sync/atomic"

	"github.com/megaease/easegress/v2/pkg/supervisor"
)

const (
	// Category is the category of the WAFController.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of the WAFController.
	Kind = "WAFController"
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
		// TODO: placeholder, it needs to be refactored.
		HandleWAFRequest(ctx *context.Context)
	}

	// WAFController is the controller for WAF.
	WAFController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec
	}

	// Spec is the specification for WAFController.
	Spec struct {
		RuleGroups []*RuleGroupSpec `json:"ruleGroups" jsonschema:"required"`
	}

	// RuleGroupSpec defines the specification for a WAF rule group.
	RuleGroupSpec struct {
		Name        string `json:"name" jsonschema:"required"`
		Description string `json:"description,omitempty"`
		Rules       []Rule `json:"rules" jsonschema:"required"`
	}

	// Rule defines a WAF rule.
	Rule struct {
		SQLInjection SQLInjectionSpec `json:"sqlInjection,omitempty"`
		CustomRules  []CustomRule     `json:"customRules,omitempty"`
	}

	// SQLInjectionSpec defines the specification for SQL injection detection.
	SQLInjectionSpec struct {
		Enabled bool `json:"enabled" jsonschema:"required"`
		// Additional fields can be added here for SQL injection detection configuration.
	}

	// CustomRule defines a custom WAF rule.
	CustomRule struct {
		Spec CustomRuleSpec `json:"customRule" jsonschema:"required"`
	}

	// CustomRuleSpec defines the specification for a custom WAF rule.
	CustomRuleSpec struct {
		Enabled     bool   `json:"enabled" jsonschema:"required"`
		ModSecurity string `json:"modSecurity,omitempty"` // ModSecurity rule string.
		// Additional fields can be added here for custom rule configuration.
	}
)

func (waf *WAFController) Category() supervisor.ObjectCategory {
	//TODO implement me
	panic("implement me")
}

func (waf *WAFController) Kind() string {
	//TODO implement me
	panic("implement me")
}

func (waf *WAFController) DefaultSpec() interface{} {
	//TODO implement me
	panic("implement me")
}

func (waf *WAFController) Status() *supervisor.Status {
	//TODO implement me
	panic("implement me")
}

func (waf *WAFController) Close() {
	//TODO implement me
	panic("implement me")
}

func init() {
	supervisor.Register(&WAFController{})
}
