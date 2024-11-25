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

// Package routers provides the router interface and the implementation of
// different routing policies.
package routers

import (
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
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
		protocols.Route

		// Rewrite for path rewriting.
		Rewrite(context *RouteContext)
		// GetBackend is used to get the backend corresponding to the route.
		GetBackend() string
		// GetClientMaxBodySize is used to get the clientMaxBodySize corresponding to the route.
		GetClientMaxBodySize() int64

		// NOTE: Currently we only support path information in readonly.
		// Without further requirements, we choose not to expose too much information.

		// GetExactPath is used to get the exact path corresponding to the route.
		GetExactPath() string
		// GetPathPrefix is used to get the path prefix corresponding to the route.
		GetPathPrefix() string
		// GetPathRegexp is used to get the path regexp corresponding to the route.
		GetPathRegexp() string
	}

	// Params are used to store the variables in the search path and their corresponding values.
	Params struct {
		Keys, Values []string
	}

	// RouteContext is the context container in the route search
	RouteContext struct {
		// Request is httpprot.Request.
		Request *httpprot.Request
		// Path represents the path of the request.
		Path string
		// Method represents the MethodType corresponding to the http method.
		Method  MethodType
		host    string
		queries url.Values

		// Params are used to store the variables in the search path and their corresponding values.
		Params   Params
		captures map[string]string

		// Cacheable means whether the route can be cached or not.
		Cacheable bool
		// Route represents the results of this search
		Route                                                     Route
		HeaderMismatch, MethodMismatch, QueryMismatch, IPMismatch bool
	}

	// MethodType represents the bit-operated representation of the http method.
	MethodType uint
)

const (
	mSTUB MethodType = 1 << iota
	mCONNECT
	mDELETE
	mGET
	mHEAD
	mOPTIONS
	mPATCH
	mPOST
	mPUT
	mTRACE
)

var (
	// MALL represents the methodType that can match all methods.
	MALL = mCONNECT | mDELETE | mGET | mHEAD |
		mOPTIONS | mPATCH | mPOST | mPUT | mTRACE

	// Methods represents the mapping of http method and methodType.
	Methods = map[string]MethodType{
		http.MethodGet:     mGET,
		http.MethodHead:    mHEAD,
		http.MethodPost:    mPOST,
		http.MethodPut:     mPUT,
		http.MethodPatch:   mPATCH,
		http.MethodDelete:  mDELETE,
		http.MethodConnect: mCONNECT,
		http.MethodOptions: mOPTIONS,
		http.MethodTrace:   mTRACE,
	}

	kinds = map[string]*Kind{}
)

// Register registers a router kind.
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

// NewContext creates a context instance.
func NewContext(req *httpprot.Request) *RouteContext {
	path := req.Path()

	context := &RouteContext{
		Path:    path,
		Request: req,
		Method:  Methods[req.Method()],
	}

	return context
}

// GetCaptures is used to get and cache path parameter mappings.
func (ctx *RouteContext) GetCaptures() map[string]string {
	if ctx.captures != nil {
		return ctx.captures
	}

	ctx.captures = make(map[string]string)

	if len(ctx.Params.Keys) == 0 {
		return ctx.captures
	}

	for i, key := range ctx.Params.Keys {
		value := ctx.Params.Values[i]
		ctx.captures[key] = value
	}

	return ctx.captures
}

// GetHost is used to get and cache host.
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

// GetQueries is used to get and cache query params.
func (ctx *RouteContext) GetQueries() url.Values {
	if ctx.queries != nil {
		return ctx.queries
	}
	ctx.queries = ctx.Request.Std().URL.Query()
	return ctx.queries
}

// GetHeader is used to get request http header.
func (ctx *RouteContext) GetHeader() http.Header {
	return ctx.Request.HTTPHeader()
}
