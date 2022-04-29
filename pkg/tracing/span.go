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
	"time"

	zipkingo "github.com/openzipkin/zipkin-go"
	"github.com/openzipkin/zipkin-go/propagation/b3"

	"github.com/megaease/easegress/pkg/util/fasttime"
)

type (
	// Span is the span of the Tracing.
	Span interface {
		zipkingo.Span

		// Tracer returns the Tracer that created this Span.
		Tracer() *Tracer

		// NewChild creates a child span.
		NewChild(name string) Span

		// NewChildWithStart creates a child span with start time.
		NewChildWithStart(name string, startAt time.Time) Span

		// InjectHTTP injects span context into an HTTP request.
		InjectHTTP(r *http.Request)
	}

	span struct {
		zipkingo.Span
		tracer *Tracer
	}
)

// NoopSpan does nothing.
var NoopSpan *span

// IsNoop returns whether the span is a noop span.
func (s *span) IsNoop() bool {
	return s == NoopSpan
}

// Tracer returns the tracer of the span.
func (s *span) Tracer() *Tracer {
	return s.tracer
}

// NewChild creates a new child span.
func (s *span) NewChild(name string) Span {
	if s.IsNoop() {
		return s
	}
	return s.newChildWithStart(name, fasttime.Now())
}

// NewChildWithStart creates a new child span with specified start time.
func (s *span) NewChildWithStart(name string, startAt time.Time) Span {
	if s.IsNoop() {
		return s
	}
	return s.newChildWithStart(name, startAt)
}

func (s *span) newChildWithStart(name string, startAt time.Time) Span {
	child := s.tracer.tracer.StartSpan(name,
		zipkingo.Parent(s.Context()),
		zipkingo.StartTime(startAt))

	return &span{
		tracer: s.tracer,
		Span:   child,
	}
}

// InjectHTTP injects span context into an HTTP request.
func (s *span) InjectHTTP(r *http.Request) {
	inject := b3.InjectHTTP(r, b3.WithSingleHeaderOnly())
	inject(s.Context())
}
