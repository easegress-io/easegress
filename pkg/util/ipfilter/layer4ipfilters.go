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
	"reflect"
	"sync/atomic"
)

type (
	// Layer4IpFilters layer4 ip filters
	Layer4IpFilters struct {
		rules atomic.Value
	}

	ipFiltersRules struct {
		spec     *Spec
		ipFilter *IPFilter
	}
)

// NewLayer4IPFilters create layer4 ip filters
func NewLayer4IPFilters(spec *Spec) *Layer4IpFilters {
	if spec == nil {
		return nil
	}

	m := &Layer4IpFilters{}
	m.rules.Store(&ipFiltersRules{
		spec:     spec,
		ipFilter: New(spec),
	})
	return m
}

// AllowIP check whether the IP is allowed to pass
func (i *Layer4IpFilters) AllowIP(ip string) bool {
	rules := i.rules.Load().(*ipFiltersRules)
	if rules == nil || rules.spec == nil {
		return true
	}
	return rules.ipFilter.Allow(ip)
}

// ReloadRules reload layer4 ip filters rules
func (i *Layer4IpFilters) ReloadRules(spec *Spec) {
	if spec == nil {
		i.rules.Store(&ipFiltersRules{})
		return
	}

	old := i.rules.Load().(*ipFiltersRules)
	if reflect.DeepEqual(old.spec, spec) {
		return
	}

	rules := &ipFiltersRules{
		spec:     spec,
		ipFilter: New(spec),
	}
	i.rules.Store(rules)
}

func (r *ipFiltersRules) pass(clientIP string) bool {
	if r.ipFilter == nil {
		return true
	}
	return r.ipFilter.Allow(clientIP)
}
