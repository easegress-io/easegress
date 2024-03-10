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

// Package redirector implements a filter to handle HTTP redirects.
package redirector

import (
	"errors"
	"regexp"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

const (
	// Kind is the kind of Redirector.
	Kind = "Redirector"

	resultRedirected = "redirected"
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
	Results:     []string{resultRedirected},
	DefaultSpec: func() filters.Spec {
		return &Spec{
			MatchPart:  matchPartURI,
			StatusCode: 301,
		}
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
		MatchPart   string `json:"matchPart,omitempty" jsonschema:"enum=uri,enum=path,enum=full"` // default uri
		Replacement string `json:"replacement" jsonschema:"required"`
		StatusCode  int    `json:"statusCode,omitempty"` // default 301
	}
)

// Validate validates the spec.
func (s *Spec) Validate() error {
	if _, ok := statusCodeMap[s.StatusCode]; !ok {
		return errors.New("invalid status code of Redirector, support 300, 301, 302, 303, 304, 307, 308")
	}
	s.MatchPart = strings.ToLower(s.MatchPart)
	if !stringtool.StrInSlice(s.MatchPart, []string{matchPartURI, matchPartFull, matchPartPath}) {
		return errors.New("invalid match part of Redirector, only uri, full and path are supported")
	}
	if s.Match == "" || s.Replacement == "" {
		return errors.New("match and replacement of Redirector can't be empty")
	}
	_, err := regexp.Compile(s.Match)
	if err != nil {
		return err
	}
	return nil
}

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

func (r *Redirector) updateResponse(resp *httpprot.Response, newLocation string) {
	resp.SetStatusCode(r.spec.StatusCode)
	resp.SetPayload([]byte(statusCodeMap[r.spec.StatusCode]))
	resp.Header().Add("Location", newLocation)
}

// Handle Redirector Context.
func (r *Redirector) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)
	matchInput := r.getMatchInput(req)
	newLocation := r.re.ReplaceAllString(matchInput, r.spec.Replacement)

	// if matchInput is not matched, newLocation will be the same as matchInput
	// consider we have multiple Redirector filters, we should not redirect the request
	// if the request is not matched by the current Redirector filter
	// so we return "" to indicate the request is not matched by the current Redirector filter
	// and the request will be handled by the next filter.
	if newLocation == matchInput {
		return ""
	}

	resp, _ := httpprot.NewResponse(nil)
	r.updateResponse(resp, newLocation)
	ctx.SetOutputResponse(resp)
	return resultRedirected
}

// Status returns status.
func (r *Redirector) Status() interface{} {
	return nil
}

// Close closes Redirector.
func (r *Redirector) Close() {
}
