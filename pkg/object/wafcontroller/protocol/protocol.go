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

package protocol

import (
	"reflect"

	"github.com/corazawaf/coraza/v3/types"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

type (
	// PreWAFProcessor defines a function type for preprocessing requests before applying WAF rules.
	PreWAFProcessor func(ctx *context.Context, tx types.Transaction, req *httpprot.Request) *WAFResult

	// WAFResultType defines the type of WAF result.
	WAFResultType string

	// WAFResult defines the result structure for WAF rules.
	WAFResult struct {
		// Interruption indicates whether the request was interrupted by the WAF.
		Interruption *types.Interruption
		Message      string        `json:"message,omitempty"`
		Result       WAFResultType `json:"result,omitempty"`
	}

	// RuleType defines the type of WAF rule.
	RuleType string

	// Rule defines the interface for a WAF rule.
	Rule interface {
		// Type returns the type of the rule.
		Type() RuleType
	}

	// RuleGroupSpec defines the specification for a WAF rule group.
	RuleGroupSpec struct {
		Name string `json:"name" jsonschema:"required"`
		// LoadOwaspCrs indicates whether to load the OWASP Core Rule Set.
		// Please check https://github.com/corazawaf/coraza-coreruleset for more details.
		LoadOwaspCrs bool     `json:"loadOwaspCrs,omitempty"`
		Rules        RuleSpec `json:"rules" jsonschema:"required"`
	}

	// RuleSpec defines a WAF rule.
	RuleSpec struct {
		// OwaspRules defines the OWASP rules to be applied.
		// See the example of https://github.com/corazawaf/coraza-coreruleset for more details.
		OwaspRules   *OwaspRulesSpec   `json:"owaspRules,omitempty"`
		Customs      *CustomsSpec      `json:"customRules,omitempty"`
		IPBlocker    *IPBlockerSpec    `json:"ipBlocker,omitempty"`
		GeoIPBlocker *GeoIPBlockerSpec `json:"geoIPBlocker,omitempty"`
	}

	// IPBlockerSpec defines the specification for IP blocking.
	IPBlockerSpec struct {
		WhiteList []string `json:"whitelist,omitempty"`
		BlackList []string `json:"blacklist,omitempty"`
	}

	GeoIPBlockerSpec struct {
		DBPath string `json:"dbPath" jsonschema:"required"`
		// DBUpdateCron     string   `json:"dbUpdateCron" jsonschema:"required"`
		AllowedCountries []string `json:"allowedCountries,omitempty"`
		DeniedCountries  []string `json:"deniedCountries,omitempty"`
	}

	// OwaspRulesSpec defines the specification for OWASP rules.
	OwaspRulesSpec []string

	// CustomsSpec defines a custom WAF rule.
	CustomsSpec string
)

const (
	// TypeCustoms defines the type for custom WAF rules.
	TypeCustoms RuleType = "Customs"
	// TypeOwaspRules defines the type for OWASP rules.
	TypeOwaspRules RuleType = "OwaspRules"
	// TypeSQLInjection defines the type for SQL injection rules.
	TypeSQLInjection RuleType = "SQLInjection"
	// TypeIPBlocker defines the type for IP blocking rules.
	TypeIPBlocker RuleType = "IPBlocker"
	// TypeGeoIPBlocker defines the type for GeoIP blocking rules.
	TypeGeoIPBlocker RuleType = "GeoIPBlocker"
)

const (
	// ResultOk indicates that the request is allowed. In easegress, this is empty string.
	ResultOk WAFResultType = ""
	// ResultBlocked indicates that the request is blocked.
	ResultBlocked WAFResultType = "Blocked"
	// ResultError indicates that an internal error occurred while processing the request.
	ResultError WAFResultType = "InternalError"
)

var _ Rule = (*CustomsSpec)(nil)
var _ Rule = (*OwaspRulesSpec)(nil)
var _ Rule = (*IPBlockerSpec)(nil)
var _ Rule = (*GeoIPBlockerSpec)(nil)

// Type returns the type of the OWASP rule.
func (owasp *OwaspRulesSpec) Type() RuleType {
	return TypeOwaspRules
}

// Type returns the type of the custom rule.
func (rule *CustomsSpec) Type() RuleType {
	return TypeCustoms
}

func (blocker *IPBlockerSpec) Type() RuleType {
	return TypeIPBlocker
}

func (blocker *GeoIPBlockerSpec) Type() RuleType {
	return TypeGeoIPBlocker
}

// GetSpec retrieves the specific rule based on its type name.
func (rule *RuleSpec) GetSpec(typeName string) Rule {
	v := reflect.ValueOf(rule)
	field := v.FieldByName(typeName)
	if !field.IsValid() {
		return nil
	}
	if field.Kind() == reflect.Struct {
		if spec, ok := field.Interface().(Rule); ok {
			return spec
		}
		return nil
	}
	return nil
}
