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
	"io"
	"time"

	"github.com/megaease/easegress/pkg/tracing/zipkin"
	zipkingo "github.com/openzipkin/zipkin-go"
)

type (
	// Spec describes Tracing.
	Spec struct {
		ServiceName string            `yaml:"serviceName" jsonschema:"required"`
		Tags        map[string]string `yaml:"tags" jsonschema:"omitempty"`
		Zipkin      *zipkin.Spec      `yaml:"zipkin" jsonschema:"omitempty"`
	}

	// Tracing is the tracing.
	Tracing struct {
		Tracer *zipkingo.Tracer
		tags   map[string]string
		closer io.Closer
	}

	noopCloser struct{}
)

// NoopTracing is the tracing doing nothing.
var NoopTracing = &Tracing{
	Tracer: zipkin.CreateNoopTracer(),
	closer: nil,
}

// New creates a Tracing.
func New(spec *Spec) (*Tracing, error) {
	if spec == nil {
		return NoopTracing, nil
	}

	tracer, closer, err := zipkin.New(spec.ServiceName, spec.Zipkin, spec.Tags)
	if err != nil {
		return nil, err
	}

	return &Tracing{
		Tracer: tracer,
		closer: closer,
	}, nil
}

// IsNoopTracer checks whether tracer is noop tracer.
func (t *Tracing) IsNoopTracer() bool {
	return t == NoopTracing
}

// Close closes Tracing.
func (t *Tracing) Close() error {
	if t.closer != nil {
		return t.closer.Close()
	}

	return nil
}

// CreateSpanWithContext creates new span with given name and starttime and adds it to the context.
func CreateSpanWithContext(ctx context.Context, tracing *Tracing, spanName string, startTime time.Time) context.Context {
	span := tracing.Tracer.StartSpan(spanName, zipkingo.StartTime(startTime))
	return zipkingo.NewContext(ctx, span)
}
