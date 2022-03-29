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

package context

import (
	"fmt"

	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/tracing"
)

const (
	// DefaultRequestID is the ID of the default request.
	DefaultRequestID = "default"
	// DefaultResponseID is the ID of the default response.
	DefaultResponseID = "default"
)

type (
	// Handler is the common interface for all traffic handlers,
	// which handle the traffic represented by ctx.
	Handler interface {
		Handle(ctx Context) string
	}

	// MuxMapper gets the traffic handler by name.
	MuxMapper interface {
		GetHandler(name string) (Handler, bool)
	}

	// Context defines the common interface of the context.
	Context interface {
		Span() tracing.Span
		// StatMetric() *httpstat.Metric

		AddTag(tag string)
		AddLazyTag(LazyTagFunc)

		// questions
		// 1. setrequest, setresponse should only be used by pipeline
		//    filters like request builder and response builder should use method SetRequestByID and SetResponseByID
		// 2. how about gc when replace old request and response by new request and response
		//    only happen in function SetRequestByID and SetResponseByID, propose to add close option in function signature
		//    propose to change SetRequest(req Request) and SetResponse(resp Response) to UseRequest(id string) and UseResponse(id string)
		//    this can help avoid potential memory leak.
		UseRequest(id string)
		Request() protocols.Request
		Requests() map[string]protocols.Request
		GetRequest(id string) protocols.Request
		SetRequest(id string, req protocols.Request)
		DeleteRequest(id string)

		Response() protocols.Response
		Responses() map[string]protocols.Response
		GetResponse(id string) protocols.Response
		SetResponse(id string, resp protocols.Response)
		DeleteResponse(id string)

		// change put setkv getkv into context not request or response maybe better
		SetKV(key, val interface{})
		GetKV(key interface{}) interface{}

		OnFinish(fn func())
		Finish()
	}

	LazyTagFunc func() string

	// context manage requests and responses
	// there are no HTTPContext and MQTTContext, but only HTTPRequest, HTTPResponse,
	// MQTTRequest and MQTTResponse
	context struct {
		lazyTags []LazyTagFunc

		request   protocols.Request
		requests  map[string]protocols.Request
		response  protocols.Response
		responses map[string]protocols.Response

		kv          map[interface{}]interface{}
		finishFuncs []func()
	}
)

var _ Context = (*context)(nil)

// New creates a new Context.
func New(req protocols.Request, resp protocols.Response, tracer *tracing.Tracing, spanName string) Context {
	ctx := &context{
		request: req,
		requests: map[string]protocols.Request{
			DefaultRequestID: req,
		},
		response: resp,
		responses: map[string]protocols.Response{
			DefaultResponseID: resp,
		},
		kv: map[interface{}]interface{}{},
	}
	return ctx
}

func (ctx *context) AddTag(tag string) {
	ctx.lazyTags = append(ctx.lazyTags, func() string { return tag })
}

// questions about tags
// how to access statistics data from every requests?
// add new method when finish?
// add tag to every single request or add tag to context
func (ctx *context) AddLazyTag(lazyTagFunc LazyTagFunc) {
	ctx.lazyTags = append(ctx.lazyTags, lazyTagFunc)
}

func (ctx *context) Request() protocols.Request {
	return ctx.request
}

func (ctx *context) UseRequest(id string) {
	req := ctx.requests[id]
	if req == nil {
		panic(fmt.Errorf("request %s does not exist", id))
	}
	ctx.request = req
}

func (ctx *context) Requests() map[string]protocols.Request {
	return ctx.requests
}

func (ctx *context) GetRequest(id string) protocols.Request {
	return ctx.requests[id]
}

func (ctx *context) SetRequest(id string, req protocols.Request) {
	prev := ctx.requests[id]
	if prev != nil && prev != req {
		prev.Close()
	}
	ctx.requests[id] = req
}

func (ctx *context) DeleteRequest(id string) {
	req := ctx.requests[id]
	if req != nil {
		req.Close()
		delete(ctx.requests, id)
	}
}

func (ctx *context) Response() protocols.Response {
	return ctx.response
}

func (ctx *context) Responses() map[string]protocols.Response {
	return ctx.responses
}

func (ctx *context) GetResponse(id string) protocols.Response {
	return ctx.responses[id]
}

func (ctx *context) SetResponse(id string, resp protocols.Response) {
	prev := ctx.responses[id]
	if prev != nil && prev != resp {
		prev.Close()
	}
	ctx.responses[id] = resp
}

func (ctx *context) DeleteResponse(id string) {
	resp := ctx.responses[id]
	if resp != nil {
		resp.Close()
		delete(ctx.responses, id)
	}
}

func (ctx *context) SetKV(key, val interface{}) {
	ctx.kv[key] = val
}

func (ctx *context) GetKV(key interface{}) interface{} {
	return ctx.kv[key]
}

func (ctx *context) OnFinish(fn func()) {
	ctx.finishFuncs = append(ctx.finishFuncs, fn)
}

func (ctx *context) Finish() {
	// TODO: add tags here
	for _, req := range ctx.requests {
		req.Close()
	}
	for _, resp := range ctx.responses {
		resp.Close()
	}
	for _, fn := range ctx.finishFuncs {
		fn()
	}
}

func (ctx *context) Span() tracing.Span {
	// TODO: add span
	return nil
}
