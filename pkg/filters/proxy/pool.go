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

package proxy

import (
	stdcontext "context"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	gohttpstat "github.com/tcnksm/go-httpstat"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/readers"
)

// serverPoolError is the error returned by handler function of
// a server pool.
type serverPoolError struct {
	code   int
	result string
}

// Error implements error.
func (spe serverPoolError) Error() string {
	return fmt.Sprintf("server pool error, status code=%d, result=%s", spe.code, spe.result)
}

// Code returns the status code.
func (spe serverPoolError) Code() int {
	return spe.code
}

// Result returns the result string.
func (spe serverPoolError) Result() string {
	return spe.result
}

// serverPoolContext records the context information in calling the
// handler function.
type serverPoolContext struct {
	*context.Context
	span      tracing.Span
	startTime time.Time

	req     *httpprot.Request
	stdReq  *http.Request
	resp    *httpprot.Response
	stdResp *http.Response

	respCallbackBody *readers.CallbackReader
}

// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

func removeHopByHopHeaders(h http.Header) {
	// removes hop-by-hop headers listed in the "Connection" header of h.
	// See RFC 7230, section 6.1
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = textproto.TrimString(sf); sf != "" {
				h.Del(sf)
			}
		}
	}

	for _, hbh := range hopByHopHeaders {
		h.Del(hbh)
	}

	// TODO: trailer support
	//
	// Few HTTP clients, servers, or proxies support HTTP trailers.
	// It seems we need to do more than below to support it.
	/*
		// tell backend applications that care about trailer support
		// that we support trailers.
		if httpguts.HeaderValuesContainsToken(in["Te"], "trailers") {
			out.Set("Te", "trailers")
		}
	*/
}

func (spCtx *serverPoolContext) prepareRequest(svr *Server, ctx stdcontext.Context, mirror bool) error {
	req := spCtx.req

	url := svr.URL + req.Path()
	if rq := req.Std().URL.RawQuery; rq != "" {
		url += "?" + rq
	}

	var payload io.Reader
	if mirror && spCtx.req.IsStream() {
		payload = strings.NewReader("cannot send a stream body to mirror")
	} else {
		payload = req.GetPayload()
	}
	stdr, err := http.NewRequestWithContext(ctx, req.Method(), url, payload)
	if err != nil {
		return err
	}

	stdr.Header = req.HTTPHeader().Clone()
	removeHopByHopHeaders(stdr.Header)

	// only set host when server address is not host name OR
	// server is explicitly told to keep the host of the request.
	if !svr.addrIsHostName || svr.KeepHost {
		stdr.Host = req.Host()
	}

	if spCtx.span != nil {
		spCtx.span.InjectHTTP(stdr)
	}

	spCtx.stdReq = stdr
	return nil
}

// ServerPool defines a server pool.
type ServerPool struct {
	BaseServerPool

	proxy        *Proxy
	spec         *ServerPoolSpec
	failureCodes map[int]struct{}

	timeout               time.Duration
	retryWrapper          resilience.Wrapper
	circuitBreakerWrapper resilience.Wrapper

	httpStat    *httpstat.HTTPStat
	memoryCache *MemoryCache
}

// ServerPoolSpec is the spec for a server pool.
type ServerPoolSpec struct {
	BaseServerPoolSpec `json:",inline"`

	SpanName             string           `json:"spanName" jsonschema:"omitempty"`
	ServerMaxBodySize    int64            `json:"serverMaxBodySize" jsonschema:"omitempty"`
	Timeout              string           `json:"timeout" jsonschema:"omitempty,format=duration"`
	RetryPolicy          string           `json:"retryPolicy" jsonschema:"omitempty"`
	CircuitBreakerPolicy string           `json:"circuitBreakerPolicy" jsonschema:"omitempty"`
	MemoryCache          *MemoryCacheSpec `json:"memoryCache,omitempty" jsonschema:"omitempty"`

	// FailureCodes would be 5xx if it isn't assigned any value.
	FailureCodes []int `json:"failureCodes" jsonschema:"omitempty,uniqueItems=true"`
}

// ServerPoolStatus is the status of Pool.
type ServerPoolStatus struct {
	Stat *httpstat.Status `json:"stat"`
}

// NewServerPool creates a new server pool according to spec.
func NewServerPool(proxy *Proxy, spec *ServerPoolSpec, name string) *ServerPool {
	sp := &ServerPool{
		proxy:    proxy,
		spec:     spec,
		httpStat: httpstat.New(),
	}
	sp.BaseServerPool.Init(proxy.super, name, &spec.BaseServerPoolSpec)

	if spec.MemoryCache != nil {
		sp.memoryCache = NewMemoryCache(spec.MemoryCache)
	}

	if spec.Timeout != "" {
		sp.timeout, _ = time.ParseDuration(spec.Timeout)
	}

	sp.failureCodes = map[int]struct{}{}
	for _, code := range spec.FailureCodes {
		sp.failureCodes[code] = struct{}{}
	}

	return sp
}

func (sp *ServerPool) status() *ServerPoolStatus {
	s := &ServerPoolStatus{Stat: sp.httpStat.Status()}
	return s
}

// InjectResiliencePolicy injects resilience policies to the server pool.
func (sp *ServerPool) InjectResiliencePolicy(policies map[string]resilience.Policy) {
	name := sp.spec.RetryPolicy
	if name != "" {
		p := policies[name]
		if p == nil {
			panic(fmt.Errorf("retry policy %s not found", name))
		}
		policy, ok := p.(*resilience.RetryPolicy)
		if !ok {
			panic(fmt.Errorf("policy %s is not a retry policy", name))
		}
		sp.retryWrapper = policy.CreateWrapper()
	}

	name = sp.spec.CircuitBreakerPolicy
	if name != "" {
		p := policies[name]
		if p == nil {
			panic(fmt.Errorf("circuitbreaker policy %s not found", name))
		}
		policy, ok := p.(*resilience.CircuitBreakerPolicy)
		if !ok {
			panic(fmt.Errorf("policy %s is not a circuitBreaker policy", name))
		}
		sp.circuitBreakerWrapper = policy.CreateWrapper()
	}
}

func (sp *ServerPool) collectMetrics(spCtx *serverPoolContext) {
	metric := &httpstat.Metric{}

	metric.StatusCode = spCtx.resp.StatusCode()
	metric.ReqSize = uint64(spCtx.req.MetaSize())
	metric.ReqSize += uint64(spCtx.req.PayloadSize())
	metric.RespSize = uint64(spCtx.resp.MetaSize())

	collect := func() {
		metric.Duration = fasttime.Since(spCtx.startTime)
		sp.httpStat.Stat(metric)
		spCtx.LazyAddTag(func() string {
			return sp.name + "#duration: " + metric.Duration.String()
		})
	}

	// Collect all metrics directly if not a stream.
	if !spCtx.resp.IsStream() {
		metric.RespSize += uint64(spCtx.resp.PayloadSize())
		collect()
		return
	}

	body := spCtx.respCallbackBody

	// Collect when reach EOF or meet an error.
	body.OnAfter(func(total int, p []byte, err error) {
		if err != nil {
			metric.RespSize += uint64(total)
			collect()
		}
	})

	// Drain off the body to make sure collect is called even no one
	// read the stream.
	body.OnClose(func() {
		io.Copy(io.Discard, spCtx.resp.GetPayload())
	})
}

func (sp *ServerPool) handleMirror(spCtx *serverPoolContext) {
	svr := sp.LoadBalancer().ChooseServer(spCtx.req)
	if svr == nil {
		return
	}

	err := spCtx.prepareRequest(svr, spCtx.req.Context(), true)
	if err != nil {
		logger.Debugf("%s: failed to prepare request: %v", sp.name, err)
		return
	}

	resp, err := fnSendRequest(spCtx.stdReq, sp.proxy.client)
	if err != nil {
		return
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func (sp *ServerPool) handle(ctx *context.Context, mirror bool) string {
	spCtx := &serverPoolContext{
		Context: ctx,
		req:     ctx.GetInputRequest().(*httpprot.Request),
	}

	if mirror {
		sp.handleMirror(spCtx)
		return ""
	}

	spCtx.startTime = fasttime.Now()
	defer sp.collectMetrics(spCtx)

	if sp.buildResponseFromCache(spCtx) {
		if sp.inFailureCodes(spCtx.resp.StatusCode()) {
			return resultFailureCode
		}
		return ""
	}

	// wrap the handler function to meet the requirement of resilience
	// wrappers.
	handler := func(stdctx stdcontext.Context) error {
		if sp.timeout > 0 {
			var cancel stdcontext.CancelFunc
			stdctx, cancel = stdcontext.WithTimeout(stdctx, sp.timeout)
			defer cancel()
		}

		// this function could be called more than once, and these
		// fields need to be reset before each call.
		spCtx.stdReq = nil
		spCtx.resp = nil
		spCtx.stdResp = nil
		spCtx.respCallbackBody = nil

		spanName := sp.spec.SpanName
		if spanName == "" {
			spanName = sp.name
		}
		spCtx.span = ctx.Span().NewChild(spanName)
		defer spCtx.span.Finish()

		return sp.doHandle(stdctx, spCtx)
	}

	// resilience wrappers, note that it is impossible to retry a stream
	// request as its body can only be read once.
	if sp.retryWrapper != nil && !spCtx.req.IsStream() {
		handler = sp.retryWrapper.Wrap(handler)
	}
	if sp.circuitBreakerWrapper != nil {
		handler = sp.circuitBreakerWrapper.Wrap(handler)
	}

	// call the handler.
	err := handler(spCtx.req.Context())
	if err == nil {
		return ""
	}

	// CircuitBreaker is the most outside resiliencer, if the error
	// is ErrShortCircuited, we are sure the response is nil.
	if err == resilience.ErrShortCircuited {
		logger.Debugf("%s: short circuited by circuit break policy", sp.name)
		spCtx.AddTag("short circuited")
		sp.buildFailureResponse(spCtx, http.StatusServiceUnavailable)
		return resultShortCircuited
	}

	// The error must be a serverPoolError now, we need to build a
	// response in most cases, but for failure status codes, the
	// response is already there.
	if spe, ok := err.(serverPoolError); ok {
		if spCtx.resp == nil {
			sp.buildFailureResponse(spCtx, spe.code)
		}
		return spe.Result()
	}

	panic(fmt.Errorf("should not reach here"))
}

func (sp *ServerPool) doHandle(stdctx stdcontext.Context, spCtx *serverPoolContext) error {
	svr := sp.LoadBalancer().ChooseServer(spCtx.req)

	// if there's no available server.
	if svr == nil {
		logger.Debugf("%s: no available server", sp.name)
		return serverPoolError{http.StatusServiceUnavailable, resultInternalError}
	}

	// prepare the request to send.
	statResult := &gohttpstat.Result{}
	stdctx = gohttpstat.WithHTTPStat(stdctx, statResult)
	if err := spCtx.prepareRequest(svr, stdctx, false); err != nil {
		logger.Debugf("%s: failed to prepare request: %v", sp.name, err)
		return serverPoolError{http.StatusInternalServerError, resultInternalError}
	}

	resp, err := fnSendRequest(spCtx.stdReq, sp.proxy.client)
	if err != nil {
		logger.Debugf("%s: failed to send request: %v", sp.name, err)

		statResult.End(fasttime.Now())
		spCtx.LazyAddTag(func() string {
			return fmt.Sprintf("trace %v", statResult)
		})

		if err := spCtx.stdReq.Context().Err(); err == nil {
			return serverPoolError{http.StatusServiceUnavailable, resultServerError}
		} else if err == stdcontext.DeadlineExceeded {
			return serverPoolError{http.StatusRequestTimeout, resultTimeout}
		}

		// NOTE: return 499 if client is Disconnected.
		// TODO: define a constant for 499
		return serverPoolError{499, resultClientError}
	}

	spCtx.stdResp = resp
	if err = sp.buildResponse(spCtx); err != nil {
		return serverPoolError{http.StatusInternalServerError, resultInternalError}
	}

	// manipulate by load balancer
	sp.LoadBalancer().Manipulate(svr, spCtx.req, spCtx.resp)

	spCtx.LazyAddTag(func() string {
		return fmt.Sprintf("status code: %d", resp.StatusCode)
	})

	// If the status code is one of the failure codes, change result to
	// resultFailureCode, but don't touch the response itself.
	//
	// This may be incorrect, but failure code is different from other
	// errors, and it seems impossible to find a perfect solution.
	if sp.inFailureCodes(resp.StatusCode) {
		return serverPoolError{resp.StatusCode, resultFailureCode}
	}

	return nil
}

func (sp *ServerPool) mergeResponseHeader(dst, src http.Header) http.Header {
	for k, v := range src {
		// CORS Headers
		if strings.HasPrefix(k, "Access-Control-") {
			dst[k] = v
		} else {
			dst[k] = append(dst[k], v...)
		}
	}
	return dst
}

func (sp *ServerPool) buildResponse(spCtx *serverPoolContext) (err error) {
	removeHopByHopHeaders(spCtx.stdResp.Header)

	body := readers.NewCallbackReader(spCtx.stdResp.Body)
	spCtx.stdResp.Body = body
	spCtx.respCallbackBody = body

	if sp.proxy.compression != nil {
		if sp.proxy.compression.compress(spCtx.stdReq, spCtx.stdResp) {
			spCtx.AddTag("gzip")
		}
	}

	resp, err := httpprot.NewResponse(spCtx.stdResp)
	if err != nil {
		logger.Debugf("%s: NewResponse returns an error: %v", sp.name, err)
		body.Close()
		return err
	}

	maxBodySize := sp.spec.ServerMaxBodySize
	if maxBodySize == 0 {
		maxBodySize = sp.proxy.spec.ServerMaxBodySize
	}
	if err = resp.FetchPayload(maxBodySize); err != nil {
		logger.Debugf("%s: failed to fetch response payload: %v", sp.name, err)
		body.Close()
		return err
	}

	if !resp.IsStream() {
		body.Close()
	}

	if sp.memoryCache != nil {
		sp.memoryCache.Store(spCtx.req, resp)
	}

	if r, _ := spCtx.GetOutputResponse().(*httpprot.Response); r != nil {
		header := sp.mergeResponseHeader(r.HTTPHeader(), resp.HTTPHeader())
		resp.Std().Header = header

		// reuse the existing output response, this is to align with
		// buildResponseFromCache and buildFailureResponse and other filters.
		*r = *resp
		resp = r
	}

	spCtx.resp = resp
	spCtx.SetOutputResponse(resp)
	return nil
}

func (sp *ServerPool) buildResponseFromCache(spCtx *serverPoolContext) bool {
	if sp.memoryCache == nil {
		return false
	}

	ce := sp.memoryCache.Load(spCtx.req)
	if ce == nil {
		return false
	}

	resp, _ := spCtx.GetOutputResponse().(*httpprot.Response)
	reqHasOrigin := spCtx.req.HTTPHeader().Get("Origin") != ""
	respHasCORS := (resp != nil) && (resp.HTTPHeader().Get("Access-Control-Allow-Origin") != "")
	cacheHasCORS := ce.Header.Get("Access-Control-Allow-Origin") != ""

	// This is a CORS request, but we don't have the required response headers.
	if reqHasOrigin && !respHasCORS && !cacheHasCORS {
		return false
	}

	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}

	header := resp.HTTPHeader()
	// 1. If existing response has CORS headers, we keep them and discard the
	//    ones from the cache (if exists).
	// 2. If existing response doesn't have CORS headers:
	//    2.1. If the request is a CORS one, we use the CORS headers from the
	//         cache.
	//    2.2. If the request is not a CORS one, we discard the CORS headers
	//         from the cache (if exists).
	for k, v := range ce.Header {
		if !strings.HasPrefix(k, "Access-Control-") {
			header[k] = append(header[k], v...)
		} else if reqHasOrigin && !respHasCORS {
			header[k] = v
		}
	}

	resp.SetStatusCode(ce.StatusCode)
	resp.SetPayload(ce.Body)

	spCtx.resp = resp
	spCtx.SetOutputResponse(resp)
	return true
}

func (sp *ServerPool) buildFailureResponse(spCtx *serverPoolContext, statusCode int) {
	resp, _ := spCtx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}

	resp.SetStatusCode(statusCode)
	spCtx.resp = resp
	spCtx.SetOutputResponse(resp)
}

func (sp *ServerPool) inFailureCodes(code int) bool {
	if len(sp.failureCodes) == 0 {
		if code >= 500 && code < 600 {
			return true
		}
		return false
	}

	_, exists := sp.failureCodes[code]
	return exists
}
