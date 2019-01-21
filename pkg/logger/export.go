package logger

import (
	"fmt"
	"net/url"
	"time"

	"github.com/megaease/easegateway/pkg/common"

	"go.uber.org/zap"
)

var (
	Debugf logfFunc
	Infof  logfFunc
	Warnf  logfFunc
	Errorf logfFunc
)

type httpServerLogger struct {
	defaultLogger *zap.SugaredLogger
}

func (l *httpServerLogger) Printf(template string, args ...interface{}) {
	l.defaultLogger.Errorf(template, args...)
}

func HTTPServerLogger() *httpServerLogger {
	return &httpServerLogger{
		defaultLogger: defaultLogger,
	}
}

func Close() {
	defaultLogger.Sync()
	stderrLogger.Sync()
	gatewayLogger.Sync()
	httpPluginAccessLogger.Sync()
	httpPluginDumpLogger.Sync()
	restAPILogger.Sync()
}

func APIAccess(
	method, remoteAddr, path string,
	code int,
	bodyBytedReceived, bodyBytesSent int64,
	requestTime time.Time,
	processTime time.Duration) {
	entry := fmt.Sprintf("%s %s %s %v rx:%dB tx:%dB start:%v process:%v",
		method, remoteAddr, path, code,
		bodyBytedReceived, bodyBytesSent,
		requestTime.Format(time.RFC3339), processTime)

	restAPILogger.Debug(entry)
}

func HTTPAccess(remoteAddr, proto, method, path, referer, agent, realIP string,
	code int, bodyBytesSent int64,
	requestTime time.Duration, upstreamResponseTime time.Duration,
	upstreamAddr string, upstreamCode int, clientWriteBodyTime, clientReadBodyTime,
	routeTime time.Duration) {
	// mock nginx log_format:
	// '$remote_addr - $remote_user [$time_local] "$request" '
	// '$status $body_bytes_sent "$http_referer" '
	// '"$http_user_agent" "$http_x_forwarded_for" '
	// '$request_time $upstream_response_time $upstream_addr $upstream_status $pipe '
	// '$client_write_body_time' '$client_read_body_time' '$route_time';

	if referer == "" {
		referer = "-"
	}

	if agent == "" {
		agent = "-"
	} else {
		if a, err := url.QueryUnescape(agent); err == nil {
			agent = a
		}
	}

	if realIP == "" {
		realIP = "-"
	}

	if upstreamAddr == "" {
		upstreamAddr = "-"
	} else {
		if addr, err := url.QueryUnescape(upstreamAddr); err == nil {
			upstreamAddr = addr
		}
	}

	line := fmt.Sprintf(
		`%v - - [%v] "%s %s %s" `+
			`%v %v "%s" `+
			`"%s" "%s" `+
			`%f %f %v %v . `+
			`%f %f %f`,
		remoteAddr, common.Now().Local(), method, path, proto,
		code, bodyBytesSent, referer,
		agent, realIP,
		requestTime.Seconds(), upstreamResponseTime.Seconds(), upstreamAddr, upstreamCode,
		clientWriteBodyTime.Seconds(), clientReadBodyTime.Seconds(), routeTime.Seconds())

	httpPluginAccessLogger.Debug(line)
}

type DumpRequest func() (string, error)
type DumpResponse func() (string, error)

func HTTPReqDump(pipelineName, pluginName, pluginInstanceId string, taskId int64, dump DumpRequest) {
	s, err := dump()
	if err != nil {
		Warnf("dump http request to log failed: %s", err)
		return
	}

	entry := fmt.Sprintf("%s/%s@%s/task#%d - - [%v]:\n%s",
		pipelineName, pluginName, pluginInstanceId, taskId, common.Now().Local(), s)

	httpPluginDumpLogger.Debug(entry)
}

func HTTPRespDump(pipelineName, pluginName, pluginInstanceId string, taskId int64, dump DumpResponse) {
	s, err := dump()
	if err != nil {
		Warnf("dump http response to log failed: %s", err)
		return
	}

	entry := fmt.Sprintf("%s/%s@%s/task#%d - - [%v]:\n%s",
		pipelineName, pluginName, pluginInstanceId, taskId, common.Now().Local(), s)

	httpPluginDumpLogger.Debug(entry)
}
