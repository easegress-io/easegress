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
	"github.com/megaease/easegress/pkg/object/rawconfigtrafficcontroller"
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
	httppipeline.Register(&APIAggregator{})
}

type (
	// APIAggregator is a filter to aggregate several HTTP API responses.
	APIAggregator struct {
		filterSpec *httppipeline.FilterSpec
		spec       *Spec

		// rctc is for getting Pipeline in default namespace.
		rctc *rawconfigtrafficcontroller.RawConfigTrafficController
	}

	// Spec is APIAggregator's spec.
	Spec struct {
		// MaxBodyBytes in [0, 10MB]
		MaxBodyBytes int64 `yaml:"maxBodyBytes" jsonschema:"omitempty,minimum=0,maximum=102400"`

		// PartialSucceed indicates wether Whether regards the result of the original request as successful
		// or not when a request to some of the API pipelines fails.
		PartialSucceed bool `yaml:"partialSucceed"`

		// Timeout is the request duration for each APIs.
		Timeout string `yaml:"timeout" jsonschema:"omitempty,format=duration"`

		// MergeResponse indicates whether to merge JSON response bodies or not.
		MergeResponse bool `yaml:"mergeResponse"`

		// User describes HTTP service target via an existing Pipeline.
		Pipelines []*Pipeline `yaml:"pipelines" jsonschema:"required"`

		timeout *time.Duration
	}

	// Pipeline is the single API HTTP Pipeline in default namespace.
	Pipeline struct {
		// Name is the name of pipeline in EG
		Name string `yaml:"name" jsonschema:"required"`

		// Method is the HTTP method for requesting this pipeline.
		Method string `yaml:"method" jsonschema:"omitempty,format=httpmethod"`

		// Path is the HTTP request path adaptor for requesting this pipeline.
		Path *pathadaptor.Spec `yaml:"path,omitempty" jsonschema:"omitempty"`

		// Header is the HTTP header adaptor for requestring this pipeline.
		Header *httpheader.AdaptSpec `yaml:"header,omitempty" jsonschema:"omitempty"`

		// DisableBody discart this pipeline's response body if it set to true.
		DisableBody bool `yaml:"disableBody" jsonschema:"omitempty"`

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
func (aa *APIAggregator) Init(filterSpec *httppipeline.FilterSpec) {
	aa.filterSpec, aa.spec = filterSpec, filterSpec.FilterSpec().(*Spec)
	entity, exists := filterSpec.Super().GetSystemController(rawconfigtrafficcontroller.Kind)
	if !exists {
		panic(fmt.Errorf("BUG: raw config traffic controller not found"))
	}

	rctc, ok := entity.Instance().(*rawconfigtrafficcontroller.RawConfigTrafficController)
	if !ok {
		panic(fmt.Errorf("BUG: want *RawConfigTrafficController, got %T", entity.Instance()))
	}
	aa.rctc = rctc
	aa.reload()
}

// Inherit inherits previous generation of APIAggregator.
func (aa *APIAggregator) Inherit(filterSpec *httppipeline.FilterSpec, previousGeneration httppipeline.Filter) {
	previousGeneration.Close()
	aa.Init(filterSpec)
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

	for _, p := range aa.spec.Pipelines {
		if p.Path != nil {
			p.pa = pathadaptor.New(p.Path)
		}
	}
}

// Handle limits HTTPContext.
func (aa *APIAggregator) Handle(ctx context.HTTPContext) (result string) {
	result = aa.handle(ctx)
	return ctx.CallNextHandler(result)
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
			ctx.AddTag(fmt.Sprintf("apiAggregator: read request body failed: %v", err))
			ctx.Response().SetStatusCode(http.StatusBadRequest)
			return resultFailed
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(aa.spec.Pipelines))

	httpResps := make([]context.HTTPResponse, len(aa.spec.Pipelines))
	for i, p := range aa.spec.Pipelines {
		req, err := aa.newHTTPReq(ctx, p, buff)
		if err != nil {
			logger.Errorf("BUG: new HTTP request failed: %v, pipelinename: %s", err, aa.spec.Pipelines[i].Name)
			ctx.Response().SetStatusCode(http.StatusBadRequest)
			return resultFailed
		}

		go func(i int, name string, req *http.Request) {
			defer wg.Done()
			handler, exists := aa.rctc.GetHTTPPipeline(name)
			if !exists {
				logger.Errorf("pipeline: %s not found in current namespace", name)
				return
			}
			w := httptest.NewRecorder()
			copyCtx := context.New(w, req, tracing.NoopTracing, "no trace")
			handler.Handle(copyCtx)
			rsp := copyCtx.Response()

			if rsp != nil && rsp.StatusCode() == http.StatusOK {
				httpResps[i] = rsp
			}
		}(i, p.Name, req)
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

	// Get all HTTPPipeline response' body
	for i, resp := range httpResps {
		if resp == nil && !aa.spec.PartialSucceed {
			ctx.Response().Std().Header().Set("X-EG-Aggregator", fmt.Sprintf("failed-in-%s",
				aa.spec.Pipelines[i].Name))
			ctx.Response().SetStatusCode(http.StatusServiceUnavailable)
			return resultFailed
		}

		if resp != nil && resp.Body() != nil {
			if res := aa.copyHTTPBody2Map(resp.Body(), ctx, data, aa.spec.Pipelines[i].Name); len(res) != 0 {
				return res
			}
		}
	}

	return aa.formatResponse(ctx, data)
}

func (aa *APIAggregator) newHTTPReq(ctx context.HTTPContext, p *Pipeline, buff *bytes.Buffer) (*http.Request, error) {
	var stdctx stdcontext.Context = ctx
	if aa.spec.timeout != nil {
		// NOTE: Cancel function could be omitted here.
		stdctx, _ = stdcontext.WithTimeout(stdctx, *aa.spec.timeout)
	}

	method := ctx.Request().Method()
	if p.Method != "" {
		method = p.Method
	}

	url := ctx.Request().Std().URL
	if p.pa != nil {
		url.Path = p.pa.Adapt(url.Path)
	}

	var body io.Reader
	if !p.DisableBody {
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
