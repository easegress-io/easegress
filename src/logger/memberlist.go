package logger

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"

	"github.com/sirupsen/logrus"
)

var (
	LOG_MEMBERLIST_FILE = "memberlist.log"

	LOG_MEMBERLIST_SET_NAME = "memberlist"

	memberList = newLoggerSet()

	LOG_MEMBERLIST_REGEX = regexp.MustCompile(`.+? \[(DEBUG|WARN|ERR|INFO)\] (.*)`)
)

func initMemberList(logLevel logrus.Level) {
	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}

	memberList.registerIOLogger(LOG_MEMBERLIST_SET_NAME, os.Stdout, formatter, logLevel)

	f, err := openLogFile(LOG_MEMBERLIST_FILE)
	if err != nil {
		Errorf("[open log file %s failed: %v]", LOG_MEMBERLIST_FILE, err)
	} else {
		memberList.registerFileLogger(LOG_MEMBERLIST_SET_NAME, f, f, LOG_MEMBERLIST_FILE, formatter, logLevel)
	}
}

func setMemberListLevel(level logrus.Level) {
	loggers := memberList.getLoggers(LOG_MEMBERLIST_SET_NAME)
	for i := range loggers {
		loggers[i].SetLevel(level)
	}
}

func Writer() *io.PipeWriter {
	reader, writer := io.Pipe()

	go writerScanner(reader, func(args ...interface{}) {
		line := fmt.Sprint(args...)
		match := LOG_MEMBERLIST_REGEX.FindStringSubmatch(line)

		for _, l := range memberList.getLoggers("memberlist") {
			if len(match) == 3 {
				output := l.Error
				if match[1] == "DEBUG" {
					output = l.Debug
				} else if match[1] == "WARN" {
					output = l.Warn
				} else if match[1] == "INFO" {
					output = l.Info
				}
				output(fmt.Sprintf("[%s]", match[2]))
			} else {
				l.Error(line)
			}
		}
	})

	runtime.SetFinalizer(writer, writerFinalizer)

	return writer
}

func writerScanner(reader *io.PipeReader, printFunc func(args ...interface{})) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		printFunc(scanner.Text())
	}

	err := scanner.Err()
	if err != nil {
		Errorf("[reading from log writer failed: %v]", err)
	}

	reader.Close()
}

func writerFinalizer(writer *io.PipeWriter) {
	writer.Close()
}
