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
	"path/filepath"

	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
)

type (
	// OwaspRules defines the specification for OWASP rules.
	OwaspRules struct {
		spec *protocol.OwaspRulesSpec
	}
)

// Type returns the type of the OWASP rules.
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
		path := filepath.Join("@owasp_crs", rule)
		directives += fmt.Sprintf("Include %s\n", path)
	}
	return directives
}

// NeedCrs indicates whether the OWASP rules require OWASP CRS.
func (owasp *OwaspRules) NeedCrs() bool {
	return true
}

// GetPreprocessor returns the preprocessor for OWASP rules.
func (owasp *OwaspRules) GetPreprocessor() protocol.PreWAFProcessor {
	// OWASP rules do not require a preprocessor.
	return nil
}

func (owasp *OwaspRules) init(ruleSpec protocol.Rule) error {
	owasp.spec = ruleSpec.(*protocol.OwaspRulesSpec)
	return nil
}

func (owasp *OwaspRules) Close() {}

func init() {
	registryRule(protocol.TypeOwaspRules, &OwaspRules{})
}
