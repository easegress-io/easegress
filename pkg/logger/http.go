package logger

import (
	"fmt"
	"net/url"
	"time"

	"github.com/megaease/easegateway/pkg/common"

	"github.com/sirupsen/logrus"
)

var (
	LOG_HTTP_ACCESS_FILE = "http_access.log"
	LOG_HTTP_DUMP_FILE   = "http_dump.log"
	LOG_HTTP_LEVEL       = logrus.DebugLevel

	httpLog = newLoggerSet()
)

func initHTTP() {
	formatter := new(httpFormatter)

	f, out, err := openBufferedLogFile(LOG_HTTP_ACCESS_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_HTTP_ACCESS_FILE, err)
	} else {
		httpLog.registerFileLogger("http_access", f, out, LOG_HTTP_ACCESS_FILE, formatter, LOG_HTTP_LEVEL)
	}

	// test example:
	// req := httptest.NewRequest("POST", "https://127.0.0.1/api/sessions", nil)
	// req.Header.Set("Referer", "http://easeteam.com/login")
	// req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/55.0.2883.75 Safari/537.36")
	// HTTPAccess(req, 504, 585, time.Duration(150*time.Millisecond))

	f, err = openLogFile(LOG_HTTP_DUMP_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_HTTP_DUMP_FILE, err)
	} else {
		// no buffer for performance, normally dump for debug or functional test only
		httpLog.registerFileLogger("http_dump", f, f, LOG_HTTP_DUMP_FILE, formatter, LOG_HTTP_LEVEL)
	}
}

type httpFormatter struct{}

func (f *httpFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(fmt.Sprintln(entry.Message)), nil
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

	for _, l := range httpLog.getLoggers("http_access") {
		l.Debugf(line)
	}
}

type DumpRequest func() (string, error)
type DumpResponse func() (string, error)

func HTTPReqDump(pipelineName, pluginName, pluginInstanceId string, taskId int64, dump DumpRequest) {
	s, err := dump()
	if err != nil {
		Warnf("[dump http request to log failed: %s]", err)
		return
	}

	for _, l := range httpLog.getLoggers("http_dump") {
		l.Debugf("%s/%s@%s/task#%d - - [%v]:\n%s]",
			pipelineName, pluginName, pluginInstanceId, taskId, common.Now().Local(), s)
	}
}

func HTTPRespDump(pipelineName, pluginName, pluginInstanceId string, taskId int64, dump DumpResponse) {
	s, err := dump()
	if err != nil {
		Warnf("[dump http response to log failed: %s]", err)
		return
	}

	for _, l := range httpLog.getLoggers("http_dump") {
		l.Debugf("%s/%s@%s/task#%d - - [%v]:\n%s]",
			pipelineName, pluginName, pluginInstanceId, taskId, common.Now().Local(), s)
	}
}
