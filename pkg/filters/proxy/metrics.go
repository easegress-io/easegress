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

package proxy

import (
	"github.com/megaease/easegress/pkg/protocols/httpprot/httpstat"
	"github.com/megaease/easegress/pkg/util/prometheushelper"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	// ProxyMetrics is the Prometheus Metrics of ProxyMetrics Object.
	metrics struct {
		TotalConnections      *prometheus.CounterVec
		TotalErrorConnections *prometheus.CounterVec
		RequestBodySize       prometheus.ObserverVec
		ResponseBodySize      prometheus.ObserverVec
	}
)

// newMetrics create the ProxyMetrics.
func (p *Proxy) newMetrics(name string) *metrics {
	commonLabels := prometheus.Labels{
		"name":         name,
		"kind":         Kind,
		"clusterName":  p.super.Options().ClusterName,
		"clusterRole":  p.super.Options().ClusterRole,
		"instanceName": p.super.Options().Name,
	}
	proxyLabels := []string{"clusterName", "clusterRole", "instanceName",
		"name", "kind", "loadBalancePolicy", "filterPolicy"}
	return &metrics{
		TotalConnections: prometheushelper.NewCounter("proxy_total_connections",
			"the total count of proxy connections",
			proxyLabels).MustCurryWith(commonLabels),
		TotalErrorConnections: prometheushelper.NewCounter("proxy_total_error_connections",
			"the total count of proxy error connections",
			proxyLabels).MustCurryWith(commonLabels),
		RequestBodySize: prometheushelper.NewHistogram("proxy_request_body_size",
			"a histogram of the total size of the request.",
			proxyLabels).MustCurryWith(commonLabels),
		ResponseBodySize: prometheushelper.NewHistogram(
			"proxy_response_body_size",
			"a histogram of the total size of the response.",
			proxyLabels).MustCurryWith(commonLabels),
	}
}

func (sp *ServerPool) exportPrometheusMetrics(stat *httpstat.Metric) {
	labels := prometheus.Labels{
		"loadBalancePolicy": "",
		"filterPolicy":      "",
	}
	if sp.spec.LoadBalance != nil {
		labels["loadBalancePolicy"] = sp.spec.LoadBalance.Policy
	}
	if sp.spec.Filter != nil {
		labels["filterPolicy"] = sp.spec.Filter.Policy
	}
	sp.metrics.TotalConnections.With(labels).Inc()
	if stat.StatusCode >= 400 {
		sp.metrics.TotalErrorConnections.With(labels).Inc()
	}
	sp.metrics.RequestBodySize.With(labels).Observe(float64(stat.ReqSize))
	sp.metrics.ResponseBodySize.With(labels).Observe(float64(stat.RespSize))
}
