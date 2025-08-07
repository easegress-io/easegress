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
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	r "github.com/megaease/easegress/v2/pkg/object/wafcontroller/rules"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	logger.InitNop()
	code := m.Run()
	os.Exit(code)
}

func TestGenerateRules(t *testing.T) {
	assert := assert.New(t)

	customRules := protocol.CustomsSpec(`
SecRule ARGS "@rx ^(.*)$" "id:123,phase:2,pass,log,msg:'Test rule'"
SecRule ARGS "@rx ^(.*)$" "id:124,phase:2,pass,log,msg:'Disabled rule'"
`)
	owaspRules := protocol.OwaspRulesSpec{
		"REQUEST-911-METHOD-ENFORCEMENT.conf",
	}

	spec := &Spec{
		RuleGroups: []*protocol.RuleGroupSpec{
			{
				Name: "testGroup",
				Rules: protocol.RuleSpec{
					OwaspRules: &owaspRules,
					Customs:    &customRules,
				},
			},
		},
	}

	rules := r.NewRules(spec.RuleGroups[0].Rules)
	assert.Equal(2, len(rules), "Expected 2 rules")
}
