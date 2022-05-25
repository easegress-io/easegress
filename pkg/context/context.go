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
	"bytes"
	"runtime/debug"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/protocols"
	"github.com/megaease/easegress/pkg/tracing"
)

// DefaultNamespace is the name of the default namespace.
const DefaultNamespace = "PIPELINE"

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

	inputNs  string
	outputNs string

	requests  map[string]protocols.Request
	responses map[string]protocols.Response

	kv          map[interface{}]interface{}
	finishFuncs []func()
}

// New creates a new Context.
func New(span tracing.Span) *Context {
	ctx := &Context{
		span:      span,
		inputNs:   DefaultNamespace,
		outputNs:  DefaultNamespace,
		requests:  map[string]protocols.Request{},
		responses: map[string]protocols.Response{},
		kv:        map[interface{}]interface{}{},
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

// LazyAddTag add a tag to the Context in a lazy fashion.
func (ctx *Context) LazyAddTag(lazyTagFunc func() string) {
	ctx.lazyTags = append(ctx.lazyTags, lazyTagFunc)
}

// UseNamespace sets the input and output namespace to input and output.
func (ctx *Context) UseNamespace(input, output string) {
	if input == "" {
		input = DefaultNamespace
	}

	if output == "" {
		output = input
	}

	ctx.inputNs = input
	ctx.outputNs = output
}

// Requests returns all requests, the caller should NOT modify the
// return value.
func (ctx *Context) Requests() map[string]protocols.Request {
	return ctx.requests
}

// GetOutputRequest returns the request of the output namespace.
func (ctx *Context) GetOutputRequest() protocols.Request {
	return ctx.requests[ctx.outputNs]
}

// SetOutputRequest sets the request of the output namespace to req.
func (ctx *Context) SetOutputRequest(req protocols.Request) {
	ctx.SetRequest(ctx.outputNs, req)
}

// GetRequest set the request of namespace ns to req.
func (ctx *Context) GetRequest(ns string) protocols.Request {
	return ctx.requests[ns]
}

// SetRequest set the request of namespace ns to req.
func (ctx *Context) SetRequest(ns string, req protocols.Request) {
	prev := ctx.requests[ns]
	if prev != nil && prev != req {
		prev.Close()
	}
	ctx.requests[ns] = req
}

// GetInputRequest returns the request of the input namespace.
func (ctx *Context) GetInputRequest() protocols.Request {
	return ctx.requests[ctx.inputNs]
}

// SetInputRequest sets the request of the input namespace to req.
func (ctx *Context) SetInputRequest(req protocols.Request) {
	ctx.SetRequest(ctx.inputNs, req)
}

// Responses returns all responses.
func (ctx *Context) Responses() map[string]protocols.Response {
	return ctx.responses
}

// GetOutputResponse returns the response of the output namespace.
func (ctx *Context) GetOutputResponse() protocols.Response {
	return ctx.responses[ctx.outputNs]
}

// SetOutputResponse sets the response of the output namespace to resp.
func (ctx *Context) SetOutputResponse(resp protocols.Response) {
	ctx.SetResponse(ctx.outputNs, resp)
}

// GetResponse returns the response of namespace ns.
func (ctx *Context) GetResponse(ns string) protocols.Response {
	return ctx.responses[ns]
}

// SetResponse set the response of namespace ns to resp.
func (ctx *Context) SetResponse(ns string, resp protocols.Response) {
	prev := ctx.responses[ns]
	if prev != nil && prev != resp {
		prev.Close()
	}
	ctx.responses[ns] = resp
}

// GetInputResponse returns the response of the input namespace.
func (ctx *Context) GetInputResponse() protocols.Response {
	return ctx.GetResponse(ctx.inputNs)
}

// SetInputResponse sets the response of the input namespace to resp.
func (ctx *Context) SetInputResponse(resp protocols.Response) {
	ctx.SetResponse(ctx.inputNs, resp)
}

// SetKV sets the value of key to val.
func (ctx *Context) SetKV(key, val interface{}) {
	ctx.kv[key] = val
}

// GetKV returns the value of key.
func (ctx *Context) GetKV(key interface{}) interface{} {
	return ctx.kv[key]
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

	for _, req := range ctx.requests {
		req.Close()
	}

	for _, resp := range ctx.responses {
		resp.Close()
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
