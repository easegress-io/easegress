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
	"io"
	"time"

	"github.com/megaease/easegress/pkg/util/fasttime"
	zipkingo "github.com/openzipkin/zipkin-go"
)

type (
	// Spec describes Tracer.
	Spec struct {
		ServiceName string            `yaml:"serviceName" jsonschema:"required"`
		Tags        map[string]string `yaml:"tags" jsonschema:"omitempty"`
		Zipkin      *ZipkinSpec       `yaml:"zipkin" jsonschema:"required"`
	}

	// Tracer is the tracer.
	Tracer struct {
		tracer *zipkingo.Tracer
		tags   map[string]string
		closer io.Closer
	}

	noopCloser struct{}
)

// NoopTracer is the tracer doing nothing.
var NoopTracer *Tracer

func init() {
	tracer, _ := zipkingo.NewTracer(nil)
	NoopTracer = &Tracer{tracer: tracer, closer: nil}
}

// New creates a Tracing.
func New(spec *Spec) (*Tracer, error) {
	if spec == nil {
		return NoopTracer, nil
	}

	tracer, closer, err := NewZipkinTracer(spec.ServiceName, spec.Zipkin)
	if err != nil {
		return nil, err
	}

	return &Tracer{
		tracer: tracer,
		tags:   spec.Tags,
		closer: closer,
	}, nil
}

// IsNoopTracer checks whether tracer is noop tracer.
func (t *Tracer) IsNoopTracer() bool {
	return t == NoopTracer
}

// Close closes Tracing.
func (t *Tracer) Close() error {
	if t.closer != nil {
		return t.closer.Close()
	}

	return nil
}

// NewSpan creates a span.
func (t *Tracer) NewSpan(name string) Span {
	if t.IsNoopTracer() {
		return NoopSpan
	}
	return t.newSpanWithStart(name, fasttime.Now())
}

// NewSpanWithStart creates a span with specify start time.
func (t *Tracer) NewSpanWithStart(name string, startAt time.Time) Span {
	if t.IsNoopTracer() {
		return NoopSpan
	}
	return t.newSpanWithStart(name, startAt)
}

func (t *Tracer) newSpanWithStart(name string, startAt time.Time) Span {
	s := t.tracer.StartSpan(name, zipkingo.StartTime(startAt))
	for k, v := range t.tags {
		s.Tag(k, v)
	}
	return &span{Span: s, tracer: t}
}
