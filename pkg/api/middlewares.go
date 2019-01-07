package api

import (
	"strconv"
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
		bodyBytesReceived, _ = strconv.ParseInt(ctx.GetHeader(context.ContentLengthHeaderKey), 10, 64)
		bodyBytesSent = ctx.GetContentLength()

		logger.APIAccess(method, remoteAddr, path, code,
			bodyBytesReceived, bodyBytesSent,
			startTime, processTime)
	}
}
