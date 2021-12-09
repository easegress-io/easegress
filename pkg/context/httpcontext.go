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
	stdcontext "context"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/tracing"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/httpheader"
	"github.com/megaease/easegress/pkg/util/httpstat"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"github.com/megaease/easegress/pkg/util/texttemplate"
)

type (
	// HandlerCaller is a helper function to call the handler
	HandlerCaller func(lastResult string) string

	// HTTPContext is all context of an HTTP processing.
	// It is not goroutine-safe, callers must use Lock/Unlock
	// to protect it by themselves.
	HTTPContext interface {
		Context
		Lock()
		Unlock()

		Span() tracing.Span

		Request() HTTPRequest
		Response() HTTPResponse

		Cancel(err error)
		Cancelled() bool
		ClientDisconnected() bool

		OnFinish(FinishFunc)         // For setting final client statistics, etc.
		AddTag(tag string)       // For debug, log, etc.
		GrowTagN(n int)          // Grow tag list by n items.
		AddLazyTag(tag *LazyTag) // For lazy evaluation of tag.
		GrowLazyTagN(num int)    // Grow Lazy Tags list by n items.

		StatMetric() *httpstat.Metric

		Finish()

		Template() texttemplate.TemplateEngine
		SetTemplate(ht *HTTPTemplate)
		SaveReqToTemplate(filterName string) error
		SaveRspToTemplate(filterName string) error

		CallNextHandler(lastResult string) string
		SetHandlerCaller(caller HandlerCaller)
	}

	// HTTPRequest is all operations for HTTP request.
	HTTPRequest interface {
		RealIP() string

		Method() string
		SetMethod(method string)

		// URL
		Scheme() string
		Host() string
		SetHost(host string)
		Path() string
		SetPath(path string)
		EscapedPath() string
		Query() string
		SetQuery(query string)
		Fragment() string

		Proto() string

		Header() *httpheader.HTTPHeader
		Cookie(name string) (*http.Cookie, error)
		Cookies() []*http.Cookie
		AddCookie(cookie *http.Cookie)

		Body() io.Reader
		SetBody(io.Reader)

		Std() *http.Request

		Size() uint64 // bytes
	}

	// HTTPResponse is all operations for HTTP response.
	HTTPResponse interface {
		StatusCode() int // Default is 200
		SetStatusCode(code int)

		Header() *httpheader.HTTPHeader
		SetCookie(cookie *http.Cookie)

		SetBody(body io.Reader)
		Body() io.Reader
		OnFlushBody(BodyFlushFunc)

		Std() http.ResponseWriter

		Size() uint64 // bytes
	}

	// HTTPResult is result for handling http request
	HTTPResult struct {
		Err error
	}

	// FinishFunc is the type of function to be called back
	// when HTTPContext is finishing.
	FinishFunc = func()

	// BodyFlushFunc is the type of function to be called back
	// when body is flushing.
	BodyFlushFunc = func(body []byte, complete bool) (newBody []byte)

	httpContext struct {
		mutex sync.Mutex

		startTime   time.Time
		finishFuncs []FinishFunc
		tags        []string
		lazyTags    []*LazyTag
		tagInd      int
		lazyTagInd  int
		caller      HandlerCaller

		r *httpRequest
		w *httpResponse

		ht             *HTTPTemplate
		span           tracing.Span
		originalReqCtx stdcontext.Context
		stdctx         stdcontext.Context
		cancelFunc     stdcontext.CancelFunc
		err            error

		metric httpstat.Metric
	}

	// TODO
	LazyTag struct {
		Namespace string
		Prefix    string
		StringMsg string

		IntMsg int

		Sep string
	}
)

func (lt *LazyTag) String() string {
	msg := lt.StringMsg
	if lt.IntMsg > 0 {
		msg = strconv.Itoa(lt.IntMsg)
	}
	return stringtool.Cat(lt.Namespace, lt.Sep, lt.Prefix, lt.Sep, msg)
}

// New creates an HTTPContext.
// NOTE: We can't use sync.Pool to recycle context.
// Reference: https://github.com/gin-gonic/gin/issues/1731
func New(stdw http.ResponseWriter, stdr *http.Request,
	tracer *tracing.Tracing, spanName string) HTTPContext {
	originalReqCtx := stdr.Context()
	stdctx, cancelFunc := stdcontext.WithCancel(originalReqCtx)
	stdr = stdr.WithContext(stdctx)

	startTime := fasttime.Now()
	span := tracing.NewSpanWithStart(tracer, spanName, startTime)
	span.SetTag("http.method", stdr.Method)
	span.SetTag("http.path", stdr.URL.Path)
	return &httpContext{
		startTime:      startTime,
		span:           span,
		originalReqCtx: originalReqCtx,
		stdctx:         stdctx,
		cancelFunc:     cancelFunc,
		r:              newHTTPRequest(stdr),
		w:              newHTTPResponse(stdw, stdr),
		tags:           make([]string, 3),
		lazyTags:       make([]*LazyTag, 0),
		tagInd:         0,
		lazyTagInd:     0,
	}
}

func (ctx *httpContext) Protocol() Protocol {
	return HTTP
}

func (ctx *httpContext) CallNextHandler(lastResult string) string {
	return ctx.caller(lastResult)
}

func (ctx *httpContext) SetHandlerCaller(caller HandlerCaller) {
	ctx.caller = caller
}

func (ctx *httpContext) Lock() {
	ctx.mutex.Lock()
}

func (ctx *httpContext) Unlock() {
	ctx.mutex.Unlock()
}

func (ctx *httpContext) Span() tracing.Span {
	return ctx.span
}

func (ctx *httpContext) AddTag(tag string) {
	if len(ctx.tags) < ctx.tagInd {
		ctx.tags[ctx.tagInd] = tag
	} else {
		ctx.tags = append(ctx.tags, tag)
	}
	ctx.tagInd++
}

func (ctx *httpContext) GrowTagN(n int) {
	newTags := make([]string, len(ctx.tags)+n)
	for i, tag := range ctx.tags {
		newTags[i] = tag
	}
	ctx.tags = newTags
}

func (ctx *httpContext) AddLazyTag(tag *LazyTag) {
	if len(ctx.lazyTags) < ctx.lazyTagInd {
		ctx.lazyTags[ctx.lazyTagInd] = tag
	} else {
		ctx.lazyTags = append(ctx.lazyTags, tag)
	}
	ctx.tagInd++
}

func (ctx *httpContext) GrowLazyTagN(n int) {
	newLazyTags := make([]*LazyTag, len(ctx.lazyTags)+n)
	for i, tag := range ctx.lazyTags {
		newLazyTags[i] = tag
	}
	ctx.lazyTags = newLazyTags
}

func (ctx *httpContext) Request() HTTPRequest {
	return ctx.r
}

func (ctx *httpContext) Response() HTTPResponse {
	return ctx.w
}

func (ctx *httpContext) Deadline() (time.Time, bool) {
	return ctx.stdctx.Deadline()
}

func (ctx *httpContext) Done() <-chan struct{} {
	return ctx.stdctx.Done()
}

func (ctx *httpContext) Err() error {
	if ctx.err != nil {
		return ctx.err
	}

	return ctx.stdctx.Err()
}

func (ctx *httpContext) Value(key interface{}) interface{} {
	return ctx.stdctx.Value(key)
}

func (ctx *httpContext) Cancel(err error) {
	if !ctx.Cancelled() {
		ctx.AddTag(stringtool.Cat("cancelErr: ", err.Error()))
		ctx.err = err
		ctx.cancelFunc()
	}
}

func (ctx *httpContext) OnFinish(fn FinishFunc) {
	ctx.finishFuncs = append(ctx.finishFuncs, fn)
}

func (ctx *httpContext) Cancelled() bool {
	return ctx.err != nil || ctx.stdctx.Err() != nil
}

func (ctx *httpContext) ClientDisconnected() bool {
	return ctx.originalReqCtx.Err() != nil
}

func (ctx *httpContext) Finish() {
	if ctx.ClientDisconnected() {
		ctx.AddTag(fmt.Sprintf("client closed connection: change code %d to 499",
			ctx.w.StatusCode()))
		ctx.w.SetStatusCode(EGStatusClientClosedRequest /* consistent with nginx */)
	}

	ctx.r.finish()
	ctx.w.finish()

	ctx.metric.StatusCode = ctx.Response().StatusCode()
	ctx.metric.Duration = fasttime.Now().Sub(ctx.startTime)
	ctx.metric.ReqSize = ctx.Request().Size()
	ctx.metric.RespSize = ctx.Response().Size()

	for _, fn := range ctx.finishFuncs {
		func() {
			defer func() {
				if err := recover(); err != nil {
					logger.Errorf("failed to handle finish actions for %s: %v, stack trace: \n%s\n",
						ctx.Request().Path(), err, debug.Stack())
				}
			}()

			fn()
		}()
	}

	logger.LazyHTTPAccess(func() string {
		stdr := ctx.r.std

		// combine here normal and Lazy tags
		allTags := make([]string, ctx.tagInd+ctx.lazyTagInd)
		for i := 0; i < ctx.tagInd; i++ {
			allTags[i] = ctx.tags[i]
		}
		for i := 0; i < ctx.lazyTagInd; i++ {
			allTags[ctx.tagInd+i] = ctx.lazyTags[i].String()
		}

		// log format:
		// [startTime]
		// [requestInfo]
		// [contextStatistics]
		// [tags]
		//
		// [$startTime]
		// [$remoteAddr $realIP $method $requestURL $proto $statusCode]
		// [$contextDuration $readBytes $writeBytes]
		// [$tags]
		return fmt.Sprintf("[%s] [%s %s %s %s %s %d] [%v rx:%dB tx:%dB] [%s]",
			fasttime.Format(ctx.startTime, fasttime.RFC3339Milli),
			stdr.RemoteAddr, ctx.r.RealIP(), stdr.Method, stdr.RequestURI, stdr.Proto, ctx.w.code,
			ctx.metric.Duration, ctx.r.Size(), ctx.w.Size(),
			strings.Join(allTags, " | "))
	})
}

func (ctx *httpContext) StatMetric() *httpstat.Metric {
	return &ctx.metric
}

// Template returns the template engine
func (ctx *httpContext) Template() texttemplate.TemplateEngine {
	return ctx.ht.Engine
}

// SetTemplate sets the http template initinaled by other module
func (ctx *httpContext) SetTemplate(ht *HTTPTemplate) {
	ctx.ht = ht
}

// SaveReqToTemplate stores http request related info into HTTP template engine
func (ctx *httpContext) SaveReqToTemplate(filterName string) error {
	return ctx.ht.SaveRequest(filterName, ctx)
}

// SaveRspToTemplate stores http response related info into HTTP template engine
func (ctx *httpContext) SaveRspToTemplate(filterName string) error {
	return ctx.ht.SaveResponse(filterName, ctx)
}
