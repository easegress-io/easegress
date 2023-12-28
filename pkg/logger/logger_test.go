package logger

import (
	"os"
	"testing"

	"github.com/megaease/easegress/v2/pkg/option"
)

func TestMustPlainLogger(t *testing.T) {
	opt := &option.Options{
		AbsLogDir: "/tmp",
	}
	defer func() {
		if rv := recover(); rv != nil {
			t.Errorf("mustPlainLogger() panic: %v", rv)
		}
	}()

	MustNewPlainLogger(opt, "test.log", 1)
	_, err := os.Stat("/tmp/test.log")
	if err == nil {
		return
	}

	if os.IsNotExist(err) {
		t.Errorf("mustPlainLogger() failed: %v", err)
	}
}

func TestMustPlainLoggerPanic(t *testing.T) {
	opt := &option.Options{
		AbsLogDir: "/",
	}
	defer func() {
		if rv := recover(); rv != nil {
			t.Logf("mustPlainLogger() panic: %v", rv)
		}
	}()

	MustNewPlainLogger(opt, "test.log", 1)
	_, err := os.Stat("/test.log")
	if err == nil {
		return
	}

	if os.IsNotExist(err) {
		t.Errorf("mustPlainLogger() failed: %v", err)
	}
}

func TestMustPlainLoggerWrite(t *testing.T) {
	opt := &option.Options{
		AbsLogDir: "/tmp/",
	}
	defer func() {
		if rv := recover(); rv != nil {
			t.Errorf("mustPlainLogger() panic: %v", rv)
		}
	}()

	l := MustNewPlainLogger(opt, "test.log", 0)
	l.Errorf("test error")
	l.Info("test info")
	l.Debug("test debug")
	l.Sync()
	t.Logf("mustPlainLogger() success")
}
