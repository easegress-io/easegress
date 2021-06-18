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

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/megaease/easegress/pkg/tracing/zipkin"
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

// NoopTracing is the tracing doing nothing.
var NoopTracing = &Tracing{
	Tracer: opentracing.NoopTracer{},
	closer: nil,
}

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
