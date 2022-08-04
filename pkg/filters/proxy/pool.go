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
	"sync"
	"sync/atomic"
	"time"

	gohttpstat "github.com/tcnksm/go-httpstat"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/readers"
	"github.com/megaease/easegress/pkg/util/stringtool"
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
}

// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
var hopHeaders = []string{
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

func cloneHeader(in http.Header) http.Header {
	out := in.Clone()

	// removeConnectionHeaders removes hop-by-hop headers listed in the
	// "Connection" header of h. See RFC 7230, section 6.1
	for _, f := range out["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = textproto.TrimString(sf); sf != "" {
				out.Del(sf)
			}
		}
	}

	for _, h := range hopHeaders {
		out.Del(h)
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

	return out
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

	stdr.Header = cloneHeader(req.HTTPHeader())

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
	proxy        *Proxy
	spec         *ServerPoolSpec
	done         chan struct{}
	wg           sync.WaitGroup
	name         string
	failureCodes map[int]struct{}

	filter                RequestMatcher
	loadBalancer          atomic.Value
	timeout               time.Duration
	retryWrapper          resilience.Wrapper
	circuitBreakerWrapper resilience.Wrapper

	httpStat    *httpstat.HTTPStat
	memoryCache *MemoryCache
}

// ServerPoolSpec is the spec for a server pool.
type ServerPoolSpec struct {
	SpanName             string              `json:"spanName" jsonschema:"omitempty"`
	Filter               *RequestMatcherSpec `json:"filter" jsonschema:"omitempty"`
	ServerMaxBodySize    int64               `json:"serverMaxBodySize" jsonschema:"omitempty"`
	ServerTags           []string            `json:"serverTags" jsonschema:"omitempty,uniqueItems=true"`
	Servers              []*Server           `json:"servers" jsonschema:"omitempty"`
	ServiceRegistry      string              `json:"serviceRegistry" jsonschema:"omitempty"`
	ServiceName          string              `json:"serviceName" jsonschema:"omitempty"`
	LoadBalance          *LoadBalanceSpec    `json:"loadBalance" jsonschema:"omitempty"`
	Timeout              string              `json:"timeout" jsonschema:"omitempty,format=duration"`
	RetryPolicy          string              `json:"retryPolicy" jsonschema:"omitempty"`
	CircuitBreakerPolicy string              `json:"circuitBreakerPolicy" jsonschema:"omitempty"`
	FailureCodes         []int               `json:"failureCodes" jsonschema:"omitempty"`
	MemoryCache          *MemoryCacheSpec    `json:"memoryCache,omitempty" jsonschema:"omitempty"`
}

// ServerPoolStatus is the status of Pool.
type ServerPoolStatus struct {
	Stat *httpstat.Status `json:"stat"`
}

// Validate validates ServerPoolSpec.
func (sps *ServerPoolSpec) Validate() error {
	if sps.ServiceName == "" && len(sps.Servers) == 0 {
		return fmt.Errorf("both serviceName and servers are empty")
	}

	serversGotWeight := 0
	for _, server := range sps.Servers {
		if server.Weight > 0 {
			serversGotWeight++
		}
	}
	if serversGotWeight > 0 && serversGotWeight < len(sps.Servers) {
		msgFmt := "not all servers have weight(%d/%d)"
		return fmt.Errorf(msgFmt, serversGotWeight, len(sps.Servers))
	}

	return nil
}

// NewServerPool creates a new server pool according to spec.
func NewServerPool(proxy *Proxy, spec *ServerPoolSpec, name string) *ServerPool {
	sp := &ServerPool{
		proxy:    proxy,
		spec:     spec,
		done:     make(chan struct{}),
		name:     name,
		httpStat: httpstat.New(),
	}

	if spec.Filter != nil {
		sp.filter = NewRequestMatcher(spec.Filter)
	}

	if spec.MemoryCache != nil {
		sp.memoryCache = NewMemoryCache(spec.MemoryCache)
	}

	if spec.ServiceRegistry == "" || spec.ServiceName == "" {
		sp.createLoadBalancer(sp.spec.Servers)
	} else {
		sp.watchServers()
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

// LoadBalancer returns the load balancer of the server pool.
func (sp *ServerPool) LoadBalancer() LoadBalancer {
	return sp.loadBalancer.Load().(LoadBalancer)
}

func (sp *ServerPool) createLoadBalancer(servers []*Server) {
	for _, server := range servers {
		server.checkAddrPattern()
	}

	spec := sp.spec.LoadBalance
	if spec == nil {
		spec = &LoadBalanceSpec{}
	}

	lb := NewLoadBalancer(spec, servers)
	sp.loadBalancer.Store(lb)
}

func (sp *ServerPool) watchServers() {
	entity := sp.proxy.super.MustGetSystemController(serviceregistry.Kind)
	registry := entity.Instance().(*serviceregistry.ServiceRegistry)

	instances, err := registry.ListServiceInstances(sp.spec.ServiceRegistry, sp.spec.ServiceName)
	if err != nil {
		msgFmt := "first try to use service %s/%s failed(will try again): %v"
		logger.Warnf(msgFmt, sp.spec.ServiceRegistry, sp.spec.ServiceName, err)
		sp.createLoadBalancer(sp.spec.Servers)
	}

	sp.useService(instances)

	watcher := registry.NewServiceWatcher(sp.spec.ServiceRegistry, sp.spec.ServiceName)
	sp.wg.Add(1)
	go func() {
		for {
			select {
			case <-sp.done:
				watcher.Stop()
				sp.wg.Done()
				return
			case event := <-watcher.Watch():
				sp.useService(event.Instances)
			}
		}
	}()
}

func (sp *ServerPool) useService(instances map[string]*serviceregistry.ServiceInstanceSpec) {
	servers := make([]*Server, 0)

	for _, instance := range instances {
		// default to true in case of sp.spec.ServerTags is empty
		match := true

		for _, tag := range sp.spec.ServerTags {
			if match = stringtool.StrInSlice(tag, instance.Tags); match {
				break
			}
		}

		if match {
			servers = append(servers, &Server{
				URL:    instance.URL(),
				Tags:   instance.Tags,
				Weight: instance.Weight,
			})
		}
	}

	if len(servers) == 0 {
		msgFmt := "%s/%s: no service instance satisfy tags: %v"
		logger.Warnf(msgFmt, sp.spec.ServiceRegistry, sp.spec.ServiceName, sp.spec.ServerTags)
		servers = sp.spec.Servers
	}

	sp.createLoadBalancer(servers)
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

	// Now, the body must be a CallbackReader.
	body := spCtx.stdResp.Body.(*readers.CallbackReader)

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
		if _, ok := sp.failureCodes[spCtx.resp.StatusCode()]; ok {
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

	spCtx.LazyAddTag(func() string {
		return fmt.Sprintf("status code: %d", resp.StatusCode)
	})

	// If the status code is one of the failure codes, change result to
	// resultFailureCode, but don't touch the response itself.
	//
	// This may be incorrect, but failure code is different from other
	// errors, and it seems impossible to find a perfect solution.
	if _, ok := sp.failureCodes[resp.StatusCode]; ok {
		return serverPoolError{resp.StatusCode, resultFailureCode}
	}

	return nil
}

func copyCORSHeaders(dst, src http.Header) bool {
	value := src.Get("Access-Control-Allow-Origin")
	if value == "" {
		return false
	}

	dst.Set("Access-Control-Allow-Origin", value)

	if value = src.Get("Access-Control-Expose-Headers"); value != "" {
		dst.Set("Access-Control-Expose-Headers", value)
	}

	if value = src.Get("Access-Control-Allow-Credentials"); value != "" {
		dst.Set("Access-Control-Allow-Credentials", value)
	}

	if !stringtool.StrInSlice("Origin", dst.Values("Vary")) {
		dst.Add("Vary", "Origin")
	}

	return true
}

func (sp *ServerPool) buildResponse(spCtx *serverPoolContext) (err error) {
	body := readers.NewCallbackReader(spCtx.stdResp.Body)
	spCtx.stdResp.Body = body

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
		copyCORSHeaders(resp.HTTPHeader(), r.HTTPHeader())

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
	header := ce.Header.Clone()

	corsHeadersCopied := false
	resp, _ := spCtx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	} else {
		corsHeadersCopied = copyCORSHeaders(header, resp.HTTPHeader())
	}

	if !corsHeadersCopied {
		if spCtx.req.HTTPHeader().Get("Origin") == "" {
			// remove these headers as the request is not a CORS request.
			header.Del("Access-Control-Allow-Origin")
			header.Del("Access-Control-Expose-Headers")
			header.Del("Access-Control-Allow-Credentials")
		} else {
			// There are 3 cases here:
			// 1. the cached response fully matches the request: we have
			//    nothing to do in this case.
			// 2. the cached response does not match the request, because
			//    the Origin is not the same: we need to update the
			//    MemoryCache.Load function to make it return false.
			// 3. the cached response does not have CORS headers: the user
			//    need to add a CORSAdaptor into the pipeline.
			//
			// So, we do nothing for now.
		}
	}

	resp.Std().Header = header
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

func (sp *ServerPool) close() {
	close(sp.done)
	sp.wg.Wait()
}
