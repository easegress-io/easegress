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
	stdctx "context"
	"net/http"
	"sync"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters/proxies"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"nhooyr.io/websocket"
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

	clntConn, err := websocket.Accept(stdw, req.Std(), nil)
	if err != nil {
		logger.Errorf("%s: failed to establish client connection: %v", sp.Name, err)
		sp.buildFailureResponse(ctx, http.StatusBadRequest)
		metric.StatusCode = http.StatusBadRequest
		return resultClientError
	}

	svrConn, resp, err := websocket.Dial(stdctx.Background(), svr.URL, nil)
	if err != nil {
		if resp != nil {
			metric.StatusCode = resp.StatusCode
		} else {
			metric.StatusCode = http.StatusServiceUnavailable
		}
		logger.Errorf("%s: dial to %s failed: %v", sp.Name, svr.URL, err)
		sp.buildFailureResponse(ctx, metric.StatusCode)
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
				logger.Errorf("%s: failed to read from client: %v", sp.Name, err)
				break
			}
			err = svrConn.Write(stdctx.Background(), t, m)
			if err != nil {
				logger.Errorf("%s: failed to write to server: %v", sp.Name, err)
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
				logger.Errorf("%s: failed to read from server: %v", sp.Name, err)
				break
			}
			err = clntConn.Write(stdctx.Background(), t, m)
			if err != nil {
				logger.Errorf("%s: failed to write to client: %v", sp.Name, err)
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
		svrConn.Close(websocket.StatusNormalClosure, "")
		clntConn.Close(websocket.StatusNormalClosure, "")
	}()

	wg.Wait()
	close(stop)

	metric.StatusCode = http.StatusSwitchingProtocols
	ctx.SetData("HTTP_METRIC", metric)
	return
}

func (sp *WebSocketServerPool) status() *ServerPoolStatus {
	return &ServerPoolStatus{
		Stat: sp.httpStat.Status(),
	}
}
