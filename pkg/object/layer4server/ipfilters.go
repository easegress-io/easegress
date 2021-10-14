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

package layer4server

import (
	"reflect"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (
	ipFilters struct {
		rules atomic.Value
	}

	ipFiltersRules struct {
		spec     *ipfilter.Spec
		ipFilter *ipfilter.IPFilter
	}
)

func newIpFilters(spec *ipfilter.Spec) *ipFilters {
	m := &ipFilters{}

	m.rules.Store(&ipFiltersRules{
		spec:     spec,
		ipFilter: newIPFilter(spec),
	})
	return m
}

func (i *ipFilters) AllowIP(ip string) bool {
	rules := i.rules.Load().(*ipFiltersRules)
	if rules == nil || rules.spec == nil {
		return true
	}
	return rules.ipFilter.Allow(ip)
}

func (i *ipFilters) reloadRules(spec *ipfilter.Spec) {
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
		ipFilter: newIPFilter(spec),
	}
	i.rules.Store(rules)
}

func newIPFilter(spec *ipfilter.Spec) *ipfilter.IPFilter {
	if spec == nil {
		return nil
	}
	return ipfilter.New(spec)
}

func (r *ipFiltersRules) pass(downstreamIP string) bool {
	if r.ipFilter == nil {
		return true
	}
	return r.ipFilter.Allow(downstreamIP)
}
