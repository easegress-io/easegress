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
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type (
	// CDNSpan is a generic interface that defines the span of edge network of different CDN vendors.
	CDNSpan interface {
		// NewSpan creates a span and a context.Context containing the newly-created span.
		// The newly-created span's id should be the unique ID provided by the CDN vendor,
		// and timestamp should be the timestamp of the traffic entering the edge network.
		NewSpan(ctx context.Context, t *Tracer, spanName string, req *http.Request) *Span

		// End completes the Span.
		End(options ...trace.SpanEndOption)
	}

	// CloudflareSpan is an implementation of the CDNSpan interface.
	CloudflareSpan struct {
		span trace.Span
	}
)

const (
	cloudFlareSpanName = "cloudflare"
	cfRayHeader        = "cf-ray"
	cfSecHeader        = "x-ts-sec"
	cfMsecHeader       = "x-ts-msec"
)

// NewSpan create a span, which describes the traffic entering Cloudflare.
func (cfs *CloudflareSpan) NewSpan(ctx context.Context, t *Tracer, spanName string, req *http.Request) *Span {
	sec := req.Header.Get(cfSecHeader)
	if sec == "" {
		return nil
	}
	msec := req.Header.Get(cfMsecHeader)
	if msec == "" {
		return nil
	}
	if timestamp, err := strconv.ParseInt(sec+msec, 10, 64); err == nil {
		cloudflareSpan := t.newSpanWithStart(ctx, cloudFlareSpanName, time.UnixMilli(timestamp))
		cloudflareSpan.Span.SetAttributes(attribute.String("cf.ray", req.Header.Get(cfRayHeader)))
		cfs.span = cloudflareSpan
		return cloudflareSpan.NewChild(spanName)
	}
	return nil
}

// End completes the Span.
func (cfs *CloudflareSpan) End(options ...trace.SpanEndOption) {
	cfs.span.End(options...)
}

func enableCloudflare(req *http.Request) bool {
	return req.Header.Get(cfRayHeader) != ""
}

func enableCDN(req *http.Request) bool {
	if enableCloudflare(req) {
		return true
	}
	return false
}
