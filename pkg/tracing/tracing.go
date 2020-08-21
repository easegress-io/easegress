package tracing

import (
	"io"

	"github.com/megaease/easegateway/pkg/tracing/zipkin"

	opentracing "github.com/opentracing/opentracing-go"
)

type (
	// Spec describes Tracing.
	Spec struct {
		ServiceName string `yaml:"serviceName" jsonschema:"required"`

		Zipkin *zipkin.Spec `yaml:"zipkin" jsonschema:"omitempty"`
	}

	// Tracing is the tracing.
	Tracing struct {
		opentracing.Tracer

		closer io.Closer
	}

	noopCloser struct{}
)

var (
	// NoopTracing is the tracing doing nothing.
	NoopTracing = &Tracing{
		Tracer: opentracing.NoopTracer{},
		closer: nil,
	}
)

// New creates a Tracing.
func New(spec *Spec) (*Tracing, error) {
	if spec == nil {
		return NoopTracing, nil
	}

	tracer, closer, err := zipkin.New(spec.ServiceName, spec.Zipkin)
	if err != nil {
		return nil, err
	}

	return &Tracing{
		Tracer: tracer,
		closer: closer,
	}, nil
}

// Close closes Tracing.
func (t *Tracing) Close() error {
	if t.closer != nil {
		return t.closer.Close()
	}

	return nil
}
