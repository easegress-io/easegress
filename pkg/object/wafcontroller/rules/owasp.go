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
	"fmt"

	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
)

type (
	// OwaspRulesSpec defines the specification for OWASP rules.
	OwaspRules struct {
		spec *protocol.OwaspRulesSpec
	}
)

func (owasp *OwaspRules) Type() protocol.RuleType {
	return protocol.TypeOwaspRules
}

// Directives returns the directives for OWASP rules.
// Check https://github.com/corazawaf/coraza-coreruleset?tab=readme-ov-file#usage for more details.
func (owasp *OwaspRules) Directives() string {
	if len(*owasp.spec) == 0 {
		return ""
	}
	directives := ""
	for _, rule := range *owasp.spec {
		directives += fmt.Sprintf("Include @owasp_crs/%s\n", rule)
	}
	return directives

}

func (owasp *OwaspRules) NeedCrs() bool {
	return true
}

func (owasp *OwaspRules) GetPreprocessor() protocol.PreprocessFn {
	// OWASP rules do not require a preprocessor.
	return nil
}

func (owasp *OwaspRules) init(ruleSpec protocol.Rule) {
	owasp.spec = ruleSpec.(*protocol.OwaspRulesSpec)
}

func init() {
	registryRule(protocol.TypeOwaspRules, &OwaspRules{})
}
