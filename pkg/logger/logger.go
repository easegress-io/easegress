package logger

import (
	"os"
	"path/filepath"
	"time"

	"github.com/megaease/easegateway/pkg/common"
	"github.com/megaease/easegateway/pkg/option"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Init initializes logger.
func Init(opt *option.Options) {
	initDefault(opt)
	initHTTPPlugin(opt)
	initRestAPI(opt)
}

const (
	stdoutFilename           = "stdout.log"
	pluginHTTPAccessFilename = "plugin_http_access.log"
	pluginHTTPDumpFilename   = "plugin_http_dump.log"
	adminAPIFilename         = "admin_api.log"
)

var (
	defaultLogger          *zap.SugaredLogger // equal stderrLogger + gatewayLogger
	stderrLogger           *zap.SugaredLogger
	gatewayLogger          *zap.SugaredLogger
	httpPluginAccessLogger *zap.Logger
	httpPluginDumpLogger   *zap.Logger
	restAPILogger          *zap.Logger
)

func initDefault(opt *option.Options) {
	timeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format(time.RFC3339))
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "", // no need
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "", // no need
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     timeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	lowestLevel := zap.InfoLevel
	if opt.Debug {
		lowestLevel = zap.DebugLevel
	}

	fr, err := newFileReopener(filepath.Join(opt.AbsLogDir, stdoutFilename))
	if err != nil {
		common.Exit(1, err.Error())
	}

	opts := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(1)}

	stderrSyncer := zapcore.AddSync(os.Stderr)
	stderrCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), stderrSyncer, lowestLevel)
	stderrLogger = zap.New(stderrCore, opts...).Sugar()

	gatewaySyncer := zapcore.AddSync(fr)
	gatewayCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), gatewaySyncer, lowestLevel)
	gatewayLogger = zap.New(gatewayCore, opts...).Sugar()

	defaultCore := zapcore.NewTee(gatewayCore, stderrCore)
	defaultLogger = zap.New(defaultCore, opts...).Sugar()
}

func initHTTPPlugin(opt *option.Options) {
	httpPluginAccessLogger = newPlainLogger(opt, pluginHTTPAccessFilename)
	httpPluginDumpLogger = newPlainLogger(opt, pluginHTTPDumpFilename)
}

func initRestAPI(opt *option.Options) {
	restAPILogger = newPlainLogger(opt, adminAPIFilename)
}

func newPlainLogger(opt *option.Options, filename string) *zap.Logger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "",
		LevelKey:      "",
		NameKey:       "",
		CallerKey:     "",
		MessageKey:    "message",
		StacktraceKey: "",
		LineEnding:    zapcore.DefaultLineEnding,
	}

	fr, err := newFileReopener(filepath.Join(opt.AbsLogDir, filename))
	if err != nil {
		common.Exit(1, err.Error())
	}

	syncer := zapcore.AddSync(fr)
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), syncer, zap.DebugLevel)

	return zap.New(core)
}
