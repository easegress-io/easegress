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

package httpserver

import (
	"net"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/object/globalfilter"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/autocertmanager"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/httpstat"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"github.com/megaease/easegress/pkg/util/topn"
)

type (
	mux struct {
		httpStat *httpstat.HTTPStat
		topN     *topn.TopN

		rules atomic.Value // *muxRules
	}

	muxRules struct {
		superSpec *supervisor.Spec
		spec      *Spec

		muxMapper protocol.MuxMapper

		cache *cache

		tracer       *tracing.Tracing
		ipFilter     *ipfilter.IPFilter
		ipFilterChan *ipfilter.IPFilters

		rules []*muxRule
	}

	muxRule struct {
		ipFilter      *ipfilter.IPFilter
		ipFilterChain *ipfilter.IPFilters

		host       string
		hostRegexp string
		hostRE     *regexp.Regexp
		paths      []*MuxPath
	}

	// MuxPath describes httpserver's path
	MuxPath struct {
		ipFilter      *ipfilter.IPFilter
		ipFilterChain *ipfilter.IPFilters

		path          string
		pathPrefix    string
		pathRegexp    string
		pathRE        *regexp.Regexp
		methods       []string
		rewriteTarget string
		backend       string
		headers       []*Header
	}

	// SearchResult is returned by SearchPath
	SearchResult string
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

func (mr *muxRules) pass(ctx context.HTTPContext) bool {
	if mr.ipFilter == nil {
		return true
	}

	return mr.ipFilter.AllowHTTPContext(ctx)
}

func (mr *muxRules) getCacheItem(ctx context.HTTPContext) *cacheItem {
	if mr.cache == nil {
		return nil
	}

	r := ctx.Request()
	key := stringtool.Cat(r.Host(), r.Method(), r.Path())
	return mr.cache.get(key)
}

func (mr *muxRules) putCacheItem(ctx context.HTTPContext, ci *cacheItem) {
	if mr.cache == nil || ci.cached {
		return
	}

	ci.cached = true
	r := ctx.Request()
	key := stringtool.Cat(r.Host(), r.Method(), r.Path())
	// NOTE: It's fine to cover the existed item because of concurrently updating cache.
	mr.cache.put(key, ci)
}

func newMuxRule(parentIPFilters *ipfilter.IPFilters, rule *Rule, paths []*MuxPath) *muxRule {
	var hostRE *regexp.Regexp

	if rule.HostRegexp != "" {
		var err error
		hostRE, err = regexp.Compile(rule.HostRegexp)
		// defensive programming
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v",
				rule.HostRegexp, err)
		}
	}

	return &muxRule{
		ipFilter:      newIPFilter(rule.IPFilter),
		ipFilterChain: newIPFilterChain(parentIPFilters, rule.IPFilter),

		host:       rule.Host,
		hostRegexp: rule.HostRegexp,
		hostRE:     hostRE,
		paths:      paths,
	}
}

func (mr *muxRule) pass(ctx context.HTTPContext) bool {
	if mr.ipFilter == nil {
		return true
	}

	return mr.ipFilter.AllowHTTPContext(ctx)
}

func (mr *muxRule) match(ctx context.HTTPContext) bool {
	if mr.host == "" && mr.hostRE == nil {
		return true
	}

	host := ctx.Request().Host()
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	if mr.host != "" && mr.host == host {
		return true
	}
	if mr.hostRE != nil && mr.hostRE.MatchString(host) {
		return true
	}

	return false
}

func newMuxPath(parentIPFilters *ipfilter.IPFilters, path *Path) *MuxPath {
	var pathRE *regexp.Regexp
	if path.PathRegexp != "" {
		var err error
		pathRE, err = regexp.Compile(path.PathRegexp)
		// defensive programming
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v",
				path.PathRegexp, err)
		}
	}

	for _, p := range path.Headers {
		p.initHeaderRoute()
	}

	return &MuxPath{
		ipFilter:      newIPFilter(path.IPFilter),
		ipFilterChain: newIPFilterChain(parentIPFilters, path.IPFilter),

		path:          path.Path,
		pathPrefix:    path.PathPrefix,
		pathRegexp:    path.PathRegexp,
		pathRE:        pathRE,
		rewriteTarget: path.RewriteTarget,
		methods:       path.Methods,
		backend:       path.Backend,
		headers:       path.Headers,
	}
}

func (mp *MuxPath) pass(ctx context.HTTPContext) bool {
	if mp.ipFilter == nil {
		return true
	}

	return mp.ipFilter.AllowHTTPContext(ctx)
}

func (mp *MuxPath) matchPath(ctx context.HTTPContext) bool {
	r := ctx.Request()

	if mp.path == "" && mp.pathPrefix == "" && mp.pathRE == nil {
		return true
	}

	if mp.path != "" && mp.path == r.Path() {
		return true
	}

	if mp.pathPrefix != "" && strings.HasPrefix(r.Path(), mp.pathPrefix) {
		return true
	}

	if mp.pathRE != nil && mp.pathRE.MatchString(r.Path()) {
		return true
	}

	return false
}

func (mp *MuxPath) handleRewrite(ctx context.HTTPContext) {
	if mp.rewriteTarget == "" {
		return
	}

	path := ctx.Request().Path()

	if mp.path != "" && mp.path == path {
		ctx.Request().SetPath(mp.rewriteTarget)
		return
	}

	if mp.pathPrefix != "" && strings.HasPrefix(path, mp.pathPrefix) {
		path = mp.rewriteTarget + path[len(mp.pathPrefix):]
		ctx.Request().SetPath(path)
		return
	}

	// sure (mp.pathRE != nil && mp.pathRE.MatchString(path)) is true
	path = mp.pathRE.ReplaceAllString(path, mp.rewriteTarget)
	ctx.Request().SetPath(path)
}

func (mp *MuxPath) matchMethod(ctx context.HTTPContext) bool {
	if len(mp.methods) == 0 {
		return true
	}

	return stringtool.StrInSlice(ctx.Request().Method(), mp.methods)
}

func (mp *MuxPath) hasHeaders() bool {
	return len(mp.headers) > 0
}

func (mp *MuxPath) matchHeaders(ctx context.HTTPContext) bool {
	for _, h := range mp.headers {
		v := ctx.Request().Header().Get(h.Key)
		if stringtool.StrInSlice(v, h.Values) {
			return true
		}

		if h.Regexp != "" && h.headerRE.MatchString(v) {
			return true
		}
	}

	return false
}

func newMux(httpStat *httpstat.HTTPStat, topN *topn.TopN, mapper protocol.MuxMapper) *mux {
	m := &mux{
		httpStat: httpStat,
		topN:     topN,
	}

	m.rules.Store(&muxRules{
		spec:      &Spec{},
		tracer:    tracing.NoopTracing,
		muxMapper: mapper,
	})

	return m
}

func (m *mux) reloadRules(superSpec *supervisor.Spec, muxMapper protocol.MuxMapper) {
	spec := superSpec.ObjectSpec().(*Spec)

	tracer := tracing.NoopTracing
	oldRules := m.rules.Load().(*muxRules)
	if !reflect.DeepEqual(oldRules.spec.Tracing, spec.Tracing) {
		defer func() {
			err := oldRules.tracer.Close()
			if err != nil {
				logger.Errorf("close tracing failed: %v", err)
			}
		}()
		tracer0, err := tracing.New(spec.Tracing)
		if err != nil {
			logger.Errorf("create tracing failed: %v", err)
		} else {
			tracer = tracer0
		}
	} else if oldRules.tracer != nil {
		tracer = oldRules.tracer
	}

	rules := &muxRules{
		superSpec:    superSpec,
		spec:         spec,
		muxMapper:    muxMapper,
		ipFilter:     newIPFilter(spec.IPFilter),
		ipFilterChan: newIPFilterChain(nil, spec.IPFilter),
		rules:        make([]*muxRule, len(spec.Rules)),
		tracer:       tracer,
	}

	if spec.CacheSize > 0 {
		rules.cache = newCache(spec.CacheSize)
	}

	for i := 0; i < len(rules.rules); i++ {
		specRule := spec.Rules[i]

		ruleIPFilterChain := newIPFilterChain(rules.ipFilterChan, specRule.IPFilter)

		paths := make([]*MuxPath, len(specRule.Paths))
		for j := 0; j < len(paths); j++ {
			paths[j] = newMuxPath(ruleIPFilterChain, specRule.Paths[j])
		}

		// NOTE: Given the parent ipFilters not its own.
		rules.rules[i] = newMuxRule(rules.ipFilterChan, specRule, paths)
	}

	m.rules.Store(rules)
}

func (m *mux) ServeHTTP(stdw http.ResponseWriter, stdr *http.Request) {
	// HTTP-01 challenges requires HTTP server to listen on port 80, but we don't
	// know which HTTP server listen on this port (consider there's an nginx sitting
	// in front of Easegress), so all HTTP servers need to handle HTTP-01 challenges.
	if strings.HasPrefix(stdr.URL.Path, "/.well-known/acme-challenge/") {
		autocertmanager.HandleHTTP01Challenge(stdw, stdr)
		return
	}

	rules := m.rules.Load().(*muxRules)

	ctx := context.New(stdw, stdr, rules.tracer, rules.superSpec.Name())
	defer ctx.Finish()
	ctx.OnFinish(func() {
		ctx.Span().Finish()
		m.httpStat.Stat(ctx.StatMetric())
		m.topN.Stat(ctx)
	})

	ci := rules.getCacheItem(ctx)
	if ci != nil {
		m.handleRequestWithCache(rules, ctx, ci)
		return
	}

	if !rules.pass(ctx) {
		m.handleIPNotAllow(ctx)
		return
	}

	handleNotFound := func() {
		ci = &cacheItem{ipFilterChan: rules.ipFilterChan, notFound: true}
		rules.putCacheItem(ctx, ci)
		m.handleRequestWithCache(rules, ctx, ci)
	}

	result, path := SearchPath(ctx, rules.rules)
	switch result {
	case Found:
		ci = &cacheItem{ipFilterChan: path.ipFilterChain, path: path}
		rules.putCacheItem(ctx, ci)
		m.handleRequestWithCache(rules, ctx, ci)
	case FoundSkipCache:
		ci := &cacheItem{ipFilterChan: path.ipFilterChain, path: path}
		m.handleRequestWithCache(rules, ctx, ci)
	case MethodNotAllowed:
		ci := &cacheItem{ipFilterChan: path.ipFilterChain, methodNotAllowed: true}
		rules.putCacheItem(ctx, ci)
		m.handleRequestWithCache(rules, ctx, ci)
	case IPNotAllowed:
		m.handleIPNotAllow(ctx)
	case NotFound:
		handleNotFound()
	default:
		handleNotFound()
	}
}

const (
	// NotFound means no path found
	NotFound SearchResult = "not-found"
	// IPNotAllowed means context IP is not allowd
	IPNotAllowed SearchResult = "ip-not-allowed"
	// MethodNotAllowed means context method is not allowd
	MethodNotAllowed SearchResult = "method-not-allowed"
	// Found path
	Found SearchResult = "found"
	// FoundSkipCache means found path but skip caching result
	FoundSkipCache SearchResult = "found-skip-cache"
)

// SearchPath searches path among list of mux rules
func SearchPath(ctx context.HTTPContext, rulesToCheck []*muxRule) (SearchResult, *MuxPath) {
	pathFound := false
	var notAllowedPath *MuxPath
	for _, host := range rulesToCheck {
		if !host.match(ctx) {
			continue
		}

		if !host.pass(ctx) {
			return IPNotAllowed, nil
		}

		for _, path := range host.paths {
			if !path.matchPath(ctx) {
				continue
			}

			// at least one path matches
			pathFound = true
			notAllowedPath = path
			if !path.matchMethod(ctx) {
				continue
			}

			var searchResult SearchResult

			if path.hasHeaders() {
				if !path.matchHeaders(ctx) {
					continue
				}
				searchResult = FoundSkipCache
			} else {
				searchResult = Found
			}

			if !path.pass(ctx) {
				return IPNotAllowed, path
			}

			return searchResult, path
		}
	}
	if !pathFound {
		return NotFound, nil
	}
	return MethodNotAllowed, notAllowedPath
}

func (m *mux) handleIPNotAllow(ctx context.HTTPContext) {
	ctx.AddTag(stringtool.Cat("ip ", ctx.Request().RealIP(), " not allow"))
	ctx.Response().SetStatusCode(http.StatusForbidden)
}

func (m *mux) handleRequestWithCache(rules *muxRules, ctx context.HTTPContext, ci *cacheItem) {
	if ci.ipFilterChan != nil {
		if !ci.ipFilterChan.AllowHTTPContext(ctx) {
			m.handleIPNotAllow(ctx)
			return
		}
	}

	switch {
	case ci.notFound:
		ctx.Response().SetStatusCode(http.StatusNotFound)
	case ci.methodNotAllowed:
		ctx.Response().SetStatusCode(http.StatusMethodNotAllowed)
	case ci.path != nil:
		handler, exists := rules.muxMapper.GetHandler(ci.path.backend)
		if !exists {
			ctx.AddTag(stringtool.Cat("backend ", ci.path.backend, " not found"))
			ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
			return
		}

		if rules.spec.XForwardedFor {
			m.appendXForwardedFor(ctx)
		}

		ci.path.handleRewrite(ctx)

		// global filter
		globalFilter := m.getGlobalFilter(rules)
		if globalFilter == nil {
			handler.Handle(ctx)
			return
		}
		globalFilter.Handle(ctx, handler)
	}
}

func (m *mux) appendXForwardedFor(ctx context.HTTPContext) {
	v := ctx.Request().Header().Get(httpheader.KeyXForwardedFor)
	ip := ctx.Request().RealIP()

	if v == "" {
		ctx.Request().Header().Add(httpheader.KeyXForwardedFor, ip)
		return
	}

	if !strings.Contains(v, ip) {
		v = stringtool.Cat(v, ",", ip)
		ctx.Request().Header().Set(httpheader.KeyXForwardedFor, v)
	}
}

func (m *mux) getGlobalFilter(rules *muxRules) *globalfilter.GlobalFilter {
	if rules.spec.GlobalFilter == "" {
		return nil
	}
	globalFilter, ok := rules.superSpec.Super().GetBusinessController(rules.spec.GlobalFilter)
	if globalFilter == nil || !ok {
		return nil
	}
	globalFilterInstance, ok := globalFilter.Instance().(*globalfilter.GlobalFilter)
	if !ok {
		return nil
	}
	return globalFilterInstance
}

func (m *mux) close() {
	rules := m.rules.Load().(*muxRules)
	err := rules.tracer.Close()
	if err != nil {
		logger.Errorf("%s close tracer failed: %v",
			rules.superSpec.Name(), err)
	}
}
