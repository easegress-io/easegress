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
	"net/netip"

	"github.com/corazawaf/coraza/v3/types"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/wafcontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/oschwald/geoip2-golang/v2"
)

type (
	// GeoIPBlocker defines a WAF GeoIP blocking rule.
	GeoIPBlocker struct {
		spec             *protocol.GeoIPBlockerSpec
		db               *geoip2.Reader
		allowedCountries map[string]struct{}
		deniedCountries  map[string]struct{}
	}
)

// Type returns the type of the GeoIP blocking rule.
func (rule *GeoIPBlocker) Type() protocol.RuleType {
	return protocol.TypeGeoIPBlocker
}

// Directives returns the directives for GeoIP blocking rules.
func (rule *GeoIPBlocker) Directives() string {
	return ""
}

// NeedCrs indicates whether the GeoIP blocking rule requires OWASP CRS.
func (rule *GeoIPBlocker) NeedCrs() bool {
	return false
}

// GetPreprocessor returns the preprocessor for GeoIP blocking rules.
func (rule *GeoIPBlocker) GetPreprocessor() protocol.PreWAFProcessor {
	return func(ctx *context.Context, tx types.Transaction, req *httpprot.Request) *protocol.WAFResult {
		ip, err := netip.ParseAddr(req.RealIP())
		if err != nil {
			return &protocol.WAFResult{
				Interruption: nil,
				Message:      fmt.Sprintf("failed to parse IP address: %v", err),
				Result:       protocol.ResultError,
			}
		}
		record, err := rule.db.Country(ip)
		if err != nil {
			return &protocol.WAFResult{
				Interruption: nil,
				Message:      fmt.Sprintf("failed to get GeoIP record: %v", err),
				Result:       protocol.ResultError,
			}
		}
		countryCode := record.Country.ISOCode
		if len(rule.deniedCountries) > 0 {
			if _, found := rule.deniedCountries[countryCode]; found {
				return &protocol.WAFResult{
					Interruption: nil,
					Message:      fmt.Sprintf("access denied for country: %s", countryCode),
					Result:       protocol.ResultBlocked,
				}
			}
		}
		if len(rule.allowedCountries) > 0 {
			if _, found := rule.allowedCountries[countryCode]; !found {
				return &protocol.WAFResult{
					Interruption: nil,
					Message:      fmt.Sprintf("access denied for country: %s", countryCode),
					Result:       protocol.ResultBlocked,
				}
			}
		}
		return &protocol.WAFResult{
			Result: protocol.ResultOk,
		}
	}
}

func (rule *GeoIPBlocker) init(ruleSpec protocol.Rule) error {
	rule.spec = ruleSpec.(*protocol.GeoIPBlockerSpec)
	db, err := geoip2.Open(rule.spec.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open GeoIP database: %w", err)
	}
	rule.db = db
	rule.allowedCountries = make(map[string]struct{})
	rule.deniedCountries = make(map[string]struct{})
	for _, country := range rule.spec.AllowedCountries {
		rule.allowedCountries[country] = struct{}{}
	}
	for _, country := range rule.spec.DeniedCountries {
		rule.deniedCountries[country] = struct{}{}
	}
	return nil
}

func (rule *GeoIPBlocker) Close() {
	if rule.db != nil {
		rule.db.Close()
	}
}

func init() {
	registryRule(protocol.TypeGeoIPBlocker, &GeoIPBlocker{})
}
