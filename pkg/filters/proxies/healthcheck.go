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

package proxies

import "time"

// HealthCheckSpec is the spec for health check.
type HealthCheckSpec struct {
	// Interval is the interval duration for health check.
	Interval string `json:"interval,omitempty" jsonschema:"format=duration"`
	// Timeout is the timeout duration for health check, default is 3.
	Timeout string `json:"timeout,omitempty" jsonschema:"format=duration"`
	// Fails is the consecutive fails count for assert fail, default is 1.
	Fails int `json:"fails,omitempty" jsonschema:"minimum=1"`
	// Passes is the consecutive passes count for assert pass, default is 1.
	Passes int `json:"passes,omitempty" jsonschema:"minimum=1"`

	// Deprecated: other fields in this struct are general, but this field is
	// specific to HTTP health check. It should be moved to HTTP health check.
	// In HTTP health check, we should use URI instead of path.
	Path string `json:"path,omitempty"`
}

// GetTimeout returns the timeout duration.
func (s *HealthCheckSpec) GetTimeout() time.Duration {
	timeout, _ := time.ParseDuration(s.Timeout)
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return timeout
}

// GetInterval returns the interval duration.
func (s *HealthCheckSpec) GetInterval() time.Duration {
	interval, _ := time.ParseDuration(s.Interval)
	if interval <= 0 {
		interval = time.Minute
	}
	return interval
}

// HealthChecker checks whether a server is healthy or not.
type HealthChecker interface {
	BaseSpec() HealthCheckSpec
	Check(svr *Server) bool
	Close()
}
