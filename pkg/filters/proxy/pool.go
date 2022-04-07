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
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/opentracing/opentracing-go"
	gohttpstat "github.com/tcnksm/go-httpstat"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpheader"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/protocols/httpprot/memorycache"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/callbackreader"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

// ServerPool defines a server pool.
type ServerPool struct {
	proxy *Proxy
	spec  *ServerPoolSpec
	done  chan struct{}
	wg    sync.WaitGroup
	name  string

	filter       protocols.TrafficMatcher
	loadBalancer atomic.Value

	httpStat    *httpstat.HTTPStat
	memoryCache *memorycache.MemoryCache
}

// ServerPoolSpec is the spec for a server pool.
type ServerPoolSpec struct {
	SpanName        string                       `yaml:"spanName" jsonschema:"omitempty"`
	Filter          *httpprot.TrafficMatcherSpec `yaml:"filter" jsonschema:"omitempty"`
	ServersTags     []string                     `yaml:"serversTags" jsonschema:"omitempty,uniqueItems=true"`
	Servers         []*Server                    `yaml:"servers" jsonschema:"omitempty"`
	ServiceRegistry string                       `yaml:"serviceRegistry" jsonschema:"omitempty"`
	ServiceName     string                       `yaml:"serviceName" jsonschema:"omitempty"`
	LoadBalance     *httpprot.LoadBalancerSpec   `yaml:"loadBalance" jsonschema:"required"`
	MemoryCache     *memorycache.Spec            `yaml:"memoryCache,omitempty" jsonschema:"omitempty"`
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
		if server.W > 0 {
			serversGotWeight++
		}
	}
	if serversGotWeight > 0 && serversGotWeight < len(sps.Servers) {
		msgFmt := "not all servers have weight(%d/%d)"
		return fmt.Errorf(msgFmt, serversGotWeight, len(sps.Servers))
	}

	return nil
}

func newPool(proxy *Proxy, spec *ServerPoolSpec, name string, failureCodes []int) *ServerPool {
	sp := &ServerPool{
		proxy: proxy,
		spec:  spec,
		done:  make(chan struct{}),
		name:  name,
	}

	if spec.Filter != nil {
		sp.filter, _ = httpprot.NewTrafficMatcher(spec.Filter)
	}

	if spec.MemoryCache != nil {
		sp.memoryCache = memorycache.New(spec.MemoryCache)
	}

	if spec.ServiceRegistry == "" || spec.ServiceName == "" {
		sp.createLoadBalancer(sp.spec.Servers)
	} else {
		sp.watchServers()
	}

	return sp

	// writeResponse: writeResponse,
	// httpStat:    httpstat.New(),
}

func (sp *ServerPool) createLoadBalancer(servers []*Server) {
	svrs := make([]protocols.Server, len(servers))
	for _, server := range servers {
		server.checkAddrPattern()
		svrs = append(svrs, server)
	}

	spec := sp.spec.LoadBalance
	if spec == nil {
		spec = &httpprot.LoadBalancerSpec{}
	}

	lb, _ := httpprot.NewLoadBalancer(spec, svrs)
	if old := sp.loadBalancer.Swap(lb); old != nil {
		old.(protocols.LoadBalancer).Close()
	}
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
					URL:  instance.URL(),
					Tags: instance.Tags,
					W:    instance.Weight,
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

var requestPool = sync.Pool{
	New: func() interface{} {
		return &request{}
	},
}

var httpstatResultPool = sync.Pool{
	New: func() interface{} {
		return &gohttpstat.Result{}
	},
}

func (sp *ServerPool) handle(ctx context.Context, isMirror bool) string {
	req := ctx.Request()

	svr := sp.loadBalancer.Load().(protocols.LoadBalancer).ChooseServer(req)
	if svr == nil {
		if isMirror {
			return ""
		}

		ctx.AddLazyTag(func() string {
			return "no available server"
		})
		resp := httpprot.NewResponse(nil)
		resp.SetStatusCode(http.StatusServiceUnavailable)
		ctx.SetResponse(ctx.TargetResponseID(), resp)
		return resultInternalError
	}

	if isMirror {
		req = req.Clone()
		go func() {
			if resp, _ := svr.SendRequest(req); resp != nil {
				resp.Close()
				/*
					// NOTE: Need to be read to completion and closed.
					// Reference: https://golang.org/pkg/net/http/#Response
					// And we do NOT do statistics of duration and respSize
					// for it, because we can't wait for it to finish.
					defer resp.Body.Close()
					io.Copy(io.Discard, resp.Body)
				*/
			}
		}()

		return ""
	}

	req, err := sp.newRequest(ctx, server, reqBody, requestPool, httpstatResultPool)
	if err != nil {
		msg := stringtool.Cat("prepare request failed: ", err.Error())
		logger.Errorf("BUG: %s", msg)
		addLazyTag("bug", msg, -1)
		setStatusCode(http.StatusInternalServerError)
		return resultInternalError
	}

	resp, span, err := sp.doRequest(ctx, req, client)
	if err != nil {
		// NOTE: May add option to cancel the tracing if failed here.
		// ctx.Span().Cancel()

		addLazyTag("doRequestErr", fmt.Sprintf("%v", err), -1)
		addLazyTag("trace", req.detail(), -1)
		if ctx.ClientDisconnected() {
			// NOTE: The HTTPContext will set 499 by itself if client is Disconnected.
			// w.SetStatusCode((499)
			return resultClientError
		}

		setStatusCode(http.StatusServiceUnavailable)
		return resultServerError
	}

	addLazyTag("code", "", resp.StatusCode)

	ctx.Lock()
	defer ctx.Unlock()
	// NOTE: The code below can't use addTag and setStatusCode in case of deadlock.

	respBody := sp.statRequestResponse(ctx, req, resp, span)

	ctx.Response().SetStatusCode(resp.StatusCode)
	ctx.Response().Header().AddFromStd(resp.Header)
	ctx.Response().SetBody(respBody)

	return ""
}

func (sp *ServerPool) doRequest(ctx context.Context, req *request, client *http.Client) (*http.Response, tracing.Span, error) {
	req.start()

	spanName := sp.spec.SpanName
	if spanName == "" {
		spanName = req.server.URL
	}

	span := ctx.Span().NewChildWithStart(spanName, req.startTime())
	span.Tracer().Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.std.Header))

	resp, err := fnSendRequest(req.std, client)
	if err != nil {
		return nil, nil, err
	}
	return resp, span, nil
}

var httpstatMetricPool = sync.Pool{
	New: func() interface{} {
		return &httpstat.Metric{}
	},
}

func (sp *ServerPool) statRequestResponse(ctx context.Context,
	req *request, resp *http.Response, span tracing.Span) io.Reader {

	var count int

	callbackBody := callbackreader.New(resp.Body)
	callbackBody.OnAfter(func(num int, p []byte, n int, err error) ([]byte, int, error) {
		count += n
		if err == io.EOF {
			req.finish()
			span.Finish()
		}

		return p, n, err
	})

	ctx.OnFinish(func() {
		if !sp.writeResponse {
			req.finish()
			span.Finish()
		}
		duration := req.total()
		ctx.AddLazyTag(func() string {
			return stringtool.Cat(sp.name, "#duration: ", duration.String())
		})
		// use recycled object
		metric := httpstatMetricPool.Get().(*httpstat.Metric)
		metric.StatusCode = resp.StatusCode
		metric.Duration = duration
		metric.ReqSize = ctx.Request().Size()
		metric.RespSize = uint64(responseMetaSize(resp) + count)

		if !sp.writeResponse {
			metric.RespSize = 0
		}
		sp.httpStat.Stat(metric)
		// recycle struct instances
		httpstatMetricPool.Put(metric)
		httpstatResultPool.Put(req.statResult)
		requestPool.Put(req)
	})

	return callbackBody
}

func responseMetaSize(resp *http.Response) int {
	text := http.StatusText(resp.StatusCode)
	if text == "" {
		text = "status code " + strconv.Itoa(resp.StatusCode)
	}

	// meta length is the length of:
	// resp.Proto + " "
	// + strconv.Itoa(resp.StatusCode) + " "
	// + text + "\r\n",
	// + resp.Header().Dump() + "\r\n\r\n"
	//
	// but to improve performance, we won't build this string

	size := len(resp.Proto) + 1
	if resp.StatusCode >= 100 && resp.StatusCode < 1000 {
		size += 3 + 1
	} else {
		size += len(strconv.Itoa(resp.StatusCode)) + 1
	}
	size += len(text) + 2
	size += httpheader.New(resp.Header).Length() + 4

	return size
}

func (sp *ServerPool) close() {
	close(sp.done)
	sp.wg.Wait()

	if lb := sp.loadBalancer.Load(); lb != nil {
		lb.(protocols.LoadBalancer).Close()
	}
}
