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
	"sync"
	"sync/atomic"
	"time"

	"github.com/opentracing/opentracing-go"
	gohttpstat "github.com/tcnksm/go-httpstat"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/fasttime"
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
	endTime   time.Time

	req     *httpprot.Request
	stdReq  *http.Request
	resp    *httpprot.Response
	stdResp *http.Response
}

func (spCtx *serverPoolContext) prepareRequest(svr *Server, ctx stdcontext.Context) error {
	stdr := spCtx.req.Std()

	url := svr.URL + spCtx.req.Path()
	if stdr.URL.RawQuery != "" {
		url += "?" + stdr.URL.RawQuery
	}

	payload := spCtx.req.GetPayload()
	stdr, err := http.NewRequestWithContext(ctx, stdr.Method, url, payload)
	if err != nil {
		logger.Errorf("prepare request failed: %v", err)
		return err
	}

	stdr.Header = spCtx.req.HTTPHeader()
	if !svr.addrIsHostName {
		stdr.Host = spCtx.req.Host()
	}

	spCtx.stdReq = stdr
	return nil
}

func (spCtx *serverPoolContext) start(spanName string) {
	spCtx.startTime = fasttime.Now()
	span := spCtx.Span().NewChildWithStart(spanName, spCtx.startTime)
	carrier := opentracing.HTTPHeadersCarrier(spCtx.req.HTTPHeader())
	span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, carrier)
	spCtx.span = span
}

func (spCtx *serverPoolContext) finish() {
	if spCtx.endTime.IsZero() {
		now := fasttime.Now()
		spCtx.endTime = now
		spCtx.span.Finish()
	}
}

func (spCtx *serverPoolContext) duration() time.Duration {
	return spCtx.endTime.Sub(spCtx.startTime)
}

// ServerPool defines a server pool.
type ServerPool struct {
	proxy        *Proxy
	spec         *ServerPoolSpec
	done         chan struct{}
	wg           sync.WaitGroup
	name         string
	failureCodes map[int]struct{}

	filter              RequestMatcher
	loadBalancer        atomic.Value
	timeout             time.Duration
	retryWrapper        resilience.Wrapper
	circuitbreakWrapper resilience.Wrapper

	httpStat    *httpstat.HTTPStat
	memoryCache *MemoryCache
}

// ServerPoolSpec is the spec for a server pool.
type ServerPoolSpec struct {
	SpanName           string              `yaml:"spanName" jsonschema:"omitempty"`
	Filter             *RequestMatcherSpec `yaml:"filter" jsonschema:"omitempty"`
	ServersTags        []string            `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
	Servers            []*Server           `yaml:"servers" jsonschema:"omitempty"`
	ServiceRegistry    string              `yaml:"serviceRegistry" jsonschema:"omitempty"`
	ServiceName        string              `yaml:"serviceName" jsonschema:"omitempty"`
	LoadBalance        *LoadBalanceSpec    `yaml:"loadBalance" jsonschema:"required"`
	Timeout            string              `yaml:"timeout" jsonschema:"omitempty,format=duration"`
	RetryPolicy        string              `yaml:"retryPolic" jsonschema:"omitempty"`
	CircuitBreakPolicy string              `yaml:"circuitBreakPolicy" jsonschema:"omitempty"`
	FailureCodes       []int               `yaml:"failureCodes" jsonschema:"omitempty"`
	MemoryCache        *MemoryCacheSpec    `yaml:"memoryCache,omitempty" jsonschema:"omitempty"`
}

// ServerPoolStatus is the status of Pool.
type ServerPoolStatus struct {
	Stat *httpstat.Status `yaml:"stat"`
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
		for _, tag := range sp.spec.ServersTags {
			if stringtool.StrInSlice(tag, instance.Tags) {
				servers = append(servers, &Server{
					URL:    instance.URL(),
					Tags:   instance.Tags,
					Weight: instance.Weight,
				})
				break
			}
		}
	}

	if len(servers) == 0 {
		msgFmt := "%s/%s: no service instance satisfy tags: %v"
		logger.Warnf(msgFmt, sp.spec.ServiceRegistry, sp.spec.ServiceName, sp.spec.ServersTags)
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

	name = sp.spec.CircuitBreakPolicy
	if name != "" {
		p := policies[name]
		if p == nil {
			panic(fmt.Errorf("circuit break policy %s not found", name))
		}
		policy, ok := p.(*resilience.CircuitBreakPolicy)
		if !ok {
			panic(fmt.Errorf("policy %s is not a circuit break policy", name))
		}
		sp.circuitbreakWrapper = policy.CreateWrapper()
	}
}

func (sp *ServerPool) collectMetrics(spCtx *serverPoolContext) {
	spCtx.finish()

	duration := spCtx.duration()
	spCtx.LazyAddTag(func() string {
		return stringtool.Cat(sp.name, "#duration: ", duration.String())
	})

	metric := &httpstat.Metric{}
	metric.StatusCode = spCtx.resp.StatusCode()
	metric.Duration = duration

	metric.ReqSize = uint64(spCtx.req.MetaSize())
	metric.ReqSize += uint64(spCtx.req.PayloadLength())

	metric.RespSize = uint64(spCtx.resp.MetaSize())
	metric.RespSize += uint64(spCtx.resp.PayloadLength())

	sp.httpStat.Stat(metric)
}

func (sp *ServerPool) handleMirror(spCtx *serverPoolContext) {
	svr := sp.LoadBalancer().ChooseServer(spCtx.req)
	if svr == nil {
		return
	}

	err := spCtx.prepareRequest(svr, spCtx.req.Context())
	if err != nil {
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
		req:     ctx.Request().(*httpprot.Request),
	}

	if mirror {
		sp.handleMirror(spCtx)
		return ""
	}

	spCtx.start(sp.spec.SpanName)
	defer sp.collectMetrics(spCtx)

	if sp.buildResponseFromCache(spCtx) {
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

		return sp.doHandle(stdctx, spCtx)
	}

	// resilience wrappers.
	if sp.retryWrapper != nil {
		handler = sp.retryWrapper.Wrap(handler)
	}
	if sp.circuitbreakWrapper != nil {
		handler = sp.circuitbreakWrapper.Wrap(handler)
	}

	// call the handler.
	err := handler(spCtx.req.Context())
	if err == nil {
		return ""
	}

	// Circuit breaker is the most outside resiliencer, if the error
	// is ErrShortCircuited, we are sure the response is nil.
	if err == resilience.ErrShortCircuited {
		spCtx.LazyAddTag(func() string {
			return "short circuited"
		})
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
		spCtx.LazyAddTag(func() string {
			return "no available server"
		})
		return serverPoolError{http.StatusServiceUnavailable, resultInternalError}
	}

	// prepare the request to send.
	statResult := &gohttpstat.Result{}
	stdctx = gohttpstat.WithHTTPStat(stdctx, statResult)
	if err := spCtx.prepareRequest(svr, stdctx); err != nil {
		spCtx.LazyAddTag(func() string {
			return "prepare request failed: " + err.Error()
		})
		return serverPoolError{http.StatusInternalServerError, resultInternalError}
	}

	resp, err := fnSendRequest(spCtx.stdReq, sp.proxy.client)
	if err != nil {
		// NOTE: May add option to cancel the tracing if failed here.
		// spCtx.Span().Cancel()

		spCtx.LazyAddTag(func() string {
			return fmt.Sprintf("send request error: %v", err)
		})

		statResult.End(fasttime.Now())
		spCtx.LazyAddTag(func() string {
			return fmt.Sprintf("trace %v", statResult)
		})

		if err := spCtx.stdReq.Context().Err(); err == nil {
			return serverPoolError{http.StatusServiceUnavailable, resultServerError}
		} else if err == stdcontext.DeadlineExceeded {
			return serverPoolError{http.StatusRequestTimeout, resultTimeout}
		}

		// NOTE: The HTTPContext will set 499 by itself if client is
		// Disconnected. TODO: define a constant for 499
		return serverPoolError{499, resultClientError}
	}

	spCtx.stdResp = resp
	if err = sp.buildResponse(spCtx); err != nil {
		return serverPoolError{http.StatusInternalServerError, resultInternalError}
	}

	spCtx.LazyAddTag(func() string {
		return fmt.Sprintf("code: %d", resp.StatusCode)
	})

	// If the status code is one of the failure codes, change result to
	// resultFailureCode, but don't touch the response itself.
	//
	// This may be incorrect, but failure code is different from other
	// errors, and it seems impossible to find a perfect solution.
	if _, ok := sp.failureCodes[resp.StatusCode]; ok {
		return serverPoolError{resp.StatusCode, resultFailureCode}
	}

	if sp.memoryCache != nil {
		sp.memoryCache.Store(spCtx.req, spCtx.resp)
	}

	return nil
}

func (sp *ServerPool) buildResponse(spCtx *serverPoolContext) error {
	body := spCtx.stdResp.Body
	defer body.Close()

	if sp.proxy.compression.compress(spCtx.stdReq, spCtx.stdResp) {
		spCtx.Context.AddTag("gzip")
	}

	resp, err := httpprot.NewResponse(spCtx.stdResp)
	if err != nil {
		return err
	}

	if _, err = resp.FetchPayload(); err != nil {
		return err
	}

	spCtx.resp = resp
	spCtx.SetResponse(spCtx.TargetResponseID(), resp)
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

	resp, _ := httpprot.NewResponse(nil)
	resp.SetStatusCode(ce.StatusCode)
	resp.Std().Header = ce.Header.Clone()
	resp.SetPayload(ce.Body)

	spCtx.resp = resp
	spCtx.SetResponse(spCtx.TargetResponseID(), resp)
	return true
}

func (sp *ServerPool) buildFailureResponse(spCtx *serverPoolContext, statusCode int) {
	resp, _ := httpprot.NewResponse(nil)
	resp.SetStatusCode(statusCode)
	spCtx.resp = resp
	spCtx.SetResponse(spCtx.TargetResponseID(), resp)
}

func (sp *ServerPool) close() {
	close(sp.done)
	sp.wg.Wait()
}
