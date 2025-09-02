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

package fileserver

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/megaease/easegress/v2/pkg/util/prometheushelper"
)

type (
	bufferPoolMetrics struct {
		BufferPoolHits        *prometheus.CounterVec
		BufferPoolMiss        *prometheus.CounterVec
		BufferPoolEvicts      *prometheus.CounterVec
		BufferPoolTTLEvict    *prometheus.CounterVec
		BufferPoolCounterSize *prometheus.GaugeVec
		BufferPoolFiles       *prometheus.GaugeVec
	}

	mmapMetrics struct {
		MMapHits  *prometheus.CounterVec
		MMapMiss  *prometheus.CounterVec
		MMapFiles *prometheus.GaugeVec
	}
)

func (bp *BufferPool) newMetrics() *bufferPoolMetrics {
	m := &bufferPoolMetrics{
		BufferPoolHits: prometheushelper.NewCounter(
			"fileserver_buffer_pool_hits_total",
			"Total number of buffer pool hits",
			[]string{},
		),
		BufferPoolMiss: prometheushelper.NewCounter(
			"fileserver_buffer_pool_miss_total",
			"Total number of buffer pool miss",
			[]string{},
		),
		BufferPoolEvicts: prometheushelper.NewCounter(
			"fileserver_buffer_pool_evicts_total",
			"Total number of buffer pool evicts",
			[]string{},
		),
		BufferPoolTTLEvict: prometheushelper.NewCounter(
			"fileserver_buffer_pool_ttl_evicts_total",
			"Total number of buffer pool ttl evicts",
			[]string{},
		),
		BufferPoolCounterSize: prometheushelper.NewGauge(
			"fileserver_buffer_pool_size",
			"Current size of buffer pool in bytes",
			[]string{},
		),
		BufferPoolFiles: prometheushelper.NewGauge(
			"fileserver_buffer_pool_files",
			"Current number of files in buffer pool",
			[]string{},
		),
	}
	return m
}

func (mc *MmapCache) newMetrics() *mmapMetrics {
	mm := &mmapMetrics{
		MMapHits: prometheushelper.NewCounter(
			"fileserver_mmap_hits_total",
			"Total number of mmap hits",
			[]string{},
		),
		MMapMiss: prometheushelper.NewCounter(
			"fileserver_mmap_miss_total",
			"Total number of mmap miss",
			[]string{},
		),
		MMapFiles: prometheushelper.NewGauge(
			"fileserver_mmap_files",
			"Current number of files in mmap cache",
			[]string{},
		),
	}
	return mm
}
