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
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
)

type (
	Custom struct {
		spec *protocol.CustomsSpec
	}
)

func (rule *Custom) Type() protocol.RuleType {
	return protocol.TypeCustoms
}

func (rule *Custom) Directives() string {
	return string(*rule.spec)
}

func (rule *Custom) NeedCrs() bool {
	// If user need to load OWASP CRS, they should use LoadOwaspCrs flag in RuleGroupSpec.
	return false
}

func (rule *Custom) GetPreprocessor() protocol.PreprocessFn {
	return nil // No preprocessor for custom rules
}

func (rule *Custom) init(ruleSpec protocol.Rule) {
	rule.spec = ruleSpec.(*protocol.CustomsSpec)
}

func init() {
	registryRule(protocol.TypeCustoms, &Custom{})
}
