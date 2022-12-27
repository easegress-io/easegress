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

package redirector

import (
	"regexp"
	"strings"

	"github.com/megaease/easegress/pkg/context"
	"github.com/megaease/easegress/pkg/filters"
	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols/httpprot"
	"github.com/megaease/easegress/pkg/util/stringtool"
)

const (
	// Kind is the kind of Redirector.
	Kind = "Redirector"

	resultMismatch = "mismatch"
)

const (
	matchPartFull = "full"
	matchPartURI  = "uri"
	matchPartPath = "path"
)

var statusCodeMap = map[int]string{
	301: "Moved Permanently",
	302: "Found",
	303: "See Other",
	304: "Not Modified",
	307: "Temporary Redirect",
	308: "Permanent Redirect",
}

var kind = &filters.Kind{
	Name:        Kind,
	Description: "Redirector redirect HTTP requests.",
	Results:     []string{resultMismatch},
	DefaultSpec: func() filters.Spec {
		return &Spec{}
	},
	CreateInstance: func(spec filters.Spec) filters.Filter {
		return &Redirector{spec: spec.(*Spec)}
	},
}

func init() {
	filters.Register(kind)
}

type (
	// Redirector is filter to redirect HTTP requests.
	Redirector struct {
		spec *Spec
		re   *regexp.Regexp
	}

	// Spec describes the Redirector.
	Spec struct {
		filters.BaseSpec `json:",inline"`

		Match       string `json:"match" jsonschema:"required"`
		MatchPart   string `json:"matchPart" jsonschema:"required,enum=uri|full|path"` // default uri
		Replacement string `json:"replacement" jsonschema:"required"`
		StatusCode  int    `json:"statusCode" jsonschema:"required,enum=300|301|302|303|304|307|308"` // default 301
	}
)

// Name returns the name of the Redirector filter instance.
func (r *Redirector) Name() string {
	return r.spec.Name()
}

// Kind returns the kind of Redirector.
func (r *Redirector) Kind() *filters.Kind {
	return kind
}

// Spec returns the spec used by the Redirector
func (r *Redirector) Spec() filters.Spec {
	return r.spec
}

// Init initializes Redirector.
func (r *Redirector) Init() {
	r.reload()
}

// Inherit inherits previous generation of Redirector.
func (r *Redirector) Inherit(previousGeneration filters.Filter) {
	r.Init()
}

func (r *Redirector) reload() {
	if r.spec.StatusCode == 0 {
		r.spec.StatusCode = 301
	}
	if _, ok := statusCodeMap[r.spec.StatusCode]; !ok {
		logger.Warnf("invalid status code of Redirector, support 300, 301, 302, 303, 304, 307, 308, use 301 instead")
		r.spec.StatusCode = 301
	}

	if r.spec.MatchPart == "" {
		r.spec.MatchPart = matchPartURI
	}
	r.spec.MatchPart = strings.ToLower(r.spec.MatchPart)
	if !stringtool.StrInSlice(r.spec.MatchPart, []string{matchPartURI, matchPartFull, matchPartPath}) {
		logger.Warnf("invalid match part of Redirector, only uri, full and path are supported, use uri instead")
		r.spec.MatchPart = matchPartURI
	}
	r.re = regexp.MustCompile(r.spec.Match)
}

func (r *Redirector) getMatchInput(req *httpprot.Request) string {
	switch r.spec.MatchPart {
	case matchPartURI:
		return req.URL().RequestURI()
	case matchPartFull:
		return req.URL().String()
	default:
		// in spec we have already checked the value of MatchPart, only "uri", "full" and "path" are allowed
		return req.URL().Path
	}
}

func (r *Redirector) updateResponse(resp *httpprot.Response, matchResult string) {
	resp.SetStatusCode(r.spec.StatusCode)
	val, ok := statusCodeMap[r.spec.StatusCode]
	if !ok {
		resp.SetPayload([]byte(statusCodeMap[301]))
	} else {
		resp.SetPayload([]byte(val))
	}
	resp.Header().Add("Location", matchResult)
}

// Handle Redirector Context.
func (r *Redirector) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	matchInput := r.getMatchInput(req)
	matchResult := r.re.ReplaceAllString(matchInput, r.spec.Replacement)

	resp, _ := httpprot.NewResponse(nil)
	r.updateResponse(resp, matchResult)
	ctx.SetOutputResponse(resp)
	return ""
}

// Status returns status.
func (r *Redirector) Status() interface{} {
	return nil
}

// Close closes Redirector.
func (r *Redirector) Close() {
}
