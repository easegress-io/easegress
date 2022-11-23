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
	"sync"

	"github.com/megaease/easegress/pkg/logger"
)

// Server is proxy server.
type Server struct {
	URL            string   `json:"url" jsonschema:"required,format=url"`
	Tags           []string `json:"tags" jsonschema:"omitempty,uniqueItems=true"`
	Weight         int      `json:"weight" jsonschema:"omitempty,minimum=0,maximum=100"`
	KeepHost       bool     `json:"keepHost" jsonschema:"omitempty,default=false"`
	addrIsHostName bool
	unhealthy      bool
	fails          int
	passes         int
	mu             sync.Mutex
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

// healthy return health status
func (s *Server) healthy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.unhealthy
}

// recordHealth records health status, return true if status changes
func (s *Server) recordHealth(pass bool, passThrehold, failThrehold int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pass {
		s.passes++
		s.fails = 0
	} else {
		s.passes = 0
		s.fails++
	}
	if s.passes >= passThrehold && s.unhealthy {
		s.unhealthy = false
		logger.Warnf("server:%v becomes healthy.", s.ID())
		return true
	}
	if s.fails >= failThrehold && !s.unhealthy {
		logger.Warnf("server:%v becomes unhealthy!", s.ID())
		s.unhealthy = true
		return true
	}
	return false
}
