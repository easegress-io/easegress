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
	"github.com/hashicorp/consul/api"
)

// ToConsulCatalogService transforms service registry info to consul's service
func (rcs *Server) ToConsulCatalogService(serviceInfo *ServiceRegistryInfo) []*api.CatalogService {
	var (
		svcs []*api.CatalogService
		svc  api.CatalogService
	)

	svc.Address = serviceInfo.Ins.IP
	svc.ServiceName = serviceInfo.Ins.ServiceName
	svc.ServicePort = int(serviceInfo.Ins.Port)
	svc.ID = serviceInfo.Ins.InstanceID
	svc.ServiceAddress = serviceInfo.Ins.IP

	svcs = append(svcs, &svc)
	return svcs
}

// ToConsulHealthService transforms service registry info to consul's serviceEntry
func (rcs *Server) ToConsulHealthService(serviceInfo *ServiceRegistryInfo) []*api.ServiceEntry {
	var (
		svc  api.ServiceEntry
		svcs []*api.ServiceEntry
	)

	svc.Service = &api.AgentService{
		ID:      serviceInfo.Ins.InstanceID,
		Port:    int(serviceInfo.Ins.Port),
		Address: serviceInfo.Ins.IP,
		Service: serviceInfo.Ins.ServiceName,
	}
	svc.Checks = make(api.HealthChecks, 0)
	svcs = append(svcs, &svc)
	return svcs
}

// ToConsulServices transforms registry center's service info to map[string][]string structure
func (rcs *Server) ToConsulServices(serviceInfos []*ServiceRegistryInfo) map[string][]string {
	var (
		svcs     = make(map[string][]string)
		emptyTag []string
	)

	for _, v := range serviceInfos {
		svcs[v.Ins.ServiceName] = emptyTag
	}
	return svcs
}
