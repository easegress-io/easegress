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

package prometheushelper

import (
	"fmt"
	"regexp"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

const module = "prometheus_helper"

var (
	counterMap   = make(map[string]*prometheus.CounterVec)
	gaugeMap     = make(map[string]*prometheus.GaugeVec)
	histogramMap = make(map[string]*prometheus.HistogramVec)
	summaryMap   = make(map[string]*prometheus.SummaryVec)
)

var (
	validMetric = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	validLabel  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// NewCounter create the counter metric
func NewCounter(metric string, help string, labels []string) *prometheus.
	CounterVec {

	metricName, err := getAndValid(metric, labels)
	if err != nil {
		logger.Errorf("[%s] %v", module, err)
		return nil
	}

	if m, find := counterMap[metricName]; find {
		logger.Debugf("[%s] Counter <%s> already created!", module, metricName)
		return m
	}

	counterMap[metricName] = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: metricName,
			Help: help,
		},
		labels,
	)
	prometheus.MustRegister(counterMap[metricName])

	logger.Infof("[%s] Counter <%s> is created!", module, metricName)
	return counterMap[metricName]
}

// NewGauge create the gauge metric
func NewGauge(metric string, help string, labels []string) *prometheus.
	GaugeVec {

	metricName, err := getAndValid(metric, labels)
	if err != nil {
		logger.Errorf("[%s] %v", module, err)
		return nil
	}
	if m, find := gaugeMap[metricName]; find {
		logger.Debugf("[%s] Gauge <%s> already created!", module, metricName)
		return m
	}

	gaugeMap[metricName] = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricName,
			Help: help,
		},
		labels,
	)
	prometheus.MustRegister(gaugeMap[metricName])

	logger.Infof("[%s] Gauge <%s> is created!", module, metricName)
	return gaugeMap[metricName]
}

// NewHistogram create the Histogram metric
func NewHistogram(metric string, help string, labels []string) *prometheus.HistogramVec {

	metricName, err := getAndValid(metric, labels)
	if err != nil {
		logger.Errorf("[%s] %v", module, err)
		return nil
	}
	if m, find := histogramMap[metricName]; find {
		logger.Debugf("[%s] Histogram <%s> already created!", module, metricName)
		return m
	}

	histogramMap[metricName] = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: metricName,
			Help: help,
		},
		labels,
	)
	prometheus.MustRegister(histogramMap[metricName])
	logger.Infof("[%s] Histogram <%s> already created!", module, metricName)
	return histogramMap[metricName]
}

// NewSummary create the NewSummary metric
func NewSummary(metric string, help string, labels []string) *prometheus.SummaryVec {

	metricName, err := getAndValid(metric, labels)
	if err != nil {
		logger.Errorf("[%s] %v", module, err)
		return nil
	}
	if m, find := summaryMap[metricName]; find {
		logger.Debugf("[%s] Summary <%s> already created!", module, metricName)
		return m
	}

	summaryMap[metricName] = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: metricName,
			Help: help,
		},
		labels,
	)
	prometheus.MustRegister(summaryMap[metricName])
	logger.Infof("[%s] Summary <%s> already created!", module, metricName)
	return summaryMap[metricName]
}

func getAndValid(metricName string, labels []string) (string, error) {
	if ValidMetricName(metricName) == false {
		return "", fmt.Errorf("Invalid metric name: %s", metricName)
	}

	for _, l := range labels {
		if ValidLabelName(l) == false {
			return "", fmt.Errorf("Invalid label name: %s", l)
		}
	}
	return metricName, nil
}

// ValidMetricName check if the metric name is valid
func ValidMetricName(name string) bool {
	return validMetric.MatchString(name)
}

// ValidLabelName check if the label name is valid
func ValidLabelName(label string) bool {
	return validLabel.MatchString(label)
}
