package logger

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	LOG_API_ACCESS_FILE  = "api_access.log"
	LOG_API_ACCESS_LEVEL = logrus.DebugLevel

	apiLog = newLoggerSet()
)

func initAPI() {
	formatter := new(apiFormatter)

	f, out, err := openBufferedLogFile(LOG_API_ACCESS_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_API_ACCESS_FILE, err)
	} else {
		apiLog.registerFileLogger("api_access", f, out, LOG_API_ACCESS_FILE, formatter, LOG_API_ACCESS_LEVEL)
	}
}

type apiFormatter struct{}

func (f *apiFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(fmt.Sprintln(entry.Message)), nil
}

func APIAccess(method, remoteAddr, path string, code int,
	bodyBytedReceived, bodyBytesSent int64,
	requestTime time.Time, processTime time.Duration) {
	line := fmt.Sprintf("%s %s %s %v rx:%dB tx:%dB start:%v process:%v",
		method, remoteAddr, path, code,
		bodyBytedReceived, bodyBytesSent,
		requestTime, processTime)

	for _, l := range apiLog.getLoggers("api_access") {
		l.Debugf(line)
	}
}
