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
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/util/ipfilter"
	"github.com/megaease/easegress/v2/pkg/util/stringtool"
)

// Rules represents the set of rules.
type Rules []*Rule

// Paths represents the set of paths.
type Paths []*Path

// Host defines the host match rule.
type Host struct {
	IsRegexp bool   `json:"isRegexp,omitempty"`
	Value    string `json:"value" jsonschema:"required"`
	prefix   string `json:"-"`
	suffix   string `json:"-"`
	re       *regexp.Regexp
}

// Rule is first level entry of router.
type Rule struct {
	// NOTICE: If the field is a pointer, it must have `omitempty` in tag `json`
	// when it has `omitempty` in tag `jsonschema`.
	// Otherwise it will output null value, which is invalid in json schema (the type is object).
	// the original reason is the jsonscheme(genjs) has not support multiple types.
	// Reference: https://github.com/alecthomas/jsonschema/issues/30
	// In the future if we have the scenario where we need marshal the field, but omitempty
	// in the schema, we are suppose to support multiple types on our own.
	IPFilterSpec *ipfilter.Spec `json:"ipFilter,omitempty"`
	Host         string         `json:"host,omitempty"`
	HostRegexp   string         `json:"hostRegexp,omitempty" jsonschema:"format=regexp"`
	Hosts        []Host         `json:"hosts,omitempty"`
	Paths        Paths          `json:"paths,omitempty"`

	ipFilter *ipfilter.IPFilter
}

// Path is second level entry of router.
type Path struct {
	IPFilterSpec      *ipfilter.Spec `json:"ipFilter,omitempty"`
	Path              string         `json:"path,omitempty" jsonschema:"pattern=^/"`
	PathPrefix        string         `json:"pathPrefix,omitempty" jsonschema:"pattern=^/"`
	PathRegexp        string         `json:"pathRegexp,omitempty" jsonschema:"format=regexp"`
	RewriteTarget     string         `json:"rewriteTarget,omitempty"`
	Methods           []string       `json:"methods,omitempty" jsonschema:"uniqueItems=true,format=httpmethod-array"`
	Backend           string         `json:"backend" jsonschema:"required"`
	ClientMaxBodySize int64          `json:"clientMaxBodySize,omitempty"`
	Headers           Headers        `json:"headers,omitempty"`
	Queries           Queries        `json:"queries,omitempty"`
	MatchAllHeader    bool           `json:"matchAllHeader,omitempty"`
	MatchAllQuery     bool           `json:"matchAllQuery,omitempty"`

	ipFilter             *ipfilter.IPFilter
	method               MethodType
	cacheable, matchable bool
}

// Headers represents the set of headers.
type Headers []*Header

// Queries represents the set of queries.
type Queries []*Query

// Header is the third level entry of router. A header entry is always under a specific path entry, that is to mean
// the headers entry will only be checked after a path entry matched. However, the headers entry has a higher priority
// than the path entry itself.
type Header struct {
	Key    string   `json:"key" jsonschema:"required"`
	Values []string `json:"values,omitempty" jsonschema:"uniqueItems=true"`
	Regexp string   `json:"regexp,omitempty" jsonschema:"format=regexp"`

	re *regexp.Regexp
}

// Query is the third level entry.
type Query struct {
	Key    string   `json:"key" jsonschema:"required"`
	Values []string `json:"values,omitempty" jsonschema:"uniqueItems=true"`
	Regexp string   `json:"regexp,omitempty" jsonschema:"format=regexp"`

	re *regexp.Regexp
}

// Init is the initialization portal for Rules.
func (rules Rules) Init() {
	for _, rule := range rules {
		rule.Init()
	}
}

// Init is the initialization portal for Rule.
func (rule *Rule) Init() {
	if len(rule.Host) > 0 {
		rule.Hosts = append(rule.Hosts, Host{Value: rule.Host})
	}
	if len(rule.HostRegexp) > 0 {
		rule.Hosts = append(rule.Hosts, Host{IsRegexp: true, Value: rule.HostRegexp})
	}

	for i := range rule.Hosts {
		h := &rule.Hosts[i]
		if !h.IsRegexp {
			if h.Value != "" {
				count := strings.Count(h.Value, "*")
				if count == 0 {
					continue
				} else if count > 1 {
					logger.Errorf("invalid host %s, only one wildcard is allowed", h.Value)
					continue
				}
				if h.Value[0] == '*' {
					h.suffix = h.Value[1:]
				} else if h.Value[len(h.Value)-1] == '*' {
					h.prefix = h.Value[:len(h.Value)-1]
				} else {
					logger.Errorf("invalid host %s, only wildcard prefix or suffix is allowed", h.Value)
				}
			}
			continue
		}
		if re, err := regexp.Compile(h.Value); err != nil {
			logger.Errorf("failed to compile %q failed: %v", h.Value, err)
		} else {
			h.re = re
		}
	}

	rule.ipFilter = ipfilter.New(rule.IPFilterSpec)
	for _, p := range rule.Paths {
		p.Init(rule.ipFilter)
	}
}

// MatchHost matches the host of the request to the rule.
func (rule *Rule) MatchHost(ctx *RouteContext) bool {
	if len(rule.Hosts) == 0 {
		return true
	}

	host := ctx.GetHost()
	for i := range rule.Hosts {
		h := &rule.Hosts[i]
		if h.IsRegexp {
			if h.re != nil && h.re.MatchString(host) {
				return true
			}
		} else if host == h.Value {
			return true
		} else if h.prefix != "" && strings.HasPrefix(host, h.prefix) {
			return true
		} else if h.suffix != "" && strings.HasSuffix(host, h.suffix) {
			return true
		}
	}

	return false
}

// AllowIP return if rule ipFilter allows the incoming ip.
func (rule *Rule) AllowIP(ip string) bool {
	return rule.ipFilter.Allow(ip)
}

// Init is the initialization portal for Path
func (p *Path) Init(parentIPFilter *ipfilter.IPFilter) {
	p.ipFilter = ipfilter.New(p.IPFilterSpec)

	p.Headers.init()
	p.Queries.init()

	method := MALL
	if len(p.Methods) != 0 {
		method = 0
		for _, m := range p.Methods {
			method |= Methods[m]
		}
	}

	p.method = method
	p.matchable = true

	if len(p.Headers) == 0 && len(p.Queries) == 0 && p.ipFilter == nil {
		if parentIPFilter == nil {
			p.cacheable = true
		}
		if len(p.Methods) == 0 {
			p.matchable = false
		}
	}
}

// Validate validates Path.
func (p *Path) Validate() error {
	if (stringtool.IsAllEmpty(p.Path, p.PathPrefix, p.PathRegexp)) && p.RewriteTarget != "" {
		return fmt.Errorf("rewriteTarget is specified but path is empty")
	}

	return nil
}

// AllowIP return if rule ipFilter allows the incoming ip.
func (p *Path) AllowIP(ip string) bool {
	return p.ipFilter.Allow(ip)
}

// Match is the matching function of path.
func (p *Path) Match(context *RouteContext) bool {
	context.Cacheable = p.cacheable

	if !p.matchable {
		return true
	}

	// method match
	req := context.Request
	ip := req.RealIP()

	if context.Method&p.method == 0 {
		context.MethodMismatch = true
		return false
	}

	if len(p.Headers) > 0 && !p.Headers.Match(context.GetHeader(), p.MatchAllHeader) {
		context.HeaderMismatch = true
		return false
	}

	if len(p.Queries) > 0 && !p.Queries.Match(context.GetQueries(), p.MatchAllQuery) {
		context.QueryMismatch = true
		return false
	}

	if !p.AllowIP(ip) {
		context.IPMismatch = true
		return false
	}

	return true
}

// GetBackend is used to get the backend corresponding to the route.
func (p *Path) GetBackend() string {
	return p.Backend
}

// GetClientMaxBodySize is used to get the clientMaxBodySize corresponding to the route.
func (p *Path) GetClientMaxBodySize() int64 {
	return p.ClientMaxBodySize
}

// GetExactPath returns the exact path of the route.
func (p *Path) GetExactPath() string {
	return p.Path
}

// GetPathPrefix returns the path prefix of the route.
func (p *Path) GetPathPrefix() string {
	return p.PathPrefix
}

// GetPathRegexp returns the path regexp of the route.
func (p *Path) GetPathRegexp() string {
	return p.PathRegexp
}

func (hs Headers) init() {
	for _, h := range hs {
		if h.Regexp != "" {
			h.re = regexp.MustCompile(h.Regexp)
		}
	}
}

// Validate validates Headers.
func (hs Headers) Validate() error {
	for _, h := range hs {
		if len(h.Values) == 0 && h.Regexp == "" {
			return fmt.Errorf("both of values and regexp are empty for key: %s", h.Key)
		}
	}
	return nil
}

// Match is the matching function of Headers.
func (hs Headers) Match(headers http.Header, matchAll bool) bool {
	if len(hs) == 0 {
		return true
	}

	if matchAll {
		for _, h := range hs {
			v := headers.Get(h.Key)
			if len(h.Values) > 0 && !stringtool.StrInSlice(v, h.Values) {
				return false
			}

			if h.Regexp != "" && !h.re.MatchString(v) {
				return false
			}
		}
	} else {
		for _, h := range hs {
			v := headers.Get(h.Key)
			if stringtool.StrInSlice(v, h.Values) {
				return true
			}

			if h.Regexp != "" && h.re.MatchString(v) {
				return true
			}
		}
	}

	return matchAll
}

func (qs Queries) init() {
	for _, q := range qs {
		if q.Regexp != "" {
			q.re = regexp.MustCompile(q.Regexp)
		}
	}
}

// Validate validates Queries.
func (qs Queries) Validate() error {
	for _, q := range qs {
		if len(q.Values) == 0 && q.Regexp == "" {
			return fmt.Errorf("both of values and regexp are empty for key: %s", q.Key)
		}
	}
	return nil
}

// Match is the matching function of Queries.
func (qs Queries) Match(query url.Values, matchAll bool) bool {
	if len(qs) == 0 {
		return true
	}

	if matchAll {
		for _, q := range qs {
			v := query.Get(q.Key)
			if len(q.Values) > 0 && !stringtool.StrInSlice(v, q.Values) {
				return false
			}

			if q.Regexp != "" && !q.re.MatchString(v) {
				return false
			}
		}
	} else {
		for _, q := range qs {
			v := query.Get(q.Key)
			if stringtool.StrInSlice(v, q.Values) {
				return true
			}

			if q.Regexp != "" && q.re.MatchString(v) {
				return true
			}
		}
	}

	return matchAll
}
