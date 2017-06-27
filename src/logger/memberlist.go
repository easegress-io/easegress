package logger

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"

	"github.com/sirupsen/logrus"

	"common"
)

var (
	LOG_MEMBERLIST_FILE      = "memberlist.log"
	LOG_MEMBERLIST_LEVEL     = logrus.InfoLevel
	LOG_MEMBERLIST_TTY_LEVEL = logrus.DebugLevel

	memberList = New()

	LOG_MEMBERLIST_REGEX = regexp.MustCompile(`.+? \[(DEBUG|WARN)\] (.*)`)
)

func initMemberList() {
	if common.Stage == "prod" {
		LOG_MEMBERLIST_TTY_LEVEL = logrus.InfoLevel
	}
	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}

	memberList.registerLogger("memberlist", os.Stdout, formatter, LOG_MEMBERLIST_TTY_LEVEL)

	f, err := openLogFile(LOG_MEMBERLIST_FILE)
	if err != nil {
		Errorf("[logger:initCluster] - [open file %v failed: %v]", LOG_MEMBERLIST_FILE, err)
	} else {
		memberList.registerLogger("memberlist", f, formatter, LOG_MEMBERLIST_LEVEL)
	}
}

func Writer() *io.PipeWriter {
	reader, writer := io.Pipe()

	go writerScanner(reader, func(args ...interface{}) {
		line := fmt.Sprint(args...)
		match := LOG_MEMBERLIST_REGEX.FindStringSubmatch(line)

		for _, l := range memberList.getLoggers("memberlist") {
			output := l.Error
			if len(match) == 3 {
				if match[1] == "DEBUG" {
					output = l.Debug
				} else if match[1] == "WARN" {
					output = l.Warn
				}
				output(match[2])
			} else {
				output(line)
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
