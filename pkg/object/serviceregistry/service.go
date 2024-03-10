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

package serviceregistry

import (
	"fmt"
	"reflect"
)

type (
	// ServiceInstanceSpec is the service instance spec in Easegress.
	ServiceInstanceSpec struct {
		// RegistryName is required.
		RegistryName string `json:"registryName"`
		// ServiceName is required.
		ServiceName string `json:"serviceName"`
		// InstanceID is required.
		InstanceID string `json:"instanceID"`

		// Address is required.
		Address string `json:"address"`
		// Port is required.
		Port uint16 `json:"port"`

		// Scheme is optional.
		Scheme string `json:"scheme"`
		// Tags is optional.
		Tags []string `json:"tags"`
		// Weight is optional.
		Weight int `json:"weight"`
	}
)

// DeepCopy deep copies ServiceInstanceSpec.
func (s *ServiceInstanceSpec) DeepCopy() *ServiceInstanceSpec {
	copy := *s

	if s.Tags != nil {
		copy.Tags = append(copy.Tags, s.Tags...)
	}

	return &copy
}

// Validate validates itself.
func (s *ServiceInstanceSpec) Validate() error {
	if s.RegistryName == "" {
		return fmt.Errorf("registryName is empty")
	}

	if s.ServiceName == "" {
		return fmt.Errorf("serviceName is empty")
	}

	if s.InstanceID == "" {
		return fmt.Errorf("instanceID is empty")
	}

	if s.Address == "" {
		return fmt.Errorf("address is empty")
	}

	if s.Port == 0 {
		return fmt.Errorf("port is empty")
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

	return fmt.Sprintf("%s://%s:%d", scheme, s.Address, s.Port)
}

// NewRegistryEventFromDiff creates a registry event from diff old and new specs.
// It only uses Apply and Delete excluding Replace.
// External drivers should use event.Replace in first time, then use this utility to generate next events.
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
	}

	for _, oldSpec := range oldSpecs {
		_, exists := newSpecs[oldSpec.Key()]
		if !exists {
			copy := oldSpec.DeepCopy()

			if event.Delete == nil {
				event.Delete = make(map[string]*ServiceInstanceSpec)
			}

			event.Delete[oldSpec.Key()] = copy
		}
	}

	for _, newSpec := range newSpecs {
		oldSpec, exists := oldSpecs[newSpec.Key()]
		if !exists || !reflect.DeepEqual(oldSpec, newSpec) {
			copy := newSpec.DeepCopy()

			if event.Apply == nil {
				event.Apply = make(map[string]*ServiceInstanceSpec)
			}

			event.Apply[newSpec.Key()] = copy
		}
	}

	return event
}
