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

// Package AIGatewayController provides AIGatewayController to manage certificates automatically.
package aigatewaycontroller

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/common"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/aicontext"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/middlewares"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/providers"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// Category is the category of AIGatewayController.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of AIGatewayController.
	Kind = "AIGatewayController"
)

var (
	aliases = []string{
		"aigatewaycontroller",
		"aigatewaycontrollers",
		"aigateway",
	}

	globalAGC atomic.Value
)

func init() {
	supervisor.Register(&AIGatewayController{})
	api.RegisterObject(&api.APIResource{
		Category:    Category,
		Kind:        Kind,
		Name:        strings.ToLower(Kind),
		Aliases:     aliases,
		ValiateHook: validateHook,
	})
}

func validateHook(operationType api.OperationType, spec *supervisor.Spec) error {
	if operationType != api.OperationTypeCreate || spec.Kind() != Kind {
		return nil
	}

	agcs := []string{}
	supervisor.GetGlobalSuper().WalkControllers(func(controller *supervisor.ObjectEntity) bool {
		if controller.Spec().Kind() == Kind {
			agcs = append(agcs, controller.Spec().Name())
		}
		return true
	})

	if len(agcs) >= 1 {
		return fmt.Errorf("only one AIGatewayController is allowed, existed: %v", agcs)
	}

	return nil
}

type (
	// AIGatewayHandler is used to handle AI traffic.
	AIGatewayHandler interface {
		Handle(ctx *context.Context, providerName string, middlewares []string) string
	}

	// AIGatewayController is the controller for AI Gateway.
	AIGatewayController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		providers   map[string]providers.Provider
		middlewares map[string]middlewares.Middleware
		metricshub  *metricshub.MetricsHub
	}

	// Spec describes AIGatewayController.
	Spec struct {
		Providers   []*aicontext.ProviderSpec     `json:"providers,omitempty"`
		Middlewares []*middlewares.MiddlewareSpec `json:"middlewares,omitempty"`
	}

	ProviderSpec struct {
		Name         string            `json:"name"`
		ProviderType string            `json:"providerType"`
		BaseURL      string            `json:"baseURL"`
		APIKey       string            `json:"apiKey"`
		Headers      map[string]string `json:"headers,omitempty"`
	}

	ObservabilitySpec struct {
		// TODO: add observability options
	}

	Status struct{}
)

// Validate validates the spec of AIGatewayController.
func (spec *Spec) Validate() error {
	nameSet := make(map[string]struct{})
	for _, p := range spec.Providers {
		if p.Name == "" {
			return fmt.Errorf("provider name cannot be empty")
		}
		if common.ValidateName(p.Name) != nil {
			return fmt.Errorf("invalid provider name: %s", p.Name)
		}
		if _, exists := nameSet[p.Name]; exists {
			return fmt.Errorf("duplicate provider name: %s", p.Name)
		}
		nameSet[p.Name] = struct{}{}

		if err := providers.ValidateSpec(p); err != nil {
			return err
		}
	}
	for _, m := range spec.Middlewares {
		err := middlewares.ValidateSpec(m)
		if err != nil {
			return fmt.Errorf("middleware %s has invalid spec: %w", m.Name, err)
		}
	}

	return nil
}

// Category returns the category of AIGatewayController.
func (agc *AIGatewayController) Category() supervisor.ObjectCategory {
	return Category
}

// Kind return the kind of AIGatewayController.
func (agc *AIGatewayController) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of AIGatewayController.
func (agc *AIGatewayController) DefaultSpec() interface{} {
	return &Spec{}
}

// Init initializes AIGatewayController.
func (agc *AIGatewayController) Init(superSpec *supervisor.Spec) {
	agc.superSpec = superSpec
	agc.spec = superSpec.ObjectSpec().(*Spec)
	agc.super = superSpec.Super()

	agc.reload(nil)
}

// Inherit inherits previous generation of AIGatewayController.
func (agc *AIGatewayController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	// call this first to unregister api and set globalAGC to nil
	previousGeneration.(*AIGatewayController).InheritClose()
	prev := previousGeneration.(*AIGatewayController)

	agc.superSpec = superSpec
	agc.spec = superSpec.ObjectSpec().(*Spec)
	agc.super = superSpec.Super()
	agc.reload(prev)
}

func (agc *AIGatewayController) reload(prev *AIGatewayController) {
	agc.providers = make(map[string]providers.Provider)
	for _, s := range agc.spec.Providers {
		provider := providers.NewProvider(s)
		agc.providers[s.Name] = provider
	}
	for _, m := range agc.spec.Middlewares {
		middleware := middlewares.NewMiddleware(m)
		agc.middlewares[m.Name] = middleware
	}

	if prev != nil && prev.metricshub != nil {
		agc.metricshub = prev.metricshub
		logger.Infof("AIGatewayController reusing MetricsHub from previous generation")
	} else {
		agc.metricshub = metricshub.New(agc.superSpec)
		logger.Infof("AIGatewayController created new MetricsHub for AIGatewayController")
	}
	globalAGC.Store(agc)

	agc.registerAPIs()
}

// Status returns the status of AIGatewayController.
func (agc *AIGatewayController) Status() *supervisor.Status {
	stats := agc.metricshub.GetStats()

	status := make(map[string]interface{})
	status["providerStats"] = stats
	return &supervisor.Status{ObjectStatus: status}
}

func (agc *AIGatewayController) InheritClose() {
	logger.Infof("close previous generation of AIGatewayController because of inherit")
	agc.unregisterAPIs()
	globalAGC.CompareAndSwap(agc, (*AIGatewayController)(nil))
}

// Close closes AIGatewayController.
func (agc *AIGatewayController) Close() {
	logger.Infof("closing AIGatewayController")
	agc.metricshub.Close()
	agc.unregisterAPIs()
	globalAGC.CompareAndSwap(agc, (*AIGatewayController)(nil))
}

func (agc *AIGatewayController) Handle(ctx *context.Context, providerName string, middlewares []string) string {
	if _, ok := agc.providers[providerName]; !ok || providerName == "" {
		agc.setErrResponse(ctx, fmt.Errorf("provider %s not found", providerName))
		return string(aicontext.ResultProviderError)
	}

	aiCtx, err := aicontext.New(ctx, agc.providers[providerName].Spec())
	if err != nil {
		agc.setErrResponse(ctx, fmt.Errorf("failed to create AI context: %w", err))
		return string(aicontext.ResultInternalError)
	}

	start := time.Now().UnixMilli()
	for _, middlewareName := range middlewares {
		if middleware, ok := agc.middlewares[middlewareName]; ok {
			middleware.Handle(aiCtx)
			if aiCtx.IsStopped() {
				agc.processResult(ctx, aiCtx, start)
				return string(aiCtx.Result())
			}
		}
	}
	provider := agc.providers[providerName]
	provider.Handle(aiCtx)
	return agc.processResult(ctx, aiCtx, start)
}

func GetGlobalAIGatewayHandler() (AIGatewayHandler, error) {
	value := globalAGC.Load()
	if value == nil {
		return nil, fmt.Errorf("no global AIGatewayController found")
	}

	agc := value.(*AIGatewayController)
	if agc == nil {
		return nil, fmt.Errorf("global AIGatewayController is nil")
	}
	return agc, nil
}

func (agc *AIGatewayController) setErrResponse(ctx *context.Context, err error) {
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
	}
	resp.SetStatusCode(http.StatusInternalServerError)
	errMsg := protocol.NewError(http.StatusInternalServerError, err.Error())
	data, _ := codectool.MarshalJSON(errMsg)
	resp.SetPayload(data)
	ctx.SetOutputResponse(resp)
}

func (agc *AIGatewayController) processResult(ctx *context.Context, aiCtx *aicontext.Context, startTime int64) string {
	endTime := time.Now().UnixMilli()
	// create easegress response
	egResp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if egResp == nil {
		egResp, _ = httpprot.NewResponse(nil)
	}
	// get AI response
	aiResp := aiCtx.GetResponse()
	if aiResp == nil {
		agc.setErrResponse(ctx, fmt.Errorf("no response found in AI context"))
		return string(aicontext.ResultInternalError)
	}

	// set ai response to easegress response
	egResp.SetStatusCode(aiResp.StatusCode)
	if aiResp.ContentLength > 0 {
		egResp.ContentLength = aiResp.ContentLength
	}
	maps.Copy(egResp.HTTPHeader(), aiResp.Header)

	var getRespBody func() []byte
	if aiResp.BodyBytes != nil {
		egResp.SetPayload(aiResp.BodyBytes)
		getRespBody = func() []byte {
			return aiResp.BodyBytes
		}
	} else if aiResp.BodyReader != nil {
		var buf bytes.Buffer
		tee := io.TeeReader(aiResp.BodyReader, &buf)
		egResp.SetPayload(tee)
		getRespBody = func() []byte {
			return buf.Bytes()
		}
	}
	ctx.SetOutputResponse(egResp)

	ctx.OnFinish(func() {
		fc := &aicontext.FinishContext{
			StatusCode: aiResp.StatusCode,
			Header:     aiResp.Header,
			RespBody:   getRespBody(),
			Duration:   endTime - startTime,
		}
		for _, cb := range aiCtx.Callbacks() {
			func() {
				defer func() {
					if err := recover(); err != nil {
						logger.Errorf("failed to execute finish action: %v, stack trace: \n%s\n", err, debug.Stack())
					}
				}()

				cb(fc)
			}()
		}
		if aiCtx.ParseMetricFn != nil {
			metric := aiCtx.ParseMetricFn(fc)
			agc.metricshub.Update(metric)
			return
		}
		metric := metricshub.Metric{
			Success:      aiResp.StatusCode == http.StatusOK,
			Provider:     aiCtx.Provider.Name,
			Duration:     fc.Duration,
			Model:        aiCtx.ReqInfo.Model,
			BaseURL:      aiCtx.Provider.BaseURL,
			ResponseType: string(aiCtx.RespType),
			ProviderType: aiCtx.Provider.ProviderType,
		}
		if aiResp.StatusCode != http.StatusOK {
			metric.Error = metricshub.MetricInternalError
		}
		agc.metricshub.Update(&metric)
	})
	return string(aiCtx.Result())
}
