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

// Package context provides the context for traffic handlers.
package context

import (
	"bytes"
	"runtime/debug"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/protocols"
	"github.com/megaease/easegress/v2/pkg/tracing"
)

// DefaultNamespace is the name of the default namespace.
const DefaultNamespace = "DEFAULT"

// Handler is the common interface for all traffic handlers,
// which handle the traffic represented by ctx.
type Handler interface {
	Handle(ctx *Context) string
}

// MuxMapper gets the traffic handler by name.
type MuxMapper interface {
	GetHandler(name string) (Handler, bool)
}

type requestRef struct {
	req     protocols.Request
	counter int
}

func (rr *requestRef) release() {
	rr.counter--
	if rr.counter == 0 {
		rr.req.Close()
	}
}

type responseRef struct {
	resp    protocols.Response
	counter int
}

func (rr *responseRef) release() {
	rr.counter--
	if rr.counter == 0 {
		rr.resp.Close()
	}
}

// Context holds requests, responses and other data that need to be passed
// through the pipeline.
type Context struct {
	span     *tracing.Span
	lazyTags []func() string

	activeNs string

	route     protocols.Route
	requests  map[string]*requestRef
	responses map[string]*responseRef

	data        map[string]interface{}
	finishFuncs []func()
}

// New creates a new Context.
func New(span *tracing.Span) *Context {
	ctx := &Context{
		span:      span,
		activeNs:  DefaultNamespace,
		requests:  map[string]*requestRef{},
		responses: map[string]*responseRef{},
		data:      map[string]interface{}{},
	}
	return ctx
}

// SetRoute sets the route.
func (ctx *Context) SetRoute(route protocols.Route) {
	ctx.route = route
}

// GetRoute returns the route with the existing flag.
func (ctx *Context) GetRoute() (protocols.Route, bool) {
	if ctx.route == nil {
		return nil, false
	}

	return ctx.route, true
}

// Span returns the span of this Context.
func (ctx *Context) Span() *tracing.Span {
	return ctx.span
}

// AddTag add a tag to the Context.
func (ctx *Context) AddTag(tag string) {
	ctx.lazyTags = append(ctx.lazyTags, func() string { return tag })
}

// LazyAddTag add a tag to the Context in a lazy fashion.
func (ctx *Context) LazyAddTag(lazyTagFunc func() string) {
	ctx.lazyTags = append(ctx.lazyTags, lazyTagFunc)
}

// UseNamespace sets the active namespace.
func (ctx *Context) UseNamespace(ns string) {
	if ns == "" {
		ctx.activeNs = DefaultNamespace
	} else {
		ctx.activeNs = ns
	}
}

// Namespace returns the active namespace.
func (ctx *Context) Namespace() string {
	return ctx.activeNs
}

// CopyRequest copies the request of namespace ns to the active namespace.
// The copied request is a new reference of the original request, that's
// they both point to the same underlying protocols.Request.
func (ctx *Context) CopyRequest(ns string) {
	if ns == "" {
		ns = DefaultNamespace
	}
	if ns == ctx.activeNs {
		return
	}
	rr := ctx.requests[ns]
	if rr == nil {
		return
	}
	prev := ctx.requests[ctx.activeNs]
	if prev != nil {
		if prev == rr {
			return
		}
		prev.release()
	}
	rr.counter++
	ctx.requests[ctx.activeNs] = rr
}

// Requests returns all requests, the caller should NOT modify the
// return value.
func (ctx *Context) Requests() map[string]protocols.Request {
	m := make(map[string]protocols.Request, len(ctx.requests))
	for k, v := range ctx.requests {
		m[k] = v.req
	}
	return m
}

// GetOutputRequest returns the request of the output namespace.
//
// Currently, the output namespace is the same as the input namespace,
// but this may change in the future, code calling this function should
// be ready for the change and not to call GetInputRequest when
// GetOutputRequest is desired, or vice versa.
func (ctx *Context) GetOutputRequest() protocols.Request {
	ref := ctx.requests[ctx.activeNs]
	if ref != nil {
		return ref.req
	}
	return nil
}

// SetOutputRequest sets the request of the output namespace to req.
//
// Currently, the output namespace is the same as the input namespace,
// but this may change in the future, code calling this function should
// be ready for the change and not to call SetInputRequest when
// SetOutputRequest is desired, or vice versa.
func (ctx *Context) SetOutputRequest(req protocols.Request) {
	ctx.SetRequest(ctx.activeNs, req)
}

// GetRequest set the request of namespace ns to req.
func (ctx *Context) GetRequest(ns string) protocols.Request {
	ref := ctx.requests[ns]
	if ref != nil {
		return ref.req
	}
	return nil
}

// SetRequest set the request of namespace ns to req.
func (ctx *Context) SetRequest(ns string, req protocols.Request) {
	prev := ctx.requests[ns]
	if prev != nil {
		if prev.req == req {
			return
		}
		prev.release()
	}
	ctx.requests[ns] = &requestRef{req, 1}
}

// GetInputRequest returns the request of the input namespace.
//
// Currently, the output namespace is the same as the input namespace,
// but this may change in the future, code calling this function should
// be ready for the change and not to call GetOutputRequest when
// GetInputRequest is desired, or vice versa.
func (ctx *Context) GetInputRequest() protocols.Request {
	ref := ctx.requests[ctx.activeNs]
	if ref != nil {
		return ref.req
	}
	return nil
}

// SetInputRequest sets the request of the input namespace to req.
//
// Currently, the output namespace is the same as the input namespace,
// but this may change in the future, code calling this function should
// be ready for the change and not to call SetOutputRequest when
// SetInputRequest is desired, or vice versa.
func (ctx *Context) SetInputRequest(req protocols.Request) {
	ctx.SetRequest(ctx.activeNs, req)
}

// CopyResponse copies the response of namespace ns to the active namespace.
// The copied response is a new reference of the original response, that's
// they both point to the same underlying protocols.Response.
func (ctx *Context) CopyResponse(ns string) {
	if ns == "" {
		ns = DefaultNamespace
	}
	if ns == ctx.activeNs {
		return
	}
	rr := ctx.responses[ns]
	if rr == nil {
		return
	}
	prev := ctx.responses[ctx.activeNs]
	if prev != nil {
		if prev == rr {
			return
		}
		prev.release()
	}
	rr.counter++
	ctx.responses[ctx.activeNs] = rr
}

// Responses returns all responses.
func (ctx *Context) Responses() map[string]protocols.Response {
	m := make(map[string]protocols.Response, len(ctx.responses))
	for k, v := range ctx.responses {
		m[k] = v.resp
	}
	return m
}

// GetOutputResponse returns the response of the output namespace.
//
// Currently, the output namespace is the same as the input namespace,
// but this may change in the future, code calling this function should
// be ready for the change and not to call GetInputResponse when
// GetOutputResponse is desired, or vice versa.
func (ctx *Context) GetOutputResponse() protocols.Response {
	ref := ctx.responses[ctx.activeNs]
	if ref != nil {
		return ref.resp
	}
	return nil
}

// SetOutputResponse sets the response of the output namespace to resp.
//
// Currently, the output namespace is the same as the input namespace,
// but this may change in the future, code calling this function should
// be ready for the change and not to call SetInputResponse when
// SetOutputResponse is desired, or vice versa.
func (ctx *Context) SetOutputResponse(resp protocols.Response) {
	ctx.SetResponse(ctx.activeNs, resp)
}

// GetResponse returns the response of namespace ns.
func (ctx *Context) GetResponse(ns string) protocols.Response {
	ref := ctx.responses[ns]
	if ref != nil {
		return ref.resp
	}
	return nil
}

// SetResponse set the response of namespace ns to resp.
func (ctx *Context) SetResponse(ns string, resp protocols.Response) {
	prev := ctx.responses[ns]
	if prev != nil {
		if prev.resp == resp {
			return
		}
		prev.release()
	}
	ctx.responses[ns] = &responseRef{resp, 1}
}

// GetInputResponse returns the response of the input namespace.
//
// Currently, the output namespace is the same as the input namespace,
// but this may change in the future, code calling this function should
// be ready for the change and not to call GetOutputResponse when
// GetInputResponse is desired, or vice versa.
func (ctx *Context) GetInputResponse() protocols.Response {
	return ctx.GetResponse(ctx.activeNs)
}

// SetInputResponse sets the response of the input namespace to resp.
//
// Currently, the output namespace is the same as the input namespace,
// but this may change in the future, code calling this function should
// be ready for the change and not to call SetOutputResponse when
// SetInputResponse is desired, or vice versa.
func (ctx *Context) SetInputResponse(resp protocols.Response) {
	ctx.SetResponse(ctx.activeNs, resp)
}

// Data returns all data that stored in the context.
func (ctx *Context) Data() map[string]interface{} {
	return ctx.data
}

// SetData sets the data of key to val.
func (ctx *Context) SetData(key string, val interface{}) {
	ctx.data[key] = val
}

// GetData returns the data of key.
func (ctx *Context) GetData(key string) interface{} {
	return ctx.data[key]
}

// Tags joins all tags into a string and returns it.
func (ctx *Context) Tags() string {
	buf := bytes.Buffer{}

	for i, fn := range ctx.lazyTags {
		if i > 0 {
			buf.WriteString(" | ")
		}
		buf.WriteString(fn())
	}

	return buf.String()
}

// OnFinish registers a function to be called in Finish.
func (ctx *Context) OnFinish(fn func()) {
	ctx.finishFuncs = append(ctx.finishFuncs, fn)
}

// Finish calls all finish functions.
func (ctx *Context) Finish() {
	const msgFmt = "failed to execute finish action: %v, stack trace: \n%s\n"

	for _, rr := range ctx.requests {
		rr.release()
	}

	for _, rr := range ctx.responses {
		rr.release()
	}

	for _, fn := range ctx.finishFuncs {
		func() {
			defer func() {
				if err := recover(); err != nil {
					logger.Errorf(msgFmt, err, debug.Stack())
				}
			}()

			fn()
		}()
	}
}
