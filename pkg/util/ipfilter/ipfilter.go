/*
 * Copyright (c) 2017, MegaEase
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

package ipfilter

import (
	"net"
	"strings"

	"github.com/yl2chen/cidranger"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
)

var (
	allOnesIPv4Mask = net.CIDRMask(net.IPv4len*8, net.IPv4len*8)
	allOnesIPv6Mask = net.CIDRMask(net.IPv6len*8, net.IPv6len*8)
)

type (
	// Spec describes IPFilter.
	Spec struct {
		BlockByDefault bool `yaml:"blockByDefault" jsonschema:"required"`

		AllowIPs []string `yaml:"allowIPs" jsonschema:"omitempty,uniqueItems=true,format=ipcidr-array"`
		BlockIPs []string `yaml:"blockIPs" jsonschema:"omitempty,uniqueItems=true,format=ipcidr-array"`
	}

	// IPFilter is the IP filter.
	IPFilter struct {
		spec *Spec

		allowRanger cidranger.Ranger
		blockRanger cidranger.Ranger
	}

	// IPFilters is the wrapper for multiple IPFilters.
	IPFilters struct {
		filters []*IPFilter
	}
)

// New creates an IPFilter.
func New(spec *Spec) *IPFilter {
	rangerFromIPCIDRs := func(ipcidrs []string) cidranger.Ranger {
		ranger := cidranger.NewPCTrieRanger()
		for _, ipcidr := range ipcidrs {
			ip := net.ParseIP(ipcidr)
			if ip != nil {
				mask := allOnesIPv4Mask
				// https://stackoverflow.com/a/48519490/1705845
				if strings.Count(ipcidr, ":") >= 2 {
					mask = allOnesIPv6Mask
				}
				ipNet := net.IPNet{IP: ip, Mask: mask}
				ranger.Insert(cidranger.NewBasicRangerEntry(ipNet))
				continue
			}

			_, ipNet, err := net.ParseCIDR(ipcidr)
			if err != nil {
				logger.Errorf("BUG: %s is an invalid ip or cidr", ipcidr)
				continue
			}
			ranger.Insert(cidranger.NewBasicRangerEntry(*ipNet))
		}

		return ranger
	}

	return &IPFilter{
		spec: spec,

		allowRanger: rangerFromIPCIDRs(spec.AllowIPs),
		blockRanger: rangerFromIPCIDRs(spec.BlockIPs),
	}
}

// AllowHTTPContext is the wrapper of Allow for HTTPContext.
func (f *IPFilter) AllowHTTPContext(ctx context.HTTPContext) bool {
	return f.Allow(ctx.Request().RealIP())
}

// Allow return if IPFilter allows the incoming ip.
func (f *IPFilter) Allow(ipstr string) bool {
	defaultResult := !f.spec.BlockByDefault

	ip := net.ParseIP(ipstr)
	if ip == nil {
		return defaultResult
	}

	allowed, err := f.allowRanger.Contains(ip)
	if err != nil {
		return defaultResult
	}

	blocked, err := f.blockRanger.Contains(ip)
	if err != nil {
		return defaultResult
	}

	switch {
	case allowed && blocked:
		return defaultResult
	case allowed:
		return true
	case blocked:
		return false
	default:
		return defaultResult
	}
}

// NewIPFilters creates an IPFilters
func NewIPFilters(filters ...*IPFilter) *IPFilters {
	return &IPFilters{filters: filters}
}

// Filters returns internal IPFilters.
func (f *IPFilters) Filters() []*IPFilter {
	return f.filters
}

// Append appends an IPFilter.
func (f *IPFilters) Append(filter *IPFilter) {
	f.filters = append(f.filters, filter)
}

// AllowHTTPContext is the wrapper of Allow for HTTPContext.
func (f *IPFilters) AllowHTTPContext(ctx context.HTTPContext) bool {
	return f.Allow(ctx.Request().RealIP())
}

// Allow return if IPFilters allows the incoming ip.
func (f *IPFilters) Allow(ipstr string) bool {
	for _, filter := range f.filters {
		if !filter.Allow(ipstr) {
			return false
		}
	}

	return true
}
