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

package serviceregistry

import (
	"fmt"
	"reflect"
	"sync"
)

type (
	// Service contains the information of all backend servers of one service.
	Service struct {
		mutex sync.Mutex

		name         string
		servers      []*Server
		closeMessage string

		updated chan struct{}
		closed  chan struct{}
	}

	// Server stands for one instance of the server.
	Server struct {
		// ServiceName is required.
		ServiceName string `yaml:"serviceName"`

		// Scheme is optional if Port is not empty.
		Scheme string `yaml:"scheme"`
		// Hostname is optional if HostIP is not empty.
		Hostname string `yaml:"hostname"`
		// HostIP is optional if Hostname is not empty.
		HostIP string `yaml:"hostIP"`
		// Port is optional if Scheme is not empty
		Port uint16 `yaml:"port"`
		// Tags is optional.
		Tags []string `yaml:"tags"`
		// Weight is optional.
		Weight int `yaml:"weight"`
	}
)

// Validate validates itself.
func (s *Server) Validate() error {
	if s.ServiceName == "" {
		return fmt.Errorf("serviceName is empty")
	}

	if s.Hostname == "" && s.HostIP == "" {
		return fmt.Errorf("both hostname and hostIP are empty")
	}

	if s.Scheme == "" && s.Port == 0 {
		return fmt.Errorf("both scheme and port are empty")
	}

	switch s.Scheme {
	case "", "http", "https":
	default:
		return fmt.Errorf("unsupported scheme %s (support http, https)", s.Scheme)
	}

	return nil
}

// URL returns the url of the server.
func (s *Server) URL() string {
	scheme := s.Scheme
	if scheme == "" {
		scheme = "http"
	}

	var host string
	if s.Hostname != "" {
		host = s.Hostname
	} else {
		host = s.HostIP
	}

	var port string
	if s.Port != 0 {
		port = fmt.Sprintf("%d", s.Port)
	}

	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}

// NewService creates a Service.
func NewService(name string, servers []*Server) (*Service, error) {
	s := &Service{
		name:    name,
		updated: make(chan struct{}),
		closed:  make(chan struct{}),
	}

	err := s.Update(servers)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Servers return the current servers.
func (s *Service) Servers() []*Server {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var servers []*Server
	for _, server := range s.servers {
		servers = append(servers, server)
	}

	return servers
}

// Name returns the service name.
func (s *Service) Name() string {
	servers := s.Servers()
	if len(servers) == 0 {
		return ""
	}
	return servers[0].ServiceName
}

// Update updates the Service with closing the channel updated.
// It does nothing if servers are not changed.
func (s *Service) Update(servers []*Server) error {
	for i, server := range servers {
		err := server.Validate()
		if err != nil {
			return fmt.Errorf("server %d is invalid: %v", i+1, err)
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !reflect.DeepEqual(s.servers, servers) {
		s.servers = servers
		close(s.updated)
		s.updated = make(chan struct{})
	}

	return nil
}

// Updated returns the notifying channel to post update.
func (s *Service) Updated() chan struct{} {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.updated
}

// Closed returns the notifying channel to post close.
func (s *Service) Closed() chan struct{} {
	return s.closed
}

// CloseMessage closes the service.
func (s *Service) CloseMessage() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.closeMessage
}

// Close closes the service.
func (s *Service) Close(message string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.closeMessage = message
	close(s.closed)
}
