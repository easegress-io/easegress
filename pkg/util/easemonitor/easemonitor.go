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

// Package easemonitor provides the common fields and interfaces for EaseMonitor metrics.
package easemonitor

import (
	"github.com/megaease/easegress/v2/pkg/util/codectool"
)

type (
	// CommonFields is the common fields of all EaseMonitor metrics.
	CommonFields struct {
		Timestamp int64  `json:"timestamp"`
		Category  string `json:"category"`
		HostName  string `json:"host_name"`
		HostIpv4  string `json:"host_ipv4"`
		System    string `json:"system"`
		Service   string `json:"service"`
		Type      string `json:"type"`
		Resource  string `json:"resource"`
		URL       string `json:"url,omitempty"`
	}

	// Metrics is an EaseMonitor metrics.
	Metrics struct {
		CommonFields
		OtherFields interface{}
	}

	// Metricer is the interface to convert metrics to EaseMonitor format.
	Metricer interface {
		ToMetrics(service string) []*Metrics
	}
)

// MarshalJSON implements json.Marshaler
func (m *Metrics) MarshalJSON() ([]byte, error) {
	result, err := codectool.MarshalJSON(&m.CommonFields)
	if err != nil {
		return nil, err
	}

	other, err := codectool.MarshalJSON(m.OtherFields)
	if err != nil {
		return nil, err
	}

	result[len(result)-1] = ','
	result = append(result, other[1:]...)
	return result, nil
}
