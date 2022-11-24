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

package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/megaease/easegress/pkg/logger"
)

// Server is proxy server.
type Server struct {
	URL            string   `json:"url" jsonschema:"required,format=url"`
	Tags           []string `json:"tags" jsonschema:"omitempty,uniqueItems=true"`
	Weight         int      `json:"weight" jsonschema:"omitempty,minimum=0,maximum=100"`
	KeepHost       bool     `json:"keepHost" jsonschema:"omitempty,default=false"`
	addrIsHostName bool
	health         *ServerHealth
}

// ServerHealth is health status of server
type ServerHealth struct {
	healthy bool
	fails   int
	passes  int
}

// String implements the Stringer interface.
func (s *Server) String() string {
	return fmt.Sprintf("%s,%v,%d", s.URL, s.Tags, s.Weight)
}

// ID return identifier for server
func (s *Server) ID() string {
	return s.URL
}

// checkAddrPattern checks whether the server address is host name or ip:port,
// not all error cases are handled.
func (s *Server) checkAddrPattern() {
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

	s.addrIsHostName = net.ParseIP(host) == nil
}

// recordHealth records health status, return healthy status and true if status changes
func (s *Server) recordHealth(pass bool, passThrehold, failThrehold int) (bool, bool) {
	if s.health == nil {
		s.health = &ServerHealth{healthy: true}
	}
	h := s.health
	if pass {
		h.passes++
		h.fails = 0
	} else {
		h.passes = 0
		h.fails++
	}
	change := false
	if h.passes >= passThrehold && !h.healthy {
		h.healthy = true
		logger.Warnf("server:%v becomes healthy.", s.ID())
		change = true
	} else if h.fails >= failThrehold && h.healthy {
		logger.Warnf("server:%v becomes unhealthy!", s.ID())
		h.healthy = false
		change = true
	}
	return h.healthy, change
}
