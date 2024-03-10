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

package grpcserver

import (
	"fmt"
	"regexp"

	"github.com/megaease/easegress/v2/pkg/util/ipfilter"
)

type (
	// Spec describe gRPC server
	Spec struct {
		Port           uint16 `json:"port" jsonschema:"required,minimum=1025"`
		MaxConnections uint32 `json:"maxConnections,omitempty" jsonschema:"minimum=1"`

		// MinTime see keepalive.EnforcementPolicy
		MinTime string `json:"minTimeClientSendPing,omitempty" jsonschema:"format=duration"`
		// PermitWithoutStream see keepalive.EnforcementPolicy, false by default.
		PermitWithoutStream bool `json:"permitClintSendPingWithoutStream,omitempty" jsonschema:"format=duration"`

		// MaxConnectionIdle see keepalive.ServerParameters
		MaxConnectionIdle string `json:"maxConnectionIdle,omitempty" jsonschema:"format=duration"`
		// MaxConnectionAge see keepalive.ServerParameters
		MaxConnectionAge string `json:"maxConnectionAge,omitempty" jsonschema:"format=duration"`
		// MaxConnectionAgeGrace see keepalive.ServerParameters
		MaxConnectionAgeGrace string `json:"maxConnectionAgeGrace,omitempty" jsonschema:"format=duration"`
		// Time see keepalive.ServerParameters
		Time string `json:"keepaliveTime,omitempty" jsonschema:"format=duration"`
		// Timeout see keepalive.ServerParameters
		Timeout       string         `json:"keepaliveTimeout,omitempty" jsonschema:"format=duration"`
		IPFilter      *ipfilter.Spec `json:"ipFilter,omitempty"`
		Rules         []*Rule        `json:"rules,omitempty"`
		CacheSize     uint32         `json:"cacheSize,omitempty"`
		GlobalFilter  string         `json:"globalFilter,omitempty"`
		XForwardedFor bool           `json:"xForwardedFor,omitempty"`
	}

	// Rule is first level entry of router.
	Rule struct {
		// NOTICE: If the field is a pointer, it must have `omitempty` in tag `json`
		// when it has `omitempty` in tag `jsonschema`.
		// Otherwise it will output null value, which is invalid in json schema (the type is object).
		// the original reason is the jsonscheme(genjs) has not support multiple types.
		// Reference: https://github.com/alecthomas/jsonschema/issues/30
		// In the future if we have the scenario where we need marshal the field, but omitempty
		// in the schema, we are suppose to support multiple types on our own.
		IPFilter   *ipfilter.Spec `json:"ipFilter,omitempty"`
		Host       string         `json:"host,omitempty"`
		HostRegexp string         `json:"hostRegexp,omitempty" jsonschema:"format=regexp"`
		Methods    []*Method      `json:"methods,omitempty"`
	}

	// Method is second level entry of router.
	Method struct {
		IPFilter       *ipfilter.Spec `json:"ipFilter,omitempty"`
		Method         string         `json:"method,omitempty" jsonschema:"pattern=^/"`
		MethodPrefix   string         `json:"methodPrefix,omitempty" jsonschema:"pattern=^/"`
		MethodRegexp   string         `json:"methodRegexp,omitempty" jsonschema:"format=regexp"`
		Backend        string         `json:"backend" jsonschema:"required"`
		Headers        []*Header      `json:"headers,omitempty"`
		MatchAllHeader bool           `json:"matchAllHeader,omitempty"`
	}

	// Header is the third level entry of router. A header entry is always under a specific path entry, that is to mean
	// the headers entry will only be checked after a path entry matched. However, the headers entry has a higher priority
	// than the path entry itself.
	Header struct {
		Key    string   `json:"key" jsonschema:"required"`
		Values []string `json:"values,omitempty" jsonschema:"uniqueItems=true"`
		Regexp string   `json:"regexp,omitempty" jsonschema:"format=regexp"`

		headerRE *regexp.Regexp
	}
)

func (h *Header) initHeaderRoute() {
	h.headerRE = regexp.MustCompile(h.Regexp)
}

// Validate validates Method.
func (m *Method) Validate() error {
	return nil
}

// Validate validates Header.
func (h *Header) Validate() error {
	if len(h.Values) == 0 && h.Regexp == "" {
		return fmt.Errorf("both of values and regexp are empty for key: %s", h.Key)
	}

	return nil
}
