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
	"context"
	"fmt"
	"github.com/megaease/easegress/pkg/util/fasttime"
	"github.com/megaease/easegress/pkg/util/stringtool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"time"
)

type (
	// Spec describes Tracer.
	Spec struct {
		ServiceName string            `json:"serviceName" jsonschema:"required"`
		Attributes  map[string]string `json:"attributes" jsonschema:"omitempty"`
		SpanLimits  *SpanLimitsSpec   `json:"spanLimits" jsonschema:"omitempty"`
		SampleRate  float64           `json:"sampleRate" jsonschema:"required,minimum=0,maximum=1"`
		BatchLimits *BatchLimitsSpec  `json:"batchLimits" jsonschema:"omitempty"`
		Exporter    *ExporterSpec     `json:"exporter" jsonschema:"required"`
	}

	// SpanLimitsSpec represents the limits of a span.
	SpanLimitsSpec struct {
		// AttributeValueLengthLimit is the maximum allowed attribute value length.
		//
		// This limit only applies to string and string slice attribute values.
		// Any string longer than this value will be truncated to this length.
		//
		// Setting this to a negative value means no limit is applied.
		AttributeValueLengthLimit int `json:"attributeValueLengthLimit" jsonschema:"default=-1,omitempty"`

		// AttributeCountLimit is the maximum allowed span attribute count. Any
		// attribute added to a span once this limit is reached will be dropped.
		//
		// Setting this to zero means no attributes will be recorded.
		//
		// Setting this to a negative value means no limit is applied.
		AttributeCountLimit int `json:"attributeCountLimit" jsonschema:"default=128,omitempty"`

		// EventCountLimit is the maximum allowed span event count. Any event
		// added to a span once this limit is reached means it will be added but
		// the oldest event will be dropped.
		//
		// Setting this to zero means no events we be recorded.
		//
		// Setting this to a negative value means no limit is applied.
		EventCountLimit int `json:"eventCountLimit" jsonschema:"default=128,omitempty"`

		// LinkCountLimit is the maximum allowed span link count. Any link added
		// to a span once this limit is reached means it will be added but the
		// oldest link will be dropped.
		//
		// Setting this to zero means no links we be recorded.
		//
		// Setting this to a negative value means no limit is applied.
		LinkCountLimit int `json:"linkCountLimit" jsonschema:"default=128,omitempty"`

		// AttributePerEventCountLimit is the maximum number of attributes allowed
		// per span event. Any attribute added after this limit reached will be
		// dropped.
		//
		// Setting this to zero means no attributes will be recorded for events.
		//
		// Setting this to a negative value means no limit is applied.
		AttributePerEventCountLimit int `json:"attributePerEventCountLimit" jsonschema:"default=128,omitempty"`

		// AttributePerLinkCountLimit is the maximum number of attributes allowed
		// per span link. Any attribute added after this limit reached will be
		// dropped.
		//
		// Setting this to zero means no attributes will be recorded for links.
		//
		// Setting this to a negative value means no limit is applied.
		AttributePerLinkCountLimit int `json:"attributePerLinkCountLimit" jsonschema:"default=128,omitempty"`
	}

	// BatchLimitsSpec describes BatchSpanProcessorOptions.
	BatchLimitsSpec struct {
		// MaxQueueSize is the maximum queue size to buffer spans for delayed processing. If the
		// queue gets full it drops the spans. Use BlockOnQueueFull to change this behavior.
		// The default value of MaxQueueSize is 2048.
		MaxQueueSize int `json:"maxQueueSize" jsonschema:"default=2048,omitempty"`

		// BatchTimeout is the maximum duration for constructing a batch. Processor
		// forcefully sends available spans when timeout is reached.
		// The default value of BatchTimeout is 5000 msec.
		BatchTimeout time.Duration `json:"batchTimeout" jsonschema:"default=5000,omitempty"`

		// ExportTimeout specifies the maximum duration for exporting spans. If the timeout
		// is reached, the export will be cancelled.
		// The default value of ExportTimeout is 30000 msec.
		ExportTimeout time.Duration `json:"exportTimeout" jsonschema:"default=30000,omitempty"`

		// MaxExportBatchSize is the maximum number of spans to process in a single batch.
		// If there are more than one batch worth of spans then it processes multiple batches
		// of spans one batch after the other without any delay.
		// The default value of MaxExportBatchSize is 512.
		MaxExportBatchSize int `json:"maxExportBatchSize" jsonschema:"default=512,omitempty"`
	}

	exporterKind string

	// ExporterSpec describes exporter.
	ExporterSpec struct {
		Kind   exporterKind `json:"kind" jsonschema:"required,enum=jaeger,enum=zipkin,enum=otlp"`
		Jaeger *JaegerSpec  `json:"jaeger" jsonschema:"omitempty"`
		Zipkin *ZipkinSpec  `json:"zipkin" jsonschema:"omitempty"`
		OTLP   *OTLPSpec    `json:"otlp" jsonschema:"omitempty"`
	}

	jaegerMode string

	// JaegerSpec describes Jaeger.
	JaegerSpec struct {
		Mode      jaegerMode `json:"mode" jsonschema:"required,enum=agent,enum=collector"`
		AgentHost string     `json:"agentHost" jsonschema:"omitempty"`
		AgentPort string     `json:"agentPort" jsonschema:"omitempty"`

		Endpoint string `json:"endpoint" jsonschema:"omitempty"`
		Username string `json:"username" jsonschema:"omitempty"`
		Password string `json:"password" jsonschema:"omitempty"`
	}

	// ZipkinSpec describes Zipkin.
	ZipkinSpec struct {
		CollectorURL string `json:"collectorURL" jsonschema:"required,format=url"`
	}

	otlpMode string
	// OTLPSpec describes OpenTelemetry exporter.
	OTLPSpec struct {
		Mode        otlpMode `json:"mode" jsonschema:"required,,enum=http,enum=grpc"`
		Endpoint    string   `json:"endpoint" jsonschema:"required"`
		Compression string   `json:"compression" jsonschema:"omitempty,enum=,enum=gzip"`
	}

	// Tracer is the tracer.
	Tracer struct {
		trace.Tracer
		tp *sdktrace.TracerProvider
	}

	// Span is the span of the Tracing.
	Span struct {
		trace.Span
		tracer *Tracer
		ctx    context.Context
	}
)

const (
	exporterKindJaeger exporterKind = "jaeger"
	exporterKindZipkin exporterKind = "zipkin"
	exporterKindOTLP   exporterKind = "otlp"

	jaegerModeAgent     jaegerMode = "agent"
	jaegerModeCollector jaegerMode = "collector"

	otlpModeHTTP otlpMode = "http"
	otlpModeGRPC otlpMode = "grpc"
)

// Validate validates Spec.
func (spec *ExporterSpec) Validate() error {
	switch spec.Kind {
	case exporterKindJaeger:
		if spec.Jaeger == nil {
			return fmt.Errorf("jaeger cannot be empty")
		}
	case exporterKindZipkin:
		if spec.Zipkin == nil {
			return fmt.Errorf("zipkin cannot be empty")
		}
	default:
		if spec.OTLP == nil {
			return fmt.Errorf("otlp cannot be empty")
		}
	}

	return nil
}

// Validate validates Spec.
func (spec *JaegerSpec) Validate() error {
	switch spec.Mode {
	case jaegerModeAgent:
		if stringtool.IsAnyEmpty(spec.AgentHost, spec.AgentPort) {
			return fmt.Errorf("agentHost or anentPort cannot be empty")
		}
	default:
		if spec.Endpoint == "" {
			return fmt.Errorf("endpoint cannot be empty")
		}

	}
	return nil
}

// NoopTracer is the tracer doing nothing.
var NoopTracer *Tracer

// NoopSpan does nothing.
var NoopSpan *Span

//GlobalPropagator is global
var GlobalPropagator propagation.TextMapPropagator = propagation.Baggage{}

func init() {
	NoopTracer = &Tracer{
		Tracer: trace.NewNoopTracerProvider().Tracer("noop"),
	}
	ctx, span := NoopTracer.Start(context.Background(), "noop")
	NoopSpan = &Span{Span: span, ctx: ctx, tracer: NoopTracer}
}

// New creates a Tracing.
func New(spec *Spec) (*Tracer, error) {
	if spec == nil {
		return NoopTracer, nil
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithRawSpanLimits(spec.newSpanLimits()),
		sdktrace.WithSampler(spec.newSampler())}

	if r, err := spec.newResource(); err == nil {
		opts = append(opts, sdktrace.WithResource(r))
	} else {
		return NoopTracer, err
	}

	if sp, err := spec.newBatchSpanProcessor(); err == nil {
		opts = append(opts, sdktrace.WithSpanProcessor(sp))
	} else {
		return NoopTracer, err
	}

	tp := sdktrace.NewTracerProvider(
		opts...,
	)

	return &Tracer{Tracer: tp.Tracer(""), tp: tp}, nil
}

func (spec *Spec) newResource() (*resource.Resource, error) {

	var attrs = []attribute.KeyValue{semconv.ServiceNameKey.String(spec.ServiceName)}
	for k, v := range spec.Attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, attrs...))
}

func (spec *Spec) newSampler() sdktrace.Sampler {
	if spec.SampleRate <= 0 {
		return sdktrace.NeverSample()
	} else if spec.SampleRate >= 1 {
		return sdktrace.AlwaysSample()
	} else {
		return sdktrace.TraceIDRatioBased(spec.SampleRate)
	}
}

func (spec *Spec) newSpanLimits() sdktrace.SpanLimits {
	if spec.SpanLimits == nil {
		return sdktrace.NewSpanLimits()
	}

	return sdktrace.SpanLimits{
		AttributeValueLengthLimit:   spec.SpanLimits.AttributeValueLengthLimit,
		AttributeCountLimit:         spec.SpanLimits.AttributeCountLimit,
		EventCountLimit:             spec.SpanLimits.AttributeCountLimit,
		LinkCountLimit:              spec.SpanLimits.LinkCountLimit,
		AttributePerEventCountLimit: spec.SpanLimits.AttributePerEventCountLimit,
		AttributePerLinkCountLimit:  spec.SpanLimits.AttributePerLinkCountLimit,
	}
}

func (spec *Spec) newBatchSpanProcessor() (sdktrace.SpanProcessor, error) {
	exp, err := spec.Exporter.newExporter()

	if err != nil {
		return nil, err
	}

	var opts []sdktrace.BatchSpanProcessorOption
	if spec.BatchLimits != nil {
		opts = []sdktrace.BatchSpanProcessorOption{
			sdktrace.WithMaxQueueSize(spec.BatchLimits.MaxQueueSize),
			sdktrace.WithBatchTimeout(spec.BatchLimits.BatchTimeout),
			sdktrace.WithExportTimeout(spec.BatchLimits.ExportTimeout),
			sdktrace.WithMaxExportBatchSize(spec.BatchLimits.MaxExportBatchSize),
		}
	}

	return sdktrace.NewBatchSpanProcessor(exp, opts...), nil

}

func (spec *ExporterSpec) newExporter() (sdktrace.SpanExporter, error) {
	switch spec.Kind {
	case exporterKindJaeger:
		return spec.Jaeger.newExporter()
	case exporterKindZipkin:
		return spec.Zipkin.newExporter()
	default:
		return spec.OTLP.newExporter()
	}
}

func (spec *JaegerSpec) newExporter() (sdktrace.SpanExporter, error) {
	switch spec.Mode {
	case jaegerModeAgent:
		return jaeger.New(jaeger.WithAgentEndpoint(jaeger.WithAgentHost(spec.AgentHost), jaeger.WithAgentPort(spec.AgentPort)))
	default:
		return jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(spec.Endpoint), jaeger.WithUsername(spec.Username), jaeger.WithPassword(spec.Password)))
	}
}

func (spec *ZipkinSpec) newExporter() (sdktrace.SpanExporter, error) {
	return zipkin.New(spec.CollectorURL)
}

func (spec *OTLPSpec) newExporter() (sdktrace.SpanExporter, error) {

	switch spec.Mode {
	case otlpModeGRPC:
		return otlptracegrpc.New(context.Background(), otlptracegrpc.WithEndpoint(spec.Endpoint), otlptracegrpc.WithCompressor(spec.Compression))
	default:
		compression := otlptracehttp.NoCompression
		if spec.Compression == "gzip" {
			compression = otlptracehttp.GzipCompression
		}
		return otlptracehttp.New(context.Background(), otlptracehttp.WithEndpoint(spec.Endpoint), otlptracehttp.WithCompression(compression))
	}

}

// IsNoopTracer checks whether tracer is noop tracer.
func (t *Tracer) IsNoopTracer() bool {
	return t == NoopTracer
}

// NewSpan creates a span.
func (t *Tracer) NewSpan(ctx context.Context, name string) *Span {
	if t.IsNoopTracer() {
		return NoopSpan
	}
	return t.newSpanWithStart(ctx, name, fasttime.Now())
}

// NewSpanWithStart creates a span with specify start time.
func (t *Tracer) NewSpanWithStart(ctx context.Context, name string, startAt time.Time) *Span {
	if t.IsNoopTracer() {
		return NoopSpan
	}
	return t.newSpanWithStart(ctx, name, fasttime.Now())
}

func (t *Tracer) newSpanWithStart(ctx context.Context, name string, startAt time.Time) *Span {
	ctx, span := t.Tracer.Start(ctx, name, trace.WithTimestamp(startAt))
	return &Span{Span: span, ctx: ctx, tracer: t}
}

// Close trace.
func (t *Tracer) Close() error {
	if t.tp != nil {
		return t.tp.Shutdown(context.Background())
	}
	return nil
}

// IsNoop returns whether the span is a noop span.
func (s *Span) IsNoop() bool {
	return s == NoopSpan
}

// Tracer returns the tracer of the span.
func (s *Span) Tracer() *Tracer {
	return s.tracer
}

// NewChild creates a new child span.
func (s *Span) NewChild(name string) *Span {
	if s.IsNoop() {
		return s
	}
	return s.newChildWithStart(name, fasttime.Now())
}

// NewChildWithStart creates a new child span with specified start time.
func (s *Span) NewChildWithStart(name string, startAt time.Time) *Span {
	if s.IsNoop() {
		return s
	}
	return s.newChildWithStart(name, startAt)
}

func (s *Span) newChildWithStart(name string, startAt time.Time) *Span {
	ctx, child := s.tracer.Start(s.ctx, name, trace.WithTimestamp(startAt))
	return &Span{
		Span:   child,
		tracer: s.tracer,
		ctx:    ctx,
	}
}

// InjectHTTP injects span context into an HTTP request.
func (s *Span) InjectHTTP(r *http.Request) {
	GlobalPropagator.Inject(s.ctx, propagation.HeaderCarrier(r.Header))
}
