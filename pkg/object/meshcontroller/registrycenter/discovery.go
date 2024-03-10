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

package registrycenter

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/megaease/easegress/v2/pkg/logger"
	"github.com/megaease/easegress/v2/pkg/object/meshcontroller/spec"
)

type (
	// ServiceRegistryInfo contains service's spec,
	// and its instance, which is the sidecar+egress port address
	ServiceRegistryInfo struct {
		Service *spec.Service
		Ins     *spec.ServiceInstanceSpec // indicates local egress
		Version int64                     // tenant Etcd key version,
	}
)

// UniqInstanceID creates a virtual uniq ID for every visible
// service in mesh
func UniqInstanceID(serviceName string) string {
	return fmt.Sprintf("ins-%s-01", serviceName)
}

// GetServiceName split instanceID by '-' then return second
// field as the service name
func GetServiceName(instanceID string) string {
	names := strings.Split(instanceID, "-")

	if len(names) != 3 {
		return ""
	}
	return names[2]
}

// defaultInstance creates default egress instance point to the sidecar's egress port
func (rcs *Server) defaultInstance(self, target *spec.Service) *spec.ServiceInstanceSpec {
	return &spec.ServiceInstanceSpec{
		ServiceName: target.Name,
		InstanceID:  UniqInstanceID(target.Name),
		IP:          self.Sidecar.Address,
		Port:        uint32(self.Sidecar.EgressPort),
	}
}

// DiscoveryService gets one service specs with default instance
func (rcs *Server) DiscoveryService(serviceName string) (*ServiceRegistryInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			msg := "registry center recover from: %v, stack trace:\n%s\n"
			logger.Errorf(msg, err, debug.Stack())
		}
	}()

	if !rcs.registered {
		return nil, spec.ErrNoRegisteredYet
	}

	target := rcs.service.GetServiceSpec(serviceName)
	if target == nil {
		return nil, spec.ErrServiceNotFound
	}

	self := rcs.service.GetServiceSpec(rcs.serviceName)
	if self == nil {
		logger.Errorf("service: %s get self spec not found", rcs.serviceName)
		return nil, spec.ErrNoRegisteredYet
	}

	tenant, info := rcs.service.GetTenantSpecWithInfo(self.RegisterTenant)
	if tenant == nil {
		msg := "BUG: can't find service: %s's registry tenant: %s"
		err := fmt.Errorf(msg, rcs.serviceName, self.RegisterTenant)
		logger.Errorf("%v", err)
		return nil, err
	}

	found := target.RegisterTenant == self.RegisterTenant

	if !found {
		if p := rcs.accessableServices.Load(); p != nil {
			found = p.(map[string]bool)[serviceName]
		}
	}

	if !found {
		tenant, info = rcs.service.GetTenantSpecWithInfo(spec.GlobalTenant)
		if tenant == nil {
			return nil, spec.ErrServiceNotFound
		}
		for _, v := range tenant.Services {
			if v == serviceName {
				found = true
				break
			}
		}
	}

	if !found {
		return nil, spec.ErrServiceNotFound
	}

	return &ServiceRegistryInfo{
		Service: target,
		Ins:     rcs.defaultInstance(self, target),
		Version: info.Version,
	}, nil
}

// Discovery gets all services' spec and default instance(local sidecar for ever)
// which are visible for local service
func (rcs *Server) Discovery() ([]*ServiceRegistryInfo, error) {
	defer func() {
		if err := recover(); err != nil {
			msg := "registry center recover from: %v, stack trace:\n%s\n"
			logger.Errorf(msg, err, debug.Stack())
		}
	}()

	var (
		serviceInfos    []*ServiceRegistryInfo
		visibleServices map[string]bool
		err             error
	)

	visibleServices = make(map[string]bool)
	if !rcs.registered {
		return serviceInfos, spec.ErrNoRegisteredYet
	}
	self := rcs.service.GetServiceSpec(rcs.serviceName)
	if self == nil {
		logger.Errorf("service: %s get self spec not found", rcs.serviceName)
		return serviceInfos, spec.ErrNoRegisteredYet
	}

	var version int64
	tenant, info := rcs.service.GetTenantSpecWithInfo(spec.GlobalTenant)
	if tenant != nil {
		version = info.Version
		for _, v := range tenant.Services {
			if v != rcs.serviceName {
				visibleServices[v] = true
			}
		}
	}

	tenant, info = rcs.service.GetTenantSpecWithInfo(self.RegisterTenant)
	if tenant == nil {
		msg := "BUG: can't find service: %s's registry tenant: %s"
		err = fmt.Errorf(msg, rcs.serviceName, self.RegisterTenant)
		logger.Errorf("%v", err)
		return serviceInfos, err
	}
	if info.Version > version {
		version = info.Version
	}

	for _, v := range tenant.Services {
		visibleServices[v] = true
	}

	if p := rcs.accessableServices.Load(); p != nil {
		svcs := p.(map[string]bool)
		for k := range svcs {
			visibleServices[k] = true
		}
	}

	for k := range visibleServices {
		var spec *spec.Service
		if k == rcs.serviceName {
			spec = self
		} else if service := rcs.service.GetServiceSpec(k); service == nil {
			logger.Errorf("service %s not found", k)
			continue
		} else {
			spec = service
		}

		serviceInfos = append(serviceInfos, &ServiceRegistryInfo{
			Service: spec,
			Ins:     rcs.defaultInstance(self, spec),
			Version: version,
		})
	}

	return serviceInfos, err
}
