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

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/megaease/easegress/pkg/tracing/base"
)

type (
	// Span is the span of the Tracing.
	Span interface {
		// Tracer returns the Tracer that created this Span.
		Tracer() opentracing.Tracer

		// Context yields the SpanContext for this Span
		Context() opentracing.SpanContext

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
	}

	span struct {
		mutex    sync.Mutex
		tracer   *Tracing
		span     opentracing.Span
		children []*span
	}
)

// NewSpan creates a span.
func NewSpan(tracer *Tracing, name string) Span {
	return newSpanWithStart(tracer, name, time.Now())
}

// NewSpanWithStart creates a span with specify start time.
func NewSpanWithStart(tracer *Tracing, name string, startAt time.Time) Span {
	return newSpanWithStart(tracer, name, startAt)
}

func newSpanWithStart(tracer *Tracing, name string, startAt time.Time) Span {
	return &span{
		tracer: tracer,
		span:   tracer.StartSpan(name, opentracing.StartTime(startAt)),
	}
}

func (s *span) Tracer() opentracing.Tracer {
	return s.tracer
}

func (s *span) Context() opentracing.SpanContext {
	return s.span.Context()
}

func (s *span) Finish() {
	s.span.Finish()
}

func (s *span) Cancel() {
	s.span.SetTag(base.CancelTagKey, "yes")
	for _, child := range s.children {
		child.Cancel()
	}
}

func (s *span) NewChild(name string) Span {
	return s.newChildWithStart(name, time.Now())
}

func (s *span) NewChildWithStart(name string, startAt time.Time) Span {
	return s.newChildWithStart(name, startAt)
}

func (s *span) newChildWithStart(name string, startAt time.Time) Span {
	childSpan := s.tracer.StartSpan(name,
		opentracing.ChildOf(s.span.Context()),
		opentracing.StartTime(startAt))
	child := &span{
		tracer: s.tracer,
		span:   childSpan,
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.children = append(s.children, child)

	return child
}

func (s *span) SetName(name string) {
	s.span.SetOperationName(name)
}

func (s *span) LogKV(kv ...interface{}) {
	s.span.LogKV(kv...)
}
