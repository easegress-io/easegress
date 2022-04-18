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
	"sync"
	"time"

	zipkingo "github.com/openzipkin/zipkin-go"
	zipkinmodel "github.com/openzipkin/zipkin-go/model"

	"github.com/megaease/easegress/pkg/tracing/base"
)

type (
	// Span is the span of the Tracing.
	Span interface {
		// Tracer returns the Tracer that created this Span.
		Tracer() zipkingo.Tracer

		// Context yields the SpanContext for this Span
		Context() zipkinmodel.SpanContext

		// Finish finishes the span.
		Finish()
		// Cancel cancels the span, it should be called before Finish called.
		// It will cancel all descendent spans.
		Cancel()

		// NewChild creates a child span.
		NewChild(name string) Span
		// NewChildWithStart creates a child span with start time.
		NewChildWithStart(name string, startAt time.Time) Span

		// SetName changes the span name.
		SetName(name string)

		// LogKV logs key:value for the span.
		//
		// The keys must all be strings. The values may be strings, numeric types,
		// bools, Go error instances, or arbitrary structs.
		//
		// Example:
		//
		//    span.LogKV(
		//        "event", "soft error",
		//        "type", "cache timeout",
		//        "waited.millis", 1500)
		LogKV(kvs ...interface{})

		// SetTag sets tag key and value.
		SetTag(key string, value string)
		// IsNoopSpan returns true if span is NoopSpan.
		IsNoopSpan() bool
	}

	span struct {
		mutex    sync.Mutex
		tracer   *Tracing
		span     zipkingo.Span
		children []*span
	}
)

func (s *span) Tracer() *zipkingo.Tracer {
	return s.tracer.Tracer
}

func (s *span) Context() zipkinmodel.SpanContext {
	return s.span.Context()
}

func (s *span) Finish() {
	s.span.Finish()
}

func (s *span) Cancel() {
	s.span.Tag(base.CancelTagKey, "yes")
	for _, child := range s.children {
		child.Cancel()
	}
}
