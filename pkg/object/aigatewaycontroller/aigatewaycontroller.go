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
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/megaease/easegress/v2/pkg/api"
	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/metricshub"
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
		Handle(ctx *context.Context, providerName string) string
	}

	// AIGatewayController is the controller for AI Gateway.
	AIGatewayController struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		providers  map[string]providers.Provider
		metricshub *metricshub.MetricsHub
	}

	// Spec describes AIGatewayController.
	Spec struct {
		Providers []*ProviderSpec `json:"providers,omitempty"`
	}

	ProviderSpec struct {
		Name         string            `json:"name"`
		ProviderType string            `json:"providerType"`
		BaseURL      string            `json:"baseURL"`
		APIKey       string            `json:"apiKey"`
		Headers      map[string]string `json:"headers,omitempty"`
	}

	Status struct {
	}
)

// Validate validates the spec of AIGatewayController.
func (spec *Spec) Validate() error {
	// TODO
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

	agc.reload()
}

// Inherit inherits previous generation of AIGatewayController.
func (agc *AIGatewayController) Inherit(superSpec *supervisor.Spec, previousGeneration supervisor.Object) {
	agc.superSpec = superSpec
	agc.spec = superSpec.ObjectSpec().(*Spec)
	agc.super = superSpec.Super()

	agc.reload()
	previousGeneration.(*AIGatewayController).Close()
}

func (agc *AIGatewayController) reload() {
	// TODO
	agc.providers = make(map[string]providers.Provider)
	for _, s := range agc.spec.Providers {
		provider := NewProvider(s)
		agc.providers[s.Name] = provider
	}
	agc.metricshub = metricshub.New()

	globalAGC.Store(agc)
}

// Status returns the status of AIGatewayController.
func (agc *AIGatewayController) Status() *supervisor.Status {
	status := &Status{}
	return &supervisor.Status{ObjectStatus: status}
}

// Close closes AIGatewayController.
func (agc *AIGatewayController) Close() {
	// TODO close
	globalAGC.CompareAndSwap(agc, (*AIGatewayController)(nil))
}

func (agc *AIGatewayController) Handle(ctx *context.Context, providerName string) string {
	if _, ok := agc.providers[providerName]; !ok || providerName == "" {
		setErrResponse(ctx, fmt.Errorf("provider %s not found", providerName))
		return resultProviderNotFoundError
	}

	req := ctx.GetInputRequest().(*httpprot.Request)
	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(nil)
		ctx.SetOutputResponse(resp)
	}

	provider := agc.providers[providerName]
	metrics, err := provider.Handle(ctx, req, resp)
	if err != nil {
		setErrResponse(ctx, err)
		return resultInternalError
	}

	agc.metricshub.Update(metrics)
	return ""
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

func setErrResponse(ctx *context.Context, err error) {
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
