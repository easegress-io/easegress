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
	"strings"
	"testing"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/trace"
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
	}
	s := spec.newSampler()
	assert.Equal(trace.NeverSample(), s)

	spec = &Spec{
		SampleRate: 1,
	}
	s = spec.newSampler()
	assert.Equal(trace.AlwaysSample(), s)

	spec = &Spec{
		SampleRate: 0.5,
	}
	s = spec.newSampler()
	assert.Equal(trace.TraceIDRatioBased(0.5), s)
}
