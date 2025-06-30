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

package metricshub

import (
	"maps"

	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/prometheushelper"
	"github.com/prometheus/client_golang/prometheus"
)

type Metric struct {
	Success      bool   `json:"success"`
	Duration     int64  `json:"duration"` // in milliseconds
	Provider     string `json:"provider"`
	ProviderType string `json:"providerType"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	Model        string `json:"model"`
	BaseURL      string `json:"baseURL"`
	Error        string `json:"error"`
}

type MetricsHub struct {
	totalRequest    *prometheus.CounterVec
	successRequest  *prometheus.CounterVec
	failedRequest   *prometheus.CounterVec
	requestDuration prometheus.ObserverVec

	promptTokens     *prometheus.CounterVec
	completionTokens *prometheus.CounterVec
}

func New(spec *supervisor.Spec) *MetricsHub {
	commonLabels := prometheus.Labels{
		"kind":         "AIGatewayController",
		"clusterName":  spec.Super().Options().ClusterName,
		"clusterRole":  spec.Super().Options().ClusterRole,
		"instanceName": spec.Super().Options().Name,
	}
	labels := []string{"provider", "providerType", "baseUrl", "model"}
	return &MetricsHub{
		totalRequest: prometheushelper.NewCounter(
			"aigateway_total_request",
			"Total number of requests received by AIGatewayController",
			labels,
		).MustCurryWith(commonLabels),
		successRequest: prometheushelper.NewCounter(
			"aigateway_success_request",
			"Total number of successful requests processed by AIGatewayController",
			labels,
		).MustCurryWith(commonLabels),
		failedRequest: prometheushelper.NewCounter(
			"aigateway_failed_request",
			"Total number of failed requests processed by AIGatewayController",
			append(labels, "error"),
		).MustCurryWith(commonLabels),
		requestDuration: prometheushelper.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "aigateway_requests_duration",
				Help:    "Request processing duration histogram of a provider by AIGatewayController",
				Buckets: prometheushelper.DefaultDurationBuckets(),
			},
			labels,
		).MustCurryWith(commonLabels),
		promptTokens: prometheushelper.NewCounter(
			"aigateway_prompt_tokens",
			"Total number of prompt tokens processed by AIGatewayController",
			labels,
		).MustCurryWith(commonLabels),
		completionTokens: prometheushelper.NewCounter(
			"aigateway_completion_tokens",
			"Total number of completion tokens processed by AIGatewayController",
			labels,
		).MustCurryWith(commonLabels),
	}
}

func (m *MetricsHub) Update(metric *Metric) {
	if metric == nil {
		return
	}

	labels := prometheus.Labels{
		"provider":     metric.Provider,
		"providerType": metric.ProviderType,
		"baseUrl":      metric.BaseURL,
		"model":        metric.Model,
	}

	m.totalRequest.With(labels).Inc()
	if !metric.Success {
		newLabels := maps.Clone(labels)
		newLabels["error"] = metric.Error
		m.failedRequest.With(newLabels).Inc()
		return
	}

	m.successRequest.With(labels).Inc()
	m.requestDuration.With(labels).Observe(float64(metric.Duration))
	m.promptTokens.With(labels).Add(float64(metric.InputTokens))
	m.completionTokens.With(labels).Add(float64(metric.OutputTokens))
}
