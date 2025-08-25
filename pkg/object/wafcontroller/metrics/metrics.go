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

package metrics

import (
	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/supervisor"
	"github.com/megaease/easegress/v2/pkg/util/prometheushelper"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	Metric struct {
		// TotalRefusedRequests is the total number of refused requests.
		TotalRefusedRequest int64
		RuleGroup           string
		RuleID              string
		Action              string
	}

	metricEvent struct {
		metric  *Metric
		statsCh chan []*MetricStats
	}

	// Metrics defines the interface for WAF metrics.
	MetricHub struct {
		// TotalRefusedRequests is the total number of refused requests.
		TotalRefusedRequests *prometheus.CounterVec

		spec    *supervisor.Spec
		stats   map[MetricLabels]*MetricDetails
		eventCh chan *metricEvent
	}

	MetricLabels struct {
		RuleGroup string
		RuleID    string
		Action    string
	}

	MetricDetails struct {
		TotalRefusedRequests int64
	}

	MetricStats struct {
		MetricLabels  `json:",inline"`
		MetricDetails `json:",inline"`
	}
)

func NewMetrics(spec *supervisor.Spec) *MetricHub {
	commonLabels := prometheus.Labels{
		"kind":         "AIGatewayController",
		"clusterName":  spec.Super().Options().ClusterName,
		"clusterRole":  spec.Super().Options().ClusterRole,
		"instanceName": spec.Super().Options().Name,
	}
	labels := []string{
		// common labels
		"kind", "clusterName", "clusterRole", "instanceName",
		// metric labels
		"ruleGroup", "ruleID", "action",
	}
	hub := &MetricHub{
		TotalRefusedRequests: prometheushelper.NewCounter(
			"waf_total_refused_requests",
			"Total number of refused requests by WAF",
			labels,
		).MustCurryWith(commonLabels),

		spec:    spec,
		stats:   make(map[MetricLabels]*MetricDetails),
		eventCh: make(chan *metricEvent, 100),
	}
	logger.Infof("WAF metrics initialized for WAFController")
	go hub.run()
	return hub
}

func (m *MetricHub) run() {
	for {
		select {
		case event := <-m.eventCh:
			if event != nil {
				logger.Infof("Stopping WAF metrics collection")
				return
			}
			if event.statsCh != nil {
				event.statsCh <- m.currentStats()
			} else if event.metric != nil {
				m.updateMetrics(event.metric)
			}
		}
	}
}

func (m *MetricHub) updateMetrics(metric *Metric) {
	labels := MetricLabels{
		RuleGroup: metric.RuleGroup,
		RuleID:    metric.RuleID,
		Action:    metric.Action,
	}
	var details *MetricDetails
	if _, exists := m.stats[labels]; !exists {
		details = &MetricDetails{}
		m.stats[labels] = details
	}

	details = m.stats[labels]
	details.TotalRefusedRequests++
}

func (m *MetricHub) currentStats() []*MetricStats {
	stats := make([]*MetricStats, 0, len(m.stats))
	for labels, details := range m.stats {
		stats = append(stats, &MetricStats{
			MetricLabels:  labels,
			MetricDetails: *details,
		})
	}
	return stats
}

func (m *MetricHub) sendEvent(event *metricEvent) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Recovered from panic in WAF metrics event handler: %v", r)
		}
	}()

	m.eventCh <- event
}

func (m *MetricHub) Update(metric *Metric) {
	if metric == nil {
		return
	}

	m.sendEvent(&metricEvent{
		metric: metric,
	})

	labels := prometheus.Labels{
		"ruleGroup": metric.RuleGroup,
		"ruleID":    metric.RuleID,
		"action":    metric.Action,
	}

	m.TotalRefusedRequests.With(labels).Inc()
}

func (m *MetricHub) GetStats() []*MetricStats {
	ch := make(chan []*MetricStats, 1)

	m.sendEvent(&metricEvent{
		statsCh: ch,
	})

	stats := <-ch
	if stats == nil {
		return nil
	}
	return stats
}

func (m *MetricHub) Close() {
	logger.Infof("Closing WAF metrics for WAFController")
	m.sendEvent(&metricEvent{
		metric:  nil,
		statsCh: nil,
	})
	close(m.eventCh)
	// m.eventCh = nil
	m.stats = nil
	logger.Infof("WAF metrics closed for WAFController")
}
