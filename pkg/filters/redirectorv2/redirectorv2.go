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
package redirectorv2

import (
	"errors"
	"net/url"
	"strings"

	"github.com/megaease/easegress/v2/pkg/context"
	"github.com/megaease/easegress/v2/pkg/filters"
	httprouters "github.com/megaease/easegress/v2/pkg/object/httpserver/routers"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"

	gwapis "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// Kind is the kind of Redirector.
	Kind = "RedirectorV2"

	resultRedirected = "redirected"
)

const (
	defaultStatusCode = 302
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
	}

	// Spec describes the Redirector.
	Spec struct {
		filters.BaseSpec `json:",inline"`

		gwapis.HTTPRequestRedirectFilter `json:",inline"`
	}
)

// Validate validates the spec.
func (s *Spec) Validate() error {
	if s.Scheme != nil {
		if *s.Scheme != "http" && *s.Scheme != "https" {
			return errors.New("invalid scheme of Redirector, only support http and https")
		}
	}

	if s.Port != nil {
		if *s.Port < 1 || *s.Port > 65535 {
			return errors.New("invalid port of Redirector, only support 1-65535")
		}
	}

	if s.StatusCode != nil {
		if _, ok := statusCodeMap[*s.StatusCode]; !ok {
			return errors.New("invalid status code of Redirector, support 300, 301, 302, 303, 304, 307, 308")
		}
	}

	if s.Path != nil {
		switch s.Path.Type {
		case gwapis.FullPathHTTPPathModifier:
			if s.Path.ReplaceFullPath == nil {
				return errors.New("invalid path of Redirector, replaceFullPath can't be empty")
			}
		case gwapis.PrefixMatchHTTPPathModifier:
			if s.Path.ReplacePrefixMatch == nil {
				return errors.New("invalid path of Redirector, replacePrefixMatch can't be empty")
			}
		default:
			return errors.New("invalid path type of Redirector, only support ReplaceFullPath and ReplacePrefixMatch")
		}
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
}

// Handle Redirector Context.
func (r *Redirector) Handle(ctx *context.Context) string {
	req := ctx.GetInputRequest().(*httpprot.Request)

	redirectURL := deepCopyURL(*req.URL())

	if r.spec.Scheme != nil {
		redirectURL.Scheme = *r.spec.Scheme
	}

	if r.spec.Hostname != nil {
		redirectURL.Host = string(*r.spec.Hostname)
	}

	// NOTE:
	// We choose not to support port, because its specification is complicated and confusing.
	// If users want to use a specific port, they can specify it in the hostname field.

	// if r.spec.Port != nil {
	// 	redirectLocation.Host = fmt.Sprintf("%s:%d", redirectLocation.Host, *r.spec.Port)
	// }

	if r.spec.Path != nil {
		switch r.spec.Path.Type {
		case gwapis.FullPathHTTPPathModifier:
			redirectURL.Path = string(*r.spec.Path.ReplaceFullPath)
		case gwapis.PrefixMatchHTTPPathModifier:
			route, existed := ctx.GetRoute()
			if !existed {
				ctx.AddTag("route not found")
				break
			}

			if route.Protocol() != "http" {
				ctx.AddTag("route is not an http route")
				break
			}

			prefix := route.(httprouters.Route).GetPathPrefix()
			if prefix == "" {
				ctx.AddTag("route has no path prefix")
				break
			}

			redirectURL.Path = r.subPrefix(redirectURL.Path,
				prefix, string(*r.spec.Path.ReplacePrefixMatch))
		}
	}

	if req.URL().String() == redirectURL.String() {
		return ""
	}

	resp, _ := httpprot.NewResponse(nil)

	statusCode := int(defaultStatusCode)
	if r.spec.StatusCode != nil {
		statusCode = int(*r.spec.StatusCode)
	}
	resp.SetStatusCode(statusCode)

	resp.SetPayload([]byte(statusCodeMap[statusCode]))
	resp.Header().Add("Location", redirectURL.String())

	ctx.SetOutputResponse(resp)

	return resultRedirected
}

func deepCopyURL(u url.URL) *url.URL {
	copied := u

	if u.User != nil {
		copiedUser := *u.User
		copied.User = &copiedUser
	}

	return &copied
}

// subPrefix replaces the prefix in the path with replacePrefixMatch if the prefix exists
func (r *Redirector) subPrefix(path, prefix, replacePrefixMatch string) string {
	if strings.HasPrefix(path, prefix) {
		return strings.Replace(path, prefix, replacePrefixMatch, 1)
	}
	return path
}

// Status returns status.
func (r *Redirector) Status() interface{} {
	return nil
}

// Close closes Redirector.
func (r *Redirector) Close() {
}
