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

// Package prometheushelper provides helper functions for prometheus.
package prometheushelper

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

const module = "prometheus_helper"

var (
	counterMap   = make(map[string]*prometheus.CounterVec)
	gaugeMap     = make(map[string]*prometheus.GaugeVec)
	histogramMap = make(map[string]*prometheus.HistogramVec)
	summaryMap   = make(map[string]*prometheus.SummaryVec)
	lock         = sync.Mutex{}
)

var (
	validMetric = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	validLabel  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// NewCounter creates a counter metric
func NewCounter(metric string, help string, labels []string) *prometheus.CounterVec {
	lock.Lock()
	defer lock.Unlock()

	metricName, err := getAndValidate(metric, labels)
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

// NewGauge creates a gauge metric
func NewGauge(metric string, help string, labels []string) *prometheus.GaugeVec {
	lock.Lock()
	defer lock.Unlock()

	metricName, err := getAndValidate(metric, labels)
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

// NewHistogram creates a Histogram metric
func NewHistogram(opt prometheus.HistogramOpts, labels []string) *prometheus.HistogramVec {
	lock.Lock()
	defer lock.Unlock()

	metricName, err := getAndValidate(opt.Name, labels)
	if err != nil {
		logger.Errorf("[%s] %v", module, err)
		return nil
	}

	if m, find := histogramMap[metricName]; find {
		logger.Debugf("[%s] Histogram <%s> already created!", module, metricName)
		return m
	}
	histogramMap[metricName] = prometheus.NewHistogramVec(
		opt,
		labels,
	)
	prometheus.MustRegister(histogramMap[metricName])

	logger.Infof("[%s] Histogram <%s> already created!", module, metricName)
	return histogramMap[metricName]
}

// NewSummary creates a NewSummary metric
func NewSummary(opt prometheus.SummaryOpts, labels []string) *prometheus.SummaryVec {
	lock.Lock()
	defer lock.Unlock()

	metricName, err := getAndValidate(opt.Name, labels)
	if err != nil {
		logger.Errorf("[%s] %v", module, err)
		return nil
	}

	if m, find := summaryMap[metricName]; find {
		logger.Debugf("[%s] Summary <%s> already created!", module, metricName)
		return m
	}
	summaryMap[metricName] = prometheus.NewSummaryVec(
		opt,
		labels,
	)
	prometheus.MustRegister(summaryMap[metricName])

	logger.Infof("[%s] Summary <%s> already created!", module, metricName)
	return summaryMap[metricName]
}

func getAndValidate(metricName string, labels []string) (string, error) {
	if !ValidateMetricName(metricName) {
		return "", fmt.Errorf("invalid metric name: %s", metricName)
	}

	for _, l := range labels {
		if !ValidateLabelName(l) {
			return "", fmt.Errorf("invalid label name: %s", l)
		}
	}
	return metricName, nil
}

// ValidateMetricName checks if the metric name is valid
func ValidateMetricName(name string) bool {
	return validMetric.MatchString(name)
}

// ValidateLabelName checks if the label name is valid
func ValidateLabelName(label string) bool {
	return validLabel.MatchString(label)
}

// DefaultDurationBuckets returns default duration buckets in milliseconds
func DefaultDurationBuckets() []float64 {
	return []float64{10, 50, 100, 200, 400, 800, 1000, 2000, 4000, 8000}
}

// DefaultBodySizeBuckets returns default body size buckets in bytes
func DefaultBodySizeBuckets() []float64 {
	return prometheus.ExponentialBucketsRange(200, 400000, 10)
}

// DefaultObjectives returns default summary objectives
func DefaultObjectives() map[float64]float64 {
	return map[float64]float64{0.25: 10, 0.5: 10, 0.75: 10, 0.9: 10, 0.95: 10, 0.99: 10}
}
