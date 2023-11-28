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

package httpproxy

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/logger"
)

const (
	regexpType   = "regexp"
	exactType    = "exact"
	containsType = "contains"
)

type HealthCheckSpec struct {
	proxies.HealthCheckSpec `json:",inline"`

	URI      string            `json:"uri,omitempty"`
	Method   string            `json:"method,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`

	Match *HealthCheckMatch `json:"match,omitempty"`
}

type HealthCheckMatch struct {
	StatusCode [][]int                  `json:"statusCode,omitempty"`
	Headers    []HealthCheckHeaderMatch `json:"headers,omitempty"`
	Body       *HealthCheckBodyMatch    `json:"body,omitempty"`
}

type HealthCheckHeaderMatch struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
	// Type is the match type, exact or regex
	Type string `json:"type,omitempty"`
	re   *regexp.Regexp
}

type HealthCheckBodyMatch struct {
	Value string `json:"value,omitempty"`
	// Type is the match type, contains or regex
	Type string `json:"type,omitempty"`
	re   *regexp.Regexp
}

var httpMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodHead:    {},
	http.MethodPost:    {},
	http.MethodPut:     {},
	http.MethodPatch:   {},
	http.MethodDelete:  {},
	http.MethodConnect: {},
	http.MethodOptions: {},
	http.MethodTrace:   {},
}

func (spec *HealthCheckSpec) Validate() error {
	if spec.URI != "" {
		_, err := url.Parse(spec.URI)
		if err != nil {
			return err
		}
	}
	if spec.Method != "" {
		if _, ok := httpMethods[spec.Method]; !ok {
			return fmt.Errorf("invalid method: %s", spec.Method)
		}
	}
	if spec.Username != "" && spec.Password == "" {
		return fmt.Errorf("empty password")
	}
	if spec.Username == "" && spec.Password != "" {
		return fmt.Errorf("empty username")
	}
	if spec.Match != nil {
		if len(spec.Match.StatusCode) != 0 {
			for _, s := range spec.Match.StatusCode {
				if len(s) != 2 {
					return fmt.Errorf("invalid status code range: %v", s)
				}
				if s[0] > s[1] {
					return fmt.Errorf("invalid status code range: %v", s)
				}
			}
		}
		if len(spec.Match.Headers) != 0 {
			for _, h := range spec.Match.Headers {
				if h.Name == "" {
					return fmt.Errorf("empty match header name")
				}
				if h.Type != "" {
					if h.Type != exactType && h.Type != regexpType {
						return fmt.Errorf("invalid match header type: %s", h.Type)
					}
				}
				if h.Type == regexpType {
					_, err := regexp.Compile(h.Value)
					if err != nil {
						return fmt.Errorf("invalid match header regex: %s", h.Value)
					}
				}
			}
		}
		if spec.Match.Body != nil {
			if spec.Match.Body.Type != "" {
				if spec.Match.Body.Type != containsType && spec.Match.Body.Type != regexpType {
					return fmt.Errorf("invalid match body type: %s", spec.Match.Body.Type)
				}
			}
			if spec.Match.Body.Type == regexpType {
				_, err := regexp.Compile(spec.Match.Body.Value)
				if err != nil {
					return fmt.Errorf("invalid match body regex: %s", spec.Match.Body.Value)
				}
			}
		}
	}
	return nil
}

type healthChecker struct {
	spec   *HealthCheckSpec
	client *http.Client
}

func (hc *healthChecker) BaseSpec() proxies.HealthCheckSpec {
	return hc.spec.HealthCheckSpec
}

func (hc *healthChecker) Check(server *proxies.Server) bool {
	s := hc.spec
	req, err := http.NewRequest(s.Method, server.URL+s.URI, bytes.NewReader([]byte(s.Body)))
	if err != nil {
		logger.Errorf("create health check request %s %s failed: %v", s.Method, server.URL+s.URI, err)
		return false
	}
	if len(s.Username) > 0 {
		req.SetBasicAuth(s.Username, s.Password)
	}
	for k, v := range s.Headers {
		if strings.EqualFold(k, "host") {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}
	// client close the connection
	req.Close = true
	resp, err := hc.client.Do(req)
	if err != nil {
		logger.Warnf("health check %s %s failed: %v", s.Method, server.URL+s.URI, err)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warnf("health check %s %s failed: %v", s.Method, server.URL+s.URI, err)
		return false
	}
	match := s.Match
	var valid bool
	for _, r := range match.StatusCode {
		if resp.StatusCode >= r[0] && resp.StatusCode <= r[1] {
			valid = true
			break
		}
	}
	if !valid {
		logger.Warnf("health check %s %s failed: invalid status code %d", s.Method, server.URL+s.URI, resp.StatusCode)
		return false
	}
	for _, h := range match.Headers {
		v := resp.Header.Get(h.Name)
		if h.re != nil {
			if !h.re.MatchString(v) {
				logger.Warnf("health check %s %s failed: invalid header %s: %s", s.Method, server.URL+s.URI, h.Name, v)
				return false
			}
		} else {
			if v != h.Value {
				logger.Warnf("health check %s %s failed: invalid header %s: %s", s.Method, server.URL+s.URI, h.Name, v)
				return false
			}
		}
	}
	if match.Body != nil {
		if match.Body.re != nil {
			if !match.Body.re.MatchString(string(body)) {
				logger.Warnf("health check %s %s failed: invalid body: %s", s.Method, server.URL+s.URI, string(body))
				return false
			}
		} else {
			if !strings.Contains(string(body), match.Body.Value) {
				logger.Warnf("health check %s %s failed: invalid body: %s", s.Method, server.URL+s.URI, string(body))
				return false
			}
		}
	}
	return true
}

func (hc *healthChecker) Close() {}

func NewHTTPHealthChecker(tlsConfig *tls.Config, spec *HealthCheckSpec) proxies.HealthChecker {
	if spec == nil {
		return nil
	}
	timeout, _ := time.ParseDuration(spec.Timeout)
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if spec.Method == "" {
		spec.Method = http.MethodGet
	}
	if spec.URI == "" && spec.Path != "" {
		spec.URI = spec.Path
	}
	if spec.Match != nil {
		if len(spec.Match.StatusCode) == 0 {
			spec.Match.StatusCode = [][]int{{200, 399}}
		}
		for i := range spec.Match.Headers {
			h := &spec.Match.Headers[i]
			if h.Type == regexpType {
				h.re = regexp.MustCompile(h.Value)
			}
		}
		if spec.Match.Body != nil {
			if spec.Match.Body.Type == regexpType {
				spec.Match.Body.re = regexp.MustCompile(spec.Match.Body.Value)
			}
		}
	} else {
		spec.Match = &HealthCheckMatch{
			StatusCode: [][]int{{200, 399}},
		}
	}
	transport := &http.Transport{
		TLSClientConfig:   tlsConfig,
		DisableKeepAlives: true,
		Proxy:             http.ProxyFromEnvironment, // use proxy from environment variables
	}
	return &healthChecker{
		spec: spec,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}
}
