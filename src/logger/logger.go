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

type loggers struct {
	v map[string][]*logrus.Logger
}

func New() *loggers {
	return &loggers{
		v: make(map[string][]*logrus.Logger, 0),
	}
}

func (l *loggers) registerLogger(name string, out io.Writer, formatter logrus.Formatter, lvl logrus.Level) {
	logger := logrus.New()
	logger.Out = out
	logger.Formatter = formatter
	logger.Level = lvl
	l.v[name] = append(l.v[name], logger)
}

func (l *loggers) getLoggers(typ string) []*logrus.Logger {
	return l.v[typ]
}

func openLogFile(name string) (*os.File, error) {
	const flag = os.O_RDWR | os.O_CREATE | os.O_APPEND

	if _, err := os.Stat(common.LogHome); os.IsNotExist(err) {
		err = os.Mkdir(common.LogHome, 0755)
		if err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(filepath.Join(common.LogHome, name), flag, 0644)
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
	initMemberList()
}
