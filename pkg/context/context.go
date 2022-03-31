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
	// InitialRequestID is the ID of the initial request.
	InitialRequestID = "initial"
	// DefaultResponseID is the ID of the initial response.
	InitialResponseID = "initial"
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
		AddLazyTag(func() string)

		// UseRequest set the requests to use.
		//
		// dflt set the default request, the next call to Request returns
		// this request, if this parameter is empty, InitialRequestID will be
		// used.
		//
		// base & target are for request producers, the request producer
		// creates a new request based on the base request, update its content
		// and then save it as the target request.
		//
		// If parameter base is empty, a new request is created with default
		// content; if paramter target is empty, InitialRequestID will be used;
		// if base equals to target, the request will be updated in place.
		UseRequest(dflt, base, target string)
		BaseRequestID() string
		TargetRequestID() string

		Request() protocols.Request
		Requests() map[string]protocols.Request
		GetRequest(id string) protocols.Request
		SetRequest(id string, req protocols.Request)
		DeleteRequest(id string)

		// UseResponse set the reponses to use.
		//
		// base & target are for response producers, the response producer
		// creates a new response based on the base response, update its content
		// and then save it as the target response.
		//
		// If parameter base is empty, a new response is created with default
		// content; if paramter target is empty, InitialResponseID will be used;
		// if base equals to target, the response will be updated in place.
		UseResponse(base, target string)
		BaseResponseID() string
		TargetResponseID() string

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

	// context manage requests and responses
	// there are no HTTPContext and MQTTContext, but only HTTPRequest, HTTPResponse,
	// MQTTRequest and MQTTResponse
	context struct {
		lazyTags []func() string

		baseRequestID   string
		targetRequestID string
		request         protocols.Request
		requests        map[string]protocols.Request

		baseResponseID   string
		targetResponseID string
		response         protocols.Response
		responses        map[string]protocols.Response

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
			InitialRequestID: req,
		},
		response: resp,
		responses: map[string]protocols.Response{
			InitialResponseID: resp,
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
func (ctx *context) AddLazyTag(lazyTagFunc func() string) {
	ctx.lazyTags = append(ctx.lazyTags, lazyTagFunc)
}

func (ctx *context) Request() protocols.Request {
	return ctx.request
}

func (ctx *context) UseRequest(dflt, base, target string) {
	if dflt == "" {
		dflt = InitialRequestID
	}

	if base != "" && ctx.requests[base] == nil {
		panic(fmt.Errorf("request %s does not exist", base))
	}

	if target == "" {
		target = InitialRequestID
	}

	if req := ctx.requests[dflt]; req == nil {
		panic(fmt.Errorf("request %s does not exist", dflt))
	} else {
		ctx.request = req
	}

	ctx.baseRequestID = base
	ctx.targetRequestID = target
}

func (ctx *context) BaseRequestID() string {
	return ctx.baseRequestID
}

func (ctx *context) TargetRequestID() string {
	return ctx.targetRequestID
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

func (ctx *context) UseResponse(base, target string) {
	if base != "" && ctx.responses[base] == nil {
		panic(fmt.Errorf("response %s does not exist", base))
	}

	if target == "" {
		target = InitialResponseID
	}

	ctx.baseRequestID = base
	ctx.targetResponseID = target
}

func (ctx *context) BaseResponseID() string {
	return ctx.baseResponseID
}

func (ctx *context) TargetResponseID() string {
	return ctx.targetResponseID
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
