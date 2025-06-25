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

type Metric struct {
	Success      bool   `json:"success"`
	Duration     int64  `json:"duration"` // in milliseconds
	Provider     string `json:"provider"`
	ProviderType string `json:"providerType"`
	InputTokens  int64  `json:"inputTokens"`
	OutputTokens int64  `json:"outputTokens"`
	Model        string `json:"model"`
	BaseURL      string `json:"baseURL"`
}

type MetricsHub struct {
}

func New() *MetricsHub {
	return &MetricsHub{}
}

func (m *MetricsHub) Update(metric *Metric) {
	// TODO: Implement the logic to update the metrics hub with the provided metric.
}
