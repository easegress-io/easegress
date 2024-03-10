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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/v2/pkg/resilience"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/fasttime"
)

var simpleHTTPProxyKind = &filters.Kind{
	Name:        "SimpleHTTPProxy",
	Description: "SimpleHTTPProxy sets the http proxy of proxy servers",
	Results: []string{
		resultInternalError,
		resultClientError,
		resultServerError,
		resultFailureCode,
		resultTimeout,
	},
	DefaultSpec: func() filters.Spec {
		return &SimpleHTTPProxySpec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &SimpleHTTPProxy{
			super: spec.Super(),
			spec:  spec.(*SimpleHTTPProxySpec),
		}
	},
}

var _ filters.Filter = (*SimpleHTTPProxy)(nil)

func init() {
	filters.Register(simpleHTTPProxyKind)
}

type (
	// SimpleHTTPProxy is the filter SimpleHTTPProxy.
	SimpleHTTPProxy struct {
		super *supervisor.Supervisor
		spec  *SimpleHTTPProxySpec

		done        chan struct{}
		client      *http.Client
		compression *compression

		timeout      time.Duration
		retryWrapper resilience.Wrapper
	}

	// SimpleHTTPProxySpec describes the SimpleHTTPProxy.
	SimpleHTTPProxySpec struct {
		filters.BaseSpec `json:",inline"`

		Compression         *CompressionSpec `json:"compression,omitempty"`
		MaxIdleConns        int              `json:"maxIdleConns,omitempty"`
		MaxIdleConnsPerHost int              `json:"maxIdleConnsPerHost,omitempty"`
		ServerMaxBodySize   int64            `json:"serverMaxBodySize,omitempty"`
		Timeout             string           `json:"timeout,omitempty" jsonschema:"format=duration"`
		RetryPolicy         string           `json:"retryPolicy,omitempty"`
	}
)

// Validate validates SimpleHTTPProxySpec.
func (s *SimpleHTTPProxySpec) Validate() error {
	// Validate MaxIdleConns
	if s.MaxIdleConns < 0 {
		return errors.New("maxIdleConns must be greater than or equal to 0")
	}

	// Validate MaxIdleConnsPerHost
	if s.MaxIdleConnsPerHost < 0 {
		return errors.New("maxIdleConnsPerHost must be greater than or equal to 0")
	}

	// Validate Timeout
	if s.Timeout != "" {
		_, err := time.ParseDuration(s.Timeout)
		if err != nil {
			return err
		}
	}

	return nil
}

// Name returns the name of the SimpleHTTPProxySpec filter instance.
func (shp *SimpleHTTPProxy) Name() string {
	return shp.spec.Name()
}

// Kind returns the kind of SimpleHTTPProxy.
func (shp *SimpleHTTPProxy) Kind() *filters.Kind {
	return simpleHTTPProxyKind
}

// Spec returns the spec used by the SimpleHTTPProxy.
func (shp *SimpleHTTPProxy) Spec() filters.Spec {
	return shp.spec
}

// Init initializes SimpleHTTPProxy.
func (shp *SimpleHTTPProxy) Init() {
	shp.reload()
}

// Inherit inherits previous generation of SimpleHTTPProxy.
func (shp *SimpleHTTPProxy) Inherit(previousGeneration filters.Filter) {
	shp.reload()
}

func (shp *SimpleHTTPProxy) reload() {
	shp.done = make(chan struct{})
	shp.timeout, _ = time.ParseDuration(shp.spec.Timeout)
	if shp.spec.Compression != nil {
		shp.compression = newCompression(shp.spec.Compression)
	}
	// create http.Client
	clientSpec := &HTTPClientSpec{
		MaxIdleConns:        shp.spec.MaxIdleConns,
		MaxIdleConnsPerHost: shp.spec.MaxIdleConnsPerHost,
	}
	shp.client = HTTPClient(nil, clientSpec, shp.timeout)
}

// Status returns SimpleHTTPProxy status.
func (shp *SimpleHTTPProxy) Status() interface{} {
	return nil
}

// Close closes SimpleHTTPProxy.
func (shp *SimpleHTTPProxy) Close() {
	close(shp.done)
	shp.client.CloseIdleConnections()
}

func (shp *SimpleHTTPProxy) buildFailureResponse(ctx *context.Context, statusCode int) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}

	resp.SetStatusCode(statusCode)
	ctx.SetOutputResponse(resp)
}

func (shp *SimpleHTTPProxy) handleConnect(ctx *context.Context) string {
	metric := &httpstat.Metric{}
	startTime := fasttime.Now()

	host := ctx.GetInputRequest().(*httpprot.Request).Host()
	w, _ := ctx.GetData("HTTP_RESPONSE_WRITER").(http.ResponseWriter)

	destConn, err := net.Dial("tcp", host)
	if err != nil {
		shp.buildFailureResponse(ctx, http.StatusServiceUnavailable)
		return resultServerError
	}
	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		shp.buildFailureResponse(ctx, http.StatusInternalServerError)
		return resultInternalError
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		shp.buildFailureResponse(ctx, http.StatusServiceUnavailable)
		return resultServerError
	}

	var wg sync.WaitGroup
	wg.Add(2)

	stop := make(chan struct{})

	go func() {
		defer wg.Done()
		n, _ := io.Copy(destConn, clientConn)
		metric.ReqSize = uint64(n)
	}()

	go func() {
		defer wg.Done()
		n, _ := io.Copy(clientConn, destConn)
		metric.RespSize = uint64(n)
	}()

	go func() {
		select {
		case <-shp.done:
		case <-stop:
		}
		destConn.Close()
		clientConn.Close()
	}()

	wg.Wait()
	close(stop)

	metric.StatusCode = http.StatusOK
	metric.Duration = fasttime.Since(startTime)
	ctx.SetData("HTTP_METRIC", metric)

	return ""
}

// Handle handles HTTPContext.
func (shp *SimpleHTTPProxy) doRequestWithRetry(req *httpprot.Request) (*http.Response, error) {
	var resp *http.Response

	handler := func(ctx stdctx.Context) error {
		payload := req.GetPayload()
		stdr, err := http.NewRequestWithContext(ctx, req.Method(), req.URL().String(), payload)
		if err != nil {
			return err
		}
		stdr.Header = req.HTTPHeader().Clone()
		removeHopByHopHeaders(stdr.Header)
		resp, err = shp.client.Do(stdr)
		return err
	}

	if shp.retryWrapper != nil && !req.IsStream() {
		handler = shp.retryWrapper.Wrap(handler)
	}

	err := handler(req.Context())
	return resp, err
}

// Handle handles HTTPContext.
func (shp *SimpleHTTPProxy) Handle(ctx *context.Context) (result string) {
	// get request from Context
	req := ctx.GetInputRequest().(*httpprot.Request)

	// if method is connect, then we are working as a forward proxy
	if req.Method() == http.MethodConnect {
		logger.Infof("%s: handling CONNECT request", shp.Name())
		return shp.handleConnect(ctx)
	}

	// send request with retry policy if set
	resp, err := shp.doRequestWithRetry(req)
	if err != nil {
		logger.Errorf("%s: failed to send request: %v", shp.Name(), err)
		return resultServerError
	}

	// use httpprot.NewResponse to build ctx.SetOutputResponse
	httpResp, _ := httpprot.NewResponse(resp)

	maxBodySize := shp.spec.ServerMaxBodySize
	if err = httpResp.FetchPayload(maxBodySize); err != nil {
		logger.Errorf("%s: failed to fetch response payload: %v, please consider to set serverMaxBodySize of SimpleHTTPProxy to -1.", shp.Name(), err)
		return resultServerError
	}

	// apply compression
	if shp.compression != nil {
		if shp.compression.compress(req.Request, resp) {
			ctx.AddTag("gzip")
		}
	}

	// set response to Context
	ctx.SetOutputResponse(httpResp)
	return ""
}

func (shp *SimpleHTTPProxy) InjectResiliencePolicy(policies map[string]resilience.Policy) {
	name := shp.spec.RetryPolicy
	if name != "" {
		p := policies[name]
		if p == nil {
			panic(fmt.Errorf("retry policy %s not found", name))
		}
		policy, ok := p.(*resilience.RetryPolicy)
		if !ok {
			panic(fmt.Errorf("policy %s is not a retry policy", name))
		}
		shp.retryWrapper = policy.CreateWrapper()
	}
}
