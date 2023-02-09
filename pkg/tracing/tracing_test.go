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

package tracing

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func init() {
	logger.InitNop()
}

func TestNew(t *testing.T) {
	assert := assert.New(t)

	tracer, err := New(nil)
	assert.Equal(NoopTracer, tracer)
	assert.Nil(err)

	spec := &Spec{
		ServiceName: "test",
		Attributes:  map[string]string{"k": "v"},
		SpanLimits:  nil,
		SampleRate:  0.5,
		BatchLimits: nil,
		Exporter: &ExporterSpec{
			Zipkin: &ZipkinSpec{Endpoint: "http://localhost:2181"},
		},
	}

	tracer, err = New(spec)
	assert.NotEqual(NoopTracer, tracer)
	assert.Nil(err)
	assert.NotNil(tracer.tp)
	assert.NotNil(tracer.Tracer)
}

func TestNewResource(t *testing.T) {
	assert := assert.New(t)

	spec := &Spec{
		ServiceName: "test",
		Attributes:  map[string]string{"k": "v"},
		SpanLimits:  nil,
		SampleRate:  0.5,
		BatchLimits: nil,
		Exporter: &ExporterSpec{
			Zipkin: &ZipkinSpec{Endpoint: "http://localhost:2181"},
		},
	}
	r, err := spec.newResource()

	assert.Nil(err)
	assert.True(strings.Contains(r.String(), "k=v,service.name=test"))
}

func TestNewSampler(t *testing.T) {
	assert := assert.New(t)

	spec := &Spec{
		SampleRate: 0,
		Exporter: &ExporterSpec{
			Zipkin: &ZipkinSpec{Endpoint: "http://localhost:2181"},
		},
	}
	s := spec.newSampler()
	assert.Equal(sdktrace.NeverSample(), s)

	spec = &Spec{
		SampleRate: 1,
		Exporter: &ExporterSpec{
			Zipkin: &ZipkinSpec{Endpoint: "http://localhost:2181"},
		},
	}
	s = spec.newSampler()
	assert.Equal(sdktrace.AlwaysSample(), s)

	spec = &Spec{
		SampleRate: 0.5,
		Exporter: &ExporterSpec{
			Zipkin: &ZipkinSpec{Endpoint: "http://localhost:2181"},
		},
	}
	s = spec.newSampler()
	assert.Equal(sdktrace.TraceIDRatioBased(0.5), s)
}

func TestNewSpanWithStart(t *testing.T) {
	assert := assert.New(t)

	spec := &Spec{
		ServiceName: "test",
		Attributes:  map[string]string{"k": "v"},
		SpanLimits:  nil,
		SampleRate:  0.5,
		BatchLimits: nil,
		Exporter: &ExporterSpec{
			Zipkin: &ZipkinSpec{Endpoint: "http://localhost:2181"},
		},
	}

	tracer, err := New(spec)
	assert.Nil(err)

	stdr, _ := http.NewRequest(http.MethodGet, "http://www.megaease.com/.well-known/acme-challenge/abc", http.NoBody)
	httpContext := InjectTraceInfo(stdr.Context(), stdr)
	span := tracer.NewSpanWithStart(httpContext, "testSpan", time.Now())
	assert.Nil(span.cdnSpan)

	stdr.Header.Set(cfRayHeader, "792a875b68972ab9-ndm")
	httpContext = InjectTraceInfo(stdr.Context(), stdr)
	span = tracer.NewSpanWithStart(httpContext, "testSpan", time.Now())
	assert.Nil(span.cdnSpan)

	stdr.Header.Set(cfSecHeader, "1675751394")
	httpContext = InjectTraceInfo(stdr.Context(), stdr)
	span = tracer.NewSpanWithStart(httpContext, "testSpan", time.Now())
	assert.Nil(span.cdnSpan)
	cfRayID, ok := span.ctx.Value(cfRayHeader).(string)
	assert.True(ok)
	assert.Equal("792a875b68972ab9-ndm", cfRayID)
	_, ok = span.ctx.Value("cf-timestamp").(int64)
	assert.False(ok)

	stdr.Header.Set(cfMsecHeader, "876")
	httpContext = InjectTraceInfo(stdr.Context(), stdr)
	span = tracer.NewSpanWithStart(httpContext, "testSpan", time.Now())
	ts, ok := span.ctx.Value(cfTs).(int64)
	assert.True(ok)
	assert.Equal(int64(1675751394876), ts)
	assert.NotNil(span.cdnSpan)
}
