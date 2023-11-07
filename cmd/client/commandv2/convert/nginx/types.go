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

package nginx

type Config struct {
	Servers []*Server `json:"servers"`
}

type ServerBase struct {
	Port    int    `json:"port"`
	Address string `json:"address"`
	HTTPS   bool   `json:"https"`

	CaCert string            `json:"caCert"`
	Certs  map[string]string `json:"certs"`
	Keys   map[string]string `json:"keys"`
}

type Server struct {
	ServerBase `json:",inline"`
	Rules      []*Rule `json:"rules"`
}

type ServerInfo struct {
	ServerBase `json:",inline"`
	Hosts      []*HostInfo `json:"hosts"`
}

type HostInfo struct {
	Value    string `json:"value"`
	IsRegexp bool   `json:"isRegexp"`
}

type Rule struct {
	Hosts []*HostInfo `json:"hosts"`
	Paths []*Path     `json:"paths"`
}

type PathType string

const (
	PathTypePrefix        PathType = "prefix"
	PathTypeExact         PathType = "exact"
	PathTypeRe            PathType = "regexp"
	PathTypeReInsensitive PathType = "caseInsensitiveRegexp"
)

type Path struct {
	Path    string     `json:"path"`
	Type    PathType   `json:"type"`
	Backend *ProxyInfo `json:"backend"`
}

type ProxyInfo struct {
	Servers       []*BackendInfo    `json:"servers"`
	SetHeaders    map[string]string `json:"setHeaders"`
	GzipMinLength int               `json:"gzipMinLength"`
}

type BackendInfo struct {
	Server string `json:"server"`
	Weight int    `json:"weight"`
}
