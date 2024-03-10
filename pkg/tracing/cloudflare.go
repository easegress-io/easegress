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

package tracing

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/megaease/easegress/v2/pkg/util/fasttime"
	"go.opentelemetry.io/otel/attribute"
)

const (
	cfSpanName   = "cloudflare"
	cfRayHeader  = "cf-ray"
	cfSecHeader  = "x-ts-sec"
	cfMsecHeader = "x-ts-msec"
)

// newSpanForCloudflare creates a new span for requests from cloudflare.
// It returns nil if the request is not from cloudflare.
// It creates a normal span if the request is from cloudflare but the
// required headers are not set.
func newSpanForCloudflare(ctx context.Context, t *Tracer, spanName string, req *http.Request) (span *Span) {
	rayID := req.Header.Get(cfRayHeader)

	// not a cloudflare request
	if rayID == "" {
		return
	}

	defer func() {
		if span == nil {
			span = t.newSpanWithStart(ctx, spanName, fasttime.Now())
			span.SetAttributes(attribute.String("cf.ray", rayID))
		}
	}()

	sec := req.Header.Get(cfSecHeader)
	if sec == "" {
		return
	}
	msec := req.Header.Get(cfMsecHeader)
	if msec == "" {
		return
	}
	tm, err := strconv.ParseInt(sec+msec, 10, 64)
	if err != nil {
		return
	}

	cfSpan := t.newSpanWithStart(ctx, cfSpanName, time.UnixMilli(tm))
	cfSpan.SetAttributes(attribute.String("cf.ray", rayID))

	span = cfSpan.NewChild(spanName)
	span.cdnSpan = cfSpan
	return
}
