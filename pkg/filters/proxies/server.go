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

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Server is a backend proxy server.
type Server struct {
	URL            string   `json:"url" jsonschema:"required,format=url"`
	Tags           []string `json:"tags,omitempty" jsonschema:"uniqueItems=true"`
	Weight         int      `json:"weight,omitempty" jsonschema:"minimum=0,maximum=100"`
	KeepHost       bool     `json:"keepHost,omitempty" jsonschema:"default=false"`
	AddrIsHostName bool     `json:"-"`
	Unhealth       bool     `json:"-"`
	// HealthCounter is used to count the number of successive health checks
	// result, positive for healthy, negative for unhealthy
	HealthCounter int `json:"-"`
}

// String implements the Stringer interface.
func (s *Server) String() string {
	return fmt.Sprintf("%s,%v,%d", s.URL, s.Tags, s.Weight)
}

// ID return identifier for server
func (s *Server) ID() string {
	return s.URL
}

// CheckAddrPattern checks whether the server address is host name or ip:port,
// not all error cases are handled.
func (s *Server) CheckAddrPattern() {
	u, err := url.Parse(s.URL)
	if err != nil {
		return
	}
	host := u.Host

	square := strings.LastIndexByte(host, ']')
	colon := strings.LastIndexByte(host, ':')

	// There is a port number, remove it.
	if colon > square {
		host = host[:colon]
	}

	// IPv6
	if square != -1 && host[0] == '[' {
		host = host[1:square]
	}

	s.AddrIsHostName = net.ParseIP(host) == nil
}

// Healthy returns whether the server is healthy
func (s *Server) Healthy() bool {
	return !s.Unhealth
}

// ServerGroup is a group of servers.
type ServerGroup struct {
	TotalWeight int
	Servers     []*Server
}

func newServerGroup(servers []*Server) *ServerGroup {
	sg := &ServerGroup{Servers: servers}
	for _, s := range servers {
		sg.TotalWeight += s.Weight
	}
	return sg
}
