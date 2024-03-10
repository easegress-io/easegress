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

package httpproxy

import (
	stdctx "context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/v2/pkg/util/fasttime"
	"nhooyr.io/websocket"
)

// WebSocketServerPool defines a server pool.
type WebSocketServerPool struct {
	BaseServerPool

	filter        RequestMatcher
	proxy         *WebSocketProxy
	spec          *WebSocketServerPoolSpec
	httpStat      *httpstat.HTTPStat
	healthChecker proxies.HealthChecker
}

// WebSocketServerPoolSpec is the spec for a server pool.
type WebSocketServerPoolSpec struct {
	BaseServerPoolSpec `json:",inline"`
	ClientMaxMsgSize   int64               `json:"clientMaxMsgSize,omitempty"`
	ServerMaxMsgSize   int64               `json:"serverMaxMsgSize,omitempty"`
	Filter             *RequestMatcherSpec `json:"filter,omitempty"`
	InsecureSkipVerify bool                `json:"insecureSkipVerify,omitempty"`
	OriginPatterns     []string            `json:"originPatterns,omitempty"`

	HealthCheck *WSProxyHealthCheckSpec `json:"healthCheck,omitempty"`
}

// NewWebSocketServerPool creates a new server pool according to spec.
func NewWebSocketServerPool(proxy *WebSocketProxy, spec *WebSocketServerPoolSpec, name string) *WebSocketServerPool {
	sp := &WebSocketServerPool{
		proxy:         proxy,
		spec:          spec,
		httpStat:      httpstat.New(),
		healthChecker: NewWebSocketHealthChecker(spec.HealthCheck),
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
	lb.Init(proxies.NewHTTPSessionSticker, sp.healthChecker, nil)
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

// buildSuccessResponse builds a success response for WebSocket.
// The response is from WebSocket dial process.
// Here we leave the response body alone, and only set the status code and headers.
// The response body will be handled by the WebSocket protocol.
func (sp *WebSocketServerPool) buildSuccessResponse(ctx *context.Context, resp *http.Response) {
	newResp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if newResp == nil {
		newResp, _ = httpprot.NewResponse(nil)
	}
	if resp != nil {
		newResp.SetStatusCode(resp.StatusCode)
		newResp.Std().Header = resp.Header.Clone()
	}
	ctx.SetOutputResponse(newResp)
}

func buildServerURL(svr *Server, req *httpprot.Request) (string, error) {
	u := *req.URL()
	u1, err := url.ParseRequestURI(svr.URL)
	if err != nil {
		return "", err
	}

	u.Host = u1.Host

	switch u1.Scheme {
	case "ws", "wss":
		u.Scheme = u1.Scheme
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("invalid scheme %s", u1.Scheme)
	}

	return u.String(), nil
}

func (sp *WebSocketServerPool) dialServer(svr *Server, req *httpprot.Request) (*websocket.Conn, *http.Response, error) {
	u, err := buildServerURL(svr, req)
	if err != nil {
		return nil, nil, err
	}

	opts := &websocket.DialOptions{
		HTTPHeader:      req.HTTPHeader().Clone(),
		CompressionMode: websocket.CompressionDisabled,
	}

	// only set host when server address is not host name OR
	// server is explicitly told to keep the host of the request.
	if !svr.AddrIsHostName || svr.KeepHost {
		opts.Host = req.Host()
	}

	opts.HTTPHeader.Del("Sec-WebSocket-Origin")
	opts.HTTPHeader.Del("Sec-WebSocket-Protocol")
	opts.HTTPHeader.Del("Sec-WebSocket-Accept")
	opts.HTTPHeader.Del("Sec-WebSocket-Extensions")

	// According to https://docs.oracle.com/en-us/iaas/Content/Balance/Reference/httpheaders.htm
	// For load balancer, we add following key-value pairs to headers
	// X-Forwarded-For: <original_client>, <proxy1>, <proxy2>
	// X-Forwarded-Host: www.example.com:8080
	// X-Forwarded-Proto: https
	const xForwardedFor = "X-Forwarded-For"
	xff := req.HTTPHeader().Get(xForwardedFor)
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if xff == "" {
			opts.HTTPHeader.Set(xForwardedFor, clientIP)
		} else {
			opts.HTTPHeader.Set(xForwardedFor, fmt.Sprintf("%s, %s", xff, clientIP))
		}
	}

	const xForwardedHost = "X-Forwarded-Host"
	xfh := req.HTTPHeader().Get(xForwardedHost)
	if xfh == "" && req.Host() != "" {
		opts.HTTPHeader.Set(xForwardedHost, req.Host())
	}

	const xForwardedProto = "X-Forwarded-Proto"
	opts.HTTPHeader.Set(xForwardedProto, "http")
	if req.TLS != nil {
		opts.HTTPHeader.Set(xForwardedProto, "https")
	}

	conn, resp, err := websocket.Dial(stdctx.Background(), u, opts)
	if err == nil && (sp.spec.ServerMaxMsgSize > 0 || sp.spec.ServerMaxMsgSize == -1) {
		conn.SetReadLimit(sp.spec.ServerMaxMsgSize)
	}
	return conn, resp, err
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

	opts := &websocket.AcceptOptions{
		InsecureSkipVerify: sp.spec.InsecureSkipVerify,
		OriginPatterns:     sp.spec.OriginPatterns,
	}
	subProtocol := req.HTTPHeader().Get("Sec-WebSocket-Protocol")
	if subProtocol != "" {
		opts.Subprotocols = []string{subProtocol}
	}
	clntConn, err := websocket.Accept(stdw, req.Std(), opts)
	if err != nil {
		logger.Errorf("%s: failed to establish client connection: %v", sp.Name, err)
		sp.buildFailureResponse(ctx, http.StatusBadRequest)
		metric.StatusCode = http.StatusBadRequest
		return resultClientError
	}
	if sp.spec.ClientMaxMsgSize > 0 || sp.spec.ClientMaxMsgSize == -1 {
		clntConn.SetReadLimit(sp.spec.ClientMaxMsgSize)
	}

	svrConn, resp, err := sp.dialServer(svr, req)
	if err != nil {
		logger.Errorf("%s: dial to %s failed: %v", sp.Name, svr.URL, err)
		clntConn.Close(websocket.StatusGoingAway, "")
		sp.buildFailureResponse(ctx, http.StatusServiceUnavailable)
		metric.StatusCode = http.StatusServiceUnavailable
		return resultServerError
	}

	var wg sync.WaitGroup
	wg.Add(2)

	stop := make(chan struct{})

	// copy messages from client to server
	go func() {
		defer wg.Done()
		for {
			t, m, err := clntConn.Read(stdctx.Background())
			if err != nil {
				if cs := websocket.CloseStatus(err); cs == websocket.StatusNormalClosure {
					svrConn.Close(websocket.StatusNormalClosure, "")
				} else {
					svrConn.Close(cs, err.Error())
					logger.Errorf("%s: failed to read from client: %v", sp.Name, err)
				}
				break
			}
			err = svrConn.Write(stdctx.Background(), t, m)
			if err != nil {
				if cs := websocket.CloseStatus(err); cs == websocket.StatusNormalClosure {
					clntConn.Close(websocket.StatusNormalClosure, "")
				} else {
					clntConn.Close(cs, err.Error())
					logger.Errorf("%s: failed to write to server: %v", sp.Name, err)
				}
				break
			}
			metric.ReqSize += uint64(len(m))
		}
	}()

	// copy messages from server to client
	go func() {
		defer wg.Done()
		for {
			t, m, err := svrConn.Read(stdctx.Background())
			if err != nil {
				if cs := websocket.CloseStatus(err); cs == websocket.StatusNormalClosure {
					clntConn.Close(websocket.StatusNormalClosure, "")
				} else {
					clntConn.Close(cs, err.Error())
					logger.Errorf("%s: failed to read from server: %v", sp.Name, err)
				}
				break
			}
			err = clntConn.Write(stdctx.Background(), t, m)
			if err != nil {
				if cs := websocket.CloseStatus(err); cs == websocket.StatusNormalClosure {
					svrConn.Close(websocket.StatusNormalClosure, "")
				} else {
					svrConn.Close(cs, err.Error())
					logger.Errorf("%s: failed to write to client: %v", sp.Name, err)
				}
				break
			}
			metric.RespSize += uint64(len(m))
		}
	}()

	go func() {
		select {
		case <-stop:
		case <-sp.Done():
		}
		svrConn.Close(websocket.StatusBadGateway, "")
		clntConn.Close(websocket.StatusBadGateway, "")
	}()

	wg.Wait()
	close(stop)

	sp.buildSuccessResponse(ctx, resp)
	metric.StatusCode = http.StatusSwitchingProtocols
	ctx.SetData("HTTP_METRIC", metric)
	return
}

func (sp *WebSocketServerPool) status() *ServerPoolStatus {
	return &ServerPoolStatus{
		Stat: sp.httpStat.Status(),
	}
}
