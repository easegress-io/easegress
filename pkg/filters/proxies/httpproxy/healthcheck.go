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

	"github.com/gorilla/websocket"
	"github.com/megaease/easegress/v2/pkg/filters/proxies"
	"github.com/megaease/easegress/v2/pkg/logger"
)

const (
	regexpType   = "regexp"
	exactType    = "exact"
	containsType = "contains"
)

// ProxyHealthCheckSpec is the spec of http proxy health check.
type ProxyHealthCheckSpec struct {
	proxies.HealthCheckSpec `json:",inline"`
	HTTPHealthCheckSpec     `json:",inline"`
}

// HTTPHealthCheckSpec is the spec of HTTP health check.
type HTTPHealthCheckSpec struct {
	Port     int               `json:"port,omitempty"`
	URI      string            `json:"uri,omitempty"`
	Method   string            `json:"method,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	Username string            `json:"username,omitempty"`
	Password string            `json:"password,omitempty"`

	Match *HealthCheckMatch `json:"match,omitempty"`
}

// HealthCheckMatch is the match spec of health check.
type HealthCheckMatch struct {
	StatusCodes [][]int                  `json:"statusCodes,omitempty"`
	Headers     []HealthCheckHeaderMatch `json:"headers,omitempty"`
	Body        *HealthCheckBodyMatch    `json:"body,omitempty"`
}

// HealthCheckHeaderMatch is the match spec of health check header.
type HealthCheckHeaderMatch struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
	// Type is the match type, exact or regex
	Type string `json:"type,omitempty"`
	re   *regexp.Regexp
}

// HealthCheckBodyMatch is the match spec of health check body.
type HealthCheckBodyMatch struct {
	Value string `json:"value,omitempty"`
	// Type is the match type, contains or regex
	Type string `json:"type,omitempty"`
	re   *regexp.Regexp
}

func (match *HealthCheckMatch) Match(resp *http.Response) error {
	var valid bool
	for _, r := range match.StatusCodes {
		if resp.StatusCode >= r[0] && resp.StatusCode <= r[1] {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid status code %d", resp.StatusCode)
	}
	for _, h := range match.Headers {
		v := resp.Header.Get(h.Name)
		if h.re != nil {
			if !h.re.MatchString(v) {
				return fmt.Errorf("invalid header %s: %s", h.Name, v)
			}
		} else {
			if v != h.Value {
				return fmt.Errorf("invalid header %s: %s", h.Name, v)
			}
		}
	}
	if match.Body != nil {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if match.Body.re != nil {
			if !match.Body.re.MatchString(string(body)) {
				return fmt.Errorf("invalid body: %s", string(body))
			}
		} else {
			if !strings.Contains(string(body), match.Body.Value) {
				return fmt.Errorf("invalid body: %s", string(body))
			}
		}
	}
	return nil
}

func (match *HealthCheckMatch) Validate() error {
	if len(match.StatusCodes) != 0 {
		for _, s := range match.StatusCodes {
			if len(s) != 2 {
				return fmt.Errorf("invalid status code range: %v", s)
			}
			if s[0] > s[1] {
				return fmt.Errorf("invalid status code range: %v", s)
			}
		}
	}
	if len(match.Headers) != 0 {
		for _, h := range match.Headers {
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
	if match.Body != nil {
		if match.Body.Type != "" {
			if match.Body.Type != containsType && match.Body.Type != regexpType {
				return fmt.Errorf("invalid match body type: %s", match.Body.Type)
			}
		}
		if match.Body.Type == regexpType {
			_, err := regexp.Compile(match.Body.Value)
			if err != nil {
				return fmt.Errorf("invalid match body regex: %s", match.Body.Value)
			}
		}
	}
	return nil
}

// Validate validates HTTPHealthCheckSpec.
func (spec *HTTPHealthCheckSpec) Validate() error {
	if spec.Port < 0 {
		return fmt.Errorf("invalid port: %d", spec.Port)
	}
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
		err := spec.Match.Validate()
		if err != nil {
			return err
		}
	}
	return nil
}

// Validate validates HealthCheckSpec.
func (spec *ProxyHealthCheckSpec) Validate() error {
	return spec.HTTPHealthCheckSpec.Validate()
}

type httpHealthChecker struct {
	spec   *ProxyHealthCheckSpec
	uri    *url.URL
	client *http.Client
}

// BaseSpec returns the base spec.
func (hc *httpHealthChecker) BaseSpec() proxies.HealthCheckSpec {
	return hc.spec.HealthCheckSpec
}

// Check checks the health of the server.
func (hc *httpHealthChecker) Check(server *proxies.Server) bool {
	s := hc.spec

	url := getURL(server, hc.uri, hc.spec.Port, false)
	req, err := http.NewRequest(s.Method, url, bytes.NewReader([]byte(s.Body)))
	if err != nil {
		logger.Errorf("create health check request %s %s failed: %v", s.Method, url, err)
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
		logger.Warnf("health check %s %s failed: %v", s.Method, url, err)
		return false
	}
	defer resp.Body.Close()

	err = s.Match.Match(resp)
	if err != nil {
		logger.Warnf("health check %s %s failed: %v", s.Method, url, err)
		return false
	}
	return true
}

// Close closes the health checker.
func (hc *httpHealthChecker) Close() {}

// NewHTTPHealthChecker creates a new HTTP health checker.
func NewHTTPHealthChecker(tlsConfig *tls.Config, spec *ProxyHealthCheckSpec) proxies.HealthChecker {
	if spec == nil {
		return nil
	}
	if spec.Method == "" {
		spec.Method = http.MethodGet
	}
	if spec.URI == "" && spec.Path != "" {
		spec.URI = spec.Path
	}
	var uri *url.URL
	if spec.URI != "" {
		uri, _ = url.Parse(spec.URI)
	}
	if spec.Match != nil {
		if len(spec.Match.StatusCodes) == 0 {
			spec.Match.StatusCodes = [][]int{{200, 399}}
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
			StatusCodes: [][]int{{200, 399}},
		}
	}
	transport := &http.Transport{
		TLSClientConfig:   tlsConfig,
		DisableKeepAlives: true,
		Proxy:             http.ProxyFromEnvironment, // use proxy from environment variables
	}
	return &httpHealthChecker{
		spec: spec,
		uri:  uri,
		client: &http.Client{
			Timeout:   spec.GetTimeout(),
			Transport: transport,
		},
	}
}

// WSProxyHealthCheckSpec is the spec of ws proxy health check.
type WSProxyHealthCheckSpec struct {
	proxies.HealthCheckSpec `json:",inline"`
	HTTP                    *HTTPHealthCheckSpec `json:"http,omitempty"`
	WS                      *WSHealthCheckSpec   `json:"ws,omitempty"`
}

type WSHealthCheckSpec struct {
	Port    int               `json:"port,omitempty"`
	URI     string            `json:"uri,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Match   *HealthCheckMatch `json:"match,omitempty"`
}

func (spec *WSProxyHealthCheckSpec) Validate() error {
	if spec.HTTP == nil && spec.WS == nil {
		return fmt.Errorf("empty health check spec")
	}
	if spec.HTTP != nil {
		err := spec.HTTP.Validate()
		if err != nil {
			return err
		}
	}
	if spec.WS != nil {
		ws := spec.WS
		if ws.Port < 0 {
			return fmt.Errorf("invalid port: %d", ws.Port)
		}
		if ws.URI != "" {
			_, err := url.Parse(ws.URI)
			if err != nil {
				return err
			}
		}
		if ws.Match != nil {
			err := ws.Match.Validate()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type wsHealthChecker struct {
	spec *WSProxyHealthCheckSpec

	httpHealthChecker proxies.HealthChecker

	uri    *url.URL
	dialer *websocket.Dialer
}

// BaseSpec returns the base spec.
func (ws *wsHealthChecker) BaseSpec() proxies.HealthCheckSpec {
	return ws.spec.HealthCheckSpec
}

// Check checks the health of the server.
func (ws *wsHealthChecker) Check(server *proxies.Server) bool {
	var res bool
	if ws.httpHealthChecker != nil {
		res = ws.httpHealthChecker.Check(server)
		if !res {
			return false
		}
	}
	if ws.dialer != nil {
		header := http.Header{}
		for k, v := range ws.spec.WS.Headers {
			header.Set(k, v)
		}
		conn, resp, err := ws.dialer.Dial(getURL(server, ws.uri, ws.spec.WS.Port, true), header)
		if err != nil {
			return false
		}
		defer func() {
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			conn.Close()
		}()
		defer resp.Body.Close()
		err = ws.spec.WS.Match.Match(resp)
		if err != nil {
			logger.Warnf("health check %s failed: %v", server.URL, err)
			return false
		}
	}
	return true
}

// Close closes the health checker.
func (ws *wsHealthChecker) Close() {}

func NewWebSocketHealthChecker(spec *WSProxyHealthCheckSpec) proxies.HealthChecker {
	if spec == nil {
		return nil
	}

	res := &wsHealthChecker{spec: spec}
	if spec.HTTP != nil {
		httpSpec := &ProxyHealthCheckSpec{
			HealthCheckSpec:     spec.HealthCheckSpec,
			HTTPHealthCheckSpec: *spec.HTTP,
		}
		res.httpHealthChecker = NewHTTPHealthChecker(nil, httpSpec)
	}
	if spec.WS == nil {
		return res
	}

	ws := spec.WS
	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: spec.GetTimeout(),
	}
	res.dialer = dialer

	if ws.URI != "" {
		uri, _ := url.Parse(ws.URI)
		res.uri = uri
	}
	if ws.Match != nil {
		if len(ws.Match.StatusCodes) == 0 {
			// default status code for websocket is 101
			ws.Match.StatusCodes = [][]int{{101, 101}}
		}
		for i := range ws.Match.Headers {
			h := &ws.Match.Headers[i]
			if h.Type == regexpType {
				h.re = regexp.MustCompile(h.Value)
			}
		}
		if ws.Match.Body != nil {
			if ws.Match.Body.Type == regexpType {
				ws.Match.Body.re = regexp.MustCompile(ws.Match.Body.Value)
			}
		}
	} else {
		ws.Match = &HealthCheckMatch{
			StatusCodes: [][]int{{101, 101}},
		}
	}
	return res
}

func getURL(server *proxies.Server, uri *url.URL, port int, ws bool) string {
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		return server.URL
	}
	target := &url.URL{}
	if uri != nil {
		target.Host = serverURL.Host
		target.Scheme = serverURL.Scheme
		target.Path = uri.Path
		target.RawQuery = uri.RawQuery
	} else {
		target = serverURL
	}
	if port != 0 {
		target.Host = fmt.Sprintf("%s:%d", target.Hostname(), port)
	}
	if ws {
		if target.Scheme == "http" {
			target.Scheme = "ws"
		} else if target.Scheme == "https" {
			target.Scheme = "wss"
		}
	} else {
		if target.Scheme == "ws" {
			target.Scheme = "http"
		} else if target.Scheme == "wss" {
			target.Scheme = "https"
		}
	}
	return target.String()
}
