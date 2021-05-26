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
	"sort"
	"sync"

	"github.com/megaease/easegress/pkg/logger"
)

var (
	// Global is the global service registry.
	Global = New()
)

type (
	// ServiceRegistry is the service center containing all drivers.
	ServiceRegistry struct {
		mutex sync.RWMutex

		//	  registryName   serviceName
		registries map[string]map[string]*Service
	}
)

// New creates a ServiceRegistry.
func New() *ServiceRegistry {
	return &ServiceRegistry{
		registries: map[string]map[string]*Service{},
	}
}

// GetService gets service.
// NOTICE: If the registryName is empty, and there is one and only one ServiceRegistry,
// it uses the only one.
func (sg *ServiceRegistry) GetService(registryName, serviceName string) (*Service, error) {
	sg.mutex.RLock()
	defer sg.mutex.RUnlock()

	var chosenRegistry map[string]*Service
	// NOTE: Try to find the unnamed service registry.
	switch len(sg.registries) {
	case 0:
		return nil, fmt.Errorf("no service registry to use")
	case 1:
		if registryName == "" {
			// Return the only one service registry if not specifying.
			for _, registry := range sg.registries {
				chosenRegistry = registry
			}
		}
	default:
		if registryName == "" {
			return nil, fmt.Errorf("no service registry specific(%d exist)", len(sg.registries))
		}
	}

	if chosenRegistry == nil {
		// NOTE: Try to find the named service registry.
		registry, exists := sg.registries[registryName]
		if !exists {
			return nil, fmt.Errorf("service registry %s not found", registryName)
		}
		chosenRegistry = registry
	}

	service, exists := chosenRegistry[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	return service, nil
}

// ReplaceServers replaces all servers of the registry.
func (sg *ServiceRegistry) ReplaceServers(registryName string, servers []*Server) {
	serversByService := map[string][]*Server{}
	for _, server := range servers {
		if err := server.Validate(); err != nil {
			logger.Errorf("serer %+v is invalid: %v", server, err)
			continue
		}
		serversByService[server.ServiceName] = append(serversByService[server.ServiceName], server)
	}
	for _, servers := range serversByService {
		// NOTE: It's stable for deep equal of server.Update.
		sort.Sort(serversByURL(servers))
	}

	sg.mutex.Lock()
	defer sg.mutex.Unlock()

	serviceNames := map[string]struct{}{}
	registry := sg.registries[registryName]
	if registry == nil {
		registry = map[string]*Service{}
	}
	for serviceName, servers := range serversByService {
		service, exists := registry[serviceName]
		if exists {
			err := service.Update(servers)
			if err != nil {
				logger.Errorf("registry %s update service %s failed: %v",
					registryName, serviceName, err)
				continue
			}
		} else {
			service, err := NewService(serviceName, servers)
			if err != nil {
				logger.Errorf("new service %s failed: %v", serviceName, err)
				continue
			}
			registry[serviceName] = service
		}
		serviceNames[serviceName] = struct{}{}
	}

	for serviceName, service := range registry {
		if _, exists := serviceNames[serviceName]; !exists {
			delete(registry, serviceName)
			service.Close(fmt.Sprintf("zero server in service registry %s", registryName))
		}
	}

	sg.registries[registryName] = registry
}

// CloseRegistry deletes the registry with closing its services.
func (sg *ServiceRegistry) CloseRegistry(registryName string) {
	sg.mutex.Lock()
	defer sg.mutex.Unlock()

	registry, exists := sg.registries[registryName]
	if !exists {
		return
	}

	for _, service := range registry {
		service.Close(fmt.Sprintf("service registry %s closed", registryName))
	}

	delete(sg.registries, registryName)
}

type serversByURL []*Server

func (s serversByURL) Less(i, j int) bool { return s[i].URL() < s[j].URL() }
func (s serversByURL) Len() int           { return len(s) }
func (s serversByURL) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
