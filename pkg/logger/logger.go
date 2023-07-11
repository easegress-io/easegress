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

// Package logger provides logger for Easegress.
package logger

import (
	"github.com/megaease/easegress/pkg/util/codectool"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/zapr"
	"go.opentelemetry.io/otel"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/megaease/easegress/pkg/common"
	"github.com/megaease/easegress/pkg/option"
	"github.com/megaease/easegress/pkg/util/fasttime"
)

// Init initializes logger.
func Init(opt *option.Options) {
	initLogsSpec(opt)
	initDefault(opt)
	initHTTPFilter(opt)
	initRestAPI(opt)
	initOTel(opt)
}

func initLogsSpec(opt *option.Options) *LogsSpec {
	if opt.LogConfig != "" {
		data, err := os.ReadFile(opt.LogConfig)
		if err != nil {
			common.Exit(1, err.Error())
		}
		codectool.MustUnmarshal(data, defaultLogsConfig)
	}
	return defaultLogsConfig
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
	// no cache for system log
	systemLogMaxCacheCount = 0

	// NOTE: Under some pressure, it's easy to produce more than 1024 log entries
	// within cacheTimeout(2s), so it's reasonable to flush them at this moment.
	trafficLogMaxCacheCount = 1024
)

type LogsSpec struct {
	// StdLog is the file to write logs from stdout to.
	StdLog *Spec `json:"stdLog" jsonschema:"omitempty"`
	// Access is the file to write logs from access to.
	Access *Spec `json:"accessLog" jsonschema:"omitempty"`
	// Dump is the file to write logs from mem dump to.
	Dump *Spec `json:"dumpLog" jsonschema:"omitempty"`
	//AdminAPI is the file to write logs of api called to.
	AdminAPI *Spec `json:"adminAPLog" jsonschema:"omitempty"`
	// EtcdServer is the file to write logs of etcd server to.
	EtcdServer *Spec `json:"etcdServerLog" jsonschema:"omitempty"`
	// EtcdClient is the file to write logs of etcd client to.
	EtcdClient *Spec `json:"etcdClientLog" jsonschema:"omitempty"`
	// OTel is the file to write logs of etcd client to.
	OTel *Spec `json:"oTel" jsonschema:"omitempty"`
}

var (
	defaultLogger          *zap.SugaredLogger // equal stderrLogger + gressLogger
	stderrLogger           *zap.SugaredLogger
	gressLogger            *zap.SugaredLogger
	httpFilterAccessLogger *zap.SugaredLogger
	httpFilterDumpLogger   *zap.SugaredLogger
	restAPILogger          *zap.SugaredLogger
	defaultLogsConfig      = &LogsSpec{
		StdLog: &Spec{
			FileName:   "stdout.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		Access: &Spec{
			FileName:   "filter_http_access.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		Dump: &Spec{
			FileName:   "filter_http_dump.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		AdminAPI: &Spec{
			FileName:   "admin-api.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		EtcdServer: &Spec{
			FileName:   "etcd-server-eg.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		EtcdClient: &Spec{
			FileName:   "etcd-client.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
		OTel: &Spec{
			FileName:   "otel.log",
			MaxSize:    128,
			MaxAge:     30,
			MaxBackups: 60,
			LocalTime:  true,
			Compress:   true,
		},
	}
)

func DefaultEtcdServerLogger(opt *option.Options) *zap.Logger {
	encoderConfig := defaultEncoderConfig()

	level := zap.NewAtomicLevel()
	if opt.Debug {
		level.SetLevel(zapcore.DebugLevel)
	} else {
		level.SetLevel(zapcore.InfoLevel)
	}
	defaultLogsConfig.EtcdServer.FileName = filepath.Join(opt.AbsLogDir, defaultLogsConfig.EtcdServer.FileName)
	gressLF, err := NewLogFile(defaultLogsConfig.EtcdServer, systemLogMaxCacheCount)
	if err != nil {
		panic(err)
	}
	syncer := zapcore.AddSync(gressLF)
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), syncer, level)
	return zap.New(core)
}

// DefaultEtcdClientLogger generates default etcd client logger.
func DefaultEtcdClientLogger(opt *option.Options) *zap.Logger {
	return etcdClientLogger(opt, defaultLogsConfig.EtcdClient)
}

// CustomerEtcdClientLogger generates customer etcd client logger.
func CustomerEtcdClientLogger(opt *option.Options, fileName string) *zap.Logger {
	return etcdClientLogger(opt, &Spec{
		FileName:   filepath.Join(opt.AbsLogDir, fileName),
		MaxSize:    defaultLogsConfig.EtcdClient.MaxSize,
		MaxBackups: defaultLogsConfig.EtcdClient.MaxBackups,
		MaxAge:     defaultLogsConfig.EtcdClient.MaxAge,
		Compress:   defaultLogsConfig.EtcdClient.Compress,
		LocalTime:  defaultLogsConfig.EtcdClient.LocalTime,
		Perm:       defaultLogsConfig.EtcdClient.Perm,
	})
}

func etcdClientLogger(opt *option.Options, spec *Spec) *zap.Logger {
	encoderConfig := defaultEncoderConfig()

	level := zap.NewAtomicLevel()
	if opt.Debug {
		level.SetLevel(zapcore.DebugLevel)
	} else {
		level.SetLevel(zapcore.InfoLevel)
	}
	spec.FileName = filepath.Join(opt.AbsLogDir, spec.FileName)
	gressLF, err := NewLogFile(spec, systemLogMaxCacheCount)
	if err != nil {
		panic(err)
	}
	syncer := zapcore.AddSync(gressLF)
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), syncer, level)
	return zap.New(core)
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

	lowestLevel := zap.InfoLevel
	if opt.Debug {
		lowestLevel = zap.DebugLevel
	}

	var err error
	var gressLF io.Writer = os.Stdout
	if opt.AbsLogDir != "" {
		defaultLogsConfig.StdLog.FileName = filepath.Join(opt.AbsLogDir, defaultLogsConfig.StdLog.FileName)
		gressLF, err = NewLogFile(defaultLogsConfig.StdLog, systemLogMaxCacheCount)
		if err != nil {
			common.Exit(1, err.Error())
		}
	}

	opts := []zap.Option{zap.AddCaller(), zap.AddCallerSkip(1)}

	stderrSyncer := zapcore.AddSync(os.Stderr)
	stderrCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), stderrSyncer, lowestLevel)
	stderrLogger = zap.New(stderrCore, opts...).Sugar()

	gressSyncer := zapcore.AddSync(gressLF)
	gressCore := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), gressSyncer, lowestLevel)
	gressLogger = zap.New(gressCore, opts...).Sugar()

	defaultCore := gressCore
	if gressLF != os.Stdout && gressLF != os.Stderr {
		defaultCore = zapcore.NewTee(gressCore, stderrCore)
	}
	defaultLogger = zap.New(defaultCore, opts...).Sugar()
}

func initHTTPFilter(opt *option.Options) {
	httpFilterAccessLogger = newPlainLogger(opt, defaultLogsConfig.Access, trafficLogMaxCacheCount)
	httpFilterDumpLogger = newPlainLogger(opt, defaultLogsConfig.Dump, trafficLogMaxCacheCount)
}

func initRestAPI(opt *option.Options) {
	restAPILogger = newPlainLogger(opt, defaultLogsConfig.AdminAPI, systemLogMaxCacheCount)
}

func initOTel(opt *option.Options) {
	otelLogger := newPlainLogger(opt, defaultLogsConfig.OTel, trafficLogMaxCacheCount)
	otel.SetLogger(zapr.NewLogger(otelLogger.Desugar()))
}

func newPlainLogger(opt *option.Options, spec *Spec, maxCacheCount uint32) *zap.SugaredLogger {
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
		spec.FileName = filepath.Join(opt.AbsLogDir, spec.FileName)
		fr, err = NewLogFile(spec, maxCacheCount)
		if err != nil {
			common.Exit(1, err.Error())
		}
	}

	syncer := zapcore.AddSync(fr)
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), syncer, zap.DebugLevel)

	return zap.New(core).Sugar()
}
