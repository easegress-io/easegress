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

package api

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// PrometheusMetricsPrefix is the prefix of Prometheus metrics exporter
	PrometheusMetricsPrefix = "/metrics"
)

func (s *Server) prometheusMetricsAPIEntries() []*Entry {
	return []*Entry{
		{
			Path:    PrometheusMetricsPrefix,
			Method:  "GET",
			Handler: promhttp.Handler().ServeHTTP,
		},
	}
}
