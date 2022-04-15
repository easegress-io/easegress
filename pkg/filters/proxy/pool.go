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
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

// ServerPool defines a server pool.
type ServerPool struct {
	proxy *Proxy
	spec  *ServerPoolSpec
	done  chan struct{}
	wg    sync.WaitGroup
	name  string

	filter       RequestMatcher
	loadBalancer atomic.Value
	resilience   *Resilience

	httpStat    *httpstat.HTTPStat
	memoryCache *MemoryCache
}

// ServerPoolSpec is the spec for a server pool.
type ServerPoolSpec struct {
	SpanName        string              `yaml:"spanName" jsonschema:"omitempty"`
	Filter          *RequestMatcherSpec `yaml:"filter" jsonschema:"omitempty"`
	ServersTags     []string            `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
	Servers         []*Server           `yaml:"servers" jsonschema:"omitempty"`
	ServiceRegistry string              `yaml:"serviceRegistry" jsonschema:"omitempty"`
	ServiceName     string              `yaml:"serviceName" jsonschema:"omitempty"`
	LoadBalance     *LoadBalanceSpec    `yaml:"loadBalance" jsonschema:"required"`
	Resilience      *ResilienceSpec     `yaml:"resilience,omitempty" jsonschema:"omitempty"`
	MemoryCache     *MemoryCacheSpec    `yaml:"memoryCache,omitempty" jsonschema:"omitempty"`
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

	var err error
	if spec.Resilience != nil {
		sp.resilience, err = newResilience(spec.Resilience, proxy.resilience)
		if err != nil {
			panic(err)
		}
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

type serverPoolContext struct {
	*context.Context
	isMirror   bool
	span       tracing.Span
	statResult *gohttpstat.Result
	startTime  time.Time
	endTime    time.Time
	svr        *Server

	req     *httpprot.Request
	stdReq  *http.Request
	resp    *httpprot.Response
	stdResp *http.Response
}

func (spCtx *serverPoolContext) prepareRequest(ctx stdcontext.Context) error {
	stdr := spCtx.req.Std()

	url := spCtx.svr.URL + spCtx.req.Path()
	if stdr.URL.RawQuery != "" {
		url += "?" + stdr.URL.RawQuery
	}

	if !spCtx.isMirror {
		spCtx.statResult = &gohttpstat.Result{}
		ctx = gohttpstat.WithHTTPStat(ctx, spCtx.statResult)
	}

	payload := spCtx.req.GetPayload()
	stdr, err := http.NewRequestWithContext(ctx, stdr.Method, url, payload)
	if err != nil {
		return err
	}

	stdr.Header = spCtx.req.HTTPHeader()
	if !spCtx.svr.addrIsHostName {
		stdr.Host = spCtx.req.Host()
	}

	spCtx.stdReq = stdr
	return nil
}

func (spCtx *serverPoolContext) start(spanName string) {
	spCtx.startTime = fasttime.Now()
	if spanName == "" {
		spanName = spCtx.svr.URL
	}

	span := spCtx.Span().NewChildWithStart(spanName, spCtx.startTime)
	carrier := opentracing.HTTPHeadersCarrier(spCtx.stdReq.Header)
	span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, carrier)
	spCtx.span = span
}

func (spCtx *serverPoolContext) finish() {
	if spCtx.endTime.IsZero() {
		now := fasttime.Now()
		spCtx.endTime = now
		spCtx.statResult.End(now)
		spCtx.span.Finish()
	}
}

func (spCtx *serverPoolContext) duration() time.Duration {
	return spCtx.endTime.Sub(spCtx.startTime)
}

func (sp *ServerPool) handle(ctx *context.Context, isMirror bool) string {
	stdctx := ctx.Request().(*httpprot.Request).Std().Context()
	var handler Handler
	handler = func(stdctx stdcontext.Context, ctx *context.Context) string {
		spCtx := &serverPoolContext{
			Context:  ctx,
			isMirror: isMirror,
			req:      ctx.Request().(*httpprot.Request),
		}
		return sp.doHandle(stdctx, spCtx)
	}

	if sp.resilience != nil {
		if sp.resilience.circuitbreak != nil {
			handler = sp.resilience.circuitbreak.wrap(handler)
		}
		if sp.resilience.timeLimit != nil {
			handler = sp.resilience.timeLimit.wrap(handler)
		}
		if sp.resilience.retry != nil {
			handler = sp.resilience.retry.wrap(handler)
		}
	}
	return handler(stdctx, ctx)
}

func (sp *ServerPool) doHandle(stdctx stdcontext.Context, spCtx *serverPoolContext) string {
	// only collect metrics on non-mirror pools.
	if !spCtx.isMirror {
		spCtx.start(sp.spec.SpanName)

		defer func() {
			spCtx.finish()
			duration := spCtx.duration()
			spCtx.LazyAddTag(func() string {
				return stringtool.Cat(sp.name, "#duration: ", duration.String())
			})
			metric := &httpstat.Metric{}
			metric.StatusCode = spCtx.resp.StatusCode()
			metric.Duration = duration
			metric.ReqSize = uint64(spCtx.req.MetaSize() + len(spCtx.req.RawPayload()))
			metric.RespSize = uint64(spCtx.resp.MetaSize() + len(spCtx.resp.RawPayload()))
			sp.httpStat.Stat(metric)
		}()
	}

	if sp.buildResponseFromCache(spCtx) {
		return ""
	}

	spCtx.svr = sp.LoadBalancer().ChooseServer(spCtx.req)

	// if there's no available server.
	if spCtx.svr == nil {
		// ctx.AddTag is not goroutine safe, so we don't call it for
		// mirror servers.
		if spCtx.isMirror {
			return ""
		}

		spCtx.LazyAddTag(func() string {
			return "no available server"
		})

		sp.buildFailureResponse(spCtx, http.StatusServiceUnavailable)
		return resultInternalError
	}

	// prepare the request to send.
	err := spCtx.prepareRequest(stdctx)
	if err != nil {
		msg := "prepare request failed: " + err.Error()
		logger.Errorf(msg)
		if spCtx.isMirror {
			return ""
		}

		spCtx.LazyAddTag(func() string { return msg })
		sp.buildFailureResponse(spCtx, http.StatusInternalServerError)
		return resultInternalError
	}

	resp, err := fnSendRequest(spCtx.stdReq, sp.proxy.client)
	if err != nil {
		if spCtx.isMirror {
			return ""
		}

		// NOTE: May add option to cancel the tracing if failed here.
		// ctx.Span().Cancel()

		spCtx.LazyAddTag(func() string {
			return fmt.Sprintf("send request error: %v", err)
		})
		spCtx.LazyAddTag(func() string {
			return fmt.Sprintf("trace %v", spCtx.statResult)
		})
		if spCtx.stdReq.Context().Err() != nil {
			// NOTE: The HTTPContext will set 499 by itself if client is
			// Disconnected. TODO: define a constant for 499
			sp.buildFailureResponse(spCtx, 499)
			return resultClientError
		}

		sp.buildFailureResponse(spCtx, http.StatusServiceUnavailable)
		return resultServerError
	}

	if spCtx.isMirror {
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		return ""
	}

	spCtx.stdResp = resp
	if err = sp.buildResponse(spCtx); err != nil {
		sp.buildFailureResponse(spCtx, http.StatusInternalServerError)
		return resultServerError
	}

	spCtx.LazyAddTag(func() string {
		return fmt.Sprintf("code: %d", resp.StatusCode)
	})

	sp.memoryCache.Store(spCtx.req, spCtx.resp)
	return ""
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
