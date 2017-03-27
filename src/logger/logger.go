package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"common"

	"github.com/sirupsen/logrus"
)

type loggers []*logrus.Logger

func New() *loggers {
	l := make(loggers, 0)
	return &l
}

func (l *loggers) registerLogger(out io.Writer, formatter logrus.Formatter, lvl logrus.Level) {
	logger := logrus.New()
	logger.Out = out
	logger.Formatter = formatter
	logger.Level = lvl
	*l = append(*l, logger)
}

func (l *loggers) getLoggers() loggers {
	return *l
}

func openLogFile(name string) (*os.File, error) {
	const flag = os.O_RDWR | os.O_CREATE | os.O_APPEND

	if _, err := os.Stat(common.LOG_HOME_DIR); os.IsNotExist(err) {
		err = os.Mkdir(common.LOG_HOME_DIR, 0755)
		if err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(filepath.Join(common.LOG_HOME_DIR, name), flag, 0644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func getSourceInfo() string {
	pc, file, line, _ := runtime.Caller(2)
	return fmt.Sprintf("%v#%v-%v", filepath.Base(file), line, runtime.FuncForPC(pc).Name())
}

func init() {
	initStd()
	initHTTP()
}
