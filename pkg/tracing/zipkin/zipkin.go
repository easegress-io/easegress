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

package zipkin

import (
	"io"
	"time"

	"github.com/megaease/easegress/pkg/tracing/base"

	opentracing "github.com/opentracing/opentracing-go"
	zipkinot "github.com/openzipkin-contrib/zipkin-go-opentracing"
	zipkingo "github.com/openzipkin/zipkin-go"
	zipkingomodel "github.com/openzipkin/zipkin-go/model"
	zipkingoreporter "github.com/openzipkin/zipkin-go/reporter"
	zipkingohttp "github.com/openzipkin/zipkin-go/reporter/http"
)

type (
	// Spec describes Zipkin.
	Spec struct {
		Hostport   string  `yaml:"hostport" jsonschema:"omitempty"`
		ServerURL  string  `yaml:"serverURL" jsonschema:"required,format=url"`
		SampleRate float64 `yaml:"sampleRate" jsonschema:"required,minimum=0,maximum=1"`
		SameSpan   bool    `yaml:"sameSpan" jsonschema:"omitempty"`
		ID128Bit   bool    `yaml:"id128Bit" jsonschema:"omitempty"`
	}

	cancellableReporter struct {
		reporter zipkingoreporter.Reporter
	}
)

func (cp *cancellableReporter) Send(sm zipkingomodel.SpanModel) {
	_, cancelled := sm.Tags[base.CancelTagKey]
	if cancelled {
		return
	}
	cp.reporter.Send(sm)
}

func (cp *cancellableReporter) Close() error { return cp.reporter.Close() }

// Validate validates Spec.
func (spec Spec) Validate() error {
	if spec.Hostport != "" {
		_, err := zipkingo.NewEndpoint("", spec.Hostport)
		if err != nil {
			return err
		}
	}

	return nil
}

// New creates zipkin tracer.
func New(serviceName string, spec *Spec) (opentracing.Tracer, io.Closer, error) {
	endpoint, err := zipkingo.NewEndpoint(serviceName, spec.Hostport)
	if err != nil {
		return nil, nil, err
	}

	sampler, err := zipkingo.NewBoundarySampler(spec.SampleRate, time.Now().Unix())
	if err != nil {
		return nil, nil, err
	}

	reporter := zipkingohttp.NewReporter(spec.ServerURL)

	nativeTracer, err := zipkingo.NewTracer(
		&cancellableReporter{reporter: reporter},
		zipkingo.WithLocalEndpoint(endpoint),
		zipkingo.WithSharedSpans(spec.SameSpan),
		zipkingo.WithTraceID128Bit(spec.ID128Bit),
		zipkingo.WithSampler(sampler),
	)
	if err != nil {
		return nil, nil, err
	}

	return zipkinot.Wrap(nativeTracer), reporter, nil
}
