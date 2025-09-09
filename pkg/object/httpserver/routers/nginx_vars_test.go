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

package routers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/megaease/easegress/v2/pkg/protocols/httpprot"
	"github.com/stretchr/testify/assert"
)

func TestParseNginxLikeVar(t *testing.T) {
	// Create a test HTTP request
	req, err := http.NewRequest("GET", "http://example.com:8080/test/path?foo=bar&baz=qux", strings.NewReader("test body"))
	assert.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "9")
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("X-Real-IP", "192.168.1.1")
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.168.1.1:12345"

	// Create httpprot.Request
	httpReq, err := httpprot.NewRequest(req)
	assert.NoError(t, err)

	// Create RouteContext
	ctx := NewContext(httpReq)

	// Add some path captures for testing
	ctx.Params.Keys = []string{"id", "name"}
	ctx.Params.Values = []string{"123", "test"}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic variable syntax
		{
			name:     "simple host variable",
			input:    "$host",
			expected: "example.com",
		},
		{
			name:     "braced host variable",
			input:    "${host}",
			expected: "example.com",
		},
		{
			name:     "concatenated string",
			input:    "${host}-backend",
			expected: "example.com-backend",
		},
		{
			name:     "multiple variables",
			input:    "$scheme://$host$uri",
			expected: "http://example.com/test/path",
		},
		{
			name:     "default value when variable is empty",
			input:    "${remote_user:anonymous}",
			expected: "anonymous",
		},
		{
			name:     "default value when variable exists",
			input:    "${host:localhost}",
			expected: "example.com",
		},

		// Request line variables
		{
			name:     "request method",
			input:    "$request_method",
			expected: "GET",
		},
		{
			name:     "request URI",
			input:    "$request_uri",
			expected: "/test/path?foo=bar&baz=qux",
		},
		{
			name:     "URI path",
			input:    "$uri",
			expected: "/test/path",
		},
		{
			name:     "scheme",
			input:    "$scheme",
			expected: "http",
		},
		{
			name:     "query string",
			input:    "$query_string",
			expected: "foo=bar&baz=qux",
		},
		{
			name:     "args alias",
			input:    "$args",
			expected: "foo=bar&baz=qux",
		},

		// Host variables
		{
			name:     "hostname",
			input:    "$hostname",
			expected: "example.com",
		},
		{
			name:     "http_host",
			input:    "$http_host",
			expected: "example.com:8080",
		},
		{
			name:     "server_name",
			input:    "$server_name",
			expected: "example.com",
		},

		// Content variables
		{
			name:     "content type",
			input:    "$content_type",
			expected: "application/json",
		},
		{
			name:     "content length",
			input:    "$content_length",
			expected: "9",
		},

		// Protocol variables
		{
			name:     "server protocol",
			input:    "$server_protocol",
			expected: "HTTP/1.1",
		},

		// HTTP headers
		{
			name:     "http header user agent",
			input:    "$http_user_agent",
			expected: "test-agent",
		},
		{
			name:     "http header x-forwarded-for",
			input:    "$http_x_forwarded_for",
			expected: "192.168.1.1",
		},

		// Query parameters
		{
			name:     "query parameter foo",
			input:    "$arg_foo",
			expected: "bar",
		},
		{
			name:     "query parameter baz",
			input:    "$arg_baz",
			expected: "qux",
		},
		{
			name:     "non-existent query parameter",
			input:    "$arg_nonexistent",
			expected: "",
		},

		// Path parameters
		{
			name:     "path parameter id",
			input:    "$id",
			expected: "123",
		},
		{
			name:     "path parameter name",
			input:    "$name",
			expected: "test",
		},

		// Complex expressions
		{
			name:     "complex URL construction",
			input:    "${scheme}://${host}${uri}?${query_string}",
			expected: "http://example.com/test/path?foo=bar&baz=qux",
		},
		{
			name:     "header with suffix",
			input:    "X-Custom-${http_user_agent}",
			expected: "X-Custom-test-agent",
		},
		{
			name:     "mixed variables and text",
			input:    "Request from ${remote_addr:unknown} to ${host}${uri}",
			expected: "Request from 192.168.1.1 to example.com/test/path",
		},
		{
			name:     "proxy_add_x_forwarded_for with existing header",
			input:    "$proxy_add_x_forwarded_for",
			expected: "192.168.1.1, 192.168.1.1",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no variables",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "invalid variable",
			input:    "$invalid_var",
			expected: "",
		},
		{
			name:     "dollar sign without variable",
			input:    "price is $100",
			expected: "price is $100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctx.ParseNginxLikeVar(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseNginxLikeVarWithCookies(t *testing.T) {
	// Create a test HTTP request with cookies
	req, err := http.NewRequest("GET", "http://example.com/test", nil)
	assert.NoError(t, err)

	req.AddCookie(&http.Cookie{Name: "session_id", Value: "abc123"})
	req.AddCookie(&http.Cookie{Name: "user_pref", Value: "dark_mode"})

	// Create httpprot.Request
	httpReq, err := httpprot.NewRequest(req)
	assert.NoError(t, err)

	// Create RouteContext
	ctx := NewContext(httpReq)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "session cookie",
			input:    "$cookie_session_id",
			expected: "abc123",
		},
		{
			name:     "user preference cookie",
			input:    "$cookie_user_pref",
			expected: "dark_mode",
		},
		{
			name:     "non-existent cookie",
			input:    "$cookie_nonexistent",
			expected: "",
		},
		{
			name:     "cookie in expression",
			input:    "user-${cookie_session_id}",
			expected: "user-abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctx.ParseNginxLikeVar(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseNginxLikeVarEdgeCases(t *testing.T) {
	// Create a minimal test HTTP request
	req, err := http.NewRequest("GET", "http://example.com/", nil)
	assert.NoError(t, err)

	// Create httpprot.Request
	httpReq, err := httpprot.NewRequest(req)
	assert.NoError(t, err)

	// Create RouteContext
	ctx := NewContext(httpReq)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty default value",
			input:    "${remote_user:}",
			expected: "",
		},
		{
			name:     "colon in default value",
			input:    "${remote_user:http://default.com}",
			expected: "http://default.com",
		},
		{
			name:     "malformed variable",
			input:    "${incomplete",
			expected: "${incomplete",
		},
		{
			name:     "variable with underscores",
			input:    "${server_protocol}",
			expected: "HTTP/1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctx.ParseNginxLikeVar(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProxyAddXForwardedFor(t *testing.T) {
	tests := []struct {
		name        string
		existingXFF string
		clientIP    string
		remoteAddr  string
		expected    string
	}{
		{
			name:        "no existing X-Forwarded-For header",
			existingXFF: "",
			clientIP:    "192.168.1.100",
			remoteAddr:  "192.168.1.100:12345",
			expected:    "192.168.1.100",
		},
		{
			name:        "existing X-Forwarded-For header",
			existingXFF: "10.0.0.1, 172.16.0.1",
			clientIP:    "192.168.1.100",
			remoteAddr:  "192.168.1.100:12345",
			expected:    "10.0.0.1, 172.16.0.1, 192.168.1.100",
		},
		{
			name:        "single existing X-Forwarded-For IP",
			existingXFF: "10.0.0.1",
			clientIP:    "192.168.1.100",
			remoteAddr:  "192.168.1.100:12345",
			expected:    "10.0.0.1, 192.168.1.100",
		},
		{
			name:        "empty client IP fallback to RemoteAddr",
			existingXFF: "",
			clientIP:    "",
			remoteAddr:  "203.0.113.50:45678",
			expected:    "203.0.113.50",
		},
		{
			name:        "empty client IP with existing XFF",
			existingXFF: "10.0.0.1",
			clientIP:    "",
			remoteAddr:  "203.0.113.50:45678",
			expected:    "10.0.0.1, 203.0.113.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test HTTP request
			req, err := http.NewRequest("GET", "http://example.com/test", nil)
			assert.NoError(t, err)

			req.RemoteAddr = tt.remoteAddr
			if tt.existingXFF != "" {
				req.Header.Set("X-Forwarded-For", tt.existingXFF)
			}
			if tt.clientIP != "" {
				req.Header.Set("X-Real-IP", tt.clientIP)
			}

			// Create httpprot.Request
			httpReq, err := httpprot.NewRequest(req)
			assert.NoError(t, err)

			// Create RouteContext
			ctx := NewContext(httpReq)

			result := ctx.ParseNginxLikeVar("$proxy_add_x_forwarded_for")
			assert.Equal(t, tt.expected, result)
		})
	}
}
