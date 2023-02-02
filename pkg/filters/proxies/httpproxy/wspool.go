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

package httpproxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters/proxies"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"golang.org/x/net/websocket"
)

// WebSocketServerPool defines a server pool.
type WebSocketServerPool struct {
	BaseServerPool

	filter   RequestMatcher
	proxy    *WebSocketProxy
	spec     *WebSocketServerPoolSpec
	httpStat *httpstat.HTTPStat
}

// WebSocketServerPoolSpec is the spec for a server pool.
type WebSocketServerPoolSpec struct {
	BaseServerPoolSpec `json:",inline"`
	Filter             *RequestMatcherSpec `json:"filter" jsonschema:"omitempty"`
}

// NewWebSocketServerPool creates a new server pool according to spec.
func NewWebSocketServerPool(proxy *WebSocketProxy, spec *WebSocketServerPoolSpec, name string) *WebSocketServerPool {
	sp := &WebSocketServerPool{
		proxy:    proxy,
		spec:     spec,
		httpStat: httpstat.New(),
	}
	if spec.Filter != nil {
		sp.filter = NewRequestMatcher(spec.Filter)
	}
	sp.Init(sp, proxy.super, name, &spec.BaseServerPoolSpec)
	return sp
}

// CreateLoadBalancer creates a load balancer according to spec.
func (sp *WebSocketServerPool) CreateLoadBalancer(spec *LoadBalanceSpec, servers []*Server) LoadBalancer {
	lb := proxies.NewGeneralLoadBalancer(spec, servers)
	lb.Init(proxies.NewHTTPSessionSticker, proxies.NewHTTPHealthChecker, nil)
	return lb
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
	u1, err := url.ParseRequestURI(svr.URL)
	if err != nil {
		return nil, err
	}
	u.Host = u1.Host
	switch u1.Scheme {
	case "ws", "wss":
		u.Scheme = u1.Scheme
		break
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return nil, fmt.Errorf("invalid scheme %s", u1.Scheme)
	}

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

	metric := &httpstat.Metric{}
	startTime := fasttime.Now()
	defer func() {
		metric.Duration = fasttime.Since(startTime)
		sp.httpStat.Stat(metric)
	}()

	// if there's no available server.
	if svr == nil {
		logger.Errorf("%s: no available server", sp.Name)
		sp.buildFailureResponse(ctx, http.StatusServiceUnavailable)
		metric.StatusCode = http.StatusServiceUnavailable
		return resultInternalError
	}

	stdw, _ := ctx.GetData("HTTP_RESPONSE_WRITER").(http.ResponseWriter)
	if stdw == nil {
		logger.Errorf("%s: cannot get response writer from context", sp.Name)
		sp.buildFailureResponse(ctx, http.StatusInternalServerError)
		metric.StatusCode = http.StatusInternalServerError
		return resultInternalError
	}

	wssvr := websocket.Server{}
	wssvr.Handler = func(clntConn *websocket.Conn) {
		// dial to the server
		svrConn, err := sp.dialServer(svr, req)
		if err != nil {
			logger.Errorf("%s: dial to %s failed: %v", sp.Name, svr.URL, err)
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
			case <-sp.Done():
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
			metric.StatusCode = http.StatusBadRequest
		}
	}()
	wssvr.ServeHTTP(stdw, req.Request)

	metric.StatusCode = http.StatusSwitchingProtocols
	ctx.SetData("HTTP_METRIC", metric)
	return
}

func (sp *WebSocketServerPool) status() *ServerPoolStatus {
	return &ServerPoolStatus{
		Stat: sp.httpStat.Status(),
	}
}
