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

package tracing

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
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
	span := tracer.NewSpanForHTTP(stdr.Context(), "testSpan", stdr)
	assert.Nil(span.cdnSpan)

	stdr.Header.Set(cfRayHeader, "792a875b68972ab9-ndm")
	span = tracer.NewSpanForHTTP(stdr.Context(), "testSpan", stdr)
	assert.Nil(span.cdnSpan)

	stdr.Header.Set(cfSecHeader, "1675751394")
	span = tracer.NewSpanForHTTP(stdr.Context(), "testSpan", stdr)
	assert.Nil(span.cdnSpan)

	stdr.Header.Set(cfMsecHeader, "876")
	span = tracer.NewSpanForHTTP(stdr.Context(), "testSpan", stdr)
	assert.NotNil(span.cdnSpan)
}

func TestSpecUnmarshalJson(t *testing.T) {
	assert := assert.New(t)

	jsonSpec := &Spec{
		ServiceName: "test",
		Attributes:  map[string]string{"k": "v"},
		SpanLimits:  nil,
		SampleRate:  0.5,
		BatchLimits: nil,
		Exporter: &ExporterSpec{
			Zipkin: &ZipkinSpec{Endpoint: "http://localhost:2181"},
		},
	}
	specData, err := json.Marshal(jsonSpec)
	assert.Nil(err)

	spec := &Spec{}
	err = spec.UnmarshalJSON(specData)
	assert.Nil(err)
	assert.Equal(jsonSpec.SampleRate, spec.SampleRate)
	assert.Equal(jsonSpec.ServiceName, spec.ServiceName)
}

func TestSpecValidate(t *testing.T) {
	assert := assert.New(t)

	exporter := &ExporterSpec{}
	zipkin := &ZipkinDeprecatedSpec{}
	tags := map[string]string{"k": "v"}
	attrs := map[string]string{"k": "v"}

	spec := &Spec{}
	spec.Exporter = nil
	spec.Zipkin = nil
	assert.NotNil(spec.Validate())

	spec.Exporter = exporter
	spec.Zipkin = zipkin
	assert.NotNil(spec.Validate())

	spec.Exporter = exporter
	spec.Zipkin = nil
	assert.Nil(spec.Validate())

	spec.Exporter = nil
	spec.Zipkin = zipkin
	assert.Nil(spec.Validate())

	spec.Tags = tags
	spec.Attributes = attrs
	assert.NotNil(spec.Validate())

	spec.Tags = nil
	spec.Attributes = nil
	assert.Nil(spec.Validate())

	spec.Tags = tags
	spec.Attributes = nil
	assert.Nil(spec.Validate())

	spec.Tags = nil
	spec.Attributes = attrs
	assert.Nil(spec.Validate())
}

func TestExporterSpec(t *testing.T) {
	assert := assert.New(t)

	s := &ExporterSpec{}
	assert.NotNil(s.Validate())

	s = (*ExporterSpec)(nil)
	assert.NotNil(s.Validate())

	s = &ExporterSpec{
		Zipkin: &ZipkinSpec{
			Endpoint: "http://localhost:2181",
		},
	}
	assert.Nil(s.Validate())
}

func TestJaegerSpecValidate(t *testing.T) {
	assert := assert.New(t)

	s := &JaegerSpec{}
	assert.NotNil(s.Validate())

	s.Endpoint = "http://localhost:2181"
	assert.Nil(s.Validate())

	s.Mode = jaegerModeAgent
	s.Endpoint = "localhost"
	assert.NotNil(s.Validate())

	s.Mode = jaegerModeAgent
	s.Endpoint = "localhost:2181"
	assert.Nil(s.Validate())
}

func TestSpecEdgeCases(t *testing.T) {
	assert := assert.New(t)

	spec := &Spec{
		SpanLimits: &SpanLimitsSpec{
			AttributeValueLengthLimit:   -1,
			AttributeCountLimit:         256,
			EventCountLimit:             256,
			LinkCountLimit:              256,
			AttributePerEventCountLimit: 256,
			AttributePerLinkCountLimit:  256,
		},
	}
	spanLimits := spec.newSpanLimits()
	assert.EqualValues(-1, spanLimits.AttributeValueLengthLimit)
	assert.EqualValues(256, spanLimits.AttributeCountLimit)
	assert.EqualValues(256, spanLimits.EventCountLimit)
	assert.EqualValues(256, spanLimits.LinkCountLimit)
	assert.EqualValues(256, spanLimits.AttributePerEventCountLimit)
	assert.EqualValues(256, spanLimits.AttributePerLinkCountLimit)

	spec.Exporter = nil
	spec.Zipkin = &ZipkinDeprecatedSpec{}
	processor, err := spec.newBatchSpanProcessors()
	assert.NotNil(processor)
	assert.Nil(err)

	spec.BatchLimits = &BatchLimitsSpec{
		MaxQueueSize:       1000,
		BatchTimeout:       1000,
		ExportTimeout:      1000,
		MaxExportBatchSize: 1000,
	}
	processor, err = spec.newBatchSpanProcessors()
	assert.NotNil(processor)
	assert.Nil(err)

	prop := spec.newPropagator()
	assert.NotNil(prop)
}

func TestEdgeCases(t *testing.T) {
	assert := assert.New(t)
	{
		s := JaegerSpec{}
		e, err := s.newExporter()
		assert.NotNil(e)
		assert.Nil(err)
	}

	{
		s := JaegerSpec{}
		s.Mode = jaegerModeAgent
		s.Endpoint = "localhost:2181"
		e, err := s.newExporter()
		assert.NotNil(e)
		assert.Nil(err)
	}

	{
		s := ZipkinDeprecatedSpec{}
		e, err := s.newExporter()
		assert.NotNil(e)
		assert.Nil(err)
	}

	{
		s := &OTLPSpec{
			Compression: "gzip",
		}
		e, err := s.newExporter()
		assert.NotNil(e)
		assert.Nil(err)
	}

	{
		s := &OTLPSpec{
			Protocol:    otlpProtocolGRPC,
			Endpoint:    "localhost:2181",
			Insecure:    true,
			Compression: "gzip",
		}
		e, err := s.newExporter()
		assert.NotNil(e)
		assert.Nil(err)
	}

	{
		t := &Tracer{
			Tracer:     trace.NewNoopTracerProvider().Tracer("noop"),
			propagator: propagation.TraceContext{},
		}
		span := t.NewSpan(context.Background(), "test")
		assert.NotNil(span)

		childSpan := span.NewChildWithStart("test1", time.Now())
		assert.NotNil(childSpan)

		req, err := http.NewRequest("GET", "http://localhost:2181", nil)
		assert.Nil(err)
		span.InjectHTTP(req)
		t.Close()
	}

	{
		span := NoopTracer.NewSpan(context.Background(), "test")
		assert.NotNil(span)
		assert.Equal(NoopSpan, span)
		assert.Equal(NoopTracer, span.Tracer())

		childSpan := span.NewChildWithStart("test1", time.Now())
		assert.NotNil(childSpan)
		assert.Equal(NoopSpan, childSpan)
		childSpan.End()
	}
}
