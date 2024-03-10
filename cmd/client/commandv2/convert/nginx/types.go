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

package nginx

// Config is config for convert.
type Config struct {
	Servers []*Server `json:"servers"`
}

// ServerBase is the base config for server.
type ServerBase struct {
	Port    int    `json:"port"`
	Address string `json:"address"`
	HTTPS   bool   `json:"https"`

	CaCert string            `json:"caCert"`
	Certs  map[string]string `json:"certs"`
	Keys   map[string]string `json:"keys"`
}

// Server is the config for server.
type Server struct {
	ServerBase `json:",inline"`
	Rules      []*Rule `json:"rules"`
}

// ServerInfo is the info config for server.
type ServerInfo struct {
	ServerBase `json:",inline"`
	Hosts      []*HostInfo `json:"hosts"`
}

// HostInfo is the info config for host.
type HostInfo struct {
	Value    string `json:"value"`
	IsRegexp bool   `json:"isRegexp"`
}

// Rule is the config for rule.
type Rule struct {
	Hosts []*HostInfo `json:"hosts"`
	Paths []*Path     `json:"paths"`
}

// PathType is the type of path.
type PathType string

const (
	// PathTypePrefix is the prefix type of path.
	PathTypePrefix PathType = "prefix"
	// PathTypeExact is the exact type of path.
	PathTypeExact PathType = "exact"
	// PathTypeRe is the regexp type of path.
	PathTypeRe PathType = "regexp"
	// PathTypeReInsensitive is the case insensitive regexp type of path.
	PathTypeReInsensitive PathType = "caseInsensitiveRegexp"
)

// Path is the config for path.
type Path struct {
	Path    string     `json:"path"`
	Type    PathType   `json:"type"`
	Backend *ProxyInfo `json:"backend"`
}

// ProxyInfo is the config for proxy.
type ProxyInfo struct {
	Servers       []*BackendInfo    `json:"servers"`
	SetHeaders    map[string]string `json:"setHeaders"`
	GzipMinLength int               `json:"gzipMinLength"`
}

// BackendInfo is the config for backend.
type BackendInfo struct {
	Server string `json:"server"`
	Weight int    `json:"weight"`
}
