/*
 * Copyright (c) 2017, MegaEase
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

package logger

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/megaease/easegress/pkg/common"
	"github.com/megaease/easegress/pkg/option"
	"github.com/megaease/easegress/pkg/util/timetool"
)

// Init initializes logger.
func Init(opt *option.Options) {
	initDefault(opt)
	initHTTPFilter(opt)
	initRestAPI(opt)
}

// InitNop initializes all logger as nop, mainly for unit testing
func InitNop() {
	nop := zap.NewNop()
	httpFilterAccessLogger = nop
	httpFilterDumpLogger = nop
	restAPILogger = nop

	defaultLogger = nop.Sugar()
	gressLogger = defaultLogger
	stderrLogger = defaultLogger
}

const (
	stdoutFilename           = "stdout.log"
	filterHTTPAccessFilename = "filter_http_access.log"
	filterHTTPDumpFilename   = "filter_http_dump.log"
	adminAPIFilename         = "admin_api.log"

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
	httpFilterAccessLogger *zap.Logger
	httpFilterDumpLogger   *zap.Logger
	restAPILogger          *zap.Logger
)

// EtcdClientLoggerConfig generates the config of etcd client logger.
func EtcdClientLoggerConfig(opt *option.Options, filename string) *zap.Config {
	encoderConfig := defaultEncoderConfig()

	level := zap.NewAtomicLevel()
	level.SetLevel(zapcore.DebugLevel)

	outputPaths := []string{common.NormalizeZapLogPath(filepath.Join(opt.AbsLogDir, filename))}

	return &zap.Config{
		Level:            level,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      outputPaths,
		ErrorOutputPaths: outputPaths,
	}
}

func defaultEncoderConfig() zapcore.EncoderConfig {
	timeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format(timetool.RFC3339Milli))
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

	lowestLevel := zap.InfoLevel
	if opt.Debug {
		lowestLevel = zap.DebugLevel
	}

	lf, err := newLogFile(filepath.Join(opt.AbsLogDir, stdoutFilename), systemLogMaxCacheCount)
	if err != nil {
		common.Exit(1, err.Error())
	}

	opts := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(1)}

	stderrSyncer := zapcore.AddSync(os.Stderr)
	stderrCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), stderrSyncer, lowestLevel)
	stderrLogger = zap.New(stderrCore, opts...).Sugar()

	gatewaySyncer := zapcore.AddSync(lf)
	gatewayCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), gatewaySyncer, lowestLevel)
	gressLogger = zap.New(gatewayCore, opts...).Sugar()

	defaultCore := zapcore.NewTee(gatewayCore, stderrCore)
	defaultLogger = zap.New(defaultCore, opts...).Sugar()
}

func initHTTPFilter(opt *option.Options) {
	httpFilterAccessLogger = newPlainLogger(opt, filterHTTPAccessFilename, trafficLogMaxCacheCount)
	httpFilterDumpLogger = newPlainLogger(opt, filterHTTPDumpFilename, trafficLogMaxCacheCount)
}

func initRestAPI(opt *option.Options) {
	restAPILogger = newPlainLogger(opt, adminAPIFilename, systemLogMaxCacheCount)
}

func newPlainLogger(opt *option.Options, filename string, maxCacheCount uint32) *zap.Logger {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "",
		LevelKey:      "",
		NameKey:       "",
		CallerKey:     "",
		MessageKey:    "message",
		StacktraceKey: "",
		LineEnding:    zapcore.DefaultLineEnding,
	}

	fr, err := newLogFile(filepath.Join(opt.AbsLogDir, filename), maxCacheCount)
	if err != nil {
		common.Exit(1, err.Error())
	}

	syncer := zapcore.AddSync(fr)
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), syncer, zap.DebugLevel)

	return zap.New(core)
}
