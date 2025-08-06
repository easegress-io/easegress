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
	coreruleset "github.com/corazawaf/coraza-coreruleset/v4"
	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/types"
	"github.com/jcchavezs/mergefs"
	"github.com/jcchavezs/mergefs/io"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/rules"
	"github.com/nacos-group/nacos-sdk-go/common/logger"
)

type (
	WAFResultType string

	// WAFResult defines the result structure for WAF rules.
	WAFResult struct {
		Interruption *types.Interruption
		Message      string        `json:"message,omitempty"`
		Result       WAFResultType `json:"result,omitempty"`
	}

	RuleGroup interface {
		Name() string
		// Handle processes the request and returns a WAF response.
		// TODO: how to handle the response?
		// TODO: should we process the stream request and stream response?
		Handle(ctx *context.Context) *WAFResult
	}

	rule struct {
		needCrs    bool
		directives string
	}

	ruleGroup struct {
		spec          *protocol.RuleGroupSpec
		preprocessors []protocol.PreprocessFn
		waf           coraza.WAF
	}
)

const (
	// ResultOk indicates that the request is allowed. In easegress, this is empty string.
	ResultOk WAFResultType = ""
	// ResultBlocked indicates that the request is blocked.
	ResultBlocked WAFResultType = "Blocked"
)

func newRuleGroup(spec *protocol.RuleGroupSpec) (RuleGroup, error) {
	rules := rules.NewRules(spec.Rules)
	loadOwaspCrs := spec.LoadOwaspCrs
	if !loadOwaspCrs {
		for _, r := range rules {
			if r.NeedCrs() {
				loadOwaspCrs = true
				break
			}
		}
	}

	directives := ""
	preprocessors := make([]protocol.PreprocessFn, 0, len(rules))
	for _, r := range rules {
		directives += r.Directives() + "\n"
		if r.GetPreprocessor() != nil {
			preprocessors = append(preprocessors, r.GetPreprocessor())
		}
	}

	config := coraza.NewWAFConfig().WithErrorCallback(corazaErrorCallback)
	if loadOwaspCrs {
		config = config.WithRootFS(mergefs.Merge(coreruleset.FS, io.OSFS))
	}
	config = config.WithDirectives(directives)

	waf, err := coraza.NewWAF(config)
	if err != nil {
		return nil, err
	}

	return &ruleGroup{
		spec:          spec,
		waf:           waf,
		preprocessors: preprocessors,
	}, nil
}

func corazaErrorCallback(mr types.MatchedRule) {
	logMsg := mr.ErrorLog()
	switch mr.Rule().Severity() {

	case types.RuleSeverityEmergency,
		types.RuleSeverityAlert,
		types.RuleSeverityCritical,
		types.RuleSeverityError:
		logger.Error(logMsg)

	case types.RuleSeverityWarning:
		logger.Warn(logMsg)

	case types.RuleSeverityNotice:
		logger.Info(logMsg)

	case types.RuleSeverityInfo:
		logger.Info(logMsg)

	case types.RuleSeverityDebug:
		logger.Debug(logMsg)
	}
}

func (rg *ruleGroup) Name() string {
	return rg.spec.Name
}

func (rg *ruleGroup) Handle(ctx *context.Context) *WAFResult {
	// TODO: Implement the logic to handle the request.
	// First go through the preprocessors if any. For example, GEOIP lookups and put the result into the context.
	// Then, prepare the request for WAF processing.
	return nil
}
