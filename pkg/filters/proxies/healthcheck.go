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

package proxies

import (
	"net/http"
	"time"
)

// HealthCheckSpec is the spec for health check.
type HealthCheckSpec struct {
	// Interval is the interval duration for health check.
	Interval string `json:"interval" jsonschema:"omitempty,format=duration"`
	// Path is the health check path for server
	Path string `json:"path" jsonschema:"omitempty"`
	// Timeout is the timeout duration for health check, default is 3.
	Timeout string `json:"timeout" jsonschema:"omitempty,format=duration"`
	// Fails is the consecutive fails count for assert fail, default is 1.
	Fails int `json:"fails" jsonschema:"omitempty,minimum=1"`
	// Passes is the consecutive passes count for assert pass, default is 1.
	Passes int `json:"passes" jsonschema:"omitempty,minimum=1"`
}

// HealthChecker checks whether a server is healthy or not.
type HealthChecker interface {
	Check(svr *Server) bool
	Close()
}

// HTTPHealthChecker is a health checker for HTTP protocol.
type HTTPHealthChecker struct {
	path   string
	client *http.Client
}

// NewHTTPHealthChecker creates a new HTTPHealthChecker.
func NewHTTPHealthChecker(spec *HealthCheckSpec) HealthChecker {
	timeout, _ := time.ParseDuration(spec.Timeout)
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	return &HTTPHealthChecker{
		path:   spec.Path,
		client: &http.Client{Timeout: timeout},
	}
}

// Check checks whether a server is healthy or not.
func (hc *HTTPHealthChecker) Check(svr *Server) bool {
	// TODO: should use url.JoinPath?
	url := svr.URL + hc.path
	resp, err := hc.client.Get(url)
	return err == nil && resp.StatusCode < 500
}

// Close closes the health checker
func (hc *HTTPHealthChecker) Close() {
}
