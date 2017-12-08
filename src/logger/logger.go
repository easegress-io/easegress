package logger

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

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

func (l *loggerSet) registerFileLogger(name string, f *os.File, out io.Writer, fileName string,
	formatter logrus.Formatter, lvl logrus.Level) {

	il := l._registerLogger(name, out, formatter, lvl)
	il.logFileName = fileName
	il.logFile = f

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

	f, err := os.OpenFile(filepath.Join(option.LogHome, fileName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return nil, err
	}

	return f, nil
}

type bufWriter struct {
	w         *bufio.Writer
	wLock     sync.Mutex
	data      chan []byte // channel used as a write queue to smooth periodic flush peak
	stop      chan struct{}
	closeLock sync.RWMutex
	closed    bool
	done      chan *struct{}
}

func newBufWriter(w io.Writer) *bufWriter {
	bw := &bufWriter{
		w:    bufio.NewWriterSize(w, 512), // block size for general VFS
		data: make(chan []byte, 2048),
		stop: make(chan struct{}),
		done: make(chan *struct{}),
	}

	flusher := func() {
		for {
			select {
			case <-time.Tick(time.Second): // deadline is a parameter?
				bw.wLock.Lock()
				bw.w.Flush()
				bw.wLock.Unlock()
			case <-bw.stop:
				bw.wLock.Lock()
				bw.w.Flush()
				bw.wLock.Unlock()
				bw.done <- nil
				return
			}
		}
	}

	bufWriter := func() {
		for {
			d := <-bw.data
			if d == nil { // closed
				return
			}

			bw.wLock.Lock()
			bw.w.Write(d)
			bw.wLock.Unlock()
		}
	}

	go flusher()
	go bufWriter()

	return bw
}

func (bw *bufWriter) Close() {
	bw.closeLock.Lock()
	defer bw.closeLock.Unlock()

	if bw.closed {
		return
	}

	for len(bw.data) > 0 {
		Debugf("[spin to wait log data writes completely]")
		time.Sleep(time.Millisecond)
	}

	close(bw.data)
	close(bw.stop)

	<-bw.done // wait flush finish

	bw.closed = true
}

func (bw *bufWriter) Write(p []byte) (n int, err error) { // async write
	bw.closeLock.RLock()
	defer bw.closeLock.RUnlock()

	if bw.closed {
		return 0, fmt.Errorf("buffer closed")
	}

	bw.data <- p

	return len(p), nil
}

func openBufferedLogFile(fileName string) (*os.File, io.Writer, error) {
	f, err := openLogFile(fileName)
	if err != nil {
		return nil, nil, err
	}

	out := newBufWriter(f)

	return f, out, nil
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

	var f *os.File
	var out io.Writer

	for _, ls := range loggers {
		for _, ils := range ls.v {
			for _, il := range ils {
				if il.logFile == nil {
					// not a file based logger
					continue
				}

				buff.Truncate(0)

				out = il.logrusLogger.Out
				// keep log output going
				il.logrusLogger.Out = buff

				// flush IO buffer
				bw, buffering := out.(*bufWriter)
				if buffering {
					bw.Close()
				}

				err := il.logFile.Close()
				if err != nil {
					// disable bottom IO if failed to reopen the log file,
					// the follow logs will discarded to prevent any corrupt IO object
					// which in the middle state between open and close.
					il.logrusLogger.Out = ioutil.Discard

					Errorf("[close log file %s failed: %v]", il.logFileName, err)
					continue
				}

				if buffering {
					f, out, err = openBufferedLogFile(il.logFileName)
					if err != nil {
						il.logrusLogger.Out = ioutil.Discard

						Errorf("[reopen log file %s failed: %v]", il.logFileName, err)
						continue
					}

					out.Write(buff.Bytes())
				} else {
					f, err = openLogFile(il.logFileName)
					if err != nil {
						il.logrusLogger.Out = ioutil.Discard

						Errorf("[reopen log file %s failed: %v]", il.logFileName, err)
						continue
					}

					f.Write(buff.Bytes())
					out = f
				}

				// FIXME(zhiyan): need to add suspend-able IO to prevent any potential log lost?
				// which might happen between both IO objects swatch?
				il.logrusLogger.Out = out
				il.logFile = f

				Infof("[log file %s reopened]", il.logFileName)
			}
		}
	}
}

func CloseLogFiles() {
	for _, ls := range loggers {
		for _, ils := range ls.v {
			for _, il := range ils {
				if il.logFile == nil {
					// not a file based logger
					continue
				}

				out := il.logrusLogger.Out
				il.logrusLogger.Out = ioutil.Discard

				// flush IO buffer
				bw, buffering := out.(*bufWriter)
				if buffering {
					bw.Close()
				}

				err := il.logFile.Close()
				if err != nil {
					Errorf("[close log file %s failed: %v]", il.logFileName, err)
				} else if buffering {
					Debugf("[log file %s flushed and closed]", il.logFileName)
				} else {
					Debugf("[log file %s closed]", il.logFileName)
				}
			}
		}
	}
}
