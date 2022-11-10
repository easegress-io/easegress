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

package opafilter

import (
	"bytes"
	stdctx "context"
	"encoding/json"
	"errors"
	"io"
	"net/textproto"
	"strings"
	"time"

	"github.com/open-policy-agent/opa/rego"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/stringtool"
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

// RegoResult is the expected result from rego policy.
type RegoResult struct {
	Allow             bool              `json:"allow"`
	AdditionalHeaders map[string]string `json:"additional_headers,omitempty"`
	StatusCode        int               `json:"status_code,omitempty"`
}

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

type OPAFilter struct {
	spec                  *Spec
	includedHeadersParsed []string `yaml:"includedHeadersParsed"`
	regoQuery             *rego.PreparedEvalQuery
}

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

func (o *OPAFilter) Name() string {
	return o.spec.Name()
}

func (o *OPAFilter) Spec() filters.Spec {
	return o.spec
}

func (o *OPAFilter) Kind() *filters.Kind {
	return kind
}

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
		logger.Errorf("create PrepareForEval rego query error: %s", err)
	} else {
		o.regoQuery = &query
	}
}

func (o *OPAFilter) Inherit(previousGeneration filters.Filter) {
	o.Init()
	previousGeneration.Close()
}

func (o *OPAFilter) Handle(ctx *context.Context) (result string) {
	req := ctx.GetInputRequest().(*httpprot.Request)
	var rw *httpprot.Response
	if rw, _ = ctx.GetOutputResponse().(*httpprot.Response); rw == nil {
		rw, _ = httpprot.NewResponse(nil)
		ctx.SetOutputResponse(rw)
	}
	return o.evalRequest(req, rw)
}

func (o *OPAFilter) Status() interface{} {
	return nil
}

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
		buf, _ := io.ReadAll(r.Body)
		body = string(buf)
		// Put the body back in the request
		r.Body = io.NopCloser(bytes.NewBuffer(buf))
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
	bp, err := o.handleRegoResult(w, results[0].Bindings["result"])
	if err != nil {
		return o.opaError(w, err)
	}
	if *bp {
		return ""
	}
	return resultFiltered
}

func boolP(val bool) *bool {
	return &val
}

func (o *OPAFilter) handleRegoResult(w *httpprot.Response, result any) (*bool, error) {
	if allowed, ok := result.(bool); ok {
		if !allowed {
			w.SetStatusCode(o.spec.DefaultStatus)
			return boolP(false), nil
		}
		return boolP(true), nil
	}
	if _, ok := result.(map[string]any); !ok {
		return nil, errOpaInvalidResultType
	}
	marshaled, err := json.Marshal(result)
	if err != nil {
		return nil, errOpaInvalidResultType
	}
	regoResult := RegoResult{
		// By default, a non-allowed request with return a 403 response.
		StatusCode:        o.spec.DefaultStatus,
		AdditionalHeaders: make(map[string]string),
	}
	if err = json.Unmarshal(marshaled, &regoResult); err != nil {
		return nil, errOpaInvalidResultType
	}
	// Set the headers on the ongoing request (overriding as necessary)
	for key, value := range regoResult.AdditionalHeaders {
		w.Header().Set(key, value)
	}
	if !regoResult.Allow {
		w.SetStatusCode(regoResult.StatusCode)
	}
	return boolP(regoResult.Allow), nil
}

func (o *OPAFilter) opaError(resp *httpprot.Response, err error) string {
	resp.Header().Set(opaErrorHeaderKey, "true")
	resp.SetStatusCode(o.spec.DefaultStatus)
	resp.SetPayload(err.Error())
	return resultFiltered
}
