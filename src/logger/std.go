package logger

import (
	"os"

	"github.com/sirupsen/logrus"

	"option"
)

var (
	LOG_STD_FILE      = "gateway.log"
	LOG_STD_LEVEL     = logrus.DebugLevel
	LOG_STD_TTY_LEVEL = logrus.DebugLevel

	std = newLoggerSet()
)

func initStd() {
	if option.Stage == "prod" {
		LOG_STD_TTY_LEVEL = logrus.InfoLevel
	}
	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}

	std.registerIOLogger("stdio", os.Stdout, formatter, LOG_STD_TTY_LEVEL)

	f, err := openLogFile(LOG_STD_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_STD_FILE, err)
	} else {
		std.registerFileLogger("stdio", f, LOG_STD_FILE, formatter, LOG_STD_LEVEL)
	}
}

func Debugf(format string, args ...interface{}) {
	for _, logger := range std.getLoggers("stdio") {
		logger.WithFields(logrus.Fields{
			"source": getSourceInfo(),
		}).Debugf(format, args...)
	}
}

func Infof(format string, args ...interface{}) {
	for _, l := range std.getLoggers("stdio") {
		l.WithFields(logrus.Fields{
			"source": getSourceInfo(),
		}).Infof(format, args...)
	}
}

func Warnf(format string, args ...interface{}) {
	for _, l := range std.getLoggers("stdio") {
		l.WithFields(logrus.Fields{
			"source": getSourceInfo(),
		}).Warnf(format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	for _, l := range std.getLoggers("stdio") {
		l.WithFields(logrus.Fields{
			"source": getSourceInfo(),
		}).Errorf(format, args...)
	}
}

// Gateway needs robustness, so there is no need to support panic/fatal level
