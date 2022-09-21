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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/serviceregistry"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"golang.org/x/net/websocket"
)

// WebSocketServerPool defines a server pool.
type WebSocketServerPool struct {
	proxy *WebSocketProxy
	spec  *WebSocketServerPoolSpec
	done  chan struct{}
	wg    sync.WaitGroup
	name  string

	filter       RequestMatcher
	loadBalancer atomic.Value
	timeout      time.Duration

	httpStat *httpstat.HTTPStat
}

// WebSocketServerPoolSpec is the spec for a server pool.
type WebSocketServerPoolSpec struct {
	SpanName        string              `json:"spanName" jsonschema:"omitempty"`
	Filter          *RequestMatcherSpec `json:"filter" jsonschema:"omitempty"`
	ServerTags      []string            `json:"serverTags" jsonschema:"omitempty,uniqueItems=true"`
	Servers         []*Server           `json:"servers" jsonschema:"omitempty"`
	ServiceRegistry string              `json:"serviceRegistry" jsonschema:"omitempty"`
	ServiceName     string              `json:"serviceName" jsonschema:"omitempty"`
	LoadBalance     *LoadBalanceSpec    `json:"loadBalance" jsonschema:"omitempty"`
	Timeout         string              `json:"timeout" jsonschema:"omitempty,format=duration"`
}

// Validate validates WebSocketServerPoolSpec.
func (sps *WebSocketServerPoolSpec) Validate() error {
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

// NewWebSocketServerPool creates a new server pool according to spec.
func NewWebSocketServerPool(proxy *WebSocketProxy, spec *WebSocketServerPoolSpec, name string) *WebSocketServerPool {
	sp := &WebSocketServerPool{
		proxy:    proxy,
		spec:     spec,
		done:     make(chan struct{}),
		name:     name,
		httpStat: httpstat.New(),
	}

	if spec.Filter != nil {
		sp.filter = NewRequestMatcher(spec.Filter)
	}

	if spec.ServiceRegistry == "" || spec.ServiceName == "" {
		sp.createLoadBalancer(sp.spec.Servers)
	} else {
		sp.watchServers()
	}

	if spec.Timeout != "" {
		sp.timeout, _ = time.ParseDuration(spec.Timeout)
	}

	return sp
}

// LoadBalancer returns the load balancer of the server pool.
func (sp *WebSocketServerPool) LoadBalancer() LoadBalancer {
	return sp.loadBalancer.Load().(LoadBalancer)
}

func (sp *WebSocketServerPool) createLoadBalancer(servers []*Server) {
	for _, server := range servers {
		u := strings.ToLower(server.URL)
		if strings.HasPrefix(u, "http") {
			u = "ws" + u[4:]
		}
		server.URL = u
		server.checkAddrPattern()
	}

	spec := sp.spec.LoadBalance
	if spec == nil {
		spec = &LoadBalanceSpec{}
	}

	lb := NewLoadBalancer(spec, servers)
	sp.loadBalancer.Store(lb)
}

func (sp *WebSocketServerPool) watchServers() {
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

func (sp *WebSocketServerPool) useService(instances map[string]*serviceregistry.ServiceInstanceSpec) {
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

func (sp *WebSocketServerPool) buildFailureResponse(ctx *context.Context, statusCode int) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}

	resp.SetStatusCode(statusCode)
	ctx.SetOutputResponse(resp)
}

func (sp *WebSocketServerPool) doHandle(conn *websocket.Conn) {
}

func (sp *WebSocketServerPool) handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	svr := sp.LoadBalancer().ChooseServer(req)

	// if there's no available server.
	if svr == nil {
		logger.Debugf("%s: no available server", sp.name)
		sp.buildFailureResponse(ctx, http.StatusServiceUnavailable)
		return resultInternalError
	}

	stdw, _ := ctx.GetData("HTTP_RESPONSE_WRITER").(http.ResponseWriter)
	if stdw == nil {
		sp.buildFailureResponse(ctx, http.StatusInternalServerError)
		return resultInternalError
	}

	svrConn, err := websocket.Dial(svr.URL, "", svr.URL)
	if err != nil {
	}

	var handler websocket.Handler = func(clntConn *websocket.Conn) {
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			io.Copy(svrConn, clntConn)
		}()

		go func() {
			defer wg.Done()
			io.Copy(clntConn, svrConn)
		}()

		go func() {
			select {
			case <-sp.done:
				svrConn.Close()
				clntConn.Close()
			case <-req.Context().Done():
				svrConn.Close()
				clntConn.Close()
			}
		}()

		wg.Wait()
	}

	ctx.SetData("HTTP_METRIS", &httpstat.Metric{
		StatusCode: 200,
		ReqSize:    100,
		RespSize:   100,
	})
	handler.ServeHTTP(stdw, req.Request)
	return ""
}

func (sp *WebSocketServerPool) status() *ServerPoolStatus {
	s := &ServerPoolStatus{Stat: sp.httpStat.Status()}
	return s
}

func (sp *WebSocketServerPool) close() {
	close(sp.done)
	sp.wg.Wait()
}
