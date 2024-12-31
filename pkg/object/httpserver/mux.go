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

package httpserver

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/megaease/easegress/v2/pkg/object/httpserver/routers"

	lru "github.com/hashicorp/golang-lru"
	"github.com/megaease/easegress/v2/pkg/object/globalfilter"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/autocertmanager"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/tracing"
	"github.com/megaease/easegress/v2/pkg/util/fasttime"
	"github.com/megaease/easegress/v2/pkg/util/ipfilter"
	"github.com/megaease/easegress/v2/pkg/util/readers"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultAccessLogFormat = "[{{Time}}] [{{RemoteAddr}} {{RealIP}} {{Method}} {{URI}} {{Proto}} {{StatusCode}}] [{{Duration}} rx:{{ReqSize}}B tx:{{RespSize}}B] [{{Tags}}]"
)

type (
	mux struct {
		httpStat *httpstat.HTTPStat
		topN     *httpstat.TopN

		inst atomic.Value // *muxInstance
	}

	muxInstance struct {
		superSpec          *supervisor.Spec
		spec               *Spec
		httpStat           *httpstat.HTTPStat
		topN               *httpstat.TopN
		metrics            *metrics
		accessLogFormatter *accessLogFormatter

		muxMapper context.MuxMapper

		cache *lru.ARCCache

		tracer   *tracing.Tracer
		ipFilter *ipfilter.IPFilter

		router routers.Router
	}

	cachedRoute struct {
		code  int
		route routers.Route
	}

	accessLogFormatter struct {
		template *template.Template
	}

	accessLog struct {
		Time        string
		RemoteAddr  string
		RealIP      string
		Method      string
		URI         string
		Proto       string
		StatusCode  int
		Duration    time.Duration
		ReqSize     uint64
		RespSize    uint64
		ReqHeaders  string
		RespHeaders string
		Tags        string
	}
)

var (
	notFound         = &cachedRoute{code: http.StatusNotFound}
	forbidden        = &cachedRoute{code: http.StatusForbidden}
	methodNotAllowed = &cachedRoute{code: http.StatusMethodNotAllowed}
	badRequest       = &cachedRoute{code: http.StatusBadRequest}
)

func (mi *muxInstance) getRouteFromCache(req *httpprot.Request) *cachedRoute {
	if mi.cache != nil {
		key := stringtool.Cat(req.Host(), req.Method(), req.Path())
		if value, ok := mi.cache.Get(key); ok {
			return value.(*cachedRoute)
		}
	}
	return nil
}

func (mi *muxInstance) putRouteToCache(req *httpprot.Request, rc *cachedRoute) {
	if mi.cache != nil {
		key := stringtool.Cat(req.Host(), req.Method(), req.Path())
		mi.cache.Add(key, rc)
	}
}

func newMux(httpStat *httpstat.HTTPStat, topN *httpstat.TopN,
	metrics *metrics, mapper context.MuxMapper,
) *mux {
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
		metrics:   metrics,
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

	routerKind := "Ordered"
	if spec.RouterKind != "" {
		routerKind = spec.RouterKind
	}

	inst := &muxInstance{
		superSpec:          superSpec,
		spec:               spec,
		muxMapper:          muxMapper,
		httpStat:           m.httpStat,
		topN:               m.topN,
		metrics:            oldInst.metrics,
		ipFilter:           ipfilter.New(spec.IPFilter),
		tracer:             tracer,
		accessLogFormatter: newAccessLogFormatter(spec.AccessLogFormat),
	}
	spec.Rules.Init()
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
		acm, exists := autocertmanager.GetGlobalAutoCertManager()
		if !exists {
			logger.Errorf("there is no one AutoCertManager")
			stdw.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		acm.HandleHTTP01Challenge(stdw, stdr)
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

func (mi *muxInstance) sendResponse(ctx *context.Context, stdw http.ResponseWriter) (int, uint64, http.Header) {
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

	var writer io.Writer
	if responseIsRealTime(resp) {
		writer = NewResponseFlushWriter(stdw)
	} else {
		writer = stdw
	}
	respBodySize, _ := io.Copy(writer, resp.GetPayload())

	return resp.StatusCode(), uint64(respBodySize) + uint64(resp.MetaSize()), header
}

// ResponseFlushWriter is a wrapper of http.ResponseWriter, which flushes the
// response immediately if the response needs to be flushed.
type ResponseFlushWriter struct {
	w     http.ResponseWriter
	flush func()
}

// Write writes the data to the connection as part of an HTTP reply.
func (w *ResponseFlushWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.flush()
	return n, err
}

// NewResponseFlushWriter creates a ResponseFlushWriter.
func NewResponseFlushWriter(w http.ResponseWriter) *ResponseFlushWriter {
	if flusher, ok := w.(http.Flusher); ok {
		return &ResponseFlushWriter{
			w:     w,
			flush: flusher.Flush,
		}
	}
	return &ResponseFlushWriter{
		w:     w,
		flush: func() {},
	}
}

// responseIsRealTime returns whether the response needs to be flushed immediately.
// The response needs to be flushed immediately if the response has no content length (chunked),
// or the response is a Server-Sent Events response.
func responseIsRealTime(resp *httpprot.Response) bool {
	// Based on https://en.wikipedia.org/wiki/Chunked_transfer_encoding
	// If the Transfer-Encoding header field is present in a response and its value is "chunked",
	// then the body of response is considered as a stream of chunks.
	if len(resp.TransferEncoding) > 0 && resp.TransferEncoding[0] == "chunked" && resp.ContentLength <= 0 {
		return true
	}

	resCTHeader := resp.Std().Header.Get("Content-Type")
	resCT, _, err := mime.ParseMediaType(resCTHeader)
	// For Server-Sent Events responses, flush immediately.
	// The MIME type is defined in https://www.w3.org/TR/eventsource/#text-event-stream
	if err == nil && resCT == "text/event-stream" {
		return true
	}

	return false
}

func (mi *muxInstance) serveHTTP(stdw http.ResponseWriter, stdr *http.Request) {
	// Replace the body of the original request with a ByteCountReader, so
	// that we can calculate the actual request size.
	body := readers.NewByteCountReader(stdr.Body)
	stdr.Body = body

	startAt := fasttime.Now()

	span := mi.tracer.NewSpanForHTTP(stdr.Context(), mi.superSpec.Name(), stdr)

	ctx := context.New(span)
	ctx.SetData("HTTP_RESPONSE_WRITER", stdw)

	// httpprot.NewRequest never returns an error.
	req, _ := httpprot.NewRequest(stdr)

	// Calculate the meta size now, as everything could be modified.
	reqMetaSize := req.MetaSize()
	ctx.SetRequest(context.DefaultNamespace, req)

	// get topN here, as the path could be modified later.
	topN := mi.topN.Stat(req.Path())

	routeCtx := routers.NewContext(req)
	route := mi.search(routeCtx)
	ctx.SetRoute(route.route)

	var respHeader http.Header

	defer func() {
		metric, _ := ctx.GetData("HTTP_METRIC").(*httpstat.Metric)

		if metric == nil {
			statusCode, respSize, header := mi.sendResponse(ctx, stdw)
			ctx.Finish()

			// Drain off the body if it has not been, so that we can get the
			// correct body size.
			io.Copy(io.Discard, body)

			metric = &httpstat.Metric{
				StatusCode: statusCode,
				ReqSize:    uint64(reqMetaSize) + uint64(body.BytesRead()),
				RespSize:   respSize,
			}
			respHeader = header
		} else { // hijacked, websocket and etc.
			ctx.Finish()
		}

		metric.Duration = fasttime.Since(startAt)
		topN.Stat(metric)
		mi.httpStat.Stat(metric)
		if route.code == 0 {
			mi.exportPrometheusMetrics(metric, route.route.GetBackend())
		}

		span.End()

		// Write access log.
		logger.LazyHTTPAccess(func() string {
			log := &accessLog{
				Time:        fasttime.Format(startAt, fasttime.RFC3339Milli),
				RemoteAddr:  stdr.RemoteAddr,
				RealIP:      req.RealIP(),
				Method:      stdr.Method,
				URI:         stdr.RequestURI,
				Proto:       stdr.Proto,
				StatusCode:  metric.StatusCode,
				Duration:    metric.Duration,
				ReqSize:     metric.ReqSize,
				RespSize:    metric.RespSize,
				Tags:        ctx.Tags(),
				ReqHeaders:  printHeader(stdr.Header),
				RespHeaders: printHeader(respHeader),
			}
			return mi.accessLogFormatter.format(log)
		})
	}()

	if route.code != 0 {
		logger.Errorf("%s: status code of result route for [%s %s]: %d", mi.superSpec.Name(), req.Method(), req.RequestURI, route.code)
		buildFailureResponse(ctx, route.code)
		return
	}

	backend := route.route.GetBackend()
	handler, ok := mi.muxMapper.GetHandler(backend)
	if !ok {
		logger.Errorf("%s: backend(Pipeline) %q for [%s %s] not found", mi.superSpec.Name(), req.Method(), req.RequestURI, backend)
		buildFailureResponse(ctx, http.StatusServiceUnavailable)
		return
	}
	logger.Debugf("%s: the matched backend(Pipeline) for [%s %s] is %q", mi.superSpec.Name(), req.Method(), req.RequestURI, backend)

	route.route.Rewrite(routeCtx)
	if mi.spec.XForwardedFor {
		appendXForwardedFor(req)
	}

	maxBodySize := route.route.GetClientMaxBodySize()
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

func (mi *muxInstance) search(context *routers.RouteContext) *cachedRoute {
	req := context.Request
	ip := req.RealIP()

	if !mi.ipFilter.Allow(ip) {
		return forbidden
	}

	// The key of the cache is req.Host + req.Method + req.URL.Path,
	// and if a path is cached, we are sure it does not contain any
	// headers, any queries, and any ipFilters.
	r := mi.getRouteFromCache(req)
	if r != nil {
		return r
	}

	mi.router.Search(context)

	if route := context.Route; context.Route != nil {
		cr := &cachedRoute{code: 0, route: route}
		if context.Cacheable {
			mi.putRouteToCache(req, cr)
		}
		return cr
	}

	if context.IPMismatch {
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

func (mi *muxInstance) exportPrometheusMetrics(stat *httpstat.Metric, backend string) {
	labels := prometheus.Labels{
		"routerKind": mi.spec.RouterKind,
		"backend":    backend,
	}
	mi.metrics.TotalRequests.With(labels).Inc()
	mi.metrics.TotalResponses.With(labels).Inc()
	if stat.StatusCode >= 400 {
		mi.metrics.TotalErrorRequests.With(labels).Inc()
	}
	mi.metrics.RequestsDuration.With(labels).Observe(float64(stat.Duration.Milliseconds()))
	mi.metrics.RequestSizeBytes.With(labels).Observe(float64(stat.ReqSize))
	mi.metrics.ResponseSizeBytes.With(labels).Observe(float64(stat.RespSize))
	mi.metrics.RequestsDurationPercentage.With(labels).Observe(float64(stat.Duration.Milliseconds()))
	mi.metrics.RequestSizeBytesPercentage.With(labels).Observe(float64(stat.ReqSize))
	mi.metrics.ResponseSizeBytesPercentage.With(labels).Observe(float64(stat.RespSize))
}

func newAccessLogFormatter(format string) *accessLogFormatter {
	if format == "" {
		format = defaultAccessLogFormat
	}
	varReg := regexp.MustCompile(`\{\{([a-zA-z]*)\}\}`)
	expr := varReg.ReplaceAllString(format, "{{.$1}}")
	escapeReg := regexp.MustCompile(`(\[|\])`)
	expr = escapeReg.ReplaceAllString(expr, "{{`$1`}}")
	tpl := template.Must(template.New("").Parse(expr))
	return &accessLogFormatter{template: tpl}
}

func (formatter *accessLogFormatter) format(log *accessLog) string {
	var buf bytes.Buffer
	if err := formatter.template.Execute(&buf, log); err != nil {
		logger.Errorf("format access log failed: %v", err)
	}
	return buf.String()
}

func printHeader(header http.Header) string {
	buf := bytes.Buffer{}
	i := 0
	for key, values := range header {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(fmt.Sprintf("%v: %v", key, values))
		i++
	}
	return buf.String()
}
