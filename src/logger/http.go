package logger

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	LOG_HTTP_ACCESS_FILE = "http_access.log"
	LOG_HTTP_DUMP_FILE   = "http_dump.log"
	LOG_HTTP_LEVEL       = logrus.DebugLevel

	httpLog = newLoggerSet()
)

func initHTTP() {
	f, out, err := openBufferedLogFile(LOG_HTTP_ACCESS_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_HTTP_ACCESS_FILE, err)
	}

	httpLog.registerFileLogger("http_access", f, out, LOG_HTTP_ACCESS_FILE, &httpFormatter{}, LOG_HTTP_LEVEL)

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
		httpLog.registerFileLogger("http_dump", f, f, LOG_HTTP_DUMP_FILE, &httpFormatter{}, LOG_HTTP_LEVEL)
	}
}

type httpFormatter struct{}

func (f *httpFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(fmt.Sprintln(entry.Message)), nil
}

func HTTPAccess(req *http.Request, code int, bodyBytesSent int64,
	requestTime time.Duration, upstreamResponseTime time.Duration,
	upstreamAddr string, upstreamCode int, clientWriteBodyTime time.Duration) {
	// mock nginx log_format:
	// '$remote_addr - $remote_user [$time_local] "$request" '
	// '$status $body_bytes_sent "$http_referer" '
	// '"$http_user_agent" "$http_x_forwarded_for" '
	// '$request_time $upstream_response_time $upstream_addr $upstream_status $pipe '
	// '$client_write_body_time';

	referer := req.Header.Get("Referer")
	if referer == "" {
		referer = "-"
	}

	agent := req.Header.Get("User-Agent")
	if agent == "" {
		agent = "-"
	} else {
		if a, err := url.QueryUnescape(agent); err == nil {
			agent = a
		}
	}

	realIP := req.Header.Get("X-Forwarded-For")
	if realIP == "" {
		realIP = "-"
	}

	if upstreamAddr == "" {
		upstreamAddr = "-"
	}

	line := fmt.Sprintf(
		`%v - - [%v] "%s %s %s" `+
			`%v %v "%s" `+
			`"%s" "%s" `+
			`%f %f %v %v . `+
			`%v`,
		req.RemoteAddr, time.Now().Local(), req.Method, req.URL.Path, req.Proto,
		code, bodyBytesSent, referer,
		agent, realIP,
		requestTime.Seconds(), upstreamResponseTime.Seconds(), upstreamAddr, upstreamCode,
		clientWriteBodyTime.Seconds())

	for _, l := range httpLog.getLoggers("http_access") {
		l.Debugf(line)
	}
}

func HTTPReqDump(pipelineName, pluginName, pluginInstanceId string, taskId int64, req *http.Request) {
	dump, err := httputil.DumpRequest(req, false /* do not dump request body */)
	if err != nil {
		Warnf("[dump http request to log failed: %s]", err)
		return
	}

	for _, l := range httpLog.getLoggers("http_dump") {
		l.Debugf("%s/%s@%s/task#%d - - [%v]:\n%s]",
			pipelineName, pluginName, pluginInstanceId, taskId, time.Now().Local(), dump)
	}
}

func HTTPRespDump(pipelineName, pluginName, pluginInstanceId string, taskId int64, resp *http.Response) {
	dump, err := httputil.DumpResponse(resp, false /* do not dump response body */)
	if err != nil {
		Warnf("[dump http response to log failed: %s]", err)
		return
	}

	for _, l := range httpLog.getLoggers("http_dump") {
		l.Debugf("%s/%s@%s/task#%d - - [%v]:\n%s]",
			pipelineName, pluginName, pluginInstanceId, taskId, time.Now().Local(), dump)
	}
}
