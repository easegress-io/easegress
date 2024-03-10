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

package grpcserver

import (
	"fmt"
	"runtime/debug"

	"github.com/megaease/easegress/v2/pkg/protocols/grpcprot"
	"github.com/megaease/easegress/v2/pkg/util/fasttime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"regexp"
	"strings"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/globalfilter"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/megaease/easegress/v2/pkg/util/ipfilter"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
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
		methods    []*MuxMethod
	}

	// MuxMethod describes gRPCserver's method
	MuxMethod struct {
		ipFilter      *ipfilter.IPFilter
		ipFilterChain *ipfilter.IPFilters

		method         string
		methodPrefix   string
		methodRegexp   string
		methodRE       *regexp.Regexp
		backend        string
		headers        []*Header
		matchAllHeader bool
	}

	route struct {
		code    codes.Code
		message string
		method  *MuxMethod
	}
)

func (mi *muxInstance) getRouteFromCache(host, method string) *route {
	if mi.cache != nil {
		key := stringtool.Cat(host, method)
		if value, ok := mi.cache.Get(key); ok {
			return value.(*route)
		}
	}
	return nil
}

func (mi *muxInstance) putRouteToCache(host, method string, r *route) {
	if mi.cache != nil && host != "" && method != "" {
		key := stringtool.Cat(host, method)
		mi.cache.Add(key, r)
	}
}

func newMuxRule(parentIPFilters *ipfilter.IPFilters, rule *Rule, methods []*MuxMethod) *muxRule {
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
		ipFilter:      ipfilter.New(rule.IPFilter),
		ipFilterChain: ipfilter.NewIPFilterChain(parentIPFilters, rule.IPFilter),

		host:       rule.Host,
		hostRegexp: rule.HostRegexp,
		hostRE:     hostRE,
		methods:    methods,
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

func newMuxMethod(parentIPFilters *ipfilter.IPFilters, method *Method) *MuxMethod {
	var methodRE *regexp.Regexp
	if method.MethodRegexp != "" {
		var err error
		methodRE, err = regexp.Compile(method.MethodRegexp)
		// defensive programming
		if err != nil {
			logger.Errorf("BUG: compile %s failed: %v", method.MethodRegexp, err)
		}
	}

	for _, p := range method.Headers {
		p.initHeaderRoute()
	}

	return &MuxMethod{
		ipFilter:      ipfilter.New(method.IPFilter),
		ipFilterChain: ipfilter.NewIPFilterChain(parentIPFilters, method.IPFilter),

		method:         method.Method,
		methodPrefix:   method.MethodPrefix,
		methodRegexp:   method.MethodRegexp,
		methodRE:       methodRE,
		backend:        method.Backend,
		headers:        method.Headers,
		matchAllHeader: method.MatchAllHeader,
	}
}

func (mm *MuxMethod) matchMethod(method string) bool {
	if mm.method == "" && mm.methodPrefix == "" && mm.methodRE == nil {
		return true
	}

	if mm.method != "" && mm.method == method {
		return true
	}
	if mm.methodPrefix != "" && strings.HasPrefix(method, mm.methodPrefix) {
		return true
	}
	if mm.methodRE != nil {
		return mm.methodRE.MatchString(method)
	}

	return false
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

func (mm *MuxMethod) matchHeaders(r *grpcprot.Request) bool {
	if mm.matchAllHeader {
		for _, h := range mm.headers {
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
		for _, h := range mm.headers {
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

	return mm.matchAllHeader
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
		ipFilter:     ipfilter.New(spec.IPFilter),
		ipFilterChan: ipfilter.NewIPFilterChain(nil, spec.IPFilter),
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

		ruleIPFilterChain := ipfilter.NewIPFilterChain(inst.ipFilterChan, specRule.IPFilter)

		methods := make([]*MuxMethod, len(specRule.Methods))
		for j := 0; j < len(methods); j++ {
			methods[j] = newMuxMethod(ruleIPFilterChain, specRule.Methods[j])
		}

		// NOTE: Given the parent ipFilters not its own.
		inst.rules[i] = newMuxRule(inst.ipFilterChan, specRule, methods)
	}

	m.inst.Store(inst)
}

// handler impl grpc.UnknownServiceHandler
func (m *mux) handler(srv interface{}, proxyAsServerStream grpc.ServerStream) error {
	return m.inst.Load().(*muxInstance).handler(grpcprot.NewRequestWithServerStream(proxyAsServerStream))
}

func buildFailureResponse(ctx *context.Context, s *status.Status) *grpcprot.Response {
	resp := grpcprot.NewResponse()
	resp.SetStatus(s)
	ctx.SetResponse(context.DefaultNamespace, resp)
	return resp
}

func (mi *muxInstance) handler(request *grpcprot.Request) (result error) {
	startAt := fasttime.Now()
	ctx := context.New(tracing.NoopSpan)
	ctx.SetRequest(context.DefaultNamespace, request)

	defer func() {
		var resp *grpcprot.Response
		if err := recover(); err != nil {
			logger.Warnf("gRPC server %s: panic %v, stack: \n%s\n", mi.superSpec.Name(), err, debug.Stack())
			resp = buildFailureResponse(ctx, status.Newf(codes.Internal, "gRPC server %s: panic", mi.superSpec.Name()))
		} else if v := ctx.GetResponse(context.DefaultNamespace); v == nil {
			resp = buildFailureResponse(ctx, status.Newf(codes.Internal, "gRPC server %s: response is nil ", mi.superSpec.Name()))
		} else if r, ok := v.(*grpcprot.Response); !ok {
			resp = buildFailureResponse(ctx, status.Newf(codes.Internal, "gRPC server %s: response type is %T", mi.superSpec.Name(), v))
		} else {
			resp = r
		}
		result = resp.Err()
		ctx.Finish()
		// Write access log.
		logger.LazyHTTPAccess(func() string {
			// log format:
			//
			// [$startTime]
			// [$clientAddr $method $statusCode]
			// [$tags]
			const logFmt = "[grpc][%s] [%s %s %d] [%s]"
			return fmt.Sprintf(logFmt,
				fasttime.Format(startAt, fasttime.RFC3339Milli),
				request.SourceHost(), request.FullMethod(), resp.StatusCode(), ctx.Tags())
		})
	}()

	rt := mi.search(request)
	if rt.code != 0 {
		logger.Debugf("%s: status result of search route: %+v", mi.superSpec.Name(), rt.code)
		buildFailureResponse(ctx, status.New(rt.code, rt.message))
		return
	}

	handler, ok := mi.muxMapper.GetHandler(rt.method.backend)
	if !ok {
		logger.Debugf("%s: backend %q not found", mi.superSpec.Name(), rt.method.backend)
		buildFailureResponse(ctx, status.Newf(codes.NotFound, "%s: backend %q not found", mi.superSpec.Name(), rt.method.backend))
		return
	}

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
	return
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
	fullMethod := request.FullMethod()
	if fullMethod == "" {
		logger.Debugf("invalid grpc stream: can not get called method info")
		return &route{
			code:    codes.NotFound,
			message: "invalid grpc stream: can not get called method info",
		}
	}

	// The key of the cache is grpc server address + called method
	// and if a method is cached, we are sure it does not contain any
	// headers.
	r := mi.getRouteFromCache(request.OnlyHost(), fullMethod)
	if r != nil {
		if r.code != 0 {
			return r
		}
		if r.method.ipFilterChain == nil {
			return r
		}
		if r.method.ipFilterChain.Allow(ip) {
			return r
		}
		return &route{
			code:    codes.PermissionDenied,
			message: "request isn't allowed",
		}
	}

	if !mi.ipFilter.Allow(ip) {
		return &route{
			code:    codes.PermissionDenied,
			message: "request isn't allowed",
		}
	}

	for _, rs := range mi.rules {
		if !rs.match(request.OnlyHost()) {
			continue
		}

		if !rs.ipFilter.Allow(ip) {
			return &route{
				code:    codes.PermissionDenied,
				message: "request isn't allowed",
			}
		}

		for _, method := range rs.methods {
			if !method.matchMethod(fullMethod) {
				continue
			}

			// The method can be put into the cache if it has no headers.
			if len(method.headers) == 0 {
				r = &route{code: 0, method: method}
				mi.putRouteToCache(request.OnlyHost(), fullMethod, r)
			} else if !method.matchHeaders(request) {
				headerMismatch = true
				continue
			}

			if !method.ipFilter.Allow(ip) {
				return &route{
					code:    codes.PermissionDenied,
					message: "request isn't allowed",
				}
			}

			return &route{code: 0, method: method}
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
	mi.putRouteToCache(request.OnlyHost(), fullMethod, notFound)
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
