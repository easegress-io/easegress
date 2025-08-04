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
	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"strings"
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

		for _, rule := range group.Rules {
			if rule.SQLInjection.Enabled && len(rule.CustomRules) > 0 {
				return fmt.Errorf("Cannot enable SQLInjection and CustomRules at the same time")
			}
			for _, customRule := range rule.CustomRules {
				if customRule.Spec.ModSecurity == "" {
					return fmt.Errorf("CustomRule ModSecurity cannot be empty")
				}
			}
			// Additional validation for other rule types can be added here.
		}
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
