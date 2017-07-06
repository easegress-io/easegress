package logger

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sirupsen/logrus"

	"option"
)

type innerLogger struct {
	logrusLogger *logrus.Logger

	logFileName string
	logFile     *os.File
}

type loggerSet struct {
	v map[string][]*innerLogger
}

var loggers []*loggerSet

func newLoggerSet() *loggerSet {
	ret := &loggerSet{
		v: make(map[string][]*innerLogger, 0),
	}

	loggers = append(loggers, ret)

	return ret
}

func (l *loggerSet) _registerLogger(name string, out io.Writer,
	formatter logrus.Formatter, lvl logrus.Level) *innerLogger {

	ret := new(innerLogger)
	ret.logrusLogger = logrus.New()
	ret.logrusLogger.Out = out
	ret.logrusLogger.Formatter = formatter
	ret.logrusLogger.Level = lvl

	return ret
}

func (l *loggerSet) registerFileLogger(name string, out *os.File, fileName string,
	formatter logrus.Formatter, lvl logrus.Level) {

	il := l._registerLogger(name, out, formatter, lvl)
	il.logFileName = fileName
	il.logFile = out

	l.v[name] = append(l.v[name], il)
}

func (l *loggerSet) registerIOLogger(name string, out io.Writer, formatter logrus.Formatter, lvl logrus.Level) {
	il := l._registerLogger(name, out, formatter, lvl)
	l.v[name] = append(l.v[name], il)
}

func (l *loggerSet) getLoggers(typ string) []*logrus.Logger {
	ret := make([]*logrus.Logger, 0)

	innerLoggers := l.v[typ]
	if innerLoggers == nil {
		return ret
	}

	for _, il := range innerLoggers {
		ret = append(ret, il.logrusLogger)
	}

	return ret
}

func openLogFile(fileName string) (*os.File, error) {
	if _, err := os.Stat(option.LogHome); os.IsNotExist(err) {
		err = os.Mkdir(option.LogHome, 0750)
		if err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(filepath.Join(option.LogHome, fileName), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
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

////

func ReOpenLogFiles() {
	buff := bytes.NewBufferString("")
	defer buff.Truncate(0) // save memory before gc

	for _, ls := range loggers {
		for _, ils := range ls.v {
			for _, il := range ils {
				if il.logFile == nil {
					// not a file based logger
					continue
				}

				buff.Truncate(0)

				// keep log output going
				il.logrusLogger.Out = buff

				err := il.logFile.Close()
				if err != nil {
					// disable bottom IO if failed to reopen the log file,
					// the follow logs will discarded to prevent any corrupt IO object
					// which in the middle state between open and close.
					il.logrusLogger.Out = ioutil.Discard

					Errorf("[close log file %s failed: %v]", il.logFileName, err)
					continue
				}

				f, err := openLogFile(il.logFileName)
				if err != nil {
					il.logrusLogger.Out = ioutil.Discard

					Errorf("[reopen log file %s failed: %v]", il.logFileName, err)
					continue
				}

				f.Write(buff.Bytes())
				// FIXME(zhiyan): need to add suspend-able IO to prevent any potential log lost?
				// which might happen between both IO objects swatch?
				il.logrusLogger.Out = f
				il.logFile = f

				Infof("[log file %s has been reopened]", il.logFileName)
			}
		}
	}
}
