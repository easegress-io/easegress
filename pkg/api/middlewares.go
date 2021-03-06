package api

import (
	"fmt"
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

				logger.Errorf("recover from %s, err: %v, stack trace:\n%s\n",
					ctx.HandlerName(), err, debug.Stack())
				if ce, ok := err.(clusterErr); ok {
					HandleAPIError(ctx, http.StatusServiceUnavailable, ce)
				} else {
					HandleAPIError(ctx, http.StatusInternalServerError, fmt.Errorf("%v", err))
				}
			}
		}()

		ctx.Next()
	}
}

func newConfigVersionAttacher(s *Server) func(context.Context) {
	return func(ctx context.Context) {
		// NOTE: It needs to add the header before the next handlers
		// write the body to the network.
		version := s._getVersion()
		ctx.ResponseWriter().Header().Set(ConfigVersionKey, fmt.Sprintf("%d", version))
		ctx.Next()
	}
}
