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
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/megaease/easegress/pkg/object/globalfilter"
	"github.com/megaease/easegress/pkg/protocols/httpprot"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/autocertmanager"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

type (
	mux struct {
		httpStat *httpstat.HTTPStat
		topN     *httpstat.TopN

		inst atomic.Value // *muxInstance
	}

	muxInstance struct {
		superSpec *supervisor.Spec
		spec      *Spec
		httpStat  *httpstat.HTTPStat
		topN      *httpstat.TopN

		muxMapper context.MuxMapper

		cache *lru.ARCCache

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

	route struct {
		code int
		path *MuxPath
	}
)

var (
	notFound         = &route{code: http.StatusNotFound}
	forbidden        = &route{code: http.StatusForbidden}
	methodNotAllowed = &route{code: http.StatusMethodNotAllowed}
	badRequest       = &route{code: http.StatusBadRequest}
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

func allowIP(ipFilter *ipfilter.IPFilter, ip string) bool {
	if ipFilter == nil {
		return true
	}

	return ipFilter.Allow(ip)
}

func (mi *muxInstance) getCacheRoute(req *httpprot.Request) *route {
	if mi.cache != nil {
		key := stringtool.Cat(req.Host(), req.Method(), req.Path())
		if value, ok := mi.cache.Get(key); ok {
			return value.(*route)
		}
	}
	return nil
}

func (mi *muxInstance) putRouteToCache(req *httpprot.Request, r *route) {
	if mi.cache != nil {
		key := stringtool.Cat(req.Host(), req.Method(), req.Path())
		mi.cache.Add(key, r)
	}
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

func (mp *MuxPath) matchPath(r *httpprot.Request) bool {
	if mp.path == "" && mp.pathPrefix == "" && mp.pathRE == nil {
		return true
	}

	path := r.Path()
	if mp.path != "" && mp.path == path {
		return true
	}
	if mp.pathPrefix != "" && strings.HasPrefix(path, mp.pathPrefix) {
		return true
	}
	if mp.pathRE != nil {
		return mp.pathRE.MatchString(path)
	}

	return false
}

func (mp *MuxPath) rewrite(r *httpprot.Request) {
	if mp.rewriteTarget == "" {
		return
	}

	path := r.Path()

	if mp.path != "" && mp.path == path {
		r.SetPath(mp.rewriteTarget)
		return
	}

	if mp.pathPrefix != "" && strings.HasPrefix(path, mp.pathPrefix) {
		path = mp.rewriteTarget + path[len(mp.pathPrefix):]
		r.SetPath(path)
		return
	}

	// sure (mp.pathRE != nil && mp.pathRE.MatchString(path)) is true
	path = mp.pathRE.ReplaceAllString(path, mp.rewriteTarget)
	r.SetPath(path)
}

func (mp *MuxPath) matchMethod(r *httpprot.Request) bool {
	if len(mp.methods) == 0 {
		return true
	}

	return stringtool.StrInSlice(r.Method(), mp.methods)
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

	m.inst.Store(&muxInstance{
		spec:      &Spec{},
		tracer:    tracing.NoopTracing,
		muxMapper: mapper,
		httpStat:  httpStat,
		topN:      topN,
	})

	return m
}

func (m *mux) reload(superSpec *supervisor.Spec, muxMapper context.MuxMapper) {
	spec := superSpec.ObjectSpec().(*Spec)

	tracer := tracing.NoopTracing
	oldInst := m.inst.Load().(*muxInstance)
	if !reflect.DeepEqual(oldInst.spec.Tracing, spec.Tracing) {
		defer func() {
			err := oldInst.tracer.Close()
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
	} else if oldInst.tracer != nil {
		tracer = oldInst.tracer
	}

	inst := &muxInstance{
		superSpec:    superSpec,
		spec:         spec,
		muxMapper:    muxMapper,
		httpStat:     m.httpStat,
		topN:         m.topN,
		ipFilter:     newIPFilter(spec.IPFilter),
		ipFilterChan: newIPFilterChain(nil, spec.IPFilter),
		rules:        make([]*muxRule, len(spec.Rules)),
		tracer:       tracer,
	}

	if spec.CacheSize > 0 {
		arc, err := lru.NewARC(int(spec.CacheSize))
		if err != nil {
			logger.Errorf("BUG: new arc cache failed: %v", err)
		}
		inst.cache = arc
	}

	for i := 0; i < len(inst.rules); i++ {
		specRule := spec.Rules[i]

		ruleIPFilterChain := newIPFilterChain(inst.ipFilterChan, specRule.IPFilter)

		paths := make([]*MuxPath, len(specRule.Paths))
		for j := 0; j < len(paths); j++ {
			paths[j] = newMuxPath(ruleIPFilterChain, specRule.Paths[j])
		}

		// NOTE: Given the parent ipFilters not its own.
		inst.rules[i] = newMuxRule(inst.ipFilterChan, specRule, paths)
	}

	m.inst.Store(inst)
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

	// Forward to the current muxInstance to handle the request.
	m.inst.Load().(*muxInstance).serveHTTP(stdw, stdr)
}

func buildFailureResponse(ctx *context.Context, statusCode int) *httpprot.Response {
	resp, _ := httpprot.NewResponse(nil)
	resp.SetStatusCode(statusCode)
	ctx.SetResponse(context.DefaultResponseID, resp)
	return resp
}

func (mi *muxInstance) serveHTTP(stdw http.ResponseWriter, stdr *http.Request) {
	// The body of the original request maybe changed by handlers, we
	// need to restore it before the return of this funtion to make
	// sure it can be correctly closed by the standard Go HTTP package.
	originalBody := stdr.Body
	bodySize := -1

	startAt := fasttime.Now()
	span := tracing.NewSpanWithStart(mi.tracer, mi.superSpec.Name(), startAt)
	ctx := context.New(span)

	// httpprot.NewRequest never returns an error.
	req, _ := httpprot.NewRequest(stdr)
	ctx.SetRequest(context.InitialRequestID, req)

	// get topN here, as the path could be modified later.
	topN := mi.topN.Stat(req.Path())

	defer func() {
		// If FetchPayload is not called yet.
		if bodySize == -1 {
			written, _ := io.Copy(io.Discard, originalBody)
			bodySize = int(written)
		}

		var resp *httpprot.Response
		if v := ctx.Response(); v != nil {
			resp = v.(*httpprot.Response)
		} else {
			resp = buildFailureResponse(ctx, http.StatusInternalServerError)
		}

		// Send the response.
		header := stdw.Header()
		for k, v := range resp.HTTPHeader() {
			header[k] = v
		}
		stdw.WriteHeader(resp.StatusCode())
		io.Copy(stdw, resp.GetPayload())

		ctx.Finish()

		metric := httpstat.Metric{
			StatusCode: resp.StatusCode(),
			Duration:   fasttime.Since(startAt),
			ReqSize:    uint64(bodySize + req.MetaSize()),
			RespSize:   uint64(resp.MetaSize() + resp.PayloadLength()),
		}
		topN.Stat(&metric)
		mi.httpStat.Stat(&metric)

		span.Finish()

		// Restore the body of the origin request.
		stdr.Body = originalBody

		// Write access log.
		logger.LazyHTTPAccess(func() string {
			// log format:
			//
			// [$startTime]
			// [$remoteAddr $realIP $method $requestURL $proto $statusCode]
			// [$contextDuration $readBytes $writeBytes]
			// [$tags]
			const logFmt = "[%s] [%s %s %s %s %s %d] [%v rx:%dB tx:%dB] [%s]"
			return fmt.Sprintf(logFmt,
				fasttime.Format(startAt, fasttime.RFC3339Milli),
				stdr.RemoteAddr, req.RealIP(), stdr.Method, stdr.RequestURI,
				stdr.Proto, resp.StatusCode(), metric.Duration, metric.ReqSize,
				metric.RespSize, ctx.Tags())
		})
	}()

	route := mi.search(req)
	if route.code != 0 {
		ctx.AddTag(fmt.Sprintf("status code: %d", route.code))
		buildFailureResponse(ctx, route.code)
		return
	}

	handler, ok := mi.muxMapper.GetHandler(route.path.backend)
	if !ok {
		ctx.AddTag(stringtool.Cat("backend ", route.path.backend, " not found"))
		buildFailureResponse(ctx, http.StatusServiceUnavailable)
		return
	}

	route.path.rewrite(req)
	if mi.spec.XForwardedFor {
		mi.appendXForwardedFor(req)
	}

	bodySize, err := req.FetchPayload()
	if err != nil {
		ctx.AddTag(fmt.Sprintf("failed to read request body: %v", err))
		buildFailureResponse(ctx, http.StatusBadRequest)
		return
	}

	// global filter
	globalFilter := mi.getGlobalFilter()
	if globalFilter == nil {
		handler.Handle(ctx)
	} else {
		globalFilter.Handle(ctx, handler)
	}
}

func (mi *muxInstance) search(req *httpprot.Request) *route {
	headerMismatch, methodMismatch := false, false

	ip := req.RealIP()

	// The key of the cache is req.Host + req.Method + req.URL.Path,
	// and if a path is cached, we are sure it does not contain any
	// headers.
	r := mi.getCacheRoute(req)
	if r != nil {
		if r.code != 0 {
			return r
		}
		if r.path.ipFilterChain == nil {
			return r
		}
		if r.path.ipFilterChain.Allow(ip) {
			return r
		}
		return forbidden
	}

	if !allowIP(mi.ipFilter, ip) {
		return forbidden
	}

	for _, host := range mi.rules {
		if !host.match(req) {
			continue
		}

		if !allowIP(host.ipFilter, ip) {
			return forbidden
		}

		for _, path := range host.paths {
			if !path.matchPath(req) {
				continue
			}

			if !path.matchMethod(req) {
				methodMismatch = true
				continue
			}

			// The path can be put into the cache if it has no headers.
			if len(path.headers) == 0 {
				r = &route{code: 0, path: path}
				mi.putRouteToCache(req, r)
			} else if !path.matchHeaders(req) {
				headerMismatch = true
				continue
			}

			if !allowIP(path.ipFilter, ip) {
				return forbidden
			}

			return r
		}
	}

	if headerMismatch {
		return badRequest
	}

	if methodMismatch {
		mi.putRouteToCache(req, methodNotAllowed)
		return methodNotAllowed
	}

	mi.putRouteToCache(req, notFound)
	return notFound
}

func (mi *muxInstance) appendXForwardedFor(r *httpprot.Request) {
	const xForwardedFor = "X-Forwarded-For"

	v := r.HTTPHeader().Get(xForwardedFor)
	ip := r.RealIP()

	if v == "" {
		r.Header().Add(xForwardedFor, ip)
		return
	}

	if !strings.Contains(v, ip) {
		v = stringtool.Cat(v, ",", ip)
		r.Header().Set(xForwardedFor, v)
	}
}

func (mi *muxInstance) getGlobalFilter() *globalfilter.GlobalFilter {
	if mi.spec.GlobalFilter == "" {
		return nil
	}
	globalFilter, ok := mi.superSpec.Super().GetBusinessController(mi.spec.GlobalFilter)
	if globalFilter == nil || !ok {
		return nil
	}
	globalFilterInstance, ok := globalFilter.Instance().(*globalfilter.GlobalFilter)
	if !ok {
		return nil
	}
	return globalFilterInstance
}

func (mi *muxInstance) close() {
	if err := mi.tracer.Close(); err != nil {
		logger.Errorf("%s close tracer failed: %v", mi.superSpec.Name(), err)
	}
}

func (m *mux) close() {
	m.inst.Load().(*muxInstance).close()
}
