package tracing

import (
	"github.com/megaease/easegress/pkg/logger"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/sdk/trace"
	"strings"
	"testing"
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
			Kind:   exporterKindZipkin,
			Zipkin: &ZipkinSpec{CollectorURL: "http://localhost:2181"},
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
			Kind:   exporterKindZipkin,
			Zipkin: &ZipkinSpec{CollectorURL: "http://localhost:2181"},
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
