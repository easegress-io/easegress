package registrycenter

import (
	"github.com/nacos-group/nacos-sdk-go/model"
)

// ToNacosInstanceInfo transforms service registry info to nacos's instance
func (rcs *Server) ToNacosInstanceInfo(serviceInfo *ServiceRegistryInfo) *model.Instance {
	var ins model.Instance

	ins.Ip = serviceInfo.Ins.IP
	ins.Valid = true
	ins.InstanceId = serviceInfo.Ins.InstanceID
	ins.ServiceName = serviceInfo.Ins.ServiceName
	ins.Port = uint64(serviceInfo.Ins.Port)
	ins.Healthy = true

	return &ins
}

// ToNacosService transforms servie registry info to nacos's service
func (rcs *Server) ToNacosService(serviceInfo *ServiceRegistryInfo) *model.Service {
	var svc model.Service
	svc.Hosts = append(svc.Hosts, *rcs.ToNacosInstanceInfo(serviceInfo))
	svc.Dom = serviceInfo.Ins.ServiceName
	svc.CacheMillis = 500
	svc.Name = serviceInfo.Ins.ServiceName
	return &svc
}

// ToNacosServiceList transforms registry center's service info to eureka's apps
func (rcs *Server) ToNacosServiceList(serviceInfos []*ServiceRegistryInfo) *model.ServiceList {
	var list model.ServiceList
	for _, v := range serviceInfos {
		list.Doms = append(list.Doms, v.Ins.ServiceName)
	}

	list.Count = int64(len(serviceInfos))
	return &list
}

// ToNacosServiceDetail transforms servie registry info to nacos's service
func (rcs *Server) ToNacosServiceDetail(serviceInfo *ServiceRegistryInfo) *model.ServiceDetail {
	var svc model.ServiceDetail
	svc.Service.Name = serviceInfo.Ins.ServiceName
	svc.Service.Group = "DEFAULT_GROUP"
	return &svc
}
