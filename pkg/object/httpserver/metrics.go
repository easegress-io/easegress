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

package httpserver

import (
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/util/prometheushelper"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	metrics struct {
		Health             *prometheus.GaugeVec
		TotalRequests      *prometheus.CounterVec
		TotalResponse      *prometheus.CounterVec
		TotalErrorRequests *prometheus.CounterVec
		RequestsDuration   prometheus.ObserverVec
		RequestSizeBytes   prometheus.ObserverVec
		ResponseSizeBytes  prometheus.ObserverVec
	}
)

// newMetrics create the HttpServerMetrics.
func (r *runtime) newMetrics(name string) *metrics {
	commonLabels := prometheus.Labels{
		"name":         name,
		"kind":         Kind,
		"clusterName":  r.superSpec.Super().Options().ClusterName,
		"clusterRole":  r.superSpec.Super().Options().ClusterRole,
		"instanceName": r.superSpec.Super().Options().Name,
	}
	httpserverLabels := []string{"clusterName", "clusterRole",
		"instanceName", "name", "kind", "routerKind", "backend"}
	return &metrics{
		Health: prometheushelper.NewGauge("httpserver_health",
			"show the status for the http server: 1 for ready, 0 for down",
			httpserverLabels[:5]).MustCurryWith(commonLabels),
		TotalRequests: prometheushelper.NewCounter("httpserver_total_requests",
			"the total count of http requests",
			httpserverLabels).MustCurryWith(commonLabels),
		TotalResponse: prometheushelper.NewCounter("httpserver_total_response",
			"the total count of http resposne",
			httpserverLabels).MustCurryWith(commonLabels),
		TotalErrorRequests: prometheushelper.NewCounter("httpserver_total_error_requests",
			"the total count of http error requests",
			httpserverLabels).MustCurryWith(commonLabels),
		RequestsDuration: prometheushelper.NewHistogram("httpserver_requests_duration",
			"request processing duration histogram",
			httpserverLabels).MustCurryWith(commonLabels),
		RequestSizeBytes: prometheushelper.NewHistogram("httpserver_requests_size_bytes",
			"a histogram of the total size of the request. Includes body",
			httpserverLabels).MustCurryWith(commonLabels),
		ResponseSizeBytes: prometheushelper.NewHistogram("httpserver_response_size_bytes",
			"a histogram of the total size of the returned response body",
			httpserverLabels).MustCurryWith(commonLabels),
	}
}

func (mi *muxInstance) exportPrometheusMetrics(stat *httpstat.Metric, backend string) {
	labels := prometheus.Labels{
		"routerKind": mi.spec.RouterKind,
		"backend":    backend,
	}
	mi.metrics.TotalRequests.With(labels).Inc()
	mi.metrics.TotalResponse.With(labels).Inc()
	if stat.StatusCode >= 400 {
		mi.metrics.TotalErrorRequests.With(labels).Inc()
	}
	mi.metrics.RequestsDuration.With(labels).Observe(float64(stat.Duration.Milliseconds()))
	mi.metrics.RequestSizeBytes.With(labels).Observe(float64(stat.ReqSize))
	mi.metrics.ResponseSizeBytes.With(labels).Observe(float64(stat.RespSize))
}

func (r *runtime) exportState(state stateType) {
	if state == stateRunning {
		r.metrics.Health.WithLabelValues().Set(1)
	} else {
		r.metrics.Health.WithLabelValues().Set(0)
	}
}
