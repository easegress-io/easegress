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

package apiaggregator

import (
	"bytes"
	stdcontext "context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/httppipeline"
	"github.com/megaease/easegress/pkg/protocol"
	"github.com/megaease/easegress/pkg/supervisor"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/pathadaptor"
)

const (
	// Kind is the kind of APIAggregator.
	Kind = "APIAggregator"

	resultFailed = "failed"
)

var results = []string{resultFailed}

func init() {
	// FIXME: Rewrite APIAggregator because the HTTPProxy is eliminated
	// I(@xxx7xxxx) think we should not empower filter to cross pipelines.

	// httppipeline.Register(&APIAggregator{})
}

type (
	// APIAggregator is the entity to complete rate limiting.
	APIAggregator struct {
		super    *supervisor.Supervisor
		pipeSpec *httppipeline.FilterSpec
		spec     *Spec

		muxMapper protocol.MuxMapper
	}

	// Spec describes APIAggregator.
	Spec struct {
		// MaxBodyBytes in [0, 10MB]
		MaxBodyBytes   int64  `yaml:"maxBodyBytes" jsonschema:"omitempty,minimum=0,maximum=102400"`
		PartialSucceed bool   `yaml:"partialSucceed"`
		Timeout        string `yaml:"timeout" jsonschema:"omitempty,format=duration"`
		MergeResponse  bool   `yaml:"mergeResponse"`

		// User describes HTTP service target via an existing HTTPProxy
		APIProxies []*APIProxy `yaml:"apiProxies" jsonschema:"required"`

		timeout *time.Duration
	}

	// APIProxy describes the single API in EG's HTTPProxy object.
	APIProxy struct {
		// HTTPProxy's name in EG
		HTTPProxyName string `yaml:"httpProxyName" jsonschema:"required"`

		// Describes details about the request-target
		Method      string                `yaml:"method" jsonschema:"omitempty,format=httpmethod"`
		Path        *pathadaptor.Spec     `yaml:"path,omitempty" jsonschema:"omitempty"`
		Header      *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`
		DisableBody bool                  `yaml:"disableBody" jsonschema:"omitempty"`

		pa *pathadaptor.PathAdaptor
	}
)

// Kind returns the kind of APIAggregator.
func (aa *APIAggregator) Kind() string {
	return Kind
}

// DefaultSpec returns default spec of APIAggregator.
func (aa *APIAggregator) DefaultSpec() interface{} {
	return &Spec{
		Timeout:      "60s",
		MaxBodyBytes: 10240,
	}
}

// Description returns the description of APIAggregator.
func (aa *APIAggregator) Description() string {
	return "APIAggregator aggregates apis."
}

// Results returns the results of APIAggregator.
func (aa *APIAggregator) Results() []string {
	return results
}

// Init initializes APIAggregator.
func (aa *APIAggregator) Init(pipeSpec *httppipeline.FilterSpec, super *supervisor.Supervisor) {
	aa.pipeSpec, aa.spec, aa.super = pipeSpec, pipeSpec.FilterSpec().(*Spec), super
	aa.reload()
}

// Inherit inherits previous generation of APIAggregator.
func (aa *APIAggregator) Inherit(pipeSpec *httppipeline.FilterSpec,
	previousGeneration httppipeline.Filter, super *supervisor.Supervisor) {

	previousGeneration.Close()
	aa.Init(pipeSpec, super)
}

func (aa *APIAggregator) reload() {
	if aa.spec.Timeout != "" {
		timeout, err := time.ParseDuration(aa.spec.Timeout)
		if err != nil {
			logger.Errorf("BUG: parse duration %s failed: %v",
				aa.spec.Timeout, err)
		} else {
			aa.spec.timeout = &timeout
		}
	}

	for _, proxy := range aa.spec.APIProxies {
		if proxy.Path != nil {
			proxy.pa = pathadaptor.New(proxy.Path)
		}
	}
}

// Handle limits HTTPContext.
func (aa *APIAggregator) Handle(ctx context.HTTPContext) (result string) {
	result = aa.handle(ctx)
	return ctx.CallNextHandler(result)
}

// InjectMuxMapper injects mux mapper into APIAggregator.
func (aa *APIAggregator) InjectMuxMapper(mapper protocol.MuxMapper) {
	aa.muxMapper = mapper
}

func (aa *APIAggregator) handle(ctx context.HTTPContext) (result string) {
	buff := bytes.NewBuffer(nil)
	if aa.spec.MaxBodyBytes > 0 {
		written, err := io.CopyN(buff, ctx.Request().Body(), aa.spec.MaxBodyBytes+1)
		if written > aa.spec.MaxBodyBytes {
			ctx.AddTag(fmt.Sprintf("apiAggregator: request body exceed %dB", aa.spec.MaxBodyBytes))
			ctx.Response().SetStatusCode(http.StatusRequestEntityTooLarge)
			return resultFailed
		}
		if err != io.EOF {
			ctx.AddTag((fmt.Sprintf("apiAggregator: read request body failed: %v", err)))
			ctx.Response().SetStatusCode(http.StatusBadRequest)
			return resultFailed
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(aa.spec.APIProxies))

	httpResps := make([]context.HTTPResponse, len(aa.spec.APIProxies))
	// Using supervisor to call HTTPProxy object's Handle function
	for i, proxy := range aa.spec.APIProxies {
		req, err := aa.newHTTPReq(ctx, proxy, buff)
		if err != nil {
			logger.Errorf("BUG: new HTTPProxy request failed %v proxyname[%d]", err, aa.spec.APIProxies[i].HTTPProxyName)
			ctx.Response().SetStatusCode(http.StatusBadRequest)
			return resultFailed
		}

		go func(i int, name string, req *http.Request) {
			defer wg.Done()
			copyCtx, err := aa.newCtx(ctx, req, buff)
			if err != nil {
				httpResps[i] = nil
				return
			}

			handler, exists := aa.muxMapper.GetHandler(name)

			if !exists {
				httpResps[i] = nil
			} else {
				handler.Handle(copyCtx)
				httpResps[i] = copyCtx.Response()
			}
		}(i, proxy.HTTPProxyName, req)
	}

	wg.Wait()

	for _, resp := range httpResps {
		_resp := resp

		if resp != nil {
			if body, ok := _resp.Body().(io.ReadCloser); ok {
				defer body.Close()
			}
		}
	}

	data := make(map[string][]byte)

	// Get all HTTPProxy response' body
	for i, resp := range httpResps {
		if resp == nil && !aa.spec.PartialSucceed {
			ctx.AddTag(fmt.Sprintf("apiAggregator: failed in HTTPProxy %s",
				aa.spec.APIProxies[i].HTTPProxyName))
			ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
			return resultFailed
		}

		// call HTTPProxy could be successful even with a no exist backend
		// so resp is not nil, but resp.Body() is nil.
		if resp != nil && resp.Body() != nil {
			if res := aa.copyHTTPBody2Map(resp.Body(), ctx, data, aa.spec.APIProxies[i].HTTPProxyName); len(res) != 0 {
				return res
			}
		}
	}

	return aa.formatResponse(ctx, data)
}

func (aa *APIAggregator) newCtx(ctx context.HTTPContext, req *http.Request, buff *bytes.Buffer) (context.HTTPContext, error) {
	// Construct a new context for the HTTPProxy
	// responseWriter is an HTTP responseRecorder, no the original context's real
	// responseWriter, or these Proxies will overwritten each others
	w := httptest.NewRecorder()
	var stdctx stdcontext.Context = ctx
	if aa.spec.timeout != nil {
		stdctx, _ = stdcontext.WithTimeout(stdctx, *aa.spec.timeout)
	}

	copyCtx := context.New(w, req, tracing.NoopTracing, "no trace")

	return copyCtx, nil
}

func (aa *APIAggregator) newHTTPReq(ctx context.HTTPContext, proxy *APIProxy, buff *bytes.Buffer) (*http.Request, error) {
	var stdctx stdcontext.Context = ctx
	if aa.spec.timeout != nil {
		// NOTE: Cancel function could be omitted here.
		stdctx, _ = stdcontext.WithTimeout(stdctx, *aa.spec.timeout)
	}

	method := ctx.Request().Method()
	if proxy.Method != "" {
		method = proxy.Method
	}

	url := ctx.Request().Std().URL
	if proxy.pa != nil {
		url.Path = proxy.pa.Adapt(url.Path)
	}

	var body io.Reader
	if !proxy.DisableBody {
		body = bytes.NewReader(buff.Bytes())
	}

	return http.NewRequestWithContext(stdctx, method, url.String(), body)
}

func (aa *APIAggregator) copyHTTPBody2Map(body io.Reader, ctx context.HTTPContext, data map[string][]byte, name string) string {
	respBody := bytes.NewBuffer(nil)

	written, err := io.CopyN(respBody, body, aa.spec.MaxBodyBytes)
	if written > aa.spec.MaxBodyBytes {
		ctx.AddTag(fmt.Sprintf("apiAggregator: response body exceed %dB", aa.spec.MaxBodyBytes))
		ctx.Response().SetStatusCode(http.StatusInsufficientStorage)
		return resultFailed
	}
	if err != io.EOF {
		ctx.AddTag(fmt.Sprintf("apiAggregator: read response body failed: %v", err))
		ctx.Response().SetStatusCode(http.StatusInternalServerError)
		return resultFailed
	}

	data[name] = respBody.Bytes()

	return ""
}

func (aa *APIAggregator) formatResponse(ctx context.HTTPContext, data map[string][]byte) string {
	if aa.spec.MergeResponse {
		result := map[string]interface{}{}
		for _, resp := range data {
			err := jsoniter.Unmarshal(resp, &result)
			if err != nil {
				ctx.AddTag(fmt.Sprintf("apiAggregator: unmarshal %s to json object failed: %v",
					resp, err))
				ctx.Response().SetStatusCode(context.EGStatusBadResponse)
				return resultFailed
			}
		}
		buff, err := jsoniter.Marshal(result)
		if err != nil {
			ctx.AddTag(fmt.Sprintf("apiAggregator: marshal %#v to json failed: %v",
				result, err))
			logger.Errorf("apiAggregator: marshal %#v to json failed: %v", result, err)
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
			return resultFailed
		}

		ctx.Response().SetBody(bytes.NewReader(buff))
	} else {
		result := []map[string]interface{}{}
		for _, resp := range data {
			ele := map[string]interface{}{}
			err := jsoniter.Unmarshal(resp, &ele)
			if err != nil {
				ctx.AddTag(fmt.Sprintf("apiAggregator: unmarshal %s to json object failed: %v",
					resp, err))
				ctx.Response().SetStatusCode(context.EGStatusBadResponse)
				return resultFailed
			}
			result = append(result, ele)
		}
		buff, err := jsoniter.Marshal(result)
		if err != nil {
			ctx.AddTag(fmt.Sprintf("apiAggregator: marshal %#v to json failed: %v",
				result, err))
			ctx.Response().SetStatusCode(http.StatusInternalServerError)
			return resultFailed
		}

		ctx.Response().SetBody(bytes.NewReader(buff))
	}

	return ""
}

// Status returns status.
func (aa *APIAggregator) Status() interface{} {
	return nil
}

// Close closes APIAggregator.
func (aa *APIAggregator) Close() {
}
