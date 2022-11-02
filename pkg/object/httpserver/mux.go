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
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"

	"github.com/megaease/easegress/pkg/object/httpserver/routers"

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
	"github.com/megaease/easegress/pkg/util/readers"
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

		tracer       *tracing.Tracer
		ipFilter     *ipfilter.IPFilter
		ipFilterChan *ipfilter.IPFilters

		router routers.Router
	}

	routeCache struct {
		code  int
		route routers.Route
	}
)

var (
	notFound         = &routeCache{code: http.StatusNotFound}
	forbidden        = &routeCache{code: http.StatusForbidden}
	methodNotAllowed = &routeCache{code: http.StatusMethodNotAllowed}
	badRequest       = &routeCache{code: http.StatusBadRequest}
)

func (mi *muxInstance) getRouteFromCache(req *httpprot.Request) *routeCache {
	if mi.cache != nil {
		key := stringtool.Cat(req.Host(), req.Method(), req.Path())
		if value, ok := mi.cache.Get(key); ok {
			return value.(*routeCache)
		}
	}
	return nil
}

func (mi *muxInstance) putRouteToCache(req *httpprot.Request, rc *routeCache) {
	if mi.cache != nil {
		key := stringtool.Cat(req.Host(), req.Method(), req.Path())
		mi.cache.Add(key, rc)
	}
}

func newMux(httpStat *httpstat.HTTPStat, topN *httpstat.TopN, mapper context.MuxMapper) *mux {
	m := &mux{
		httpStat: httpStat,
		topN:     topN,
	}

	m.inst.Store(&muxInstance{
		spec:      &Spec{},
		tracer:    tracing.NoopTracer,
		muxMapper: mapper,
		httpStat:  httpStat,
		topN:      topN,
	})

	return m
}

func (m *mux) reload(superSpec *supervisor.Spec, muxMapper context.MuxMapper) {
	spec := superSpec.ObjectSpec().(*Spec)

	tracer := tracing.NoopTracer
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

	routerKind := "Order"
	if spec.RouterKind != "" {
		routerKind = spec.RouterKind
	}

	inst := &muxInstance{
		superSpec:    superSpec,
		spec:         spec,
		muxMapper:    muxMapper,
		httpStat:     m.httpStat,
		topN:         m.topN,
		ipFilter:     ipfilter.New(spec.IPFilterSpec),
		ipFilterChan: ipfilter.NewIPFilterChain(nil, spec.IPFilterSpec),
		tracer:       tracer,
	}
	spec.Rules.Init(inst.ipFilterChan)
	inst.router = routers.Create(routerKind, spec.Rules)

	if spec.CacheSize > 0 {
		arc, err := lru.NewARC(int(spec.CacheSize))
		if err != nil {
			logger.Errorf("BUG: new arc cache failed: %v", err)
		}
		inst.cache = arc
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
	ctx.SetResponse(context.DefaultNamespace, resp)
	return resp
}

func (mi *muxInstance) sendResponse(ctx *context.Context, stdw http.ResponseWriter) (int, uint64) {
	var resp *httpprot.Response
	if v := ctx.GetResponse(context.DefaultNamespace); v == nil {
		logger.Errorf("%s: response is nil", mi.superSpec.Name())
		resp = buildFailureResponse(ctx, http.StatusServiceUnavailable)
	} else if r, ok := v.(*httpprot.Response); !ok {
		logger.Errorf("%s: expect an HTTP response", mi.superSpec.Name())
		resp = buildFailureResponse(ctx, http.StatusServiceUnavailable)
	} else {
		resp = r
	}

	// Send the response
	header := stdw.Header()
	for k, v := range resp.HTTPHeader() {
		header[k] = v
	}
	stdw.WriteHeader(resp.StatusCode())
	respBodySize, _ := io.Copy(stdw, resp.GetPayload())

	return resp.StatusCode(), uint64(respBodySize) + uint64(resp.MetaSize())
}

func (mi *muxInstance) serveHTTP(stdw http.ResponseWriter, stdr *http.Request) {
	// Replace the body of the original request with a ByteCountReader, so
	// that we can calculate the actual request size.
	body := readers.NewByteCountReader(stdr.Body)
	stdr.Body = body

	startAt := fasttime.Now()
	span := mi.tracer.NewSpanWithStart(mi.superSpec.Name(), startAt)
	ctx := context.New(span)
	ctx.SetData("HTTP_RESPONSE_WRITER", stdw)

	// httpprot.NewRequest never returns an error.
	req, _ := httpprot.NewRequest(stdr)

	// Calculate the meta size now, as everything could be modified.
	reqMetaSize := req.MetaSize()
	ctx.SetRequest(context.DefaultNamespace, req)

	// get topN here, as the path could be modified later.
	topN := mi.topN.Stat(req.Path())

	defer func() {
		metric, _ := ctx.GetData("HTTP_METRIC").(*httpstat.Metric)

		if metric == nil {
			statusCode, respSize := mi.sendResponse(ctx, stdw)
			ctx.Finish()

			// Drain off the body if it has not been, so that we can get the
			// correct body size.
			io.Copy(io.Discard, body)

			metric = &httpstat.Metric{
				StatusCode: statusCode,
				ReqSize:    uint64(reqMetaSize) + uint64(body.BytesRead()),
				RespSize:   respSize,
			}
		} else { // hijacked, websocket and etc.
			ctx.Finish()
		}

		metric.Duration = fasttime.Since(startAt)
		topN.Stat(metric)
		mi.httpStat.Stat(metric)

		span.Finish()

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
				stdr.Proto, metric.StatusCode, metric.Duration, metric.ReqSize,
				metric.RespSize, ctx.Tags())
		})
	}()

	routeCtx := routers.NewContext(req)
	routeCache := mi.search(routeCtx)
	if routeCache.code != 0 {
		logger.Errorf("%s: status code of result route for [%s %s]: %d", mi.superSpec.Name(), req.Method(), req.RequestURI, routeCache.code)
		buildFailureResponse(ctx, routeCache.code)
		return
	}

	backend := routeCache.route.GetBackend()
	handler, ok := mi.muxMapper.GetHandler(backend)
	if !ok {
		logger.Errorf("%s: backend(Pipeline) %q for [%s %s] not found", mi.superSpec.Name(), req.Method(), req.RequestURI, backend)
		buildFailureResponse(ctx, http.StatusServiceUnavailable)
		return
	}
	logger.Debugf("%s: the matched backend(Pipeline) for [%s %s] is %q", mi.superSpec.Name(), req.Method(), req.RequestURI, backend)

	routeCache.route.Rewrite(routeCtx)
	if mi.spec.XForwardedFor {
		appendXForwardedFor(req)
	}

	maxBodySize := routeCache.route.GetClientMaxBodySize()
	if maxBodySize == 0 {
		maxBodySize = mi.spec.ClientMaxBodySize
	}
	err := req.FetchPayload(maxBodySize)
	if err == httpprot.ErrRequestEntityTooLarge {
		logger.Errorf("%s: %s, you may need to increase 'clientMaxBodySize' or set it to -1", mi.superSpec.Name(), err.Error())
		buildFailureResponse(ctx, http.StatusRequestEntityTooLarge)
		return
	}
	if err != nil {
		logger.Errorf("%s: failed to read request body: %v", mi.superSpec.Name(), err)
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

func (mi *muxInstance) search(context *routers.RouteContext) *routeCache {
	req := context.Request
	ip := req.RealIP()

	// The key of the cache is req.Host + req.Method + req.URL.Path,
	// and if a path is cached, we are sure it does not contain any
	// headers or any queries.
	r := mi.getRouteFromCache(req)
	if r != nil {
		if r.code != 0 {
			return r
		}

		if r.route.AllowIPChain(ip) {
			return r
		}

		return forbidden
	}

	if !mi.ipFilter.Allow(ip) {
		return forbidden
	}

	mi.router.Search(context)

	route := context.Route

	if context.Route != nil {
		rc := &routeCache{code: 0, route: route}
		if context.Cacheable {
			mi.putRouteToCache(req, rc)
		}

		if context.IPNotAllowed {
			return forbidden
		}
		return rc
	}

	if context.IPNotAllowed {
		return forbidden
	}

	if context.HeaderMismatch || context.QueryMismatch {
		return badRequest
	}

	if context.MethodMismatch {
		mi.putRouteToCache(req, methodNotAllowed)
		return methodNotAllowed
	}

	mi.putRouteToCache(req, notFound)
	return notFound
}

func appendXForwardedFor(r *httpprot.Request) {
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
