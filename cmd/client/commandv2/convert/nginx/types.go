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
	Servers []*Server
}

type ServerBase struct {
	Port    int
	Address string
	HTTPS   bool

	CaCert string
	Certs  map[string]string
	Keys   map[string]string
}

type Server struct {
	ServerBase
	Rules []*Rule
}

type ServerInfo struct {
	ServerBase
	Hosts []*HostInfo
}

type HostInfo struct {
	Value    string
	IsRegexp bool
}

type Rule struct {
	Hosts []*HostInfo
	Paths []*Path
}

type PathType string

const (
	PathTypePrefix        PathType = "prefix"
	PathTypeExact         PathType = "exact"
	PathTypeRe            PathType = "regexp"
	PathTypeReInsensitive PathType = "caseInsensitiveRegexp"
)

type Path struct {
	Path    string
	Type    PathType
	Backend *ProxyInfo
}

type ProxyInfo struct {
	Servers    []*BackendInfo
	SetHeaders map[string]string
}

type BackendInfo struct {
	Server string
	Weight int
}
