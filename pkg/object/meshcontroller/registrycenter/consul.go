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

//ToConsulHealthService transforms service registry info to consul's serviceEntry
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
	svcs = append(svcs, &svc)
	return svcs
}

// ToConsulServices transforms registry center's serivce info to map[string][]string structure
func (rcs *Server) ToConsulServices(serviceInfos []*ServiceRegistryInfo) map[string][]string {
	var (
		svcs     map[string][]string = make(map[string][]string)
		emptyTag []string
	)

	for _, v := range serviceInfos {
		svcs[v.Ins.ServiceName] = emptyTag
	}
	return svcs
}
