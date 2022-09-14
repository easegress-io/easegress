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

package grpcserver

import (
	"fmt"
	"github.com/megaease/easegress/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"regexp"
	"strings"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/globalfilter"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/ipfilter"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

type (
	// mux impl grpc proxy by grpc.UnknownServiceHandler
	mux struct {
		inst atomic.Value // *muxInstance
	}

	muxInstance struct {
		superSpec *supervisor.Spec
		spec      *Spec

		muxMapper context.MuxMapper

		cache *lru.ARCCache

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

		path           string
		pathPrefix     string
		pathRegexp     string
		pathRE         *regexp.Regexp
		rewriteTarget  string
		backend        string
		headers        []*Header
		matchAllHeader bool
	}

	route struct {
		code    codes.Code
		message string
		path    *MuxPath
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

func allowIP(ipFilter *ipfilter.IPFilter, ip string) bool {
	if ipFilter == nil {
		return true
	}

	return ipFilter.Allow(ip)
}

func (mi *muxInstance) getRouteFromCache(host, path string) *route {
	if mi.cache != nil {
		key := stringtool.Cat(host, path)
		if value, ok := mi.cache.Get(key); ok {
			return value.(*route)
		}
	}
	return nil
}

func (mi *muxInstance) putRouteToCache(host, path string, r *route) {
	if mi.cache != nil && host != "" && path != "" {
		key := stringtool.Cat(host, path)
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
			logger.Errorf("BUG: compile %s failed: %v", rule.HostRegexp, err)
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

func (mr *muxRule) match(host string) bool {
	if mr.host == "" && mr.hostRE == nil {
		return true
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
			logger.Errorf("BUG: compile %s failed: %v", path.PathRegexp, err)
		}
	}

	for _, p := range path.Headers {
		p.initHeaderRoute()
	}

	return &MuxPath{
		ipFilter:      newIPFilter(path.IPFilter),
		ipFilterChain: newIPFilterChain(parentIPFilters, path.IPFilter),

		path:           path.Path,
		pathPrefix:     path.PathPrefix,
		pathRegexp:     path.PathRegexp,
		pathRE:         pathRE,
		rewriteTarget:  path.RewriteTarget,
		backend:        path.Backend,
		headers:        path.Headers,
		matchAllHeader: path.MatchAllHeader,
	}
}

func (mp *MuxPath) matchPath(path string) bool {
	if mp.path == "" && mp.pathPrefix == "" && mp.pathRE == nil {
		return true
	}

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

func (mp *MuxPath) rewrite(r *grpcprot.Request) {
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

func matchHeader(header string, h *Header) bool {
	if stringtool.StrInSlice(header, h.Values) {
		return true
	}

	if h.Regexp != "" && h.headerRE.MatchString(header) {
		return true
	}
	return false
}

func (mp *MuxPath) matchHeaders(r *grpcprot.Request) bool {
	if mp.matchAllHeader {
		for _, h := range mp.headers {
			v := r.RawHeader().RawGet(h.Key)
			if len(v) == 0 {
				if !matchHeader("", h) {
					return false
				}
			} else {
				for _, vv := range v {
					if !matchHeader(vv, h) {
						return false
					}
				}
			}
		}
	} else {
		for _, h := range mp.headers {
			v := r.RawHeader().RawGet(h.Key)
			if len(v) == 0 {
				if matchHeader("", h) {
					return true
				}
			} else {
				for _, vv := range v {
					if matchHeader(vv, h) {
						return true
					}
				}
			}
		}
	}

	return mp.matchAllHeader
}

func newMux(mapper context.MuxMapper) *mux {
	m := &mux{}

	m.inst.Store(&muxInstance{
		spec:      &Spec{},
		muxMapper: mapper,
	})

	return m
}

func (m *mux) reload(superSpec *supervisor.Spec, muxMapper context.MuxMapper) {
	spec := superSpec.ObjectSpec().(*Spec)

	inst := &muxInstance{
		superSpec:    superSpec,
		spec:         spec,
		muxMapper:    muxMapper,
		ipFilter:     newIPFilter(spec.IPFilter),
		ipFilterChan: newIPFilterChain(nil, spec.IPFilter),
		rules:        make([]*muxRule, len(spec.Rules)),
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

// handler impl grpc.UnknownServiceHandler
func (m *mux) handler(srv interface{}, proxyAsServerStream grpc.ServerStream) error {
	// Forward to the current muxInstance to handle the request.
	c := make(chan error, 1)
	go m.inst.Load().(*muxInstance).handler(c, grpcprot.NewRequestWithServerStream(proxyAsServerStream))
	select {
	case <-proxyAsServerStream.Context().Done():
		return status.Errorf(codes.Canceled, "client cancelled, abandoning...")
	case v := <-c:
		close(c)
		return v
	}
}

func buildFailureResponse(ctx *context.Context, s *status.Status) *grpcprot.Response {
	resp := grpcprot.NewResponse()
	resp.SetStatus(s)
	ctx.SetResponse(context.DefaultNamespace, resp)
	return resp
}

func (mi *muxInstance) handler(c chan<- error, request *grpcprot.Request) {
	startAt := fasttime.Now()
	ctx := context.New(tracing.NoopSpan)
	ctx.SetRequest(context.DefaultNamespace, request)

	defer func() {
		var resp *grpcprot.Response
		if err := recover(); err != nil {
			logger.Warnf("gRPC server %s: panic %v", mi.superSpec.Name(), err)
			resp = buildFailureResponse(ctx, status.Newf(codes.Internal, "gRPC server %s: panic", mi.superSpec.Name()))
		} else if v := ctx.GetResponse(context.DefaultNamespace); v == nil {
			resp = buildFailureResponse(ctx, status.Newf(codes.Internal, "gRPC server %s: response is nil ", mi.superSpec.Name()))
		} else if r, ok := v.(*grpcprot.Response); !ok {
			resp = buildFailureResponse(ctx, status.Newf(codes.Internal, "gRPC server %s: response type is %T", mi.superSpec.Name(), v))
		} else {
			resp = r
		}

		c <- resp.Err()
		ctx.Finish()
		// Write access log.
		logger.LazyHTTPAccess(func() string {
			// log format:
			//
			// [$startTime]
			// [$clientAddr $path $statusCode]
			// [$tags]
			const logFmt = "[grpc][%s] [%s %s %d] [%s]"
			return fmt.Sprintf(logFmt,
				fasttime.Format(startAt, fasttime.RFC3339Milli),
				request.SourceHost(), request.Path(), resp.StatusCode(), ctx.Tags())
		})
	}()

	rt := mi.search(request)
	if rt.code != 0 {
		logger.Debugf("%s: status result of search route: %+v", mi.superSpec.Name(), rt.code)
		buildFailureResponse(ctx, status.New(rt.code, rt.message))
		return
	}

	handler, ok := mi.muxMapper.GetHandler(rt.path.backend)
	if !ok {
		logger.Debugf("%s: backend %q not found", mi.superSpec.Name(), rt.path.backend)
		buildFailureResponse(ctx, status.Newf(codes.NotFound, "%s: backend %q not found", mi.superSpec.Name(), rt.path.backend))
		return
	}

	rt.path.rewrite(request)
	if mi.spec.XForwardedFor {
		appendXForwardedFor(request)
	}

	// global filter
	globalFilter := mi.getGlobalFilter()

	if globalFilter == nil {
		handler.Handle(ctx)
	} else {
		globalFilter.Handle(ctx, handler)
	}

}

func (mi *muxInstance) search(request *grpcprot.Request) *route {
	headerMismatch := false
	ip := request.RealIP()
	if ip == "" {
		logger.Debugf("invalid gRPC stream, can not get client IP address")
		return &route{
			code:    codes.PermissionDenied,
			message: "invalid gRPC stream, can not get client IP address",
		}
	}

	// grpc's method equals request.path in standard lib
	method := request.Path()
	if method == "" {
		logger.Debugf("invalid grpc stream: can not get called method info")
		return &route{
			code:    codes.NotFound,
			message: "invalid grpc stream: can not get called method info",
		}
	}

	// The key of the cache is grpc server address + called method
	// in grpc, called method means url path
	// and if a path is cached, we are sure it does not contain any
	// headers.
	r := mi.getRouteFromCache(request.Host(), method)
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
		return &route{
			code:    codes.PermissionDenied,
			message: "request isn't allowed",
		}
	}

	if !allowIP(mi.ipFilter, ip) {
		return &route{
			code:    codes.PermissionDenied,
			message: "request isn't allowed",
		}
	}

	for _, rs := range mi.rules {
		if !rs.match(request.Host()) {
			continue
		}

		if !allowIP(rs.ipFilter, ip) {
			return &route{
				code:    codes.PermissionDenied,
				message: "request isn't allowed",
			}
		}

		for _, path := range rs.paths {
			if !path.matchPath(method) {
				continue
			}

			// The path can be put into the cache if it has no headers.
			if len(path.headers) == 0 {
				r = &route{code: 0, path: path}
				mi.putRouteToCache(request.Host(), method, r)
			} else if !path.matchHeaders(request) {
				headerMismatch = true
				continue
			}

			if !allowIP(path.ipFilter, ip) {
				return &route{
					code:    codes.PermissionDenied,
					message: "request isn't allowed",
				}
			}

			return &route{code: 0, path: path}
		}
	}

	if headerMismatch {
		return &route{
			code:    codes.NotFound,
			message: "grpc stream header mismatch",
		}
	}

	notFound := &route{
		code:    codes.NotFound,
		message: "grpc stream miss match any conditions",
	}
	mi.putRouteToCache(request.Host(), method, notFound)
	return notFound
}

func appendXForwardedFor(r *grpcprot.Request) {
	const xForwardedFor = "X-Forwarded-For"

	v := r.RawHeader().RawGet(xForwardedFor)
	ip := r.RealIP()

	if v == nil {
		r.RawHeader().Set(xForwardedFor, ip)
		return
	}

	for _, vv := range v {
		if vv == ip {
			return
		}
	}

	r.Header().Add(xForwardedFor, ip)

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

}

func (m *mux) close() {
	m.inst.Load().(*muxInstance).close()
}
