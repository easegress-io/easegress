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

// Package opafilter implements OpenPolicyAgent function.
package opafilter

import (
	stdctx "context"
	"errors"
	"fmt"
	"net/textproto"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/rego"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

// https://www.openpolicyagent.org/docs/v0.35.0/
const (
	kindName          = "OPAFilter"
	resultFiltered    = "opaDenied"
	opaErrorHeaderKey = "x-eg-opa-error"
)

var (
	errOpaNoResult          = errors.New("received no results from rego policy. Are you setting data.http.allow")
	errOpaInvalidResultType = errors.New("got an invalid type from repo policy. Only a boolean or map is valid")
)

var kind = &filters.Kind{
	Name:        kindName,
	Description: "OPAFilter implement OpenPolicyAgent function",
	Results:     []string{resultFiltered},
	DefaultSpec: func() filters.Spec {
		return &Spec{
			DefaultStatus: 403,
		}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &OPAFilter{spec: spec.(*Spec)}
	},
}

// OPAFilter is the filter for OpenPolicyAgent.
type OPAFilter struct {
	spec                  *Spec
	includedHeadersParsed []string `yaml:"includedHeadersParsed"`
	regoQuery             *rego.PreparedEvalQuery
}

// Spec is the spec of the OPAFilter.
type Spec struct {
	filters.BaseSpec `yaml:",inline"`
	DefaultStatus    int    `yaml:"defaultStatus"`
	IncludedHeaders  string `yaml:"includedHeaders"`
	ReadBody         bool   `yaml:"readBody"`
	Policy           string `yaml:"policy"`
}

func init() {
	filters.Register(kind)
}

// Name returns the name of the OPAFilter filter instance.
func (o *OPAFilter) Name() string {
	return o.spec.Name()
}

// Spec returns the spec of the OPAFilter filter instance.
func (o *OPAFilter) Spec() filters.Spec {
	return o.spec
}

// Kind returns the kind of the OPAFilter filter instance.
func (o *OPAFilter) Kind() *filters.Kind {
	return kind
}

// Init initialize the filter instance.
func (o *OPAFilter) Init() {
	o.includedHeadersParsed = strings.Split(o.spec.IncludedHeaders, ",")
	if o.spec.DefaultStatus == 0 {
		o.spec.DefaultStatus = 403
	}
	n := 0
	for i := range o.includedHeadersParsed {
		scrubbed := strings.ReplaceAll(o.includedHeadersParsed[i], " ", "")
		if scrubbed != "" {
			o.includedHeadersParsed[n] = textproto.CanonicalMIMEHeaderKey(scrubbed)
			n++
		}
	}
	o.includedHeadersParsed = o.includedHeadersParsed[:n]
	ctx, cancelFunc := stdctx.WithTimeout(stdctx.Background(), 5*time.Second)
	query, err := rego.New(
		rego.Query("result = data.http.allow"),
		rego.Module("inline.rego.policy", o.spec.Policy),
	).PrepareForEval(ctx)
	defer cancelFunc()
	if err != nil {
		panic(fmt.Sprintf("cannot create PrepareForEval rego query: %s", err))
	} else {
		o.regoQuery = &query
	}
}

// Inherit inherits previous generation of filter instance.
func (o *OPAFilter) Inherit(previousGeneration filters.Filter) {
	o.Init()
	previousGeneration.Close()
}

// Handle handles the request.
func (o *OPAFilter) Handle(ctx *context.Context) (result string) {
	req := ctx.GetInputRequest().(*httpprot.Request)
	var rw *httpprot.Response
	if rw, _ = ctx.GetOutputResponse().(*httpprot.Response); rw == nil {
		rw, _ = httpprot.NewResponse(nil)
		ctx.SetOutputResponse(rw)
	}
	return o.evalRequest(req, rw)
}

// Status returns the status of the filter instance.
func (o *OPAFilter) Status() interface{} {
	return nil
}

// Close closes the filter instance.
func (o *OPAFilter) Close() {
}

func (o *OPAFilter) evalRequest(r *httpprot.Request, w *httpprot.Response) string {
	headers := map[string]string{}

	for key, value := range r.HTTPHeader() {
		if len(value) > 0 && stringtool.StrInSlice(key, o.includedHeadersParsed) {
			headers[key] = strings.Join(value, ", ")
		}
	}

	var body string
	if o.spec.ReadBody {
		if !r.IsStream() {
			body = string(r.RawPayload())
		} else {
			logger.Errorf("body is stream, cannot set as input request body")
		}
	}
	pathParts := strings.Split(strings.Trim(r.Path(), "/"), "/")
	input := map[string]any{
		"request": map[string]any{
			"method":     r.Std().Method,
			"path":       r.Std().URL.Path,
			"path_parts": pathParts,
			"raw_query":  r.Std().URL.RawQuery,
			"query":      map[string][]string(r.Std().URL.Query()),
			"headers":    headers,
			"scheme":     r.Scheme(),
			"realIP":     r.RealIP(),
			"body":       body,
		},
	}
	results, err := o.regoQuery.Eval(r.Context(), rego.EvalInput(input))
	if err != nil {
		return o.opaError(w, err)
	}
	if len(results) == 0 {
		return o.opaError(w, errOpaNoResult)
	}
	result := results[0].Bindings["result"]
	var allow, ok bool
	if allow, ok = result.(bool); !ok {
		return o.opaError(w, errOpaInvalidResultType)
	}
	if !allow {
		w.SetStatusCode(o.spec.DefaultStatus)
		return resultFiltered
	}
	return ""
}

func (o *OPAFilter) opaError(resp *httpprot.Response, err error) string {
	resp.Header().Set(opaErrorHeaderKey, "true")
	resp.SetStatusCode(o.spec.DefaultStatus)
	resp.SetPayload(err.Error())
	return resultFiltered
}
