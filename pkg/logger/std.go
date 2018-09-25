package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

var (
	LOG_STD_FILE = "gateway.log"

	LOG_STD_SET_NAME = "stdio"

	std = newLoggerSet()
)

// this interface supports current std logger to be used
// in other libraries, such as fasthttp.Server.Logger
type Logger interface {
	// Printf must have the same semantics as log.Printf.
	Printf(format string, args ...interface{})
}

type logger struct {
}

var defaultStdLogger = &logger{}

func (l *logger) Printf(format string, args ...interface{}) {
	Errorf(format, args...)
}

func initStd(logLevel logrus.Level) {
	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}

	std.registerIOLogger(LOG_STD_SET_NAME, os.Stdout, formatter, logLevel)

	f, out, err := openBufferedLogFile(LOG_STD_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_STD_FILE, err)
	} else {
		std.registerFileLogger(LOG_STD_SET_NAME, f, out, LOG_STD_FILE, formatter, logLevel)
	}
}

func StdLogger() Logger {
	return defaultStdLogger
}

func setStdLevel(level logrus.Level) {
	loggers := std.getLoggers(LOG_STD_SET_NAME)
	for i := range loggers {
		loggers[i].SetLevel(level)
	}
}

func Debugf(format string, args ...interface{}) {
	for _, logger := range std.getLoggers(LOG_STD_SET_NAME) {
		logger.WithFields(logrus.Fields{
			"source": getSourceInfo(),
		}).Debugf(format, args...)
	}
}

func Infof(format string, args ...interface{}) {
	for _, l := range std.getLoggers(LOG_STD_SET_NAME) {
		l.WithFields(logrus.Fields{
			"source": getSourceInfo(),
		}).Infof(format, args...)
	}
}

func Warnf(format string, args ...interface{}) {
	for _, l := range std.getLoggers(LOG_STD_SET_NAME) {
		l.WithFields(logrus.Fields{
			"source": getSourceInfo(),
		}).Warnf(format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	for _, l := range std.getLoggers(LOG_STD_SET_NAME) {
		l.WithFields(logrus.Fields{
			"source": getSourceInfo(),
		}).Errorf(format, args...)
	}
}

// Gateway needs robustness, so there is no need to support panic/fatal level
