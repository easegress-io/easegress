package registrycenter

import (
	"github.com/hashicorp/consul/api"
)

// ToConsulCatalogService transfors serivce registry info to consul's service
func (rcs *Server) ToConsulCatalogService(serviceInfo *ServiceRegistryInfo) *api.CatalogService {
	var svc api.CatalogService

	svc.Address = serviceInfo.Ins.IP
	svc.ServiceName = serviceInfo.Ins.ServiceName
	svc.ServicePort = int(serviceInfo.Ins.Port)
	svc.ID = serviceInfo.Ins.InstanceID
	svc.ServiceAddress = serviceInfo.Ins.IP

	return &svc

}

// ToConsulServices transfors registry center's serivce info to map[string][]string structure
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
