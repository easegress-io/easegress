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
	"net"

	"github.com/megaease/easegress/v2/pkg/logger"

	"github.com/corazawaf/coraza/v3/types"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/yl2chen/cidranger"
)

type (
	// IPBlocker defines a WAF IP blocking rule.
	IPBlocker struct {
		spec      *protocol.IPBlockerSpec
		whiteList cidranger.Ranger
		blackList cidranger.Ranger
	}
)

// Type returns the type of the IP blocking rule.
func (rule *IPBlocker) Type() protocol.RuleType {
	return protocol.TypeIPBlocker
}

// Directives returns the directives for IP blocking rules.
func (rule *IPBlocker) Directives() string {
	return ""
}

// NeedCrs indicates whether the IP blocking rule requires OWASP CRS.
func (rule *IPBlocker) NeedCrs() bool {
	return false
}

// GetPreprocessor returns the preprocessor for IP blocking rules.
func (rule *IPBlocker) GetPreprocessor() protocol.PreWAFProcessor {
	return func(ctx *context.Context, tx types.Transaction, req *httpprot.Request) *protocol.WAFResult {
		ip := net.ParseIP(req.RealIP())
		if rule.blackList.Len() > 0 {
			contains, err := rule.blackList.Contains(ip)
			if err != nil {
				return &protocol.WAFResult{
					Result:  protocol.ResultError,
					Message: fmt.Sprintf("IPBlocker blackList check error: %v", err),
				}
			}
			if contains {
				// Block the request
				return &protocol.WAFResult{
					Result:  protocol.ResultBlocked,
					Message: fmt.Sprintf("IP %s is blocked", ip.String()),
				}
			}
		}
		if rule.whiteList.Len() > 0 {
			contains, err := rule.whiteList.Contains(ip)
			if err != nil {
				return &protocol.WAFResult{
					Result:  protocol.ResultError,
					Message: fmt.Sprintf("IPBlocker whiteList check error: %v", err),
				}
			}
			if !contains {
				return &protocol.WAFResult{
					Result:  protocol.ResultBlocked,
					Message: fmt.Sprintf("IP %s is not in whitelist", ip.String()),
				}
			}
		}
		return &protocol.WAFResult{
			Result: protocol.ResultOk,
		}
	}
}

func (rule *IPBlocker) init(ruleSpec protocol.Rule) error {
	rule.spec = ruleSpec.(*protocol.IPBlockerSpec)
	rule.whiteList = cidranger.NewPCTrieRanger()
	rule.blackList = cidranger.NewPCTrieRanger()

	for _, cidr := range rule.spec.WhiteList {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			logger.Errorf("Invalid CIDR in whitelist: " + cidr)
			continue
		}
		err = rule.whiteList.Insert(cidranger.NewBasicRangerEntry(*network))
		if err != nil {
			logger.Errorf("Failed to insert CIDR %s into whitelist: %v", cidr, err)
		}
	}

	for _, cidr := range rule.spec.BlackList {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			logger.Errorf("Invalid CIDR in blacklist: " + cidr)
			continue
		}
		err = rule.blackList.Insert(cidranger.NewBasicRangerEntry(*network))
		if err != nil {
			logger.Errorf("Failed to insert CIDR %s into blacklist: %v", cidr, err)
		}
	}
	return nil
}

func (rule *IPBlocker) Close() {}

func init() {
	registryRule(protocol.TypeIPBlocker, &IPBlocker{})
}
