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

package routers

import (
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

type (
	// Kind contains the meta data and functions of a router kind.
	Kind struct {
		// Name is the name of the router kind.
		Name string

		// Description is the description of the router.
		Description string

		// CreateInstance creates a new router instance of the kind.
		CreateInstance func(rules Rules) Router
	}

	// Router is the interface for route search.
	Router interface {
		// Search performs a route search and populates the context with search-related information.
		Search(context *RouteContext)
	}

	// Route is the corresponding route interface for different routing policies.
	Route interface {
		// Rewrite for path rewriting.
		Rewrite(context *RouteContext)
		// GetBackend is used to get the backend corresponding to the route.
		GetBackend() string
		// GetClientMaxBodySize is used to get the clientMaxBodySize corresponding to the route.
		GetClientMaxBodySize() int64
	}

	// RouteParams are used to store the variables in the search path and their corresponding values.
	RouteParams struct {
		Keys, Values []string
	}

	// RouteContext is the context container in the route search
	RouteContext struct {
		Path    string
		Method  MethodType
		host    string
		queries url.Values

		Request *httpprot.Request

		RouteParams RouteParams
		captures    map[string]string

		Cacheable                                                 bool
		Route                                                     Route
		HeaderMismatch, MethodMismatch, QueryMismatch, IPMismatch bool
	}

	MethodType uint
)

const (
	MSTUB MethodType = 1 << iota
	MCONNECT
	MDELETE
	MGET
	MHEAD
	MOPTIONS
	MPATCH
	MPOST
	MPUT
	MTRACE
)

var (
	MALL = MCONNECT | MDELETE | MGET | MHEAD |
		MOPTIONS | MPATCH | MPOST | MPUT | MTRACE

	Methods = map[string]MethodType{
		http.MethodGet:     MGET,
		http.MethodHead:    MHEAD,
		http.MethodPost:    MPOST,
		http.MethodPut:     MPUT,
		http.MethodPatch:   MPATCH,
		http.MethodDelete:  MDELETE,
		http.MethodConnect: MCONNECT,
		http.MethodOptions: MOPTIONS,
		http.MethodTrace:   MTRACE,
	}

	kinds = map[string]*Kind{}
)

func Register(k *Kind) {
	name := k.Name
	if name == "" {
		panic(fmt.Errorf("%T: empty router name", k))
	}

	if k1 := kinds[k.Name]; k1 != nil {
		msgFmt := "%T and %T got same name: %s"
		panic(fmt.Errorf(msgFmt, k, k1, k.Name))
	}

	kinds[name] = k
}

// Create creates a router instance of kind.
func Create(kind string, rules Rules) Router {
	k := kinds[kind]
	if k == nil {
		return nil
	}
	return k.CreateInstance(rules)
}

func NewContext(req *httpprot.Request) *RouteContext {
	path := req.Path()

	context := &RouteContext{
		Path:    path,
		Request: req,
		Method:  Methods[req.Method()],
	}

	return context
}

func (ctx *RouteContext) GetCaptures() map[string]string {
	if ctx.captures != nil {
		return ctx.captures
	}

	ctx.captures = make(map[string]string)

	if len(ctx.RouteParams.Keys) == 0 {
		return ctx.captures
	}

	for i, key := range ctx.RouteParams.Keys {
		value := ctx.RouteParams.Values[i]
		ctx.captures[key] = value
	}

	return ctx.captures
}

func (ctx *RouteContext) GetHost() string {
	if ctx.host != "" {
		return ctx.host
	}

	host := ctx.Request.Host()
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	ctx.host = host
	return host
}

func (ctx *RouteContext) GetQueries() url.Values {
	if ctx.queries != nil {
		return ctx.queries
	}
	ctx.queries = ctx.Request.Std().URL.Query()
	return ctx.queries
}

func (ctx *RouteContext) GetHeader() http.Header {
	return ctx.Request.HTTPHeader()
}
