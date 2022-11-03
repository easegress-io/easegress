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

	"github.com/megaease/easegress/pkg/protocols/httpprot"
)

type (
	Kind struct {
		// Name is the name of the router kind.
		Name string

		// Description is the description of the router.
		Description string

		// CreateInstance creates a new router instance of the kind.
		CreateInstance func(rules Rules) Router
	}

	Router interface {
		Search(context *RouteContext)
	}

	Route interface {
		AllowIPChain(ip string) bool
		Rewrite(context *RouteContext)
		GetBackend() string
		GetClientMaxBodySize() int64
	}

	RouteParams struct {
		Keys, Values []string
	}

	RouteContext struct {
		Path    string
		Request *httpprot.Request

		RouteParams RouteParams
		captures    map[string]string

		Cacheable                                                   bool
		Route                                                       Route
		HeaderMismatch, MethodMismatch, QueryMismatch, IPNotAllowed bool
	}
)

var kinds = map[string]*Kind{}

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

	for i := 0; i < len(ctx.RouteParams.Keys); i++ {
		key := ctx.RouteParams.Keys[i]
		value := ctx.RouteParams.Values[i]
		ctx.captures[key] = value
	}

	return ctx.captures
}
