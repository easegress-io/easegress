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

// Package corsadaptor implements a filter that adapts CORS stuff.
package corsadaptor

import (
	"net/http"
	"net/http/httptest"

	"github.com/rs/cors"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
)

const (
	// Kind is kind of CORSAdaptor.
	Kind = "CORSAdaptor"

	resultPreflighted = "preflighted"
	resultRejected    = "rejected"
)

var kind = &filters.Kind{
	Name:        Kind,
	Description: "CORSAdaptor adapts CORS stuff.",
	Results:     []string{resultPreflighted, resultRejected},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &CORSAdaptor{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// CORSAdaptor is filter for CORS request.
	CORSAdaptor struct {
		spec *Spec
		cors *cors.Cors
	}

	// Spec describes of CORSAdaptor.
	Spec struct {
		filters.BaseSpec `json:",inline"`

		AllowedOrigins   []string `json:"allowedOrigins,omitempty"`
		AllowedMethods   []string `json:"allowedMethods,omitempty" jsonschema:"uniqueItems=true,format=httpmethod-array"`
		AllowedHeaders   []string `json:"allowedHeaders,omitempty"`
		AllowCredentials bool     `json:"allowCredentials,omitempty"`
		ExposedHeaders   []string `json:"exposedHeaders,omitempty"`
		MaxAge           int      `json:"maxAge,omitempty"`
	}
)

// Name returns the name of the CORSAdaptor filter instance.
func (a *CORSAdaptor) Name() string {
	return a.spec.Name()
}

// Kind returns the kind of CORSAdaptor.
func (a *CORSAdaptor) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the CORSAdaptor
func (a *CORSAdaptor) Spec() filters.Spec {
	return a.spec
}

// Init initializes CORSAdaptor.
func (a *CORSAdaptor) Init() {
	a.reload()
}

// Inherit inherits previous generation of CORSAdaptor.
func (a *CORSAdaptor) Inherit(_ filters.Filter) {
	a.Init()
}

func (a *CORSAdaptor) reload() {
	a.cors = cors.New(cors.Options{
		AllowedOrigins:   a.spec.AllowedOrigins,
		AllowedMethods:   a.spec.AllowedMethods,
		AllowedHeaders:   a.spec.AllowedHeaders,
		AllowCredentials: a.spec.AllowCredentials,
		ExposedHeaders:   a.spec.ExposedHeaders,
		MaxAge:           a.spec.MaxAge,
	})
}

// Handle handles cross-origin requests.
func (a *CORSAdaptor) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)

	// not a CORS request
	if req.HTTPHeader().Get("Origin") == "" {
		return ""
	}

	isPreflight := req.HTTPHeader().Get("Access-Control-Request-Method") != ""
	isPreflight = isPreflight && (req.Method() == http.MethodOptions)

	rw := httptest.NewRecorder()
	a.cors.HandlerFunc(rw, req.Std())

	resp, _ := ctx.GetOutputResponse().(*httpprot.Response)
	if resp == nil {
		resp, _ = httpprot.NewResponse(rw.Result())
		ctx.SetOutputResponse(resp)
	} else {
		h := resp.HTTPHeader()
		for k, v := range rw.Header() {
			if k == "Vary" {
				h[k] = append(h[k], v...)
			} else {
				h[k] = v
			}
		}
	}

	if isPreflight {
		return resultPreflighted
	}

	// rejected by CORS
	if rw.Header().Get("Access-Control-Allow-Origin") == "" {
		return resultRejected
	}

	return ""
}

// Status return status.
func (a *CORSAdaptor) Status() interface{} {
	return nil
}

// Close closes CORSAdaptor.
func (a *CORSAdaptor) Close() {
}
