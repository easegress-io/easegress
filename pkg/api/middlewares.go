package api

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/logger"

	"github.com/kataras/iris/context"
)

func newAPILogger() func(context.Context) {
	return func(ctx context.Context) {
		var (
			method            string
			remoteAddr        string
			path              string
			code              int
			bodyBytesReceived int64
			bodyBytesSent     int64
			startTime         time.Time
			processTime       time.Duration
		)

		startTime = common.Now()
		ctx.Next()
		processTime = common.Now().Sub(startTime)

		method = ctx.Method()
		remoteAddr = ctx.RemoteAddr()
		path = ctx.Path()
		code = ctx.GetStatusCode()
		bodyBytesReceived = ctx.GetContentLength()
		bodyBytesSent = int64(ctx.ResponseWriter().Written())

		logger.APIAccess(method, remoteAddr, path, code,
			bodyBytesReceived, bodyBytesSent,
			startTime, processTime)
	}
}

func newRecoverer() func(context.Context) {
	return func(ctx context.Context) {
		defer func() {
			if err := recover(); err != nil {
				if ctx.IsStopped() {
					return
				}

				if err, ok := err.(clusterErr); ok {
					handleAPIError(ctx, http.StatusServiceUnavailable, err)
				} else {
					logger.Errorf("recovered from %s, stack trace:\n%s\n",
						ctx.HandlerName(), debug.Stack())
					handleAPIError(ctx, http.StatusInternalServerError, err)
				}
			}
		}()

		ctx.Next()
	}
}

func newJSONContentType() func(context.Context) {
	return func(ctx context.Context) {
		ctx.Header("Content-Type", "application/json")

		ctx.Next()
	}
}
