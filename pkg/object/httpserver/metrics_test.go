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

package httpserver

import (
	"github.com/megaease/easegress/v2/pkg/util/prometheushelper"
	"github.com/prometheus/client_golang/prometheus"
)

func newMockMetrics() *metrics {
	commonLabels := prometheus.Labels{
		"httpServerName": "name",
		"kind":           Kind,
	}
	mockLabels := []string{"httpServerName", "kind", "routerKind", "backend"}
	return &metrics{
		Health: prometheushelper.NewGauge("mock_httpserver_health",
			"show the status for the http server: 1 for ready, 0 for down",
			mockLabels[:2]).MustCurryWith(commonLabels),
		TotalRequests: prometheushelper.NewCounter(
			"mock_httpserver_total_requests",
			"the total count of http requests",
			mockLabels).MustCurryWith(commonLabels),
		TotalResponses: prometheushelper.NewCounter(
			"mock_httpserver_total_response",
			"the total count of http resposne",
			mockLabels).MustCurryWith(commonLabels),
		TotalErrorRequests: prometheushelper.NewCounter(
			"mock_httpserver_total_error_requests",
			"the total count of http error requests",
			mockLabels).MustCurryWith(commonLabels),
		RequestsDuration: prometheushelper.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "mock_httpserver_requests_duration",
				Help:    "request processing duration histogram",
				Buckets: prometheushelper.DefaultDurationBuckets(),
			},
			mockLabels).MustCurryWith(commonLabels),
		RequestSizeBytes: prometheushelper.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "mock_httpserver_requests_size_bytes",
				Help:    "a histogram of the total size of the request. Includes body",
				Buckets: prometheushelper.DefaultBodySizeBuckets(),
			},
			mockLabels).MustCurryWith(commonLabels),
		ResponseSizeBytes: prometheushelper.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "mock_httpserver_responses_size_bytes",
				Help:    "a histogram of the total size of the returned response body",
				Buckets: prometheushelper.DefaultBodySizeBuckets(),
			},
			mockLabels).MustCurryWith(commonLabels),
		RequestsDurationPercentage: prometheushelper.NewSummary(
			prometheus.SummaryOpts{
				Name:       "mock_httpserver_requests_duration_percentage",
				Help:       "request processing duration summary",
				Objectives: prometheushelper.DefaultObjectives(),
			},
			mockLabels).MustCurryWith(commonLabels),
		RequestSizeBytesPercentage: prometheushelper.NewSummary(
			prometheus.SummaryOpts{
				Name:       "mock_httpserver_requests_size_bytes_percentage",
				Help:       "a summary of the total size of the request. Includes body",
				Objectives: prometheushelper.DefaultObjectives(),
			},
			mockLabels).MustCurryWith(commonLabels),
		ResponseSizeBytesPercentage: prometheushelper.NewSummary(
			prometheus.SummaryOpts{
				Name:       "mock_httpserver_responses_size_bytes_percentage",
				Help:       "a summary of the total size of the returned response body",
				Objectives: prometheushelper.DefaultObjectives(),
			},
			mockLabels).MustCurryWith(commonLabels),
	}
}
