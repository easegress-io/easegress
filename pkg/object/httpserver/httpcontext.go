package httpserver

import (
	stdcontext "context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/util/httpheader"
	"github.com/megaease/easegateway/pkg/util/readercounter"
	"github.com/tomasen/realip"
)

type (

	// FinishFunc is the type of function to be called back
	// when HTTPContext is finishing.
	FinishFunc = func()

	httpContext struct {
		startTime   *time.Time
		endTime     *time.Time
		finishFuncs []FinishFunc
		tags        []string

		r *httpRequest
		w *httpResponse

		stdctx     stdcontext.Context
		cancelFunc stdcontext.CancelFunc
		err        error
	}
)

// NOTE: We can't use sync.Pool to recycle context.
// Reference: https://github.com/gin-gonic/gin/issues/1731
func newHTTPContext(startTime *time.Time, stdw http.ResponseWriter, stdr *http.Request) *httpContext {
	stdctx, cancelFunc := stdcontext.WithCancel(stdr.Context())
	stdr = stdr.WithContext(stdctx)

	return &httpContext{
		startTime:  startTime,
		stdctx:     stdctx,
		cancelFunc: cancelFunc,

		r: &httpRequest{
			std:    stdr,
			header: httpheader.New(stdr.Header),
			body:   readercounter.New(stdr.Body),
			realIP: realip.FromRequest(stdr),
		},
		w: &httpResponse{
			std:    stdw,
			code:   http.StatusOK,
			header: httpheader.New(stdw.Header()),
		},
	}
}

func (ctx *httpContext) AddTag(tag string) {
	ctx.tags = append(ctx.tags, tag)
}

func (ctx *httpContext) Request() context.HTTPRequest {
	return ctx.r
}

func (ctx *httpContext) Response() context.HTTPReponse {
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
		ctx.AddTag(fmt.Sprintf("cancelErr:%s", err))
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

func (ctx *httpContext) Duration() time.Duration {
	if ctx.endTime != nil {
		return ctx.endTime.Sub(*ctx.startTime)
	}

	return time.Now().Sub(*ctx.startTime)
}

func (ctx *httpContext) finish() {
	ctx.w.finish()

	endTime := time.Now()
	ctx.endTime = &endTime

	for _, fn := range ctx.finishFuncs {
		fn()
	}

	logger.HTTPAccess(ctx.Log())
}

func (ctx *httpContext) Log() string {
	stdr := ctx.r.std

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
	return fmt.Sprintf("[%s] "+
		"[%s %s %s %s %s %d] "+
		"[%v rx:%dB tx:%dB] "+
		"[%s]",
		ctx.startTime.Format(time.RFC3339),
		stdr.RemoteAddr, ctx.r.RealIP(), stdr.Method, stdr.RequestURI, stdr.Proto, ctx.w.code,
		ctx.Duration(), ctx.r.body.Count(), ctx.w.bodyWritten,
		strings.Join(ctx.tags, " | "))
}
