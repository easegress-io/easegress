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
	"io"
	"net"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/object/globalfilter"
	"github.com/megaease/easegress/pkg/protocols/httpprot"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/autocertmanager"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

type (
	mux struct {
		httpStat *httpstat.HTTPStat
		topN     *httpstat.TopN

		rules atomic.Value // *muxRules
	}

	muxRules struct {
		superSpec *supervisor.Spec
		spec      *Spec

		muxMapper context.MuxMapper

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

func (mr *muxRules) pass(r *httpprot.Request) bool {
	if mr.ipFilter == nil {
		return true
	}

	return mr.ipFilter.Allow(r.RealIP())
}

func (mr *muxRules) getCacheItem(r *httpprot.Request) *cacheItem {
	if mr.cache == nil {
		return nil
	}

	key := stringtool.Cat(r.Host(), r.Method(), r.Path())
	return mr.cache.get(key)
}

func (mr *muxRules) putCacheItem(r *httpprot.Request, ci *cacheItem) {
	if mr.cache == nil || ci.cached {
		return
	}

	ci.cached = true
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

func (mr *muxRule) pass(r *httpprot.Request) bool {
	if mr.ipFilter == nil {
		return true
	}

	return mr.ipFilter.Allow(r.RealIP())
}

func (mr *muxRule) match(r *httpprot.Request) bool {
	if mr.host == "" && mr.hostRE == nil {
		return true
	}

	host := r.Host()
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

func (mp *MuxPath) pass(r *httpprot.Request) bool {
	if mp.ipFilter == nil {
		return true
	}

	return mp.ipFilter.Allow(r.RealIP())
}

func (mp *MuxPath) matchPath(r *httpprot.Request) bool {
	if mp.path == "" && mp.pathPrefix == "" && mp.pathRE == nil {
		return true
	}

	if mp.path != "" && mp.path == r.Path() {
		return true
	}
	if mp.pathPrefix != "" && strings.HasPrefix(r.Path(), mp.pathPrefix) {
		return true
	}
	if mp.pathRE != nil {
		return mp.pathRE.MatchString(r.Path())
	}

	return false
}

func (mp *MuxPath) matchMethod(r *httpprot.Request) bool {
	if len(mp.methods) == 0 {
		return true
	}

	return stringtool.StrInSlice(r.Method(), mp.methods)
}

func (mp *MuxPath) hasHeaders() bool {
	return len(mp.headers) > 0
}

func (mp *MuxPath) matchHeaders(r *httpprot.Request) bool {
	for _, h := range mp.headers {
		v := r.HTTPHeader().Get(h.Key)
		if stringtool.StrInSlice(v, h.Values) {
			return true
		}

		if h.Regexp != "" && h.headerRE.MatchString(v) {
			return true
		}
	}

	return false
}

func newMux(httpStat *httpstat.HTTPStat, topN *httpstat.TopN, mapper context.MuxMapper) *mux {
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

func (m *mux) reloadRules(superSpec *supervisor.Spec, muxMapper context.MuxMapper) {
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
	// HTTP-01 challenges requires HTTP server to listen on port 80, but we
	// don't know which HTTP server listen on this port (consider there's an
	// nginx sitting in front of Easegress), so all HTTP servers need to
	// handle HTTP-01 challenges.
	if strings.HasPrefix(stdr.URL.Path, "/.well-known/acme-challenge/") {
		autocertmanager.HandleHTTP01Challenge(stdw, stdr)
		return
	}

	// The body of the original request maybe changed by handlers, we
	// need to restore it before the return of this funtion to make
	// sure it can be correctly closed by the standard Go HTTP package.
	originalBody := stdr.Body

	// Get the mux rules to use.
	rules := m.rules.Load().(*muxRules)

	// Create the context.
	req := httpprot.NewRequest(stdr)
	resp := httpprot.NewResponse(nil)
	ctx := context.New(req, resp, rules.tracer, rules.superSpec.Name())

	defer func() {
		// flush the response, note ctx.Response may return a different
		// response from the one we used to create the context.
		resp = ctx.Response().(*httpprot.Response)
		header := stdw.Header()
		for k, v := range resp.HTTPHeader() {
			header[k] = v
		}
		stdw.WriteHeader(resp.StatusCode())
		io.Copy(stdw, resp.GetPayload())

		ctx.Finish()

		// TODO: should be called in ctx.Finish()
		ctx.Span().Finish()

		// TODO:
		//	m.httpStat.Stat(ctx.StatMetric())
		m.topN.Stat(ctx)

		// restore the body of the origin request.
		stdr.Body = originalBody
	}()

	ci := rules.getCacheItem(req)
	if ci != nil {
		m.handleRequestWithCache(rules, ctx, ci)
		return
	}

	if !rules.pass(req) {
		m.handleIPNotAllow(ctx)
		return
	}

	handleNotFound := func() {
		ci = &cacheItem{ipFilterChan: rules.ipFilterChan, notFound: true}
		rules.putCacheItem(req, ci)
		m.handleRequestWithCache(rules, ctx, ci)
	}

	result, path := SearchPath(req, rules.rules)
	switch result {
	case Found:
		ci = &cacheItem{ipFilterChan: path.ipFilterChain, path: path}
		rules.putCacheItem(req, ci)
		m.handleRequestWithCache(rules, ctx, ci)
	case FoundSkipCache:
		ci := &cacheItem{ipFilterChan: path.ipFilterChain, path: path}
		m.handleRequestWithCache(rules, ctx, ci)
	case MethodNotAllowed:
		ci := &cacheItem{ipFilterChan: path.ipFilterChain, methodNotAllowed: true}
		rules.putCacheItem(req, ci)
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
func SearchPath(req *httpprot.Request, rulesToCheck []*muxRule) (SearchResult, *MuxPath) {
	pathFound := false
	var notAllowedPath *MuxPath
	for _, host := range rulesToCheck {
		if !host.match(req) {
			continue
		}

		if !host.pass(req) {
			return IPNotAllowed, nil
		}

		for _, path := range host.paths {
			if !path.matchPath(req) {
				continue
			}

			// at least one path matches
			pathFound = true
			notAllowedPath = path
			if !path.matchMethod(req) {
				continue
			}

			var searchResult SearchResult

			if path.hasHeaders() {
				if !path.matchHeaders(req) {
					continue
				}
				searchResult = FoundSkipCache
			} else {
				searchResult = Found
			}

			if !path.pass(req) {
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

func (m *mux) handleIPNotAllow(ctx *context.Context) {
	req := ctx.Request().(*httpprot.Request)
	resp := ctx.Response().(*httpprot.Response)
	ctx.AddTag(stringtool.Cat("ip ", req.RealIP(), " not allow"))
	resp.SetStatusCode(http.StatusForbidden)
}

func (m *mux) handleRequestWithCache(rules *muxRules, ctx *context.Context, ci *cacheItem) {
	req := ctx.Request().(*httpprot.Request)
	resp := ctx.Response().(*httpprot.Response)

	if ci.ipFilterChan != nil {
		if !ci.ipFilterChan.Allow(req.RealIP()) {
			m.handleIPNotAllow(ctx)
			return
		}
	}

	switch {
	case ci.notFound:
		resp.SetStatusCode(http.StatusNotFound)
	case ci.methodNotAllowed:
		resp.SetStatusCode(http.StatusMethodNotAllowed)
	case ci.path != nil:
		handler, exists := rules.muxMapper.GetHandler(ci.path.backend)
		if !exists {
			ctx.AddTag(stringtool.Cat("backend ", ci.path.backend, " not found"))
			resp.SetStatusCode(http.StatusServiceUnavailable)
			return
		}

		if rules.spec.XForwardedFor {
			m.appendXForwardedFor(req)
		}

		if ci.path.pathRE != nil && ci.path.rewriteTarget != "" {
			path := req.Path()
			path = ci.path.pathRE.ReplaceAllString(path, ci.path.rewriteTarget)
			req.SetPath(path)
		}
		// global filter
		globalFilter := m.getGlobalFilter(rules)
		if globalFilter == nil {
			handler.Handle(ctx)
			return
		}
		globalFilter.Handle(ctx, handler)
	}
}

func (m *mux) appendXForwardedFor(r *httpprot.Request) {
	v := r.HTTPHeader().Get(httpprot.KeyXForwardedFor)
	ip := r.RealIP()

	if v == "" {
		r.Header().Add(httpprot.KeyXForwardedFor, ip)
		return
	}

	if !strings.Contains(v, ip) {
		v = stringtool.Cat(v, ",", ip)
		r.Header().Set(httpprot.KeyXForwardedFor, v)
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
