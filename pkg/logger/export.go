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

package logger

import (
	"net/url"
	"time"

	"github.com/megaease/easegress/v2/pkg/option"
	"github.com/megaease/easegress/v2/pkg/util/fasttime"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
	"github.com/openzipkin/zipkin-go/model"
	"go.uber.org/zap"
)

// MustNewPlainLogger creates a plain logger, it panics if any error occurs.
func MustNewPlainLogger(opt *option.Options, filename string, maxCacheCount uint32) *zap.SugaredLogger {
	l := mustNewPlainLogger(opt, filename, maxCacheCount)
	lh.register(filename, l)

	return l
}

type lazyLogBuilder struct {
	fn func() string
}

func (llb lazyLogBuilder) String() string {
	return llb.fn()
}

// Debugf is the wrapper of default logger Debugf.
func Debugf(template string, args ...interface{}) {
	defaultLogger.Debugf(template, args...)
}

// LazyDebug logs debug log in lazy mode. if debug log is disabled by configuration,
// it skips the the built of log message to improve performance
func LazyDebug(fn func() string) {
	defaultLogger.Debug(lazyLogBuilder{fn})
}

// Infof is the wrapper of default logger Infof.
func Infof(template string, args ...interface{}) {
	defaultLogger.Infof(template, args...)
}

// Warnf is the wrapper of default logger Warnf.
func Warnf(template string, args ...interface{}) {
	defaultLogger.Warnf(template, args...)
}

// Errorf is the wrapper of default logger Errorf.
func Errorf(template string, args ...interface{}) {
	defaultLogger.Errorf(template, args...)
}

// Sync syncs all logs, must be called after calling Init().
func Sync() {
	defaultLogger.Sync()
	stderrLogger.Sync()
	gressLogger.Sync()
	httpFilterAccessLogger.Sync()
	httpFilterDumpLogger.Sync()
	restAPILogger.Sync()

	lh.sync()
}

// APIAccess logs admin api log.
func APIAccess(
	method, remoteAddr, path string,
	code int,
	bodyBytedReceived, bodyBytesSent int64,
	requestTime time.Time,
	processTime time.Duration) {

	restAPILogger.Debugf("%s %s %s %v rx:%dB tx:%dB start:%v process:%v",
		method, remoteAddr, path, code,
		bodyBytedReceived, bodyBytesSent,
		fasttime.Format(requestTime, fasttime.RFC3339), processTime)
}

// HTTPAccess logs http access log.
func HTTPAccess(template string, args ...interface{}) {
	httpFilterAccessLogger.Debugf(template, args...)
}

// LazyHTTPAccess logs http access log in lazy mode, if http access log is disabled
// by configuration, it skips the the built of log message to improve performance
func LazyHTTPAccess(fn func() string) {
	httpFilterAccessLogger.Debug(lazyLogBuilder{fn})
}

// NginxHTTPAccess is DEPRECATED, replaced by HTTPAccess.
func NginxHTTPAccess(remoteAddr, proto, method, path, referer, agent, realIP string,
	code int, bodyBytesSent int64,
	requestTime time.Duration, upstreamResponseTime time.Duration,
	upstreamAddr string, upstreamCode int, clientWriteBodyTime, clientReadBodyTime,
	routeTime time.Duration) {
	// mock nginx log_format:
	// '$remote_addr - $remote_user [$time_local] "$request" '
	// '$status $body_bytes_sent "$http_referer" '
	// '"$http_user_agent" "$http_x_forwarded_for" '
	// '$request_time $upstream_response_time $upstream_addr $upstream_status $pipe '
	// '$client_write_body_time' '$client_read_body_time' '$route_time';

	if referer == "" {
		referer = "-"
	}

	if agent == "" {
		agent = "-"
	} else {
		if a, err := url.QueryUnescape(agent); err == nil {
			agent = a
		}
	}

	if realIP == "" {
		realIP = "-"
	}

	if upstreamAddr == "" {
		upstreamAddr = "-"
	} else {
		if addr, err := url.QueryUnescape(upstreamAddr); err == nil {
			upstreamAddr = addr
		}
	}

	httpFilterAccessLogger.Debugf(`%v - - [%v] "%s %s %s" `+
		`%v %v "%s" `+
		`"%s" "%s" `+
		`%f %f %v %v . `+
		`%f %f %f`,
		remoteAddr, fasttime.Now().Local(), method, path, proto,
		code, bodyBytesSent, referer,
		agent, realIP,
		requestTime.Seconds(), upstreamResponseTime.Seconds(), upstreamAddr, upstreamCode,
		clientWriteBodyTime.Seconds(), clientReadBodyTime.Seconds(), routeTime.Seconds())
}

func getSpanTemplate(context *model.SpanContext, template string) string {
	if context == nil {
		return stringtool.Cat("tid=<nil> sid=<nil> ", template)
	}
	return stringtool.Cat("tid=", context.TraceID.String(), " sid=", context.ID.String(), " ", template)
}

// SpanDebugf is the wrapper of default logger Debugf to log tracing message
func SpanDebugf(context *model.SpanContext, template string, args ...interface{}) {
	temp := getSpanTemplate(context, template)
	defaultLogger.Debugf(temp, args...)
}

// SpanErrorf is the wrapper of default logger Debugf to log tracing message
func SpanErrorf(context *model.SpanContext, template string, args ...interface{}) {
	temp := getSpanTemplate(context, template)
	defaultLogger.Errorf(temp, args...)
}
