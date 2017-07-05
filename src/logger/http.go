package logger

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	LOG_HTTP_FILE  = "http_access.log"
	LOG_HTTP_LEVEL = logrus.DebugLevel

	httpstd = newLoggerSet()
)

func initHTTP() {
	f, err := openLogFile(LOG_HTTP_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_HTTP_FILE, err)
	} else {
		httpstd.registerFileLogger("http", f, LOG_HTTP_FILE, &httpFormatter{}, LOG_HTTP_LEVEL)
	}

	// test example:
	// req := httptest.NewRequest("POST", "https://127.0.0.1/api/sessions", nil)
	// req.Header.Set("Referer", "http://easeteam.com/login")
	// req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/55.0.2883.75 Safari/537.36")
	// HTTPAccess(req, 504, 585, time.Duration(150*time.Millisecond))
}

type httpFormatter struct{}

func (f *httpFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(fmt.Sprintln(entry.Message)), nil
}

func HTTPAccess(req *http.Request, code int, bodyBytesSent int,
	requestTime time.Duration,
	upstreamResponseTime time.Duration) {
	// mock nginx log_format:
	// '$remote_addr - $remote_user [$time_local] "$request" '
	// '$status $body_bytes_sent "$http_referer" '
	// '"$http_user_agent" "$http_x_forwarded_for" '
	// '$request_time $upstream_response_time $pipe';

	referer := req.Header.Get("Referer")
	if referer == "" {
		referer = "-"
	}
	agent := req.Header.Get("User-Agent")
	if agent == "" {
		agent = "-"
	}
	realIP := req.Header.Get("X-Forwarded-For")
	if realIP == "" {
		realIP = "-"
	}

	line := fmt.Sprintf(
		`%v - - [%v] "%s %s %s" `+
			`%v %v "%s" `+
			`"%s" "%s" `+
			`%v %v .`,
		req.RemoteAddr, time.Now().Local(), req.Method, req.URL.Path, req.Proto,
		code, bodyBytesSent, referer,
		agent, realIP,
		requestTime, upstreamResponseTime)

	for _, logger := range httpstd.getLoggers("http") {
		logger.Debugf(line)
	}
}
