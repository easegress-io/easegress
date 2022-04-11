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
	"github.com/megaease/easegress/pkg/util/fasttime"
)

const (
	// InitialRequestID is the ID of the initial request.
	InitialRequestID = "initial"
	// InitialResponseID is the ID of the initial response.
	InitialResponseID = "initial"
)

// Handler is the common interface for all traffic handlers,
// which handle the traffic represented by ctx.
type Handler interface {
	Handle(ctx *Context) string
}

// MuxMapper gets the traffic handler by name.
type MuxMapper interface {
	GetHandler(name string) (Handler, bool)
}

// Context holds requests, responses and other data that need to be passed
// through the pipeline.
type Context struct {
	span     tracing.Span
	lazyTags []func() string

	targetRequestID string
	request         protocols.Request
	requests        map[string]protocols.Request

	targetResponseID string
	response         protocols.Response
	responses        map[string]protocols.Response

	kv          map[interface{}]interface{}
	finishFuncs []func()
}

// New creates a new Context.
func New(req protocols.Request, resp protocols.Response, tracer *tracing.Tracing, spanName string) *Context {
	now := fasttime.Now()
	span := tracing.NewSpanWithStart(tracer, spanName, now)

	ctx := &Context{
		span:    span,
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

// Span returns the span of this Context.
func (ctx *Context) Span() tracing.Span {
	return ctx.span
}

// AddTag add a tag to the Context.
func (ctx *Context) AddTag(tag string) {
	ctx.lazyTags = append(ctx.lazyTags, func() string { return tag })
}

// AddLazyTag add a lazy tag to the Context.
// questions about tags
// how to access statistics data from every requests?
// add new method when finish?
// add tag to every single request or add tag to Context
func (ctx *Context) AddLazyTag(lazyTagFunc func() string) {
	ctx.lazyTags = append(ctx.lazyTags, lazyTagFunc)
}

// Request returns the default request.
func (ctx *Context) Request() protocols.Request {
	return ctx.request
}

// UseRequest set the requests to use.
//
// dflt set the default request, the next call to Request returns
// this request, if this parameter is empty, InitialRequestID will be
// used.
//
// target is for request producers, if the request exists, the request
// producer update it in place, if the request does not exist, the
// request procuder creates a new request and save it as the target
// request.
//
// If target is an empty string, InitialRequestID will be used.
func (ctx *Context) UseRequest(dflt, target string) {
	if dflt == "" {
		dflt = InitialRequestID
	}

	if target == "" {
		target = InitialRequestID
	}

	if req := ctx.requests[dflt]; req == nil {
		panic(fmt.Errorf("request %s does not exist", dflt))
	} else {
		ctx.request = req
	}

	ctx.targetRequestID = target
}

// TargetRequestID returns the ID of the target request.
func (ctx *Context) TargetRequestID() string {
	return ctx.targetRequestID
}

// Requests returns all requests.
func (ctx *Context) Requests() map[string]protocols.Request {
	return ctx.requests
}

// GetRequest returns the request for id.
func (ctx *Context) GetRequest(id string) protocols.Request {
	return ctx.requests[id]
}

// SetRequest sets the request of id to req.
func (ctx *Context) SetRequest(id string, req protocols.Request) {
	prev := ctx.requests[id]
	if prev != nil && prev != req {
		prev.Close()
	}
	ctx.requests[id] = req
}

// DeleteRequest deletes the request of id.
func (ctx *Context) DeleteRequest(id string) {
	req := ctx.requests[id]
	if req != nil {
		req.Close()
		delete(ctx.requests, id)
	}
}

// UseResponse set the reponses to use.
//
// target is for response producers, if the response exists, the
// response producer update it in place, if the reponse does not exist,
// response producer creates a new response and save it as the target
// response.
//
// If target is an empty string, InitialResponseID will be used;
func (ctx *Context) UseResponse(target string) {
	if target == "" {
		target = InitialResponseID
	}

	ctx.targetResponseID = target
}

// TargetResponseID returns the ID of the target response.
func (ctx *Context) TargetResponseID() string {
	return ctx.targetResponseID
}

// Response returns the default response.
func (ctx *Context) Response() protocols.Response {
	return ctx.response
}

// Responses returns all responses.
func (ctx *Context) Responses() map[string]protocols.Response {
	return ctx.responses
}

// GetResponse returns the response for id.
func (ctx *Context) GetResponse(id string) protocols.Response {
	return ctx.responses[id]
}

// SetResponse sets the response of id to req.
func (ctx *Context) SetResponse(id string, resp protocols.Response) {
	prev := ctx.responses[id]
	if prev != nil && prev != resp {
		prev.Close()
	}
	ctx.responses[id] = resp
}

// DeleteResponse delete the response of id.
func (ctx *Context) DeleteResponse(id string) {
	resp := ctx.responses[id]
	if resp != nil {
		resp.Close()
		delete(ctx.responses, id)
	}
}

// SetKV sets the value of key to val.
func (ctx *Context) SetKV(key, val interface{}) {
	ctx.kv[key] = val
}

// GetKV returns the value of key.
func (ctx *Context) GetKV(key interface{}) interface{} {
	return ctx.kv[key]
}

// OnFinish registers a function to be called in Finish.
func (ctx *Context) OnFinish(fn func()) {
	ctx.finishFuncs = append(ctx.finishFuncs, fn)
}

// Finish calls all finish functions.
func (ctx *Context) Finish() {
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
