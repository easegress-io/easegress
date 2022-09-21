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
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

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

	httpStat *httpstat.HTTPStat
}

// WebSocketServerPoolSpec is the spec for a server pool.
type WebSocketServerPoolSpec struct {
	Filter          *RequestMatcherSpec `json:"filter" jsonschema:"omitempty"`
	ServerTags      []string            `json:"serverTags" jsonschema:"omitempty,uniqueItems=true"`
	Servers         []*Server           `json:"servers" jsonschema:"omitempty"`
	ServiceRegistry string              `json:"serviceRegistry" jsonschema:"omitempty"`
	ServiceName     string              `json:"serviceName" jsonschema:"omitempty"`
	LoadBalance     *LoadBalanceSpec    `json:"loadBalance" jsonschema:"omitempty"`
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

func (sp *WebSocketServerPool) dialServer(svr *Server, req *httpprot.Request) (*websocket.Conn, error) {
	origin := req.HTTPHeader().Get("Origin")
	if len(origin) == 0 {
		origin = sp.proxy.spec.DefaultOrigin
	}

	u := *req.URL()
	u1, _ := url.Parse(svr.URL)
	u.Host = u1.Host
	u.Scheme = u1.Scheme

	config, err := websocket.NewConfig(u.String(), origin)
	if err != nil {
		return nil, err
	}

	// copies headers from req to the dialer and forward them to the
	// destination.
	//
	// According to https://docs.oracle.com/en-us/iaas/Content/Balance/Reference/httpheaders.htm
	// For load balancer, we add following key-value pairs to headers
	// X-Forwarded-For: <original_client>, <proxy1>, <proxy2>
	// X-Forwarded-Host: www.example.com:8080
	// X-Forwarded-Proto: https
	//
	// The websocket library discards some of the headers, so we just clone
	// the original headers and add the above headers.
	config.Header = req.HTTPHeader().Clone()

	const xForwardedFor = "X-Forwarded-For"
	xff := req.HTTPHeader().Get(xForwardedFor)
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if xff == "" {
			config.Header.Set(xForwardedFor, clientIP)
		} else {
			config.Header.Set(xForwardedFor, fmt.Sprintf("%s, %s", xff, clientIP))
		}
	}

	const xForwardedHost = "X-Forwarded-Host"
	xfh := req.HTTPHeader().Get(xForwardedHost)
	if xfh == "" && req.Host() != "" {
		config.Header.Set(xForwardedHost, req.Host())
	}

	const xForwardedProto = "X-Forwarded-Proto"
	config.Header.Set(xForwardedProto, "http")
	if req.TLS != nil {
		config.Header.Set(xForwardedProto, "https")
	}

	return websocket.DialConfig(config)
}

func (sp *WebSocketServerPool) handle(ctx *context.Context) (result string) {
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

	metric := &httpstat.Metric{StatusCode: http.StatusSwitchingProtocols}

	wssvr := websocket.Server{}
	wssvr.Handler = func(clntConn *websocket.Conn) {
		// dial to the server
		svrConn, err := sp.dialServer(svr, req)
		if err != nil {
			logger.Errorf("%s: dial to %s failed: %v", sp.name, svr.URL, err)
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)

		stop := make(chan struct{})

		// copy messages from client to server
		go func() {
			defer wg.Done()
			l, _ := io.Copy(svrConn, clntConn)
			metric.ReqSize = uint64(l)
			svrConn.Close()
		}()

		// copy messages from server to client
		go func() {
			defer wg.Done()
			l, _ := io.Copy(clntConn, svrConn)
			metric.RespSize = uint64(l)
			clntConn.Close()
		}()

		go func() {
			select {
			case <-stop:
				break
			case <-sp.done:
				svrConn.Close()
				clntConn.Close()
			}
		}()

		wg.Wait()
		close(stop)
	}

	// ServeHTTP may panic due to failed to upgrade the protocol.
	defer func() {
		if err := recover(); err != nil {
			result = resultClientError
			sp.buildFailureResponse(ctx, http.StatusBadRequest)
		}
	}()
	wssvr.ServeHTTP(stdw, req.Request)

	ctx.SetData("HTTP_METRIC", metric)
	return
}

func (sp *WebSocketServerPool) status() *ServerPoolStatus {
	return &ServerPoolStatus{
		Stat: sp.httpStat.Status(),
	}
}

func (sp *WebSocketServerPool) close() {
	close(sp.done)
	sp.wg.Wait()
}
