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
)

type (
	// ServiceInstanceSpec is the service instance spec in Easegress.
	ServiceInstanceSpec struct {
		// RegistryName is required.
		RegistryName string `yaml:"registryName"`
		// ServiceName is required.
		ServiceName string `yaml:"serviceName"`
		// InstanceID is required.
		InstanceID string `yaml:"name"`

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

// DeepCopy deep copies ServiceInstanceSpec.
func (s *ServiceInstanceSpec) DeepCopy() *ServiceInstanceSpec {
	copy := *s
	return &copy
}

// Validate validates itself.
func (s *ServiceInstanceSpec) Validate() error {
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

// Key returns the unique key for the service instance.
func (s *ServiceInstanceSpec) Key() string {
	return fmt.Sprintf("%s/%s/%s", s.RegistryName, s.ServiceName, s.InstanceID)
}

// URL returns the url of the server.
func (s *ServiceInstanceSpec) URL() string {
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

// NewRegistryEventFromDiff creates a registry event from diff old and new specs.
// It only uses Apply and Delete excluding Replace.
// External drivers should use event.Replace in first time, then use this utiliy to generate next events.
// registryName is only assigned to the event, the registry name of service instance spec won't change.
func NewRegistryEventFromDiff(registryName string, oldSpecs, newSpecs map[string]*ServiceInstanceSpec) *RegistryEvent {
	if oldSpecs == nil {
		oldSpecs = make(map[string]*ServiceInstanceSpec)
	}

	if newSpecs == nil {
		newSpecs = make(map[string]*ServiceInstanceSpec)
	}

	event := &RegistryEvent{
		SourceRegistryName: registryName,

		Delete: make(map[string]*ServiceInstanceSpec),
		Apply:  make(map[string]*ServiceInstanceSpec),
	}

	for _, oldSpec := range oldSpecs {
		_, exists := newSpecs[oldSpec.ServiceName]
		if !exists {
			copy := oldSpec.DeepCopy()
			event.Delete[oldSpec.ServiceName] = copy
		}
	}

	for _, newSpec := range newSpecs {
		oldSpec, exists := oldSpecs[newSpec.ServiceName]
		if exists && !reflect.DeepEqual(oldSpec, newSpec) {
			copy := newSpec.DeepCopy()
			event.Apply[newSpec.ServiceName] = copy
		}
	}

	return event
}
