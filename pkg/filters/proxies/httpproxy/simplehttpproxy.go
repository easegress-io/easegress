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
	"errors"
	"fmt"
	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/resilience"
	"github.com/megaease/easegress/pkg/supervisor"
	"net/http"
	"time"
)

const (
	// SimpleHttpProxyKind is the kind of SimpleHTTPProxy.
	SimpleHttpProxyKind = "SimpleHTTPProxy"
)

var simpleHTTPProxyKind = &filters.Kind{
	Name:        SimpleHttpProxyKind,
	Description: "SimpleHTTPProxy sets the http proxy of proxy servers",
	Results: []string{
		resultInternalError,
		resultClientError,
		resultServerError,
		resultFailureCode,
		resultTimeout,
		resultShortCircuited,
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

		client      *http.Client
		compression *compression

		timeout      time.Duration
		retryWrapper resilience.Wrapper
	}

	// SimpleHTTPProxySpec describes the SimpleHTTPProxy.
	SimpleHTTPProxySpec struct {
		filters.BaseSpec `json:",inline"`

		Compression         *CompressionSpec `json:"compression,omitempty" jsonschema:"omitempty"`
		MaxIdleConns        int              `json:"maxIdleConns" jsonschema:"omitempty"`
		MaxIdleConnsPerHost int              `json:"maxIdleConnsPerHost" jsonschema:"omitempty"`
		ServerMaxBodySize   int64            `json:"serverMaxBodySize" jsonschema:"omitempty"`
		Timeout             string           `json:"timeout" jsonschema:"omitempty,format=duration"`
		RetryPolicy         string           `json:"retryPolicy" jsonschema:"omitempty"`
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
	shp.timeout, _ = time.ParseDuration(shp.spec.Timeout)
	if shp.spec.Compression != nil {
		shp.compression = newCompression(shp.spec.Compression)
	}
	// create http.Client
	shp.client = HTTPClient(nil, shp.spec.MaxIdleConns, shp.spec.MaxIdleConnsPerHost, shp.timeout)
}

// Status returns SimpleHTTPProxy status.
func (shp *SimpleHTTPProxy) Status() interface{} {
	return nil
}

// Close closes SimpleHTTPProxy.
func (shp *SimpleHTTPProxy) Close() {
	shp.client.CloseIdleConnections()
}

// Handle handles HTTPContext.
func (shp *SimpleHTTPProxy) doRequestWithRetry(request *httpprot.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	handler := func(ctx stdctx.Context) error {
		resp, err = shp.client.Do(request.Request)
		return err
	}

	if shp.retryWrapper != nil && !request.IsStream() {
		handler = shp.retryWrapper.Wrap(handler)
	}

	err = handler(request.Context())
	return resp, err
}

// Handle handles HTTPContext.
func (shp *SimpleHTTPProxy) Handle(ctx *context.Context) (result string) {
	// get request from Context
	inputReq := ctx.GetInputRequest().(*httpprot.Request)

	// send request with retry policy if set
	resp, err := shp.doRequestWithRetry(inputReq)
	if err != nil {
		return err.Error()
	}

	// use httpprot.NewResponse to build ctx.SetOutputResponse
	httpResp, err := httpprot.NewResponse(resp)
	if err != nil {
		return err.Error()
	}

	maxBodySize := shp.spec.ServerMaxBodySize
	if err = httpResp.FetchPayload(maxBodySize); err != nil {
		logger.Errorf("%s: failed to fetch response payload: %v", shp.Name, err)
		return err.Error()
	}

	// apply compression
	if shp.compression != nil {
		if shp.compression.compress(inputReq.Request, resp) {
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
