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

// Package tracing implements the tracing.
package tracing

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/megaease/easegress/v2/pkg/util/fasttime"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

type (
	headerFormat string
	// Spec describes Tracer.
	Spec struct {
		ServiceName  string                `json:"serviceName" jsonschema:"required,minLength=1"`
		Tags         map[string]string     `json:"tags,omitempty"`
		Attributes   map[string]string     `json:"attributes,omitempty"`
		SpanLimits   *SpanLimitsSpec       `json:"spanLimits,omitempty"`
		SampleRate   float64               `json:"sampleRate,omitempty" jsonschema:"minimum=0,maximum=1,default=1"`
		BatchLimits  *BatchLimitsSpec      `json:"batchLimits,omitempty"`
		Exporter     *ExporterSpec         `json:"exporter,omitempty"`
		Zipkin       *ZipkinDeprecatedSpec `json:"zipkin,omitempty"`
		HeaderFormat headerFormat          `json:"headerFormat,omitempty" jsonschema:"default=trace-context,enum=trace-context,enum=b3"`
	}

	// SpanLimitsSpec represents the limits of a span.
	SpanLimitsSpec struct {
		// AttributeValueLengthLimit is the maximum allowed attribute value length.
		//
		// This limit only applies to string and string slice attribute values.
		// Any string longer than this value will be truncated to this length.
		//
		// Setting this to a negative value means no limit is applied.
		AttributeValueLengthLimit int `json:"attributeValueLengthLimit,omitempty" jsonschema:"default=-1"`

		// AttributeCountLimit is the maximum allowed span attribute count. Any
		// attribute added to a span once this limit is reached will be dropped.
		//
		// Setting this to zero means no attributes will be recorded.
		//
		// Setting this to a negative value means no limit is applied.
		AttributeCountLimit int `json:"attributeCountLimit,omitempty" jsonschema:"default=128"`

		// EventCountLimit is the maximum allowed span event count. Any event
		// added to a span once this limit is reached means it will be added but
		// the oldest event will be dropped.
		//
		// Setting this to zero means no events we be recorded.
		//
		// Setting this to a negative value means no limit is applied.
		EventCountLimit int `json:"eventCountLimit,omitempty" jsonschema:"default=128"`

		// LinkCountLimit is the maximum allowed span link count. Any link added
		// to a span once this limit is reached means it will be added but the
		// oldest link will be dropped.
		//
		// Setting this to zero means no links we be recorded.
		//
		// Setting this to a negative value means no limit is applied.
		LinkCountLimit int `json:"linkCountLimit,omitempty" jsonschema:"default=128"`

		// AttributePerEventCountLimit is the maximum number of attributes allowed
		// per span event. Any attribute added after this limit reached will be
		// dropped.
		//
		// Setting this to zero means no attributes will be recorded for events.
		//
		// Setting this to a negative value means no limit is applied.
		AttributePerEventCountLimit int `json:"attributePerEventCountLimit,omitempty" jsonschema:"default=128"`

		// AttributePerLinkCountLimit is the maximum number of attributes allowed
		// per span link. Any attribute added after this limit reached will be
		// dropped.
		//
		// Setting this to zero means no attributes will be recorded for links.
		//
		// Setting this to a negative value means no limit is applied.
		AttributePerLinkCountLimit int `json:"attributePerLinkCountLimit,omitempty" jsonschema:"default=128"`
	}

	// BatchLimitsSpec describes BatchSpanProcessorOptions.
	BatchLimitsSpec struct {
		// MaxQueueSize is the maximum queue size to buffer spans for delayed processing. If the
		// queue gets full it drops the spans. Use BlockOnQueueFull to change this behavior.
		// The default value of MaxQueueSize is 2048.
		MaxQueueSize int `json:"maxQueueSize,omitempty" jsonschema:"default=2048"`

		// BatchTimeout is the maximum duration for constructing a batch. Processor
		// forcefully sends available spans when timeout is reached.
		// The default value of BatchTimeout is 5000 msec.
		BatchTimeout int64 `json:"batchTimeout,omitempty" jsonschema:"default=5000"`

		// ExportTimeout specifies the maximum duration for exporting spans. If the timeout
		// is reached, the export will be cancelled.
		// The default value of ExportTimeout is 30000 msec.
		ExportTimeout int64 `json:"exportTimeout,omitempty" jsonschema:"default=30000"`

		// MaxExportBatchSize is the maximum number of spans to process in a single batch.
		// If there are more than one batch worth of spans then it processes multiple batches
		// of spans one batch after the other without any delay.
		// The default value of MaxExportBatchSize is 512.
		MaxExportBatchSize int `json:"maxExportBatchSize,omitempty" jsonschema:"default=512"`
	}

	// ExporterSpec describes exporter.
	ExporterSpec struct {
		Jaeger *JaegerSpec `json:"jaeger,omitempty"`
		Zipkin *ZipkinSpec `json:"zipkin,omitempty"`
		OTLP   *OTLPSpec   `json:"otlp,omitempty"`
	}

	jaegerMode string

	// JaegerSpec describes Jaeger.
	JaegerSpec struct {
		Mode     jaegerMode `json:"mode" jsonschema:"required,enum=agent,enum=collector"`
		Endpoint string     `json:"endpoint,omitempty"`
		Username string     `json:"username,omitempty"`
		Password string     `json:"password,omitempty"`
	}

	// ZipkinSpec describes Zipkin.
	ZipkinSpec struct {
		Endpoint string `json:"endpoint" jsonschema:"required,format=url"`
	}

	otlpProtocol string
	// OTLPSpec describes OpenTelemetry exporter.
	OTLPSpec struct {
		Protocol    otlpProtocol `json:"protocol" jsonschema:"required,,enum=http,enum=grpc"`
		Endpoint    string       `json:"endpoint" jsonschema:"required"`
		Insecure    bool         `json:"insecure,omitempty"`
		Compression string       `json:"compression,omitempty" jsonschema:"enum=,enum=gzip"`
	}

	// ZipkinDeprecatedSpec describes Zipkin.
	// Deprecated: This option will be kept until the next major version
	// incremented release.
	ZipkinDeprecatedSpec struct {
		Hostport      string  `json:"hostport,omitempty"`
		ServerURL     string  `json:"serverURL" jsonschema:"required,format=url"`
		DisableReport bool    `json:"disableReport,omitempty"`
		SampleRate    float64 `json:"sampleRate" jsonschema:"required,minimum=0,maximum=1"`
		SameSpan      bool    `json:"sameSpan,omitempty"`
		ID128Bit      bool    `json:"id128Bit,omitempty"`
	}

	// Tracer is the tracer.
	Tracer struct {
		trace.Tracer
		tp         *sdktrace.TracerProvider
		propagator propagation.TextMapPropagator
	}

	// Span is the span of the Tracing.
	Span struct {
		trace.Span
		cdnSpan trace.Span
		tracer  *Tracer
		ctx     context.Context
	}
)

const (
	// see: https://www.w3.org/TR/trace-context/
	headerFormatTraceContext = "trace-context"
	headerFormatB3           = "b3"

	jaegerModeAgent     jaegerMode = "agent"
	jaegerModeCollector jaegerMode = "collector"

	otlpProtocolHTTP otlpProtocol = "http"
	otlpProtocolGRPC otlpProtocol = "grpc"
)

// UnmarshalJSON implements json.Unmarshaler.
func (spec *Spec) UnmarshalJSON(data []byte) error {
	type innerSpec Spec
	inner := &innerSpec{
		SpanLimits: &SpanLimitsSpec{
			AttributeValueLengthLimit:   sdktrace.DefaultAttributeValueLengthLimit,
			AttributeCountLimit:         sdktrace.DefaultAttributeCountLimit,
			EventCountLimit:             sdktrace.DefaultEventCountLimit,
			LinkCountLimit:              sdktrace.DefaultLinkCountLimit,
			AttributePerEventCountLimit: sdktrace.DefaultAttributePerEventCountLimit,
			AttributePerLinkCountLimit:  sdktrace.DefaultAttributePerLinkCountLimit,
		},
		SampleRate: 1,
		BatchLimits: &BatchLimitsSpec{
			MaxQueueSize:       sdktrace.DefaultMaxQueueSize,
			BatchTimeout:       sdktrace.DefaultScheduleDelay,
			ExportTimeout:      sdktrace.DefaultExportTimeout,
			MaxExportBatchSize: sdktrace.DefaultMaxExportBatchSize,
		},
		HeaderFormat: headerFormatTraceContext,
	}

	if err := json.Unmarshal(data, inner); err != nil {
		return err
	}

	*spec = Spec(*inner)
	return nil
}

// Validate validates Spec.
func (spec *Spec) Validate() error {
	if spec.Exporter == nil && spec.Zipkin == nil {
		return fmt.Errorf("exporter and zipkin cannot both be empty")
	}

	if spec.Exporter != nil && spec.Zipkin != nil {
		return fmt.Errorf("exporter and zipkin cannot exist at the same time")
	}

	if spec.Tags != nil && spec.Attributes != nil {
		return fmt.Errorf("tags and attributes cannot be configured at the same time, please use attributes to unify the management")
	}

	return nil
}

// Validate validates Spec.
func (spec *ExporterSpec) Validate() error {
	if spec == nil {
		return fmt.Errorf("exporter cannot be empty")
	}

	if spec.Jaeger == nil && spec.Zipkin == nil && spec.OTLP == nil {
		return fmt.Errorf("exporter cannot be empty")
	}

	return nil
}

// Validate validates Spec.
func (spec *JaegerSpec) Validate() error {
	if spec.Endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}

	if spec.Mode == jaegerModeAgent {
		if _, _, err := net.SplitHostPort(spec.Endpoint); err != nil {
			return fmt.Errorf("in agent mode, the endpoint must be host:port")
		}
	}

	return nil
}

// NoopTracer is the tracer doing nothing.
var NoopTracer *Tracer

// NoopSpan does nothing.
var NoopSpan *Span

func init() {
	NoopTracer = &Tracer{
		Tracer:     trace.NewNoopTracerProvider().Tracer("noop"),
		propagator: propagation.TraceContext{},
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
		sdktrace.WithSampler(spec.newSampler()),
	}

	if r, err := spec.newResource(); err == nil {
		opts = append(opts, sdktrace.WithResource(r))
	} else {
		return NoopTracer, err
	}

	if sps, err := spec.newBatchSpanProcessors(); err == nil {
		for _, sp := range sps {
			opts = append(opts, sdktrace.WithSpanProcessor(sp))
		}
	} else {
		return NoopTracer, err
	}

	tp := sdktrace.NewTracerProvider(
		opts...,
	)

	return &Tracer{Tracer: tp.Tracer(""), tp: tp, propagator: spec.newPropagator()}, nil
}

func (spec *Spec) newResource() (*resource.Resource, error) {
	attributes := spec.Attributes
	if attributes == nil {
		attributes = spec.Tags
	}

	attrs := []attribute.KeyValue{semconv.ServiceNameKey.String(spec.ServiceName)}
	for k, v := range attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	return resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, attrs...))
}

func (spec *Spec) newSampler() sdktrace.Sampler {
	sampleRate := spec.SampleRate

	if spec.Exporter == nil {
		sampleRate = spec.Zipkin.SampleRate
	}

	if sampleRate <= 0 {
		return sdktrace.NeverSample()
	}

	if sampleRate >= 1 {
		return sdktrace.AlwaysSample()
	}

	return sdktrace.TraceIDRatioBased(sampleRate)
}

func (spec *Spec) newSpanLimits() sdktrace.SpanLimits {
	if spec.SpanLimits == nil {
		return sdktrace.NewSpanLimits()
	}

	return sdktrace.SpanLimits{
		AttributeValueLengthLimit:   spec.SpanLimits.AttributeValueLengthLimit,
		AttributeCountLimit:         spec.SpanLimits.AttributeCountLimit,
		EventCountLimit:             spec.SpanLimits.EventCountLimit,
		LinkCountLimit:              spec.SpanLimits.LinkCountLimit,
		AttributePerEventCountLimit: spec.SpanLimits.AttributePerEventCountLimit,
		AttributePerLinkCountLimit:  spec.SpanLimits.AttributePerLinkCountLimit,
	}
}

func (spec *Spec) newBatchSpanProcessors() ([]sdktrace.SpanProcessor, error) {
	var exporters []sdktrace.SpanExporter
	var err error

	if spec.Exporter != nil {
		exporters, err = spec.Exporter.newExporters()
		if err != nil {
			return nil, err
		}
	} else if exp, err := spec.Zipkin.newExporter(); err == nil {
		exporters = []sdktrace.SpanExporter{exp}
	} else {
		return nil, err
	}

	var opts []sdktrace.BatchSpanProcessorOption
	if spec.BatchLimits != nil {
		opts = []sdktrace.BatchSpanProcessorOption{
			sdktrace.WithMaxQueueSize(spec.BatchLimits.MaxQueueSize),
			sdktrace.WithBatchTimeout(time.Duration(spec.BatchLimits.BatchTimeout) * time.Millisecond),
			sdktrace.WithExportTimeout(time.Duration(spec.BatchLimits.ExportTimeout) * time.Millisecond),
			sdktrace.WithMaxExportBatchSize(spec.BatchLimits.MaxExportBatchSize),
		}
	}

	bsps := make([]sdktrace.SpanProcessor, 0, len(exporters))
	for _, exp := range exporters {
		bsps = append(bsps, sdktrace.NewBatchSpanProcessor(exp, opts...))
	}

	return bsps, nil
}

func (spec *Spec) newPropagator() propagation.TextMapPropagator {
	format := spec.HeaderFormat
	if spec.Exporter == nil && spec.Zipkin != nil {
		format = headerFormatB3
	}

	if format == headerFormatB3 {
		return b3.New(b3.WithInjectEncoding(b3.B3SingleHeader))
	}

	return propagation.TraceContext{}
}

func (spec *ExporterSpec) newExporters() ([]sdktrace.SpanExporter, error) {
	var exporters []sdktrace.SpanExporter
	if spec.Jaeger != nil {
		if exp, err := spec.Jaeger.newExporter(); err == nil {
			exporters = append(exporters, exp)
		} else {
			return nil, err
		}
	}

	if spec.Zipkin != nil {
		if exp, err := spec.Zipkin.newExporter(); err == nil {
			exporters = append(exporters, exp)
		} else {
			return nil, err
		}
	}

	if spec.OTLP != nil {
		if exp, err := spec.OTLP.newExporter(); err == nil {
			exporters = append(exporters, exp)
		} else {
			return nil, err
		}
	}

	return exporters, nil
}

func (spec *JaegerSpec) newExporter() (sdktrace.SpanExporter, error) {
	switch spec.Mode {
	case jaegerModeAgent:
		host, port, _ := net.SplitHostPort(spec.Endpoint)
		return jaeger.New(jaeger.WithAgentEndpoint(jaeger.WithAgentHost(host), jaeger.WithAgentPort(port)))
	default:
		return jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(spec.Endpoint), jaeger.WithUsername(spec.Username), jaeger.WithPassword(spec.Password)))
	}
}

func (spec *ZipkinSpec) newExporter() (sdktrace.SpanExporter, error) {
	return zipkin.New(spec.Endpoint)
}

func (spec *ZipkinDeprecatedSpec) newExporter() (sdktrace.SpanExporter, error) {
	return zipkin.New(spec.ServerURL)
}

func (spec *OTLPSpec) newExporter() (sdktrace.SpanExporter, error) {
	switch spec.Protocol {
	case otlpProtocolGRPC:
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(spec.Endpoint), otlptracegrpc.WithCompressor(spec.Compression)}
		if spec.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.New(context.Background(), opts...)
	default:
		compression := otlptracehttp.NoCompression
		if spec.Compression == "gzip" {
			compression = otlptracehttp.GzipCompression
		}
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(spec.Endpoint), otlptracehttp.WithCompression(compression)}
		if spec.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		return otlptracehttp.New(context.Background(), opts...)
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

func (t *Tracer) newSpanWithStart(ctx context.Context, name string, startAt time.Time) *Span {
	ctx, span := t.Tracer.Start(ctx, name, trace.WithTimestamp(startAt))
	return &Span{Span: span, ctx: ctx, tracer: t}
}

// NewSpanForHTTP creates a span for http request.
func (t *Tracer) NewSpanForHTTP(ctx context.Context, name string, req *http.Request) *Span {
	if t.IsNoopTracer() {
		return NoopSpan
	}

	span := newSpanForCloudflare(ctx, t, name, req)
	if span != nil {
		return span
	}

	return t.newSpanWithStart(ctx, name, fasttime.Now())
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
	s.tracer.propagator.Inject(s.ctx, propagation.HeaderCarrier(r.Header))
}

// End completes the Span. Override trace.Span.End function.
func (s *Span) End(options ...trace.SpanEndOption) {
	if s.cdnSpan != nil {
		s.cdnSpan.End(options...)
	}
	s.Span.End(options...)
}
