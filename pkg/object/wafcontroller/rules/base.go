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

package rules

import (
	"reflect"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
)

type (
	// Rule defines a WAF rule.
	Rule interface {
		Type() protocol.RuleType
		Directives() string
		NeedCrs() bool
		// Preprocess processes the request before applying the rule.
		// For example to do GEOIP lookups.
		GetPreprocessor() protocol.PreprocessFn

		init(protocol.Rule)
	}
)

var ruleTypeRegistry = map[protocol.RuleType]reflect.Type{}

func registryRule(typeName protocol.RuleType, rule Rule) {
	if _, exists := ruleTypeRegistry[typeName]; exists {
		panic("rule type already registered: " + string(typeName))
	}
	ruleTypeRegistry[typeName] = reflect.TypeOf(rule).Elem()
}

func NewRules(ruleSpec protocol.RuleSpec) []Rule {
	v := reflect.ValueOf(ruleSpec)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	rules := make([]Rule, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			fieldInterface := field.Interface()
			protocolRule, ok := fieldInterface.(protocol.Rule)
			if !ok {
				logger.Errorf("field %s is not a valid rule type", t.Field(i).Name)
				continue
			}

			typeName := protocolRule.Type()
			if ruleType, ok := ruleTypeRegistry[typeName]; ok {
				rule := reflect.New(ruleType).Interface().(Rule)
				rule.init(field.Interface().(protocol.Rule))
				rules = append(rules, rule)
			}
		}
	}
	return rules
}
