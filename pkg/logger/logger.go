/*
 * Copyright (c) 2017, The Easegress Authors
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package logger provides logger for Easegress.
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/zapr"
	"go.opentelemetry.io/otel"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/megaease/easegress/v2/pkg/common"
	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/util/fasttime"
)

func init() {
	globalLogLevel = zap.NewAtomicLevel()
	globalLogLevel.SetLevel(zap.InfoLevel)
}

// Init initializes logger.
func Init(opt *option.Options) {
	if opt.Debug {
		globalLogLevel.SetLevel(zap.DebugLevel)
	}

	initDefault(opt)
	initHTTPFilter(opt)
	initRestAPI(opt)
	initOTel(opt)
}

// InitNop initializes all logger as nop, mainly for unit testing
func InitNop() {
	nop := zap.NewNop()
	httpFilterAccessLogger = nop.Sugar()
	httpFilterDumpLogger = nop.Sugar()
	restAPILogger = nop.Sugar()

	defaultLogger = nop.Sugar()
	gressLogger = defaultLogger
	stderrLogger = defaultLogger
}

// InitMock initializes all logger to print stdout, mainly for unit testing
func InitMock() {
	mock := zap.NewExample()
	httpFilterAccessLogger = mock.Sugar()
	httpFilterDumpLogger = mock.Sugar()
	restAPILogger = mock.Sugar()

	defaultLogger = mock.Sugar()
	gressLogger = defaultLogger
	stderrLogger = defaultLogger
}

const (
	stdoutFilename           = "stdout.log"
	filterHTTPAccessFilename = "filter_http_access.log"
	filterHTTPDumpFilename   = "filter_http_dump.log"
	adminAPIFilename         = "admin_api.log"
	otelFilename             = "otel.log"

	// EtcdClientFilename is the filename of etcd client log.
	EtcdClientFilename = "etcd_client.log"

	// no cache for system log
	systemLogMaxCacheCount = 0

	// NOTE: Under some pressure, it's easy to produce more than 1024 log entries
	// within cacheTimeout(2s), so it's reasonable to flush them at this moment.
	trafficLogMaxCacheCount = 1024
)

var (
	defaultLogger          *zap.SugaredLogger // equal stderrLogger + gressLogger
	stderrLogger           *zap.SugaredLogger
	gressLogger            *zap.SugaredLogger
	httpFilterAccessLogger *zap.SugaredLogger
	httpFilterDumpLogger   *zap.SugaredLogger
	restAPILogger          *zap.SugaredLogger
	globalLogLevel         zap.AtomicLevel

	stdoutLogPath string
)

// SetLogLevel sets log level. Only support debug and info.
func SetLogLevel(level zapcore.Level) {
	globalLogLevel.SetLevel(level)
}

// GetLogLevel returns log level.
func GetLogLevel() string {
	return globalLogLevel.String()
}

// GetLogPath returns the path of stdout log.
func GetLogPath() string {
	return stdoutLogPath
}

// EtcdClientLoggerConfig generates the config of etcd client logger.
func EtcdClientLoggerConfig(opt *option.Options, filename string) *zap.Config {
	encoderConfig := defaultEncoderConfig()

	cfg := &zap.Config{
		Level:            globalLogLevel,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	if opt.AbsLogDir != "" {
		cfg.OutputPaths = []string{common.NormalizeZapLogPath(filepath.Join(opt.AbsLogDir, filename))}
		cfg.ErrorOutputPaths = cfg.OutputPaths
	}
	return cfg
}

func defaultEncoderConfig() zapcore.EncoderConfig {
	timeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(fasttime.Format(t, fasttime.RFC3339Milli))
	}

	return zapcore.EncoderConfig{
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
}

func initDefault(opt *option.Options) {
	encoderConfig := defaultEncoderConfig()

	var err error
	var gressLF io.Writer = os.Stdout
	if opt.AbsLogDir != "" {
		gressLF, err = newLogFile(filepath.Join(opt.AbsLogDir, stdoutFilename), systemLogMaxCacheCount)
		if err != nil {
			common.Exit(1, err.Error())
		}
		stdoutLogPath = gressLF.(*logFile).filename
	}

	opts := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(1)}

	stderrSyncer := zapcore.AddSync(os.Stderr)
	stderrCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), stderrSyncer, globalLogLevel)
	stderrLogger = zap.New(stderrCore, opts...).Sugar()

	gressSyncer := zapcore.AddSync(gressLF)
	gressCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), gressSyncer, globalLogLevel)
	gressLogger = zap.New(gressCore, opts...).Sugar()

	defaultCore := gressCore
	if gressLF != os.Stdout && gressLF != os.Stderr {
		defaultCore = zapcore.NewTee(gressCore, stderrCore)
	}
	defaultLogger = zap.New(defaultCore, opts...).Sugar()
}

func initHTTPFilter(opt *option.Options) {
	httpFilterAccessLogger = newPlainLogger(opt, filterHTTPAccessFilename, trafficLogMaxCacheCount)
	httpFilterDumpLogger = newPlainLogger(opt, filterHTTPDumpFilename, trafficLogMaxCacheCount)
}

func initRestAPI(opt *option.Options) {
	restAPILogger = newPlainLogger(opt, adminAPIFilename, systemLogMaxCacheCount)
}

func initOTel(opt *option.Options) {
	otelLogger := newPlainLogger(opt, otelFilename, trafficLogMaxCacheCount)
	otel.SetLogger(zapr.NewLogger(otelLogger.Desugar()))
}

func newPlainLogger(opt *option.Options, filename string, maxCacheCount uint32) *zap.SugaredLogger {
	if opt.DisableAccessLog {
		return zap.NewNop().Sugar()
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "",
		LevelKey:      "",
		NameKey:       "",
		CallerKey:     "",
		MessageKey:    "message",
		StacktraceKey: "",
		LineEnding:    zapcore.DefaultLineEnding,
	}

	var err error
	var fr io.Writer = os.Stdout
	if opt.AbsLogDir != "" {
		fr, err = newLogFile(filepath.Join(opt.AbsLogDir, filename), maxCacheCount)
		if err != nil {
			common.Exit(1, err.Error())
		}
	}

	syncer := zapcore.AddSync(fr)
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), syncer, zap.DebugLevel)

	return zap.New(core).Sugar()
}

func mustNewPlainLogger(opt *option.Options, filename string, maxCacheCount uint32) *zap.SugaredLogger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "",
		LevelKey:      "",
		NameKey:       "",
		CallerKey:     "",
		MessageKey:    "message",
		StacktraceKey: "",
		LineEnding:    zapcore.DefaultLineEnding,
	}

	var err error
	var fr io.Writer = os.Stdout
	if opt.AbsLogDir != "" {
		fr, err = newLogFile(filepath.Join(opt.AbsLogDir, filename), maxCacheCount)
		if err != nil {
			panic(fmt.Errorf("new log file %s failed: %w", filename, err))
		}
	}

	syncer := zapcore.AddSync(fr)
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), syncer, globalLogLevel) // use global log level

	return zap.New(core).Sugar()
}
