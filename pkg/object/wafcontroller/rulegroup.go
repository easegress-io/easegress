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
	"net"

	coreruleset "github.com/corazawaf/coraza-coreruleset/v4"
	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/types"
	"github.com/jcchavezs/mergefs"
	"github.com/jcchavezs/mergefs/io"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/rules"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

type (
	// RuleGroup defines the interface for a WAF rule group.
	RuleGroup interface {
		Name() string
		// Handle processes the request and returns a WAF response.
		// TODO: how to handle the response?
		// TODO: should we process the stream request and stream response?
		Handle(ctx *context.Context) *protocol.WAFResult

		Close()
	}

	// ruleGroup implements the RuleGroup interface.
	ruleGroup struct {
		spec          *protocol.RuleGroupSpec
		preprocessors []protocol.PreWAFProcessor
		waf           coraza.WAF
		rules         []rules.Rule
	}
)

func newRuleGroup(spec *protocol.RuleGroupSpec) (RuleGroup, error) {
	ruleset := rules.NewRules(spec.Rules)
	loadOwaspCrs := spec.LoadOwaspCrs
	if !loadOwaspCrs {
		for _, r := range ruleset {
			if r.NeedCrs() {
				loadOwaspCrs = true
				break
			}
		}
	}

	directives := ""
	preprocessors := make([]protocol.PreWAFProcessor, 0, len(ruleset))
	for _, r := range ruleset {
		if r.Type() == protocol.TypeCustoms {
			directives = r.Directives() + "\n" + directives
		} else {
			directives += r.Directives() + "\n"
		}
		if r.GetPreprocessor() != nil {
			preprocessors = append(preprocessors, r.GetPreprocessor())
		}
	}

	config := coraza.NewWAFConfig().WithErrorCallback(corazaErrorCallback)
	if loadOwaspCrs {
		config = config.WithRootFS(mergefs.Merge(coreruleset.FS, io.OSFS))
	}
	logger.Infof("create WAF %s with config:\n%s", spec.Name, directives)
	config = config.WithDirectives(directives)

	waf, err := coraza.NewWAF(config)
	if err != nil {
		for _, rule := range ruleset {
			rule.Close()
		}
		return nil, err
	}

	return &ruleGroup{
		spec:          spec,
		waf:           waf,
		preprocessors: preprocessors,
		rules:         ruleset,
	}, nil
}

func corazaErrorCallback(mr types.MatchedRule) {
	logMsg := mr.ErrorLog()
	switch mr.Rule().Severity() {

	case types.RuleSeverityEmergency,
		types.RuleSeverityAlert,
		types.RuleSeverityCritical,
		types.RuleSeverityError:
		logger.Errorf(logMsg)

	case types.RuleSeverityWarning:
		logger.Warnf(logMsg)

	case types.RuleSeverityNotice:
		logger.Infof(logMsg)

	case types.RuleSeverityInfo:
		logger.Infof(logMsg)

	case types.RuleSeverityDebug:
		logger.Debugf(logMsg)
	}
}

// Name returns the name of the rule group.
func (rg *ruleGroup) Name() string {
	return rg.spec.Name
}

// Handle processes the request and returns a WAF response.
func (rg *ruleGroup) Handle(ctx *context.Context) *protocol.WAFResult {
	req := ctx.GetInputRequest().(*httpprot.Request)
	tx := rg.waf.NewTransaction()
	ctx.OnFinish(func() {
		tx.ProcessLogging()
		tx.Close()
	})

	if tx.IsRuleEngineOff() {
		return &protocol.WAFResult{
			Result: protocol.ResultOk,
		}
	}

	// preprocessors
	for _, preprocessor := range rg.preprocessors {
		result := preprocessor(ctx, tx, req)
		if result.Result != protocol.ResultOk {
			return result
		}
	}

	// process the request
	result := rg.processRequest(ctx, tx, req)
	if result.Result != protocol.ResultOk {
		return result
	}

	// TODO: process the response

	return &protocol.WAFResult{
		Result: protocol.ResultOk,
	}
}

func parseServerName(host string) string {
	serverName, _, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	return serverName
}

func formMessage(in *types.Interruption) string {
	return fmt.Sprintf("WAF interruption: RuleID=%d, Action=%s, Status=%d, Data=%s",
		in.RuleID, in.Action, in.Status, in.Data)
}

func (rg *ruleGroup) processRequest(_ *context.Context, tx types.Transaction, req *httpprot.Request) *protocol.WAFResult {
	stdReq := req.Std()

	tx.ProcessConnection(req.RealIP(), 0, "", 0)
	tx.ProcessURI(stdReq.URL.String(), stdReq.Method, stdReq.Proto)

	// process headers
	for k, vs := range stdReq.Header {
		for _, v := range vs {
			tx.AddRequestHeader(k, v)
		}
	}
	if stdReq.Host != "" {
		tx.AddRequestHeader("Host", stdReq.Host)
		tx.SetServerName(parseServerName(stdReq.Host))
	}
	if req.TransferEncoding != nil {
		tx.AddRequestHeader("Transfer-Encoding", req.TransferEncoding[0])
	}
	it := tx.ProcessRequestHeaders()
	if it != nil {
		return &protocol.WAFResult{
			Interruption: it,
			Message:      formMessage(it),
			Result:       protocol.ResultBlocked,
		}
	}

	if tx.IsRequestBodyAccessible() {
		if req.IsStream() {
			// for streaming requests, we do not read the body or process it.
			return &protocol.WAFResult{
				Result: protocol.ResultOk,
			}
		}

		it, _, err := tx.ReadRequestBodyFrom(req.GetPayload())
		if err != nil {
			return &protocol.WAFResult{
				Message: fmt.Sprintf("failed to append request body: %s", err.Error()),
				Result:  protocol.ResultError,
			}
		}
		if it != nil {
			return &protocol.WAFResult{
				Interruption: it,
				Message:      formMessage(it),
				Result:       protocol.ResultBlocked,
			}
		}
	}

	// still need to call this even not RequestBodyAcces
	it, err := tx.ProcessRequestBody()
	if err != nil {
		return &protocol.WAFResult{
			Message: fmt.Sprintf("failed to process request body: %s", err.Error()),
			Result:  protocol.ResultError,
		}
	}
	if it != nil {
		return &protocol.WAFResult{
			Interruption: it,
			Message:      formMessage(it),
			Result:       protocol.ResultBlocked,
		}
	}
	return &protocol.WAFResult{
		Result: protocol.ResultOk,
	}
}

func (rg *ruleGroup) Close() {
	for _, rule := range rg.rules {
		rule.Close()
	}
}
