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
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/prometheushelper"
	"github.com/prometheus/client_golang/prometheus"
)

const saveMetricSnapshotInterval = 5 * time.Second

type (
	// Metric represents a single API call's original metric information.
	Metric struct {
		Success      bool   `json:"success"`
		Duration     int64  `json:"duration"` // in milliseconds
		Provider     string `json:"provider"`
		ProviderType string `json:"providerType"`
		InputTokens  int64  `json:"inputTokens"`
		OutputTokens int64  `json:"outputTokens"`
		Model        string `json:"model"`
		BaseURL      string `json:"baseURL"`
		ResponseType string `json:"responseType"`
		Error        string `json:"error"`
	}

	metricEvent struct {
		metric  *Metric
		statsCh chan []*MetricStats
	}

	// MetricsHub manages collection, aggregation, and exposure of all metrics.
	MetricsHub struct {
		totalRequest    *prometheus.CounterVec
		successRequest  *prometheus.CounterVec
		failedRequest   *prometheus.CounterVec
		requestDuration prometheus.ObserverVec

		promptTokens     *prometheus.CounterVec
		completionTokens *prometheus.CounterVec

		spec *supervisor.Spec
		// stats is lock-free, please access it through run goroutine only.
		stats   map[MetricLabel]*MetricDetails
		eventCh chan *metricEvent
	}

	// MetricLabel uniquely identifies a set of metric statistics by its labels.
	MetricLabel struct {
		Provider     string `json:"provider"`
		ProviderType string `json:"providerType"`
		BaseURL      string `json:"baseURL"`
		Model        string `json:"model"`
		RespType     string `json:"respType"`
	}

	// MetricDetails stores detailed statistics for a particular label.
	MetricDetails struct {
		TotalRequests          int64 `json:"totalRequests"`
		SuccessRequests        int64 `json:"successRequests"`
		FailedRequests         int64 `json:"failedRequests"`
		SuccessRequestDuration int64 `json:"successRequestDuration"`
		PromptTokens           int64 `json:"promptTokens"`
		CompletionTokens       int64 `json:"completionTokens"`
	}

	// MetricStats combines MetricLabel and MetricDetails, and includes average request duration.
	MetricStats struct {
		MetricLabel            `json:",inline"`
		MetricDetails          `json:",inline"`
		RequestAverageDuration int64 `json:"requestAverageDuration"`
	}

	// MetricSnapshot captures a full snapshot of all metrics at a specific timestamp.
	MetricSnapshot struct {
		Timestamp int64          `json:"timestamp"`
		Metrics   []*MetricStats `json:"metrics"`
	}
)

// New create a new MetricsHub instance.
func New(spec *supervisor.Spec) *MetricsHub {
	commonLabels := prometheus.Labels{
		"kind":         "AIGatewayController",
		"clusterName":  spec.Super().Options().ClusterName,
		"clusterRole":  spec.Super().Options().ClusterRole,
		"instanceName": spec.Super().Options().Name,
	}
	labels := []string{"provider", "providerType", "baseUrl", "model", "respType"}
	hub := &MetricsHub{
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

		spec:    spec,
		stats:   make(map[MetricLabel]*MetricDetails),
		eventCh: make(chan *metricEvent, 5000),
	}
	go hub.run()
	return hub
}

func (m *MetricsHub) run() {
	ticker := time.NewTicker(saveMetricSnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case event := <-m.eventCh:
			if event == nil {
				// eventCh is closed and no more events will be sent.
				logger.Infof("MetricsHub event channel closed, stopping working goroutine")
				return
			}
			if event.statsCh != nil {
				event.statsCh <- m.currentStats()
			} else if event.metric != nil {
				m.updateStats(event.metric)
			}
		case <-ticker.C:
			m.saveStats()
		}
	}
}

func (m *MetricsHub) updateStats(metric *Metric) {
	label := MetricLabel{
		Provider:     metric.Provider,
		ProviderType: metric.ProviderType,
		BaseURL:      metric.BaseURL,
		Model:        metric.Model,
		RespType:     metric.ResponseType,
	}
	var details *MetricDetails
	var ok bool
	if details, ok = m.stats[label]; !ok {
		details = &MetricDetails{}
		m.stats[label] = details
	}

	details.TotalRequests++
	if !metric.Success {
		details.FailedRequests++
		return
	}

	details.SuccessRequests++
	details.SuccessRequestDuration += metric.Duration
	details.PromptTokens += metric.InputTokens
	details.CompletionTokens += metric.OutputTokens
}

func (m *MetricsHub) currentStats() []*MetricStats {
	stats := make([]*MetricStats, 0, len(m.stats))
	for label, details := range m.stats {
		avgDuration := int64(0)
		if details.SuccessRequests > 0 {
			avgDuration = details.SuccessRequestDuration / details.SuccessRequests
		}
		stats = append(stats, &MetricStats{
			MetricLabel:            label,
			MetricDetails:          *details,
			RequestAverageDuration: avgDuration,
		})
	}
	return stats
}

func (m *MetricsHub) saveStats() {
	stats := m.currentStats()
	if len(stats) == 0 {
		return
	}

	snapshot := &MetricSnapshot{
		Timestamp: time.Now().UnixMilli(),
		Metrics:   stats,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		logger.Errorf("failed to marshal AI gateway metrics: %v", err)
		return
	}

	cluster := m.spec.Super().Cluster()
	key := cluster.Layout().AIGatewayStatsKey()
	if err := cluster.Put(key, string(data)); err != nil {
		logger.Errorf("failed to save AI gateway metrics to store: %v", err)
	}
}

func (m *MetricsHub) sendEvent(event *metricEvent) (err error) {
	// it only panics when send event to a close channel when metrichub is closed.
	defer func() {
		var ok bool
		if r := recover(); r != nil {
			if err, ok = r.(error); ok {
				return
			} else {
				err = fmt.Errorf("failed to send MetricsHub event: %v", r)
			}
		}
	}()
	m.eventCh <- event
	return
}

// Update updates the metrics with the given metric data.
func (m *MetricsHub) Update(metric *Metric) {
	if metric == nil {
		return
	}

	m.sendEvent(&metricEvent{
		metric: metric,
	})

	labels := prometheus.Labels{
		"provider":     metric.Provider,
		"providerType": metric.ProviderType,
		"baseUrl":      metric.BaseURL,
		"model":        metric.Model,
		"respType":     metric.ResponseType,
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

// GetStats returns the current stats of AI gateway metrics.
func (m *MetricsHub) GetStats() []*MetricStats {
	ch := make(chan []*MetricStats, 1)

	err := m.sendEvent(&metricEvent{
		statsCh: ch,
	})
	if err != nil {
		logger.Errorf("failed to get AI gateway metrics, send event failed: %v", err)
		return nil
	}

	result := <-ch
	return result
}

// GetAllStats get all stats from the store, it will merge the stats from all members in the cluster.
func (m *MetricsHub) GetAllStats() ([]MetricStats, error) {
	cluster := m.spec.Super().Cluster()
	key := cluster.Layout().AIGatewayStatsPrefix()
	data, err := cluster.GetPrefix(key)
	if err != nil {
		logger.Errorf("failed to get AI gateway metrics from store: %v", err)
		return nil, err
	}

	allMetricMap := map[MetricLabel]*MetricDetails{}

	for key, item := range data {
		snapshot := MetricSnapshot{}
		if err := json.Unmarshal([]byte(item), &snapshot); err != nil {
			logger.Errorf("failed to unmarshal AI gateway metrics: %v", err)
			return nil, err
		}

		if time.Now().UnixMilli()-snapshot.Timestamp > 10*saveMetricSnapshotInterval.Milliseconds() {
			logger.Infof("AIGateway %s seems offline, remove its metrics", key)
			cluster.Delete(key)
			continue
		}

		for _, stat := range snapshot.Metrics {
			label := MetricLabel{
				Provider:     stat.Provider,
				ProviderType: stat.ProviderType,
				BaseURL:      stat.BaseURL,
				Model:        stat.Model,
				RespType:     stat.RespType,
			}
			var details *MetricDetails
			var ok bool
			if details, ok = allMetricMap[label]; !ok {
				details = &MetricDetails{}
				allMetricMap[label] = details
			}
			details.TotalRequests += stat.TotalRequests
			details.SuccessRequests += stat.SuccessRequests
			details.FailedRequests += stat.FailedRequests
			details.SuccessRequestDuration += stat.SuccessRequestDuration
			details.PromptTokens += stat.PromptTokens
			details.CompletionTokens += stat.CompletionTokens
		}
	}
	if len(allMetricMap) == 0 {
		return []MetricStats{}, nil
	}
	res := make([]MetricStats, 0, len(allMetricMap))
	for label, details := range allMetricMap {
		avgDuration := int64(0)
		if details.SuccessRequests > 0 {
			avgDuration = details.SuccessRequestDuration / details.SuccessRequests
		}
		res = append(res, MetricStats{
			MetricLabel:            label,
			MetricDetails:          *details,
			RequestAverageDuration: avgDuration,
		})
	}
	return res, nil
}

// Close closes the MetricsHub and stops the goroutine.
func (m *MetricsHub) Close() {
	close(m.eventCh)
}
