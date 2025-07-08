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

// Package AIGatewayProxy provides AIGatewayProxy filter.
package AIGatewayProxy

import (
	"net/http"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller"
	"github.com/megaease/easegress/v2/pkg/object/aigatewaycontroller/protocol"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

const (
	// Kind is the kind of AIGatewayProxy.
	Kind = "AIGatewayProxy"

	resultNoController = "noAIGatewayControllerError"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Handle AI Gateway traffics.",
	Results:     append(aigatewaycontroller.HandlerResults(), resultNoController),
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &AIGatewayProxy{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// AIGatewayProxy is filter AIGatewayProxy.
	AIGatewayProxy struct {
		spec *Spec
	}

	// Spec describes the AIGatewayProxy.
	Spec struct {
		filters.BaseSpec `json:",inline"`
		ProviderName     string   `json:"providerName"`
		Middlewares      []string `json:"middlewares,omitempty"`
	}
)

// Name returns the name of the AIGatewayProxy filter instance.
func (p *AIGatewayProxy) Name() string {
	return p.spec.Name()
}

// Kind returns the kind of AIGatewayProxy.
func (p *AIGatewayProxy) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the AIGatewayProxy
func (p *AIGatewayProxy) Spec() filters.Spec {
	return p.spec
}

// Init initializes AIGatewayProxy.
func (p *AIGatewayProxy) Init() {
	p.reload()
}

// Inherit inherits previous generation of AIGatewayProxy.
func (p *AIGatewayProxy) Inherit(previousGeneration filters.Filter) {
	p.Init()
}

func (p *AIGatewayProxy) reload() {
}

// Handle AIGatewayProxys Context.
func (p *AIGatewayProxy) Handle(ctx *context.Context) string {
	handler, err := aigatewaycontroller.GetGlobalAIGatewayHandler()
	if err != nil {
		setErrResponse(ctx, err)
		return resultNoController
	}
	return handler.Handle(ctx, p.spec.ProviderName, p.spec.Middlewares)
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

// Status returns status.
func (p *AIGatewayProxy) Status() interface{} {
	return nil
}

// Close closes AIGatewayProxy.
func (p *AIGatewayProxy) Close() {
}
