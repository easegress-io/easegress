package logger

import (
	"fmt"
	"net/http"
	"time"

	"github.com/hexdecteam/easegateway/pkg/common"

	"github.com/sirupsen/logrus"
)

var (
	LOG_REST_ACCESS_FILE  = "rest_access.log"
	LOG_REST_ACCESS_LEVEL = logrus.DebugLevel

	restLog = newLoggerSet()
)

func initREST() {
	formatter := new(restFormatter)

	f, out, err := openBufferedLogFile(LOG_REST_ACCESS_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_REST_ACCESS_FILE, err)
	} else {
		restLog.registerFileLogger("rest_access", f, out, LOG_REST_ACCESS_FILE, formatter, LOG_REST_ACCESS_LEVEL)
	}
}

type restFormatter struct{}

func (f *restFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(fmt.Sprintln(entry.Message)), nil
}

func RESTAccess(req *http.Request, code int, bodyBytesSent int64,
	requestTime *time.Time, processTime *time.Duration, requestDump []byte) {
	// mock access log_format:
	// '$remote_addr - $remote_user [$time_local] '
	// '$status $body_bytes_sent '
	// '[$request_time] $process_time:\n'
	// '$request_dump';

	line := fmt.Sprintf(
		`%v - - [%v] `+
			`%v %v `+
			"[%v] %f:\n"+
			`%s`,
		req.RemoteAddr, common.Now().Local(),
		code, bodyBytesSent,
		requestTime.Local(), processTime.Seconds(),
		requestDump)

	for _, l := range restLog.getLoggers("rest_access") {
		l.Debugf(line)
	}
}
