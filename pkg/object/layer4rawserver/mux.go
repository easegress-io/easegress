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

package layer4rawserver

import (
	"net"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/util/ipfilter"
)

type (
	mux struct {
		rules atomic.Value // *muxRules
	}

	muxRules struct {
		superSpec *supervisor.Spec
		spec      *Spec

		muxMapper protocol.MuxMapper

		ipFilter     *ipfilter.IPFilter
		ipFilterChan *ipfilter.IPFilters
	}
)

// newIPFilterChain returns nil if the number of final filters is zero.
func newIPFilterChain(parentIPFilters *ipfilter.IPFilters, childSpec *ipfilter.Spec) *ipfilter.IPFilters {
	var ipFilters *ipfilter.IPFilters
	if parentIPFilters != nil {
		ipFilters = ipfilter.NewIPFilters(parentIPFilters.Filters()...)
	} else {
		ipFilters = ipfilter.NewIPFilters()
	}

	if childSpec != nil {
		ipFilters.Append(ipfilter.New(childSpec))
	}

	if len(ipFilters.Filters()) == 0 {
		return nil
	}
	return ipFilters
}

func newIPFilter(spec *ipfilter.Spec) *ipfilter.IPFilter {
	if spec == nil {
		return nil
	}

	return ipfilter.New(spec)
}

func (mr *muxRules) pass(ctx context.Layer4Context) bool {
	if mr.ipFilter == nil {
		return true
	}

	switch addr := ctx.ClientAddr().(type) {
	case *net.UDPAddr:
		return mr.ipFilter.Allow(addr.IP.String())
	case *net.TCPAddr:
		return mr.ipFilter.Allow(addr.IP.String())
	default:
		logger.Warnf("invalid remote addr type")
	}
	return false
}

func newMux(mapper protocol.MuxMapper) *mux {
	m := &mux{}

	m.rules.Store(&muxRules{
		spec:      &Spec{},
		muxMapper: mapper,
	})
	return m
}

func (m *mux) reloadRules(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	spec := superSpec.ObjectSpec().(*Spec)

	rules := &muxRules{
		superSpec:    superSpec,
		spec:         spec,
		muxMapper:    muxMapper,
		ipFilter:     newIPFilter(spec.IPFilter),
		ipFilterChan: newIPFilterChain(nil, spec.IPFilter),
	}
	m.rules.Store(rules)
}

func (m *mux) AllowIP(ipStr string) bool {
	rules := m.rules.Load().(*muxRules)
	if rules == nil {
		return true
	}
	return rules.ipFilter.Allow(ipStr)
}

func (m *mux) GetHandler(name string) (protocol.Layer4Handler, bool) {
	rules := m.rules.Load().(*muxRules)
	if rules == nil {
		return nil, false
	}
	return rules.muxMapper.GetLayer4Handler(name)
}

func (m *mux) close() {
	// TODO add close tracing
}
